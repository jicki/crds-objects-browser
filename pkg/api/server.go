package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/jicki/crds-objects-browser/pkg/informer"
)

// Server 表示API服务器
type Server struct {
	clientset       kubernetes.Interface
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	strategyManager *informer.StrategyManager
	router          *gin.Engine
	httpServer      *http.Server
	port            string
	isReady         atomic.Bool

	// 性能监控相关
	startTime       time.Time
	preloadComplete atomic.Bool
}

// NewServer 创建新的API服务器
func NewServer(config *rest.Config) (*Server, error) {
	// 优化客户端配置
	config.QPS = 100   // 增加QPS限制
	config.Burst = 200 // 增加突发限制
	config.Timeout = 30 * time.Second

	// 创建客户端
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %v", err)
	}

	// 创建Informer管理器
	informerManager := informer.NewInformerManager(dynamicClient)
	strategy := informer.DefaultStrategy()
	strategyManager := informer.NewStrategyManager(informerManager, strategy)

	server := &Server{
		clientset:       clientset,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		strategyManager: strategyManager,
		port:            "8080",
		startTime:       time.Now(),
	}

	// 初始化路由
	server.setupRoutes()

	// 创建HTTP服务器，使用正确的router
	server.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%s", server.port),
		Handler: server.router,
	}

	// 异步预加载资源
	go server.initializeCache()

	return server, nil
}

// Router 返回 gin 路由器
func (s *Server) Router() *gin.Engine {
	return s.router
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	s.router = gin.Default()

	// 启用CORS
	s.router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 添加性能监控中间件
	s.router.Use(s.performanceMiddleware())

	// API路由
	api := s.router.Group("/api")
	{
		api.GET("/crds", s.getCRDs)
		api.GET("/crds/:group/:version/:resource/objects", s.getResourceObjects)
		api.GET("/crds/:group/:version/:resource/objects/fast", s.getResourceObjectsFast) // 新增快速接口
		api.GET("/crds/:group/:version/:resource/namespaces", s.getResourceNamespaces)
		api.GET("/namespaces", s.getNamespaces)
		api.GET("/cache/stats", s.getCacheStats)
		api.GET("/cache/status", s.getCacheStatus)           // 新增缓存状态接口
		api.GET("/performance/stats", s.getPerformanceStats) // 新增性能统计接口
	}

	// 健康检查端点
	s.router.GET("/healthz", s.healthCheck)
	s.router.GET("/readyz", s.readinessCheck)
	s.router.GET("/livez", s.livenessCheck)

	// 静态文件服务
	s.router.Static("/ui", "./ui/dist")
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/ui/")
	})

	// 处理 favicon.ico
	s.router.GET("/favicon.ico", func(c *gin.Context) {
		c.File("./ui/dist/favicon.ico")
	})

	// 处理前端路由
	s.router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// 如果是以 /ui 开头的路径
		if strings.HasPrefix(path, "/ui/") {
			// 先尝试作为静态资源文件提供服务
			filePath := "./ui/dist" + strings.TrimPrefix(path, "/ui")
			if _, err := os.Stat(filePath); err == nil {
				// 设置缓存控制头
				c.Header("Cache-Control", "public, max-age=31536000")
				c.File(filePath)
				return
			}

			// 如果不是静态资源，返回 index.html（用于支持前端路由）
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
			c.File("./ui/dist/index.html")
			return
		}

		// 其他路径返回 404
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Page not found",
			"path":  path,
		})
	})
}

// performanceMiddleware 性能监控中间件
func (s *Server) performanceMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// 记录慢请求
		if param.Latency > 1*time.Second {
			klog.Warningf("Slow request: %s %s took %v", param.Method, param.Path, param.Latency)
		}

		return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %#v\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.Method,
			param.Path,
		)
	})
}

