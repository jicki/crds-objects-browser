package informer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// InformerStrategy Informer策略
type InformerStrategy struct {
	// 预加载策略
	PreloadResources []schema.GroupVersionResource
	// 懒加载策略
	LazyLoadEnabled bool
	// 自动清理策略
	AutoCleanupEnabled bool
	// 清理间隔
	CleanupInterval time.Duration
	// 资源访问超时时间
	AccessTimeout time.Duration
	// 最大并发Informer数量
	MaxConcurrentInformers int
	// 并行预加载数量
	ParallelPreloadCount int
	// 缓存同步超时
	CacheSyncTimeout time.Duration
}

// DefaultStrategy 默认策略
func DefaultStrategy() *InformerStrategy {
	return &InformerStrategy{
		PreloadResources: []schema.GroupVersionResource{
			// Kubernetes核心资源预加载
			{Group: "", Version: "v1", Resource: "pods"},
			{Group: "", Version: "v1", Resource: "services"},
			{Group: "", Version: "v1", Resource: "configmaps"},
			{Group: "", Version: "v1", Resource: "secrets"},
			{Group: "", Version: "v1", Resource: "namespaces"},
			{Group: "apps", Version: "v1", Resource: "deployments"},
			{Group: "apps", Version: "v1", Resource: "daemonsets"},
			{Group: "apps", Version: "v1", Resource: "statefulsets"},
		},
		LazyLoadEnabled:        true,
		AutoCleanupEnabled:     true,
		CleanupInterval:        5 * time.Minute, // 减少清理间隔
		AccessTimeout:          3 * time.Minute, // 减少访问超时
		MaxConcurrentInformers: 50,
		ParallelPreloadCount:   5,                // 并行预加载数量
		CacheSyncTimeout:       20 * time.Second, // 缓存同步超时
	}
}

// StrategyManager 策略管理器
type StrategyManager struct {
	informerManager *InformerManager
	strategy        *InformerStrategy
	accessTracker   map[schema.GroupVersionResource]time.Time
	accessMutex     sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc

	// 性能优化相关
	preloadComplete chan struct{}
	preloadOnce     sync.Once
}

// NewStrategyManager 创建策略管理器
func NewStrategyManager(informerManager *InformerManager, strategy *InformerStrategy) *StrategyManager {
	ctx, cancel := context.WithCancel(context.Background())

	sm := &StrategyManager{
		informerManager: informerManager,
		strategy:        strategy,
		accessTracker:   make(map[schema.GroupVersionResource]time.Time),
		ctx:             ctx,
		cancel:          cancel,
		preloadComplete: make(chan struct{}),
	}

	// 启动自动清理
	if strategy.AutoCleanupEnabled {
		go sm.startAutoCleanup()
	}

	return sm
}

// PreloadResources 预加载资源（并行优化版本）
func (sm *StrategyManager) PreloadResources(resourceList []ResourceInfo) error {
	klog.Info("Starting parallel resource preloading")

	// 创建资源映射
	resourceMap := make(map[schema.GroupVersionResource]bool)
	for _, res := range resourceList {
		gvr := schema.GroupVersionResource{
			Group:    res.Group,
			Version:  res.Version,
			Resource: res.Name,
		}
		resourceMap[gvr] = res.Namespaced
	}

	// 过滤出需要预加载的资源
	var preloadList []struct {
		gvr        schema.GroupVersionResource
		namespaced bool
	}

	for _, gvr := range sm.strategy.PreloadResources {
		if namespaced, exists := resourceMap[gvr]; exists {
			preloadList = append(preloadList, struct {
				gvr        schema.GroupVersionResource
				namespaced bool
			}{gvr, namespaced})
		}
	}

	if len(preloadList) == 0 {
		close(sm.preloadComplete)
		klog.Info("No resources to preload")
		return nil
	}

	// 使用信号量控制并发数
	semaphore := make(chan struct{}, sm.strategy.ParallelPreloadCount)
	var wg sync.WaitGroup
	var errors []error
	var errorMutex sync.Mutex

	for _, item := range preloadList {
		wg.Add(1)
		go func(gvr schema.GroupVersionResource, namespaced bool) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			klog.Infof("Preloading resource: %s", gvr.String())
			if err := sm.informerManager.StartInformer(gvr, namespaced); err != nil {
				errorMutex.Lock()
				errors = append(errors, fmt.Errorf("failed to preload resource %s: %v", gvr.String(), err))
				errorMutex.Unlock()
				klog.Errorf("Failed to preload resource %s: %v", gvr.String(), err)
			} else {
				sm.updateAccessTime(gvr)
			}
		}(item.gvr, item.namespaced)
	}

	// 等待所有预加载完成
	go func() {
		wg.Wait()
		close(sm.preloadComplete)

		if len(errors) > 0 {
			klog.Errorf("Resource preloading completed with %d errors", len(errors))
		} else {
			klog.Info("Resource preloading completed successfully")
		}
	}()

	return nil
}

