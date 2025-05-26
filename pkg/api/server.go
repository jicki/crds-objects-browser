package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
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

	// 缓存相关
	resourcesCache      []Resource
	resourcesCacheTime  time.Time
	resourcesCacheMutex sync.RWMutex
	resourcesCacheTTL   time.Duration

	// 请求去重
	requestDeduplicator map[string]*sync.Mutex
	deduplicatorMutex   sync.RWMutex
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
		clientset:           clientset,
		dynamicClient:       dynamicClient,
		discoveryClient:     discoveryClient,
		strategyManager:     strategyManager,
		port:                "8080",
		startTime:           time.Now(),
		resourcesCacheTTL:   5 * time.Minute, // 资源列表缓存5分钟
		requestDeduplicator: make(map[string]*sync.Mutex),
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

	// 测试路由
	s.router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test route works"})
	})

	// 前端调试页面
	s.router.GET("/debug-frontend", func(c *gin.Context) {
		c.File("./test/html/debug-frontend.html")
	})

	// 前端修复测试页面
	s.router.GET("/test-fix", func(c *gin.Context) {
		c.File("./test/html/test-frontend-fix.html")
	})

	// 调试页面（放在静态文件服务之前）
	s.router.GET("/debug", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CRDs Browser 调试页面</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background-color: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .section { margin-bottom: 30px; padding: 15px; border: 1px solid #ddd; border-radius: 5px; }
        .status { padding: 10px; border-radius: 4px; margin: 10px 0; }
        .status.success { background-color: #d4edda; color: #155724; border: 1px solid #c3e6cb; }
        .status.error { background-color: #f8d7da; color: #721c24; border: 1px solid #f5c6cb; }
        .status.warning { background-color: #fff3cd; color: #856404; border: 1px solid #ffeaa7; }
        button { background-color: #007bff; color: white; border: none; padding: 10px 20px; border-radius: 4px; cursor: pointer; margin: 5px; }
        button:hover { background-color: #0056b3; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔍 CRDs Objects Browser 调试页面</h1>
        
        <div class="section">
            <h3>📊 系统状态检查</h3>
            <button onclick="checkHealth()">检查健康状态</button>
            <button onclick="checkAPI()">检查API响应</button>
            <button onclick="openUIPage()">打开UI页面</button>
            <div id="healthStatus"></div>
        </div>

        <div class="section">
            <h3>📦 资源数据测试</h3>
            <button onclick="fetchResources()">获取资源列表</button>
            <button onclick="fetchNamespaces()">获取命名空间</button>
            <button onclick="testFrontendDataFlow()">测试前端数据流</button>
            <div id="resourcesStatus"></div>
        </div>
    </div>

    <script>
        async function checkHealth() {
            const statusDiv = document.getElementById('healthStatus');
            statusDiv.innerHTML = '<div class="status warning">正在检查健康状态...</div>';
            
            try {
                const response = await fetch('/healthz');
                const data = await response.json();
                
                if (response.ok) {
                    statusDiv.innerHTML = '<div class="status success">✅ 服务健康状态正常<br>服务: ' + data.service + '<br>状态: ' + data.status + '</div>';
                } else {
                    statusDiv.innerHTML = '<div class="status error">❌ 健康检查失败: ' + response.status + '</div>';
                }
            } catch (error) {
                statusDiv.innerHTML = '<div class="status error">❌ 健康检查错误: ' + error.message + '</div>';
            }
        }

        async function checkAPI() {
            const statusDiv = document.getElementById('healthStatus');
            statusDiv.innerHTML += '<div class="status warning">正在检查API响应...</div>';
            
            try {
                const response = await fetch('/api/crds');
                const data = await response.json();
                
                if (response.ok && Array.isArray(data)) {
                    statusDiv.innerHTML += '<div class="status success">✅ API响应正常<br>资源数量: ' + data.length + '</div>';
                } else {
                    statusDiv.innerHTML += '<div class="status error">❌ API响应异常: ' + response.status + '</div>';
                }
            } catch (error) {
                statusDiv.innerHTML += '<div class="status error">❌ API请求错误: ' + error.message + '</div>';
            }
        }

        async function fetchResources() {
            const statusDiv = document.getElementById('resourcesStatus');
            statusDiv.innerHTML = '<div class="status warning">正在获取资源列表...</div>';
            
            try {
                const response = await fetch('/api/crds');
                const data = await response.json();
                
                if (response.ok && Array.isArray(data)) {
                    statusDiv.innerHTML = '<div class="status success">✅ 资源列表获取成功<br>总数量: ' + data.length + '</div>';
                } else {
                    statusDiv.innerHTML = '<div class="status error">❌ 资源列表获取失败: ' + response.status + '</div>';
                }
            } catch (error) {
                statusDiv.innerHTML = '<div class="status error">❌ 资源列表获取错误: ' + error.message + '</div>';
            }
        }

        async function fetchNamespaces() {
            const statusDiv = document.getElementById('resourcesStatus');
            statusDiv.innerHTML += '<div class="status warning">正在获取命名空间...</div>';
            
            try {
                const response = await fetch('/api/namespaces');
                const data = await response.json();
                
                if (response.ok && Array.isArray(data)) {
                    statusDiv.innerHTML += '<div class="status success">✅ 命名空间获取成功<br>数量: ' + data.length + '</div>';
                } else {
                    statusDiv.innerHTML += '<div class="status error">❌ 命名空间获取失败: ' + response.status + '</div>';
                }
            } catch (error) {
                statusDiv.innerHTML += '<div class="status error">❌ 命名空间获取错误: ' + error.message + '</div>';
            }
        }

        function openUIPage() {
            window.open('/ui/', '_blank');
        }

        // 前端数据流测试
        async function testFrontendDataFlow() {
            const statusDiv = document.getElementById('resourcesStatus');
            statusDiv.innerHTML = '<div class="status warning">正在测试前端数据流...</div>';
            
            try {
                // 测试API
                const response = await fetch('/api/crds');
                const data = await response.json();
                
                if (response.ok && Array.isArray(data)) {
                    statusDiv.innerHTML += '<div class="status success">✅ API数据正常: ' + data.length + ' 个资源</div>';
                    
                    // 测试前端页面
                    const uiResponse = await fetch('/ui/');
                    if (uiResponse.ok) {
                        statusDiv.innerHTML += '<div class="status success">✅ 前端页面可访问</div>';
                        
                        // 检查前端JavaScript
                        statusDiv.innerHTML += '<div class="status warning">🔍 请打开浏览器控制台查看前端数据流</div>';
                        statusDiv.innerHTML += '<div class="status warning">📊 在主页面中，原始资源数应该是 ' + data.length + '</div>';
                        statusDiv.innerHTML += '<div class="status warning">📊 如果排序资源数为0，说明前端数据处理有问题</div>';
                        
                        // 提供调试建议
                        statusDiv.innerHTML += '<div class="status warning">' +
                            '<strong>调试建议:</strong><br>' +
                            '1. 打开 <a href="/ui/" target="_blank">主页面</a><br>' +
                            '2. 打开浏览器开发者工具 (F12)<br>' +
                            '3. 查看控制台中的数据流日志<br>' +
                            '4. 检查 sortedResources getter 是否被正确调用<br>' +
                            '5. 检查 store.state.resources 是否有数据' +
                            '</div>';
                    } else {
                        statusDiv.innerHTML += '<div class="status error">❌ 前端页面无法访问</div>';
                    }
                } else {
                    statusDiv.innerHTML += '<div class="status error">❌ API数据异常</div>';
                }
            } catch (error) {
                statusDiv.innerHTML += '<div class="status error">❌ 测试失败: ' + error.message + '</div>';
            }
        }

        window.onload = function() {
            console.log('CRDs Browser 调试页面已加载');
            checkHealth();
        };
    </script>
</body>
</html>`)
	})

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
		// 调整慢请求阈值，避免过多警告
		slowThreshold := 3 * time.Second
		if param.Latency > slowThreshold {
			klog.Warningf("Slow request: %s %s took %v", param.Method, param.Path, param.Latency)
		} else if param.Latency > 1*time.Second {
			// 1-3秒的请求记录为info级别
			klog.V(2).Infof("Moderate request: %s %s took %v", param.Method, param.Path, param.Latency)
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

	// 启动定期清理
	go s.startPeriodicCleanup()
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

// startPeriodicCleanup 启动定期清理
func (s *Server) startPeriodicCleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupRequestDeduplicator()
		}
	}
}

// cleanupRequestDeduplicator 清理请求去重器
func (s *Server) cleanupRequestDeduplicator() {
	s.deduplicatorMutex.Lock()
	defer s.deduplicatorMutex.Unlock()

	// 清理所有互斥锁，因为它们只是用于短期去重
	// 长期缓存由其他机制处理
	if len(s.requestDeduplicator) > 100 {
		klog.V(4).Infof("Cleaning up request deduplicator, current size: %d", len(s.requestDeduplicator))
		s.requestDeduplicator = make(map[string]*sync.Mutex)
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

// getCRDs 获取所有CRD资源（带缓存）
func (s *Server) getCRDs(c *gin.Context) {
	// 检查缓存
	s.resourcesCacheMutex.RLock()
	if time.Since(s.resourcesCacheTime) < s.resourcesCacheTTL && len(s.resourcesCache) > 0 {
		resources := s.resourcesCache
		s.resourcesCacheMutex.RUnlock()
		klog.V(4).Infof("Returning cached resources: %d", len(resources))
		c.JSON(http.StatusOK, resources)
		return
	}
	s.resourcesCacheMutex.RUnlock()

	// 请求去重
	requestKey := "getAllResources"
	mutex := s.getOrCreateRequestMutex(requestKey)
	mutex.Lock()
	defer mutex.Unlock()

	// 再次检查缓存（可能在等待锁的过程中已被更新）
	s.resourcesCacheMutex.RLock()
	if time.Since(s.resourcesCacheTime) < s.resourcesCacheTTL && len(s.resourcesCache) > 0 {
		resources := s.resourcesCache
		s.resourcesCacheMutex.RUnlock()
		klog.V(4).Infof("Returning cached resources after lock: %d", len(resources))
		c.JSON(http.StatusOK, resources)
		return
	}
	s.resourcesCacheMutex.RUnlock()

	// 获取资源
	resources, err := s.getAllResources()
	if err != nil {
		klog.Errorf("Failed to get CRDs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新缓存
	s.resourcesCacheMutex.Lock()
	s.resourcesCache = resources
	s.resourcesCacheTime = time.Now()
	s.resourcesCacheMutex.Unlock()

	klog.V(2).Infof("Found %d resources, cached for %v", len(resources), s.resourcesCacheTTL)
	c.JSON(http.StatusOK, resources)
}

// getOrCreateRequestMutex 获取或创建请求互斥锁
func (s *Server) getOrCreateRequestMutex(key string) *sync.Mutex {
	s.deduplicatorMutex.RLock()
	if mutex, exists := s.requestDeduplicator[key]; exists {
		s.deduplicatorMutex.RUnlock()
		return mutex
	}
	s.deduplicatorMutex.RUnlock()

	s.deduplicatorMutex.Lock()
	defer s.deduplicatorMutex.Unlock()

	// 双重检查
	if mutex, exists := s.requestDeduplicator[key]; exists {
		return mutex
	}

	mutex := &sync.Mutex{}
	s.requestDeduplicator[key] = mutex
	return mutex
}

// getResourceObjects 获取资源对象（使用Informer缓存，优化版本）
func (s *Server) getResourceObjects(c *gin.Context) {
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

	// 请求去重
	requestKey := fmt.Sprintf("objects_%s_%s_%s_%s", group, version, resource, namespace)
	mutex := s.getOrCreateRequestMutex(requestKey)
	mutex.Lock()
	defer mutex.Unlock()

	klog.V(4).Infof("Getting objects for resource: %s/%s/%s, namespace: %s", group, version, resource, namespace)

	// 检查资源是否为命名空间资源（带缓存）
	namespaced, err := s.isNamespacedResourceCached(gvr)
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

	// 优化：预分配切片容量
	result := make([]map[string]interface{}, 0, len(objects))
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

// getResourceNamespaces 获取资源的命名空间（使用Informer缓存，优化版本）
func (s *Server) getResourceNamespaces(c *gin.Context) {
	group := c.Param("group")
	version := c.Param("version")
	resource := c.Param("resource")

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

	// 请求去重
	requestKey := fmt.Sprintf("namespaces_%s_%s_%s", group, version, resource)
	mutex := s.getOrCreateRequestMutex(requestKey)
	mutex.Lock()
	defer mutex.Unlock()

	klog.V(4).Infof("Getting namespaces for resource: %s/%s/%s", group, version, resource)

	// 检查资源是否为命名空间资源（带缓存）
	namespaced, err := s.isNamespacedResourceCached(gvr)
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
	// 获取API资源列表
	_, apiResourceLists, err := s.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		// 处理部分错误，继续获取可用资源
		if discovery.IsGroupDiscoveryFailedError(err) {
			klog.Warningf("Some groups were not discoverable: %v", err)
		} else {
			return nil, fmt.Errorf("failed to get server groups and resources: %v", err)
		}
	}

	var resources []Resource

	for _, apiResourceList := range apiResourceLists {
		if apiResourceList == nil {
			continue
		}

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

			// 跳过不支持list和get操作的资源
			if !hasVerb(apiResource, "list") || !hasVerb(apiResource, "get") {
				continue
			}

			// 跳过已弃用的资源版本
			if isDeprecatedResource(apiResource.Name, gv.Group, gv.Version) {
				klog.V(4).Infof("Skipping deprecated resource: %s/%s %s", gv.Group, gv.Version, apiResource.Name)
				continue
			}

			// 跳过特殊资源
			if isSpecialResource(apiResource.Name, gv.Group) {
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

	klog.V(4).Infof("Discovered %d API resources", len(resources))
	return resources, nil
}

// hasVerb 检查资源是否支持特定操作
func hasVerb(r metav1.APIResource, verb string) bool {
	for _, v := range r.Verbs {
		if v == verb {
			return true
		}
	}
	return false
}

// isDeprecatedResource 检查是否为已弃用的资源版本
func isDeprecatedResource(name, group, version string) bool {
	// 已弃用的资源版本映射
	deprecatedResources := map[string]map[string][]string{
		"batch": {
			"v1beta1": {"cronjobs"}, // batch/v1beta1 CronJob 在 v1.21+ 中已弃用，在 v1.25+ 中不可用
		},
		"extensions": {
			"v1beta1": {"deployments", "replicasets", "daemonsets", "ingresses", "podsecuritypolicies"},
		},
		"apps": {
			"v1beta1": {"deployments", "replicasets", "daemonsets", "statefulsets"},
			"v1beta2": {"deployments", "replicasets", "daemonsets", "statefulsets"},
		},
		"networking.k8s.io": {
			"v1beta1": {"ingresses"}, // 使用 v1 版本
		},
		"policy": {
			"v1beta1": {"podsecuritypolicies"}, // PodSecurityPolicy 已弃用
		},
		"apiregistration.k8s.io": {
			"v1beta1": {"apiservices"}, // 使用 v1 版本
		},
		"admissionregistration.k8s.io": {
			"v1beta1": {"mutatingwebhookconfigurations", "validatingwebhookconfigurations"}, // 使用 v1 版本
		},
		"scheduling.k8s.io": {
			"v1beta1": {"priorityclasses"}, // 使用 v1 版本
		},
		"storage.k8s.io": {
			"v1beta1": {"storageclasses", "volumeattachments"}, // 使用 v1 版本
		},
		"rbac.authorization.k8s.io": {
			"v1beta1": {"roles", "rolebindings", "clusterroles", "clusterrolebindings"}, // 使用 v1 版本
		},
	}

	if groupMap, ok := deprecatedResources[group]; ok {
		if versionList, ok := groupMap[version]; ok {
			for _, r := range versionList {
				if r == name {
					return true
				}
			}
		}
	}
	return false
}

// isSpecialResource 检查是否为特殊资源（需要排除的资源）
func isSpecialResource(name, group string) bool {
	specialResources := map[string][]string{
		"": {
			"componentstatuses", // 已弃用的ComponentStatus
			"bindings",          // 特殊绑定资源
		},
		"authorization.k8s.io": {
			"selfsubjectrulesreviews",
			"subjectaccessreviews",
			"localsubjectaccessreviews",
			"selfsubjectaccessreviews",
		},
		"authentication.k8s.io": {
			"tokenreviews",
		},
		"metrics.k8s.io": {
			"pods",
			"nodes",
		},
		"events.k8s.io": {
			"events", // 避免重复，使用 core/v1 events
		},
	}

	if resources, ok := specialResources[group]; ok {
		for _, r := range resources {
			if r == name {
				return true
			}
		}
	}
	return false
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

// isNamespacedResourceCached 检查资源是否为命名空间资源（带缓存）
func (s *Server) isNamespacedResourceCached(gvr schema.GroupVersionResource) (bool, error) {
	// 使用缓存的资源列表进行查找
	s.resourcesCacheMutex.RLock()
	if time.Since(s.resourcesCacheTime) < s.resourcesCacheTTL && len(s.resourcesCache) > 0 {
		for _, resource := range s.resourcesCache {
			if resource.Group == gvr.Group && resource.Version == gvr.Version && resource.Name == gvr.Resource {
				s.resourcesCacheMutex.RUnlock()
				return resource.Namespaced, nil
			}
		}
	}
	s.resourcesCacheMutex.RUnlock()

	// 如果缓存中没有找到，回退到原始方法
	return s.isNamespacedResource(gvr)
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