// initializeCache 初始化缓存（优化版本）
func (s *Server) initializeCache() {
	klog.Info("Starting optimized cache initialization...")
	startTime := time.Now()

	// 获取所有资源
	resources, err := s.getAllResources()
	if err != nil {
		klog.Errorf("Failed to get resources for cache initialization: %v", err)
		return
	}

	klog.Infof("Found %d resources, starting preload...", len(resources))

	// 转换为ResourceInfo格式
	var resourceInfos []informer.ResourceInfo
	for _, res := range resources {
		resourceInfos = append(resourceInfos, informer.ResourceInfo{
			Group:      res.Group,
			Version:    res.Version,
			Name:       res.Name,
			Kind:       res.Kind,
			Namespaced: res.Namespaced,
		})
	}

	// 并行预加载资源
	if err := s.strategyManager.PreloadResources(resourceInfos); err != nil {
		klog.Errorf("Failed to start resource preloading: %v", err)
		return
	}

	// 等待预加载完成（带超时）
	preloadTimeout := 60 * time.Second
	if err := s.strategyManager.WaitForPreloadComplete(preloadTimeout); err != nil {
		klog.Warningf("Preload timeout: %v, continuing with partial cache", err)
	}

	// 等待核心资源同步完成
	coreResourcesReady := 0
	maxWait := 30 * time.Second
	checkInterval := 500 * time.Millisecond
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		stats := s.strategyManager.GetCacheStats()
		coreResourcesReady = 0

		// 检查核心资源是否就绪
		coreResources := []string{
			"/v1/pods", "/v1/services", "/v1/namespaces",
			"apps/v1/deployments", "apps/v1/daemonsets", "apps/v1/statefulsets",
		}

		for _, resource := range coreResources {
			if ready, exists := stats.SyncStatus[resource]; exists && ready {
				coreResourcesReady++
			}
		}

		if coreResourcesReady >= len(coreResources)/2 { // 至少一半核心资源就绪
			break
		}

		time.Sleep(checkInterval)
	}

	initDuration := time.Since(startTime)
	klog.Infof("Cache initialization completed in %v, %d/%d core resources ready",
		initDuration, coreResourcesReady, len([]string{
			"/v1/pods", "/v1/services", "/v1/namespaces",
			"apps/v1/deployments", "apps/v1/daemonsets", "apps/v1/statefulsets",
		}))

	// 标记预加载完成和服务就绪
	s.preloadComplete.Store(true)
	s.SetReady(true)

	// 启动后台监控
	go s.startBackgroundMonitoring()
}

// startBackgroundMonitoring 启动后台监控
func (s *Server) startBackgroundMonitoring() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats := s.strategyManager.GetCacheStats()
			readyCount := s.strategyManager.GetReadyResourcesCount()

			klog.V(2).Infof("Cache stats: %d active informers, %d ready resources, %d total objects",
				stats.ActiveInformers, readyCount, stats.TotalObjects)
		}
	}
}

// SetReady 设置服务就绪状态
func (s *Server) SetReady(ready bool) {
	s.isReady.Store(ready)
	if ready {
		klog.Info("Service is now ready")
	} else {
		klog.Info("Service is not ready")
	}
}

