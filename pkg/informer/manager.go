package informer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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
	SyncStatus      map[string]bool         `json:"syncStatus"`
}

// ResourceStat 单个资源的统计信息
type ResourceStat struct {
	ObjectCount    int           `json:"objectCount"`
	NamespaceCount int           `json:"namespaceCount"`
	LastSync       time.Time     `json:"lastSync"`
	SyncDuration   time.Duration `json:"syncDuration"`
	IsReady        bool          `json:"isReady"`
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

	// 性能优化相关
	objectPool    sync.Pool
	readyStatus   map[schema.GroupVersionResource]*atomic.Bool
	readyMutex    sync.RWMutex
	syncWaitGroup sync.WaitGroup
}

// NewInformerManager 创建新的Informer管理器
func NewInformerManager(dynamicClient dynamic.Interface) *InformerManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &InformerManager{
		dynamicClient:   dynamicClient,
		informerFactory: dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 30*time.Second),
		informers:       make(map[schema.GroupVersionResource]cache.SharedIndexInformer),
		stopChannels:    make(map[schema.GroupVersionResource]chan struct{}),
		readyStatus:     make(map[schema.GroupVersionResource]*atomic.Bool),
		ctx:             ctx,
		cancel:          cancel,
		stats: CacheStats{
			ResourceStats: make(map[string]ResourceStat),
			SyncStatus:    make(map[string]bool),
			LastUpdate:    time.Now(),
		},
		objectPool: sync.Pool{
			New: func() interface{} {
				return make([]*unstructured.Unstructured, 0, 100)
			},
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

	// 初始化就绪状态
	readyFlag := &atomic.Bool{}
	readyFlag.Store(false)
	im.readyMutex.Lock()
	im.readyStatus[gvr] = readyFlag
	im.readyMutex.Unlock()

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

	// 异步等待缓存同步
	im.syncWaitGroup.Add(1)
	go func() {
		defer im.syncWaitGroup.Done()
		startTime := time.Now()
		klog.Infof("Waiting for cache sync for %s", gvr.String())

		// 使用带超时的上下文
		syncCtx, syncCancel := context.WithTimeout(im.ctx, 60*time.Second)
		defer syncCancel()

		// 创建一个通道来接收同步结果
		syncDone := make(chan bool, 1)
		go func() {
			syncDone <- cache.WaitForCacheSync(stopCh, informer.HasSynced)
		}()

		select {
		case synced := <-syncDone:
			if synced {
				syncDuration := time.Since(startTime)
				klog.Infof("Cache synced for %s in %v", gvr.String(), syncDuration)

				// 标记为就绪
				readyFlag.Store(true)

				// 更新统计信息
				im.updateResourceStat(gvr, syncDuration, true)
			} else {
				klog.Errorf("Failed to sync cache for %s", gvr.String())
				im.updateResourceStat(gvr, time.Since(startTime), false)
			}
		case <-syncCtx.Done():
			klog.Errorf("Timeout waiting for cache sync for %s after %v", gvr.String(), time.Since(startTime))
			im.updateResourceStat(gvr, time.Since(startTime), false)
		}
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

		// 清理就绪状态
		im.readyMutex.Lock()
		delete(im.readyStatus, gvr)
		im.readyMutex.Unlock()

		klog.Infof("Stopped informer for %s", gvr.String())
	}
}

// GetObjects 获取指定资源的所有对象（优化版本）
func (im *InformerManager) GetObjects(gvr schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error) {
	im.mutex.RLock()
	informer, exists := im.informers[gvr]
	im.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("informer for %s not found", gvr.String())
	}

	if !im.IsReady(gvr) {
		return nil, fmt.Errorf("informer for %s not synced yet", gvr.String())
	}

	store := informer.GetStore()
	objects := store.List()

	// 从对象池获取切片
	result := im.objectPool.Get().([]*unstructured.Unstructured)
	result = result[:0] // 重置长度但保留容量

	defer func() {
		// 归还到对象池
		if cap(result) <= 1000 { // 避免池中对象过大
			im.objectPool.Put(result)
		}
	}()

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

		// 优化：只在需要时进行深拷贝
		result = append(result, unstructuredObj.DeepCopy())
	}

	// 创建新的切片返回，避免池对象被外部修改
	finalResult := make([]*unstructured.Unstructured, len(result))
	copy(finalResult, result)

	klog.V(4).Infof("Retrieved %d objects for %s (namespace: %s)", len(finalResult), gvr.String(), namespace)
	return finalResult, nil
}

// GetNamespaces 获取指定资源的所有命名空间
func (im *InformerManager) GetNamespaces(gvr schema.GroupVersionResource) ([]string, error) {
	im.mutex.RLock()
	informer, exists := im.informers[gvr]
	im.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("informer for %s not found", gvr.String())
	}

	if !im.IsReady(gvr) {
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

// IsReady 检查指定资源的Informer是否已就绪（优化版本）
func (im *InformerManager) IsReady(gvr schema.GroupVersionResource) bool {
	im.readyMutex.RLock()
	readyFlag, exists := im.readyStatus[gvr]
	im.readyMutex.RUnlock()

	if !exists {
		return false
	}

	return readyFlag.Load()
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
		isReady := im.IsReady(gvr)
		im.stats.SyncStatus[gvr.String()] = isReady

		if isReady {
			objectCount := len(informer.GetStore().List())
			totalObjects += objectCount

			// 更新资源统计
			if stat, exists := im.stats.ResourceStats[gvr.String()]; exists {
				stat.ObjectCount = objectCount
				stat.IsReady = true
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
			IsReady:  false,
		}
	}
}

// updateResourceStat 更新资源统计信息
func (im *InformerManager) updateResourceStat(gvr schema.GroupVersionResource, syncDuration time.Duration, isReady bool) {
	im.statsMutex.Lock()
	defer im.statsMutex.Unlock()

	stat := im.stats.ResourceStats[gvr.String()]
	stat.LastSync = time.Now()
	stat.SyncDuration = syncDuration
	stat.IsReady = isReady
	im.stats.ResourceStats[gvr.String()] = stat
	im.stats.SyncStatus[gvr.String()] = isReady
}

// WaitForInitialSync 等待初始同步完成
func (im *InformerManager) WaitForInitialSync(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		im.syncWaitGroup.Wait()
		close(done)
	}()

	select {
	case <-done:
		klog.Info("All informers initial sync completed")
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for initial sync after %v", timeout)
	}
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

	// 清理就绪状态
	im.readyMutex.Lock()
	im.readyStatus = make(map[schema.GroupVersionResource]*atomic.Bool)
	im.readyMutex.Unlock()

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
