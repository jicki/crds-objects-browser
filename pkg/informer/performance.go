package informer

import (
	"context"
	"runtime"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// PerformanceOptimizer 性能优化器
type PerformanceOptimizer struct {
	// 内存池
	objectSlicePool sync.Pool
	stringSlicePool sync.Pool

	// 批处理配置
	batchSize    int
	batchTimeout time.Duration

	// 预热配置
	warmupEnabled bool
	warmupBatch   int

	// 统计信息
	stats PerformanceStats
	mutex sync.RWMutex
}

// PerformanceStats 性能统计
type PerformanceStats struct {
	PoolHits         int64         `json:"poolHits"`
	PoolMisses       int64         `json:"poolMisses"`
	BatchOperations  int64         `json:"batchOperations"`
	WarmupOperations int64         `json:"warmupOperations"`
	MemoryUsage      int64         `json:"memoryUsage"`
	GCCount          uint32        `json:"gcCount"`
	LastGCTime       time.Time     `json:"lastGCTime"`
	AverageLatency   time.Duration `json:"averageLatency"`
}

// NewPerformanceOptimizer 创建性能优化器
func NewPerformanceOptimizer() *PerformanceOptimizer {
	return &PerformanceOptimizer{
		objectSlicePool: sync.Pool{
			New: func() interface{} {
				return make([]*unstructured.Unstructured, 0, 100)
			},
		},
		stringSlicePool: sync.Pool{
			New: func() interface{} {
				return make([]string, 0, 50)
			},
		},
		batchSize:     50,
		batchTimeout:  100 * time.Millisecond,
		warmupEnabled: true,
		warmupBatch:   10,
	}
}

// GetObjectSlice 从池中获取对象切片
func (po *PerformanceOptimizer) GetObjectSlice() []*unstructured.Unstructured {
	po.mutex.Lock()
	po.stats.PoolHits++
	po.mutex.Unlock()

	slice := po.objectSlicePool.Get().([]*unstructured.Unstructured)
	return slice[:0] // 重置长度但保留容量
}

// PutObjectSlice 归还对象切片到池
func (po *PerformanceOptimizer) PutObjectSlice(slice []*unstructured.Unstructured) {
	if cap(slice) <= 1000 { // 避免池中对象过大
		po.objectSlicePool.Put(slice)
	} else {
		po.mutex.Lock()
		po.stats.PoolMisses++
		po.mutex.Unlock()
	}
}

// GetStringSlice 从池中获取字符串切片
func (po *PerformanceOptimizer) GetStringSlice() []string {
	po.mutex.Lock()
	po.stats.PoolHits++
	po.mutex.Unlock()

	slice := po.stringSlicePool.Get().([]string)
	return slice[:0]
}

// PutStringSlice 归还字符串切片到池
func (po *PerformanceOptimizer) PutStringSlice(slice []string) {
	if cap(slice) <= 200 {
		po.stringSlicePool.Put(slice)
	} else {
		po.mutex.Lock()
		po.stats.PoolMisses++
		po.mutex.Unlock()
	}
}

// BatchProcess 批处理操作
func (po *PerformanceOptimizer) BatchProcess(ctx context.Context, items []interface{}, processor func([]interface{}) error) error {
	po.mutex.Lock()
	po.stats.BatchOperations++
	po.mutex.Unlock()

	if len(items) == 0 {
		return nil
	}

	// 分批处理
	for i := 0; i < len(items); i += po.batchSize {
		end := i + po.batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]

		// 检查上下文是否取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 处理批次
		if err := processor(batch); err != nil {
			return err
		}

		// 批次间短暂休息
		if i+po.batchSize < len(items) {
			time.Sleep(po.batchTimeout)
		}
	}

	return nil
}

// WarmupCache 预热缓存
func (po *PerformanceOptimizer) WarmupCache(ctx context.Context, gvrs []schema.GroupVersionResource, warmupFunc func(schema.GroupVersionResource) error) {
	if !po.warmupEnabled {
		return
	}

	klog.V(2).Infof("Starting cache warmup for %d resources", len(gvrs))

	// 分批预热
	semaphore := make(chan struct{}, po.warmupBatch)
	var wg sync.WaitGroup

	for _, gvr := range gvrs {
		select {
		case <-ctx.Done():
			klog.Warning("Cache warmup cancelled")
			return
		default:
		}

		wg.Add(1)
		go func(resource schema.GroupVersionResource) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := warmupFunc(resource); err != nil {
				klog.V(4).Infof("Warmup failed for %s: %v", resource.String(), err)
			} else {
				po.mutex.Lock()
				po.stats.WarmupOperations++
				po.mutex.Unlock()
			}
		}(gvr)
	}

	wg.Wait()
	klog.V(2).Info("Cache warmup completed")
}