// getCRDs 获取所有CRD资源
func (s *Server) getCRDs(c *gin.Context) {
	resources, err := s.getAllResources()
	if err != nil {
		klog.Errorf("Failed to get CRDs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	klog.Infof("Found %d resources", len(resources))
	c.JSON(http.StatusOK, resources)
}

// getResourceObjects 获取资源对象（使用Informer缓存）
func (s *Server) getResourceObjects(c *gin.Context) {
	group := c.Param("group")
	version := c.Param("version")
	resource := c.Param("resource")
	namespace := c.Query("namespace")

	// 处理core组
	if group == "core" {
		group = ""
	}

	klog.V(4).Infof("Getting objects for resource: %s/%s/%s, namespace: %s", group, version, resource, namespace)

	// 构建GVR
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// 检查资源是否为命名空间资源
	namespaced, err := s.isNamespacedResource(gvr)
	if err != nil {
		klog.Errorf("Failed to check if resource is namespaced: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 使用策略管理器获取对象
	objects, err := s.strategyManager.GetObjects(gvr, namespace, namespaced)
	if err != nil {
		klog.Errorf("Failed to get objects from cache: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换为JSON格式
	var result []map[string]interface{}
	for _, obj := range objects {
		result = append(result, obj.Object)
	}

	klog.V(4).Infof("Retrieved %d objects for %s from cache", len(result), gvr.String())
	c.JSON(http.StatusOK, result)
}

// getResourceObjectsFast 快速获取资源对象（带降级策略）
func (s *Server) getResourceObjectsFast(c *gin.Context) {
	group := c.Param("group")
	version := c.Param("version")
	resource := c.Param("resource")
	namespace := c.Query("namespace")

	// 处理core组
	if group == "core" {
		group = ""
	}

	// 构建GVR
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// 检查资源是否为命名空间资源
	namespaced, err := s.isNamespacedResource(gvr)
	if err != nil {
		klog.Errorf("Failed to check if resource is namespaced: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 使用降级策略获取对象
	objects, err := s.strategyManager.GetObjectsWithFallback(gvr, namespace, namespaced)
	if err != nil {
		klog.Errorf("Failed to get objects with fallback: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换为JSON格式
	var result []map[string]interface{}
	for _, obj := range objects {
		result = append(result, obj.Object)
	}

	// 添加加载状态信息
	response := gin.H{
		"objects": result,
		"loading": !s.strategyManager.GetCacheStats().SyncStatus[gvr.String()],
		"count":   len(result),
	}

	c.JSON(http.StatusOK, response)
}

// getResourceNamespaces 获取资源的命名空间（使用Informer缓存）
func (s *Server) getResourceNamespaces(c *gin.Context) {
	group := c.Param("group")
	version := c.Param("version")
	resource := c.Param("resource")

	// 处理core组
	if group == "core" {
		group = ""
	}

	klog.V(4).Infof("Getting namespaces for resource: %s/%s/%s", group, version, resource)

	// 构建GVR
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// 检查资源是否为命名空间资源
	namespaced, err := s.isNamespacedResource(gvr)
	if err != nil {
		klog.Errorf("Failed to check if resource is namespaced: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !namespaced {
		// 非命名空间资源返回空数组
		c.JSON(http.StatusOK, []string{})
		return
	}

	// 使用策略管理器获取命名空间
	namespaces, err := s.strategyManager.GetNamespaces(gvr, namespaced)
	if err != nil {
		klog.Errorf("Failed to get namespaces from cache: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	klog.V(4).Infof("Retrieved %d namespaces for %s from cache", len(namespaces), gvr.String())
	c.JSON(http.StatusOK, namespaces)
}

// getCacheStats 获取缓存统计信息
func (s *Server) getCacheStats(c *gin.Context) {
	stats := s.strategyManager.GetCacheStats()
	c.JSON(http.StatusOK, stats)
}

// getCacheStatus 获取缓存状态
func (s *Server) getCacheStatus(c *gin.Context) {
	stats := s.strategyManager.GetCacheStats()
	readyCount := s.strategyManager.GetReadyResourcesCount()

	status := gin.H{
		"preloadComplete": s.preloadComplete.Load(),
		"readyResources":  readyCount,
		"totalInformers":  stats.ActiveInformers,
		"totalObjects":    stats.TotalObjects,
		"uptime":          time.Since(s.startTime).String(),
	}

	c.JSON(http.StatusOK, status)
}

// getPerformanceStats 获取性能统计
func (s *Server) getPerformanceStats(c *gin.Context) {
	stats := s.strategyManager.GetCacheStats()

	// 计算平均同步时间
	var totalSyncTime time.Duration
	var syncCount int
	for _, stat := range stats.ResourceStats {
		if stat.SyncDuration > 0 {
			totalSyncTime += stat.SyncDuration
			syncCount++
		}
	}

	var avgSyncTime time.Duration
	if syncCount > 0 {
		avgSyncTime = totalSyncTime / time.Duration(syncCount)
	}

	performance := gin.H{
		"uptime":          time.Since(s.startTime).String(),
		"averageSyncTime": avgSyncTime.String(),
		"totalSyncCount":  syncCount,
		"cacheHitRate":    "N/A", // 可以后续添加缓存命中率统计
		"memoryUsage":     "N/A", // 可以后续添加内存使用统计
	}

	c.JSON(http.StatusOK, performance)
}

// getNamespaces 获取所有命名空间
func (s *Server) getNamespaces(c *gin.Context) {
	namespaces, err := s.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Failed to get namespaces: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []string
	for _, ns := range namespaces.Items {
		result = append(result, ns.Name)
	}

	sort.Strings(result)
	c.JSON(http.StatusOK, result)
}

// getAllResources 获取所有资源（保持原有逻辑）
func (s *Server) getAllResources() ([]Resource, error) {
	// ... 保持原有的getAllResources实现
	// 这里只是为了简化，实际应该保持原有的完整实现

	// 获取API资源列表
	_, apiResourceLists, err := s.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get server groups and resources: %v", err)
	}

	var resources []Resource

	for _, apiResourceList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			klog.Warningf("Failed to parse group version %s: %v", apiResourceList.GroupVersion, err)
			continue
		}

		for _, apiResource := range apiResourceList.APIResources {
			// 跳过子资源
			if strings.Contains(apiResource.Name, "/") {
				continue
			}

			resource := Resource{
				Group:      gv.Group,
				Version:    gv.Version,
				Name:       apiResource.Name,
				Kind:       apiResource.Kind,
				Namespaced: apiResource.Namespaced,
			}

			resources = append(resources, resource)
		}
	}

	// 按组和名称排序
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Group != resources[j].Group {
			return resources[i].Group < resources[j].Group
		}
		return resources[i].Name < resources[j].Name
	})

	return resources, nil
}

// isNamespacedResource 检查资源是否为命名空间资源
func (s *Server) isNamespacedResource(gvr schema.GroupVersionResource) (bool, error) {
	// 从discovery客户端获取资源信息
	_, apiResourceLists, err := s.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return false, err
	}

	for _, apiResourceList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			continue
		}

		if gv.Group == gvr.Group && gv.Version == gvr.Version {
			for _, apiResource := range apiResourceList.APIResources {
				if apiResource.Name == gvr.Resource {
					return apiResource.Namespaced, nil
				}
			}
		}
	}

	return false, fmt.Errorf("resource %s not found", gvr.String())
}

// Resource 资源结构
type Resource struct {
	Group      string `json:"group"`
	Version    string `json:"version"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Namespaced bool   `json:"namespaced"`
}

// Run 启动服务器
func (s *Server) Run(addr string) error {
	klog.Infof("Starting server on %s", addr)
	s.httpServer.Addr = addr
	s.httpServer.Handler = s.router
	return s.httpServer.ListenAndServe()
}

// Start 启动服务器
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown 关闭服务器
func (s *Server) Shutdown() {
	klog.Info("Shutting down server")
	s.strategyManager.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		klog.Errorf("Server shutdown error: %v", err)
	}
}

// healthCheck 健康检查端点
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "crds-objects-browser",
	})
}

// readinessCheck 就绪检查端点
func (s *Server) readinessCheck(c *gin.Context) {
	// 检查服务是否就绪
	if !s.isReady.Load() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "not ready",
			"message": "Service is not ready yet",
		})
		return
	}

	// 检查策略管理器是否正常
	if s.strategyManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "not ready",
			"message": "Strategy manager not initialized",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ready",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "crds-objects-browser",
	})
}

// livenessCheck 存活检查端点
func (s *Server) livenessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "alive",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "crds-objects-browser",
	})
}
