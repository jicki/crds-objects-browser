package informer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// ResourceCache 资源缓存接口
type ResourceCache interface {
	// GetObjects 获取指定资源的所有对象
	GetObjects(gvr schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error)
	// GetNamespaces 获取指定资源的所有命名空间
	GetNamespaces(gvr schema.GroupVersionResource) ([]string, error)
	// IsReady 检查指定资源的Informer是否已就绪
	IsReady(gvr schema.GroupVersionResource) bool
	// StartInformer 启动指定资源的Informer
	StartInformer(gvr schema.GroupVersionResource, namespaced bool) error
	// StopInformer 停止指定资源的Informer
	StopInformer(gvr schema.GroupVersionResource)
	// GetStats 获取缓存统计信息
	GetStats() CacheStats
}

// CacheStats 缓存统计信息
type CacheStats struct {
	ActiveInformers int                     `json:"activeInformers"`
	TotalObjects    int                     `json:"totalObjects"`
	ResourceStats   map[string]ResourceStat `json:"resourceStats"`
	LastUpdate      time.Time               `json:"lastUpdate"`
}

// ResourceStat 单个资源的统计信息
type ResourceStat struct {
	ObjectCount    int           `json:"objectCount"`
	NamespaceCount int           `json:"namespaceCount"`
	LastSync       time.Time     `json:"lastSync"`
	SyncDuration   time.Duration `json:"syncDuration"`
}

// InformerManager Informer管理器
type InformerManager struct {
	dynamicClient   dynamic.Interface
	informerFactory dynamicinformer.DynamicSharedInformerFactory
	informers       map[schema.GroupVersionResource]cache.SharedIndexInformer
	stopChannels    map[schema.GroupVersionResource]chan struct{}
	mutex           sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	stats           CacheStats
	statsMutex      sync.RWMutex
}

// NewInformerManager 创建新的Informer管理器
func NewInformerManager(dynamicClient dynamic.Interface) *InformerManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &InformerManager{
		dynamicClient:   dynamicClient,
		informerFactory: dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 30*time.Second),
		informers:       make(map[schema.GroupVersionResource]cache.SharedIndexInformer),
		stopChannels:    make(map[schema.GroupVersionResource]chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
		stats: CacheStats{
			ResourceStats: make(map[string]ResourceStat),
			LastUpdate:    time.Now(),
		},
	}
}

// StartInformer 启动指定资源的Informer
func (im *InformerManager) StartInformer(gvr schema.GroupVersionResource, namespaced bool) error {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// 检查是否已经存在
	if _, exists := im.informers[gvr]; exists {
		klog.V(4).Infof("Informer for %s already exists", gvr.String())
		return nil
	}

	klog.Infof("Starting informer for resource: %s", gvr.String())

	// 创建Informer
	var informer cache.SharedIndexInformer
	if namespaced {
		informer = im.informerFactory.ForResource(gvr).Informer()
	} else {
		informer = im.informerFactory.ForResource(gvr).Informer()
	}

	// 添加事件处理器
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			im.updateStats(gvr, "add")
			klog.V(6).Infof("Added object for %s", gvr.String())
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			im.updateStats(gvr, "update")
			klog.V(6).Infof("Updated object for %s", gvr.String())
		},
		DeleteFunc: func(obj interface{}) {
			im.updateStats(gvr, "delete")
			klog.V(6).Infof("Deleted object for %s", gvr.String())
		},
	})

	// 创建停止通道
	stopCh := make(chan struct{})
	im.stopChannels[gvr] = stopCh
	im.informers[gvr] = informer

	// 启动Informer
	go informer.Run(stopCh)

	// 等待缓存同步
	go func() {
		startTime := time.Now()
		klog.Infof("Waiting for cache sync for %s", gvr.String())

		if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
			klog.Errorf("Failed to sync cache for %s", gvr.String())
			return
		}

		syncDuration := time.Since(startTime)
		klog.Infof("Cache synced for %s in %v", gvr.String(), syncDuration)

		// 更新统计信息
		im.updateResourceStat(gvr, syncDuration)
	}()

	return nil
}

// StopInformer 停止指定资源的Informer
func (im *InformerManager) StopInformer(gvr schema.GroupVersionResource) {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	if stopCh, exists := im.stopChannels[gvr]; exists {
		close(stopCh)
		delete(im.stopChannels, gvr)
		delete(im.informers, gvr)
		klog.Infof("Stopped informer for %s", gvr.String())
	}
}