// OptimizeMemory 内存优化
func (po *PerformanceOptimizer) OptimizeMemory() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	po.mutex.Lock()
	po.stats.MemoryUsage = int64(m.Alloc)
	po.stats.GCCount = m.NumGC
	po.mutex.Unlock()

	// 如果内存使用过高，触发GC
	if m.Alloc > 100*1024*1024 { // 100MB
		klog.V(3).Info("Triggering garbage collection due to high memory usage")
		runtime.GC()

		po.mutex.Lock()
		po.stats.LastGCTime = time.Now()
		po.mutex.Unlock()
	}
}

// GetStats 获取性能统计
func (po *PerformanceOptimizer) GetStats() PerformanceStats {
	po.mutex.RLock()
	defer po.mutex.RUnlock()

	// 更新内存统计
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := po.stats
	stats.MemoryUsage = int64(m.Alloc)
	stats.GCCount = m.NumGC

	return stats
}

// UpdateLatency 更新延迟统计
func (po *PerformanceOptimizer) UpdateLatency(latency time.Duration) {
	po.mutex.Lock()
	defer po.mutex.Unlock()

	// 简单的移动平均
	if po.stats.AverageLatency == 0 {
		po.stats.AverageLatency = latency
	} else {
		po.stats.AverageLatency = (po.stats.AverageLatency + latency) / 2
	}
}

// StartPerformanceMonitoring 启动性能监控
func (po *PerformanceOptimizer) StartPerformanceMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			po.OptimizeMemory()

			stats := po.GetStats()
			klog.V(3).Infof("Performance stats: Memory=%dMB, GC=%d, PoolHits=%d, BatchOps=%d",
				stats.MemoryUsage/(1024*1024), stats.GCCount, stats.PoolHits, stats.BatchOperations)
		}
	}
}

// SmartPreloader 智能预加载器
type SmartPreloader struct {
	optimizer     *PerformanceOptimizer
	priorityQueue []schema.GroupVersionResource
	loadingStatus map[schema.GroupVersionResource]bool
	mutex         sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewSmartPreloader 创建智能预加载器
func NewSmartPreloader(optimizer *PerformanceOptimizer) *SmartPreloader {
	ctx, cancel := context.WithCancel(context.Background())

	return &SmartPreloader{
		optimizer:     optimizer,
		priorityQueue: make([]schema.GroupVersionResource, 0),
		loadingStatus: make(map[schema.GroupVersionResource]bool),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// AddToPriorityQueue 添加到优先队列
func (sp *SmartPreloader) AddToPriorityQueue(gvr schema.GroupVersionResource, priority int) {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	// 简单的优先级插入（高优先级在前）
	if priority > 5 {
		sp.priorityQueue = append([]schema.GroupVersionResource{gvr}, sp.priorityQueue...)
	} else {
		sp.priorityQueue = append(sp.priorityQueue, gvr)
	}
}

// ProcessQueue 处理队列
func (sp *SmartPreloader) ProcessQueue(loader func(schema.GroupVersionResource) error) {
	sp.mutex.RLock()
	queue := make([]schema.GroupVersionResource, len(sp.priorityQueue))
	copy(queue, sp.priorityQueue)
	sp.mutex.RUnlock()

	if len(queue) == 0 {
		return
	}

	klog.V(2).Infof("Processing preload queue with %d resources", len(queue))

	// 使用性能优化器的预热功能
	sp.optimizer.WarmupCache(sp.ctx, queue, func(gvr schema.GroupVersionResource) error {
		sp.mutex.Lock()
		sp.loadingStatus[gvr] = true
		sp.mutex.Unlock()

		err := loader(gvr)

		sp.mutex.Lock()
		sp.loadingStatus[gvr] = false
		sp.mutex.Unlock()

		return err
	})

	// 清空队列
	sp.mutex.Lock()
	sp.priorityQueue = sp.priorityQueue[:0]
	sp.mutex.Unlock()
}

// IsLoading 检查是否正在加载
func (sp *SmartPreloader) IsLoading(gvr schema.GroupVersionResource) bool {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	return sp.loadingStatus[gvr]
}

// Shutdown 关闭预加载器
func (sp *SmartPreloader) Shutdown() {
	sp.cancel()
}