// WaitForPreloadComplete 等待预加载完成
func (sm *StrategyManager) WaitForPreloadComplete(timeout time.Duration) error {
	select {
	case <-sm.preloadComplete:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for preload completion after %v", timeout)
	}
}

// EnsureInformer 确保Informer已启动（懒加载）
func (sm *StrategyManager) EnsureInformer(gvr schema.GroupVersionResource, namespaced bool) error {
	// 检查是否已经存在且就绪
	if sm.informerManager.IsReady(gvr) {
		sm.updateAccessTime(gvr)
		return nil
	}

	// 检查并发限制
	stats := sm.informerManager.GetStats()
	if stats.ActiveInformers >= sm.strategy.MaxConcurrentInformers {
		klog.Warningf("Reached max concurrent informers limit (%d), cleaning up unused informers",
			sm.strategy.MaxConcurrentInformers)
		sm.cleanupUnusedInformers()
	}

	// 启动Informer
	klog.Infof("Lazy loading informer for: %s", gvr.String())
	if err := sm.informerManager.StartInformer(gvr, namespaced); err != nil {
		return err
	}

	sm.updateAccessTime(gvr)
	return nil
}

// GetObjects 获取对象（带策略，优化版本）
func (sm *StrategyManager) GetObjects(gvr schema.GroupVersionResource, namespace string, namespaced bool) ([]*unstructured.Unstructured, error) {
	// 确保Informer已启动
	if err := sm.EnsureInformer(gvr, namespaced); err != nil {
		return nil, err
	}

	// 快速检查是否已就绪
	if sm.informerManager.IsReady(gvr) {
		return sm.informerManager.GetObjects(gvr, namespace)
	}

	// 等待缓存同步（带超时）
	ctx, cancel := context.WithTimeout(sm.ctx, sm.strategy.CacheSyncTimeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond) // 减少轮询间隔
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for cache sync for %s after %v", gvr.String(), sm.strategy.CacheSyncTimeout)
		case <-ticker.C:
			if sm.informerManager.IsReady(gvr) {
				return sm.informerManager.GetObjects(gvr, namespace)
			}
		}
	}
}

// GetNamespaces 获取命名空间（带策略，优化版本）
func (sm *StrategyManager) GetNamespaces(gvr schema.GroupVersionResource, namespaced bool) ([]string, error) {
	// 确保Informer已启动
	if err := sm.EnsureInformer(gvr, namespaced); err != nil {
		return nil, err
	}

	// 快速检查是否已就绪
	if sm.informerManager.IsReady(gvr) {
		return sm.informerManager.GetNamespaces(gvr)
	}

	// 等待缓存同步
	ctx, cancel := context.WithTimeout(sm.ctx, sm.strategy.CacheSyncTimeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for cache sync for %s after %v", gvr.String(), sm.strategy.CacheSyncTimeout)
		case <-ticker.C:
			if sm.informerManager.IsReady(gvr) {
				return sm.informerManager.GetNamespaces(gvr)
			}
		}
	}
}