// GetObjects 获取指定资源的所有对象
func (im *InformerManager) GetObjects(gvr schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error) {
	im.mutex.RLock()
	informer, exists := im.informers[gvr]
	im.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("informer for %s not found", gvr.String())
	}

	if !informer.HasSynced() {
		return nil, fmt.Errorf("informer for %s not synced yet", gvr.String())
	}

	store := informer.GetStore()
	objects := store.List()

	var result []*unstructured.Unstructured
	for _, obj := range objects {
		unstructuredObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		// 如果指定了命名空间，进行过滤
		if namespace != "" && namespace != "all" {
			if unstructuredObj.GetNamespace() != namespace {
				continue
			}
		}

		result = append(result, unstructuredObj.DeepCopy())
	}

	klog.V(4).Infof("Retrieved %d objects for %s (namespace: %s)", len(result), gvr.String(), namespace)
	return result, nil
}

// GetNamespaces 获取指定资源的所有命名空间
func (im *InformerManager) GetNamespaces(gvr schema.GroupVersionResource) ([]string, error) {
	im.mutex.RLock()
	informer, exists := im.informers[gvr]
	im.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("informer for %s not found", gvr.String())
	}

	if !informer.HasSynced() {
		return nil, fmt.Errorf("informer for %s not synced yet", gvr.String())
	}

	store := informer.GetStore()
	objects := store.List()

	namespaceSet := make(map[string]struct{})
	for _, obj := range objects {
		if unstructuredObj, ok := obj.(*unstructured.Unstructured); ok {
			if ns := unstructuredObj.GetNamespace(); ns != "" {
				namespaceSet[ns] = struct{}{}
			}
		}
	}

	var namespaces []string
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	return namespaces, nil
}

// IsReady 检查指定资源的Informer是否已就绪
func (im *InformerManager) IsReady(gvr schema.GroupVersionResource) bool {
	im.mutex.RLock()
	informer, exists := im.informers[gvr]
	im.mutex.RUnlock()

	if !exists {
		return false
	}

	return informer.HasSynced()
}

// GetStats 获取缓存统计信息
func (im *InformerManager) GetStats() CacheStats {
	im.statsMutex.RLock()
	defer im.statsMutex.RUnlock()

	// 更新活跃Informer数量和总对象数
	im.mutex.RLock()
	activeInformers := len(im.informers)
	totalObjects := 0

	for gvr, informer := range im.informers {
		if informer.HasSynced() {
			objectCount := len(informer.GetStore().List())
			totalObjects += objectCount

			// 更新资源统计
			if stat, exists := im.stats.ResourceStats[gvr.String()]; exists {
				stat.ObjectCount = objectCount
				im.stats.ResourceStats[gvr.String()] = stat
			}
		}
	}
	im.mutex.RUnlock()

	im.stats.ActiveInformers = activeInformers
	im.stats.TotalObjects = totalObjects
	im.stats.LastUpdate = time.Now()

	return im.stats
}

// updateStats 更新统计信息
func (im *InformerManager) updateStats(gvr schema.GroupVersionResource, operation string) {
	im.statsMutex.Lock()
	defer im.statsMutex.Unlock()

	if _, exists := im.stats.ResourceStats[gvr.String()]; !exists {
		im.stats.ResourceStats[gvr.String()] = ResourceStat{
			LastSync: time.Now(),
		}
	}
}

// updateResourceStat 更新资源统计信息
func (im *InformerManager) updateResourceStat(gvr schema.GroupVersionResource, syncDuration time.Duration) {
	im.statsMutex.Lock()
	defer im.statsMutex.Unlock()

	stat := im.stats.ResourceStats[gvr.String()]
	stat.LastSync = time.Now()
	stat.SyncDuration = syncDuration
	im.stats.ResourceStats[gvr.String()] = stat
}

// Shutdown 关闭所有Informer
func (im *InformerManager) Shutdown() {
	klog.Info("Shutting down informer manager")

	im.mutex.Lock()
	defer im.mutex.Unlock()

	// 停止所有Informer
	for gvr, stopCh := range im.stopChannels {
		close(stopCh)
		klog.Infof("Stopped informer for %s", gvr.String())
	}

	// 清理资源
	im.informers = make(map[schema.GroupVersionResource]cache.SharedIndexInformer)
	im.stopChannels = make(map[schema.GroupVersionResource]chan struct{})

	// 取消上下文
	im.cancel()
}

// StartAll 启动所有已注册的Informer
func (im *InformerManager) StartAll() {
	klog.Info("Starting all informers")
	im.informerFactory.Start(im.ctx.Done())
}

// WaitForCacheSync 等待所有缓存同步
func (im *InformerManager) WaitForCacheSync() bool {
	im.mutex.RLock()
	informers := make([]cache.InformerSynced, 0, len(im.informers))
	for _, informer := range im.informers {
		informers = append(informers, informer.HasSynced)
	}
	im.mutex.RUnlock()

	return cache.WaitForCacheSync(im.ctx.Done(), informers...)
}
