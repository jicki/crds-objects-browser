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
		CleanupInterval:        10 * time.Minute,
		AccessTimeout:          5 * time.Minute,
		MaxConcurrentInformers: 50,
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
	}

	// 启动自动清理
	if strategy.AutoCleanupEnabled {
		go sm.startAutoCleanup()
	}

	return sm
}

// PreloadResources 预加载资源
func (sm *StrategyManager) PreloadResources(resourceList []ResourceInfo) error {
	klog.Info("Starting resource preloading")

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

	// 预加载核心资源
	for _, gvr := range sm.strategy.PreloadResources {
		if namespaced, exists := resourceMap[gvr]; exists {
			klog.Infof("Preloading resource: %s", gvr.String())
			if err := sm.informerManager.StartInformer(gvr, namespaced); err != nil {
				klog.Errorf("Failed to preload resource %s: %v", gvr.String(), err)
				continue
			}
			sm.updateAccessTime(gvr)
		}
	}

	klog.Info("Resource preloading completed")
	return nil
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

// GetObjects 获取对象（带策略）
func (sm *StrategyManager) GetObjects(gvr schema.GroupVersionResource, namespace string, namespaced bool) ([]*unstructured.Unstructured, error) {
	// 确保Informer已启动
	if err := sm.EnsureInformer(gvr, namespaced); err != nil {
		return nil, err
	}

	// 等待缓存同步（带超时）
	ctx, cancel := context.WithTimeout(sm.ctx, 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for cache sync for %s", gvr.String())
		case <-ticker.C:
			if sm.informerManager.IsReady(gvr) {
				return sm.informerManager.GetObjects(gvr, namespace)
			}
		}
	}
}

// GetNamespaces 获取命名空间（带策略）
func (sm *StrategyManager) GetNamespaces(gvr schema.GroupVersionResource, namespaced bool) ([]string, error) {
	// 确保Informer已启动
	if err := sm.EnsureInformer(gvr, namespaced); err != nil {
		return nil, err
	}

	// 等待缓存同步
	ctx, cancel := context.WithTimeout(sm.ctx, 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for cache sync for %s", gvr.String())
		case <-ticker.C:
			if sm.informerManager.IsReady(gvr) {
				return sm.informerManager.GetNamespaces(gvr)
			}
		}
	}
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

// cleanupUnusedInformers 清理未使用的Informer
func (sm *StrategyManager) cleanupUnusedInformers() {
	sm.accessMutex.RLock()
	now := time.Now()
	var toCleanup []schema.GroupVersionResource

	for gvr, lastAccess := range sm.accessTracker {
		// 检查是否为预加载资源
		isPreloaded := false
		for _, preloadGvr := range sm.strategy.PreloadResources {
			if gvr == preloadGvr {
				isPreloaded = true
				break
			}
		}

		// 预加载资源不清理
		if isPreloaded {
			continue
		}

		// 检查是否超时
		if now.Sub(lastAccess) > sm.strategy.AccessTimeout {
			toCleanup = append(toCleanup, gvr)
		}
	}
	sm.accessMutex.RUnlock()

	// 执行清理
	for _, gvr := range toCleanup {
		klog.Infof("Cleaning up unused informer: %s", gvr.String())
		sm.informerManager.StopInformer(gvr)

		sm.accessMutex.Lock()
		delete(sm.accessTracker, gvr)
		sm.accessMutex.Unlock()
	}

	if len(toCleanup) > 0 {
		klog.Infof("Cleaned up %d unused informers", len(toCleanup))
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