// GetObjectsWithFallback 获取对象（带降级策略）
func (sm *StrategyManager) GetObjectsWithFallback(gvr schema.GroupVersionResource, namespace string, namespaced bool) ([]*unstructured.Unstructured, error) {
	// 首先尝试从缓存获取
	if sm.informerManager.IsReady(gvr) {
		sm.updateAccessTime(gvr)
		return sm.informerManager.GetObjects(gvr, namespace)
	}

	// 如果缓存未就绪，启动Informer但立即返回空结果
	if err := sm.EnsureInformer(gvr, namespaced); err != nil {
		klog.Warningf("Failed to ensure informer for %s: %v", gvr.String(), err)
	}

	// 返回空结果，让前端知道数据正在加载
	return []*unstructured.Unstructured{}, nil
}

// updateAccessTime 更新访问时间
func (sm *StrategyManager) updateAccessTime(gvr schema.GroupVersionResource) {
	sm.accessMutex.Lock()
	defer sm.accessMutex.Unlock()
	sm.accessTracker[gvr] = time.Now()
}

// startAutoCleanup 启动自动清理
func (sm *StrategyManager) startAutoCleanup() {
	ticker := time.NewTicker(sm.strategy.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.cleanupUnusedInformers()
		}
	}
}

// cleanupUnusedInformers 清理未使用的Informer（优化版本）
func (sm *StrategyManager) cleanupUnusedInformers() {
	sm.accessMutex.RLock()
	now := time.Now()
	var toCleanup []schema.GroupVersionResource

	// 创建预加载资源的快速查找映射
	preloadedMap := make(map[schema.GroupVersionResource]bool)
	for _, preloadGvr := range sm.strategy.PreloadResources {
		preloadedMap[preloadGvr] = true
	}

	for gvr, lastAccess := range sm.accessTracker {
		// 预加载资源不清理
		if preloadedMap[gvr] {
			continue
		}

		// 检查是否超时
		if now.Sub(lastAccess) > sm.strategy.AccessTimeout {
			toCleanup = append(toCleanup, gvr)
		}
	}
	sm.accessMutex.RUnlock()

	// 执行清理
	if len(toCleanup) > 0 {
		klog.Infof("Cleaning up %d unused informers", len(toCleanup))

		for _, gvr := range toCleanup {
			klog.V(4).Infof("Cleaning up unused informer: %s", gvr.String())
			sm.informerManager.StopInformer(gvr)

			sm.accessMutex.Lock()
			delete(sm.accessTracker, gvr)
			sm.accessMutex.Unlock()
		}
	}
}

// GetCacheStats 获取缓存统计信息
func (sm *StrategyManager) GetCacheStats() CacheStats {
	stats := sm.informerManager.GetStats()

	// 添加访问统计
	sm.accessMutex.RLock()
	for gvr, lastAccess := range sm.accessTracker {
		if stat, exists := stats.ResourceStats[gvr.String()]; exists {
			// 可以添加更多访问相关的统计信息
			_ = lastAccess
			stats.ResourceStats[gvr.String()] = stat
		}
	}
	sm.accessMutex.RUnlock()

	return stats
}

// GetReadyResourcesCount 获取就绪资源数量
func (sm *StrategyManager) GetReadyResourcesCount() int {
	stats := sm.informerManager.GetStats()
	readyCount := 0
	for _, isReady := range stats.SyncStatus {
		if isReady {
			readyCount++
		}
	}
	return readyCount
}

// IsPreloadComplete 检查预加载是否完成
func (sm *StrategyManager) IsPreloadComplete() bool {
	select {
	case <-sm.preloadComplete:
		return true
	default:
		return false
	}
}

// Shutdown 关闭策略管理器
func (sm *StrategyManager) Shutdown() {
	klog.Info("Shutting down strategy manager")
	sm.cancel()
	sm.informerManager.Shutdown()
}

// ResourceInfo 资源信息
type ResourceInfo struct {
	Group      string `json:"group"`
	Version    string `json:"version"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Namespaced bool   `json:"namespaced"`
}
