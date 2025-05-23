package k8s

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// CRDResource 表示CRD资源信息
type CRDResource struct {
	Group      string `json:"group"`
	Version    string `json:"version"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespaced bool   `json:"namespaced"`
}

// CRDObject 表示一个CRD对象
type CRDObject struct {
	Name              string                 `json:"name"`
	Namespace         string                 `json:"namespace,omitempty"`
	Kind              string                 `json:"kind"`
	Group             string                 `json:"group"`
	Version           string                 `json:"version"`
	CreationTimestamp string                 `json:"creationTimestamp"`
	Status            map[string]interface{} `json:"status,omitempty"`
	Spec              map[string]interface{} `json:"spec,omitempty"`
}

// Client 是Kubernetes客户端
type Client struct {
	dynamicClient    dynamic.Interface
	discoveryClient  discovery.DiscoveryInterface
	informers        map[schema.GroupVersionResource]cache.SharedIndexInformer
	informersLock    sync.RWMutex
	objectsStore     map[schema.GroupVersionResource]map[string][]unstructured.Unstructured
	objectsStoreLock sync.RWMutex
	stopChan         chan struct{}
	resyncPeriod     time.Duration
	watchBufferSize  int
}

// NewClient 创建新的Kubernetes客户端
func NewClient(kubeconfig string) (*Client, error) {
	var config *rest.Config
	var err error

	// 首先尝试使用集群内配置
	config, err = rest.InClusterConfig()
	if err != nil {
		// 如果集群内配置失败，并且提供了 kubeconfig，则使用 kubeconfig
		if kubeconfig != "" {
			config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("error building kubeconfig: %v", err)
			}
		} else {
			return nil, fmt.Errorf("error creating in-cluster config: %v", err)
		}
	}

	// 配置客户端参数
	config.QPS = 50
	config.Burst = 100
	config.Timeout = 30 * time.Second

	// 配置 TLS
	config.TLSClientConfig.Insecure = true
	config.TLSClientConfig.CAData = nil
	config.TLSClientConfig.CAFile = ""

	// 创建动态客户端
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic client: %v", err)
	}

	// 创建发现客户端
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client: %v", err)
	}

	client := &Client{
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		informers:       make(map[schema.GroupVersionResource]cache.SharedIndexInformer),
		objectsStore:    make(map[schema.GroupVersionResource]map[string][]unstructured.Unstructured),
		stopChan:        make(chan struct{}),
		resyncPeriod:    time.Hour * 12, // 降低同步频率
		watchBufferSize: 1024,           // 设置合理的缓冲区大小
	}

	// 初始化CRD资源并启动informers
	go client.initResources()

	return client, nil
}

// initResources 初始化所有CRD资源
func (c *Client) initResources() {
	for {
		crds, err := c.GetCRDs()
		if err != nil {
			fmt.Printf("Error getting CRDs: %v, retrying in 5 seconds\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// 为每个CRD创建informer
		for _, crd := range crds {
			gvr := schema.GroupVersionResource{
				Group:    crd.Group,
				Version:  crd.Version,
				Resource: crd.Name,
			}

			// 检查informer是否已经存在
			c.informersLock.RLock()
			_, exists := c.informers[gvr]
			c.informersLock.RUnlock()

			if !exists {
				// 创建新的informer
				factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
					c.dynamicClient,
					c.resyncPeriod,
					"",
					nil,
				)

				informer := factory.ForResource(gvr).Informer()

				// 配置informer选项
				informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
					if err != nil {
						fmt.Printf("Watch error for %v: %v\n", gvr, err)
					}
				})

				informer.SetTransform(func(obj interface{}) (interface{}, error) {
					// 过滤和转换对象，减少内存使用
					if u, ok := obj.(*unstructured.Unstructured); ok {
						filtered := u.DeepCopy()
						// 只保留必要的字段
						filtered.SetManagedFields(nil)
						filtered.SetAnnotations(nil)
						return filtered, nil
					}
					return obj, nil
				})

				// 设置事件处理程序
				informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
					AddFunc:    c.handleAdd(gvr),
					UpdateFunc: c.handleUpdate(gvr),
					DeleteFunc: c.handleDelete(gvr),
				})

				// 存储informer
				c.informersLock.Lock()
				c.informers[gvr] = informer
				c.informersLock.Unlock()

				// 初始化对象存储
				c.objectsStoreLock.Lock()
				c.objectsStore[gvr] = make(map[string][]unstructured.Unstructured)
				c.objectsStoreLock.Unlock()

				// 启动informer
				go informer.Run(c.stopChan)
			}
		}

		// 清理不再需要的informers
		c.cleanupUnusedInformers(crds)

		// 30分钟检查一次新的CRDs
		time.Sleep(30 * time.Minute)
	}
}

// cleanupUnusedInformers 清理不再需要的informers
func (c *Client) cleanupUnusedInformers(currentCRDs []CRDResource) {
	c.informersLock.Lock()
	defer c.informersLock.Unlock()

	c.objectsStoreLock.Lock()
	defer c.objectsStoreLock.Unlock()

	// 创建当前CRD的映射
	crdMap := make(map[schema.GroupVersionResource]bool)
	for _, crd := range currentCRDs {
		gvr := schema.GroupVersionResource{
			Group:    crd.Group,
			Version:  crd.Version,
			Resource: crd.Name,
		}
		crdMap[gvr] = true
	}

	// 清理不再存在的informers和存储
	for gvr := range c.informers {
		if !crdMap[gvr] {
			delete(c.informers, gvr)
			delete(c.objectsStore, gvr)
		}
	}
}

// handleAdd 处理添加事件
func (c *Client) handleAdd(gvr schema.GroupVersionResource) func(obj interface{}) {
	return func(obj interface{}) {
		unstrObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return
		}

		namespace := unstrObj.GetNamespace()
		if namespace == "" {
			namespace = "default" // 对于非命名空间资源使用默认命名空间键
		}

		c.objectsStoreLock.Lock()
		defer c.objectsStoreLock.Unlock()

		if _, ok := c.objectsStore[gvr]; !ok {
			c.objectsStore[gvr] = make(map[string][]unstructured.Unstructured)
		}

		objList := c.objectsStore[gvr][namespace]
		// 检查对象是否已存在
		for i, existingObj := range objList {
			if existingObj.GetName() == unstrObj.GetName() {
				objList[i] = *unstrObj.DeepCopy()
				c.objectsStore[gvr][namespace] = objList
				return
			}
		}

		// 如果不存在，添加到列表
		c.objectsStore[gvr][namespace] = append(c.objectsStore[gvr][namespace], *unstrObj.DeepCopy())
	}
}

// handleUpdate 处理更新事件
func (c *Client) handleUpdate(gvr schema.GroupVersionResource) func(oldObj, newObj interface{}) {
	return func(oldObj, newObj interface{}) {
		unstrObj, ok := newObj.(*unstructured.Unstructured)
		if !ok {
			return
		}

		namespace := unstrObj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		c.objectsStoreLock.Lock()
		defer c.objectsStoreLock.Unlock()

		if _, ok := c.objectsStore[gvr]; !ok {
			c.objectsStore[gvr] = make(map[string][]unstructured.Unstructured)
		}

		objList := c.objectsStore[gvr][namespace]
		// 更新对象
		for i, existingObj := range objList {
			if existingObj.GetName() == unstrObj.GetName() {
				objList[i] = *unstrObj.DeepCopy()
				c.objectsStore[gvr][namespace] = objList
				return
			}
		}

		// 如果不存在，添加到列表
		c.objectsStore[gvr][namespace] = append(c.objectsStore[gvr][namespace], *unstrObj.DeepCopy())
	}
}

// handleDelete 处理删除事件
func (c *Client) handleDelete(gvr schema.GroupVersionResource) func(obj interface{}) {
	return func(obj interface{}) {
		unstrObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
			if !ok {
				return
			}
			unstrObj, ok = tombstone.Obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
		}

		namespace := unstrObj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		c.objectsStoreLock.Lock()
		defer c.objectsStoreLock.Unlock()

		if _, ok := c.objectsStore[gvr]; !ok {
			return
		}

		objList := c.objectsStore[gvr][namespace]
		for i, existingObj := range objList {
			if existingObj.GetName() == unstrObj.GetName() {
				// 从列表中移除对象
				c.objectsStore[gvr][namespace] = append(objList[:i], objList[i+1:]...)
				return
			}
		}
	}
}

// GetCRDs 获取所有CRD资源
func (c *Client) GetCRDs() ([]CRDResource, error) {
	resources, err := c.discoveryClient.ServerPreferredResources()
	if err != nil {
		// 处理部分错误，继续获取可用资源
		if discovery.IsGroupDiscoveryFailedError(err) {
			// 记录错误但继续处理可用的资源组
			fmt.Printf("Warning: Some groups were not discoverable: %v\n", err)
		} else {
			return nil, fmt.Errorf("error discovering preferred resources: %v", err)
		}
	}

	var crds []CRDResource

	// 首先添加核心K8s资源
	coreResources := []CRDResource{
		{Group: "", Version: "v1", Kind: "Pod", Name: "pods", Namespaced: true},
		{Group: "", Version: "v1", Kind: "Service", Name: "services", Namespaced: true},
		{Group: "", Version: "v1", Kind: "ConfigMap", Name: "configmaps", Namespaced: true},
		{Group: "", Version: "v1", Kind: "Secret", Name: "secrets", Namespaced: true},
		{Group: "", Version: "v1", Kind: "PersistentVolume", Name: "persistentvolumes", Namespaced: false},
		{Group: "", Version: "v1", Kind: "PersistentVolumeClaim", Name: "persistentvolumeclaims", Namespaced: true},
		{Group: "", Version: "v1", Kind: "Node", Name: "nodes", Namespaced: false},
		{Group: "", Version: "v1", Kind: "Namespace", Name: "namespaces", Namespaced: false},
		{Group: "", Version: "v1", Kind: "ServiceAccount", Name: "serviceaccounts", Namespaced: true},
		{Group: "apps", Version: "v1", Kind: "Deployment", Name: "deployments", Namespaced: true},
		{Group: "apps", Version: "v1", Kind: "ReplicaSet", Name: "replicasets", Namespaced: true},
		{Group: "apps", Version: "v1", Kind: "DaemonSet", Name: "daemonsets", Namespaced: true},
		{Group: "apps", Version: "v1", Kind: "StatefulSet", Name: "statefulsets", Namespaced: true},
		{Group: "batch", Version: "v1", Kind: "Job", Name: "jobs", Namespaced: true},
		{Group: "batch", Version: "v1", Kind: "CronJob", Name: "cronjobs", Namespaced: true},
		{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress", Name: "ingresses", Namespaced: true},
		{Group: "networking.k8s.io", Version: "v1", Kind: "NetworkPolicy", Name: "networkpolicies", Namespaced: true},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role", Name: "roles", Namespaced: true},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding", Name: "rolebindings", Namespaced: true},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole", Name: "clusterroles", Namespaced: false},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding", Name: "clusterrolebindings", Namespaced: false},
	}

	crds = append(crds, coreResources...)

	for _, list := range resources {
		if list == nil || list.APIResources == nil {
			continue
		}

		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}

		for _, r := range list.APIResources {
			// 跳过不可列出或不可获取的资源
			if !hasVerb(r, "list") || !hasVerb(r, "get") {
				continue
			}

			// 跳过已经手动添加的核心资源
			if isCoreResource(r.Name, gv.Group) {
				continue
			}

			// 排除特殊资源
			if isSpecialResource(r.Name, gv.Group) {
				continue
			}

			// 排除已弃用的资源，避免Kubernetes警告
			if isDeprecatedResource(r.Name, gv.Group, gv.Version) {
				continue
			}

			crds = append(crds, CRDResource{
				Group:      gv.Group,
				Version:    gv.Version,
				Kind:       r.Kind,
				Name:       r.Name,
				Namespaced: r.Namespaced,
			})
		}
	}

	return crds, nil
}

// isCoreResource 检查是否为核心资源（已手动添加的资源）
func isCoreResource(name, group string) bool {
	coreResourceMap := map[string]map[string]bool{
		"": {
			"pods":                   true,
			"services":               true,
			"configmaps":             true,
			"secrets":                true,
			"persistentvolumes":      true,
			"persistentvolumeclaims": true,
			"nodes":                  true,
			"namespaces":             true,
			"serviceaccounts":        true,
		},
		"apps": {
			"deployments":  true,
			"replicasets":  true,
			"daemonsets":   true,
			"statefulsets": true,
		},
		"batch": {
			"jobs":     true,
			"cronjobs": true,
		},
		"networking.k8s.io": {
			"ingresses":       true,
			"networkpolicies": true,
		},
		"rbac.authorization.k8s.io": {
			"roles":               true,
			"rolebindings":        true,
			"clusterroles":        true,
			"clusterrolebindings": true,
		},
	}

	if resources, ok := coreResourceMap[group]; ok {
		return resources[name]
	}
	return false
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

// isSpecialResource 检查是否为特殊资源（需要排除的资源）
func isSpecialResource(name, group string) bool {
	specialResources := map[string][]string{
		"": {
			"componentstatuses", // 排除已弃用的ComponentStatus
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
		// 排除已弃用的资源，避免Kubernetes警告
		"policy": {
			"podsecuritypolicies", // 在v1.21+中已弃用，在v1.25+中不可用
		},
		"extensions": {
			"podsecuritypolicies", // 也可能在extensions组中
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

// isDeprecatedResource 检查是否为已弃用的资源
func isDeprecatedResource(name, group, version string) bool {
	// 已弃用的资源列表
	deprecatedResources := map[string]map[string][]string{
		"policy": {
			"v1beta1": {"podsecuritypolicies"},
		},
		"extensions": {
			"v1beta1": {"podsecuritypolicies", "deployments", "replicasets", "daemonsets"},
		},
		"apps": {
			"v1beta1": {"deployments", "replicasets", "daemonsets"},
			"v1beta2": {"deployments", "replicasets", "daemonsets"},
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

// 检查是否为内置资源
func isInternalResource(name, group string) bool {
	// 允许显示的核心资源
	allowedCoreResources := map[string]bool{
		"pods":                   true,
		"services":               true,
		"deployments":            true,
		"replicasets":            true,
		"daemonsets":             true,
		"statefulsets":           true,
		"jobs":                   true,
		"cronjobs":               true,
		"configmaps":             true,
		"secrets":                true,
		"persistentvolumes":      true,
		"persistentvolumeclaims": true,
		"ingresses":              true,
		"networkpolicies":        true,
		"nodes":                  true,
		"namespaces":             true,
		"serviceaccounts":        true,
		"roles":                  true,
		"rolebindings":           true,
		"clusterroles":           true,
		"clusterrolebindings":    true,
	}

	// 对于核心组（""）和apps组，检查是否在允许列表中
	if group == "" || group == "apps" || group == "batch" || group == "networking.k8s.io" || group == "rbac.authorization.k8s.io" {
		return !allowedCoreResources[name]
	}

	// 对于extensions组，只允许特定资源
	if group == "extensions" {
		return !allowedCoreResources[name]
	}

	// 其他组的资源都显示（主要是CRD）
	return false
}

// GetNamespaces 获取所有命名空间
func (c *Client) GetNamespaces() ([]string, error) {
	// 使用dynamicClient获取命名空间列表
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	list, err := c.dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing namespaces: %v", err)
	}

	var namespaces []string
	for _, item := range list.Items {
		namespaces = append(namespaces, item.GetName())
	}

	return namespaces, nil
}

// ListCRDObjects 列出指定CRD资源的所有对象
func (c *Client) ListCRDObjects(group, version, resource, namespace string) ([]CRDObject, error) {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	c.objectsStoreLock.RLock()
	defer c.objectsStoreLock.RUnlock()

	objStore, exists := c.objectsStore[gvr]
	if !exists {
		return nil, fmt.Errorf("no objects found for resource %s.%s/%s", resource, group, version)
	}

	var objects []CRDObject

	// 如果指定了命名空间，则返回该命名空间的对象
	if namespace != "" && namespace != "all" {
		objList, exists := objStore[namespace]
		if !exists {
			return objects, nil
		}

		for _, obj := range objList {
			crdObj := toCRDObject(obj)
			objects = append(objects, crdObj)
		}
	} else {
		// 返回所有命名空间的对象
		for _, objList := range objStore {
			for _, obj := range objList {
				crdObj := toCRDObject(obj)
				objects = append(objects, crdObj)
			}
		}
	}

	return objects, nil
}

// 将unstructured对象转换为CRDObject
func toCRDObject(obj unstructured.Unstructured) CRDObject {
	gvk := obj.GetObjectKind().GroupVersionKind()

	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")

	return CRDObject{
		Name:              obj.GetName(),
		Namespace:         obj.GetNamespace(),
		Kind:              gvk.Kind,
		Group:             gvk.Group,
		Version:           gvk.Version,
		CreationTimestamp: obj.GetCreationTimestamp().Format(time.RFC3339),
		Status:            status,
		Spec:              spec,
	}
}

// GetAllAvailableNamespaces 获取指定CRD可用的所有命名空间
func (c *Client) GetAllAvailableNamespaces(group, version, resource string) ([]string, error) {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	c.objectsStoreLock.RLock()
	defer c.objectsStoreLock.RUnlock()

	objStore, exists := c.objectsStore[gvr]
	if !exists {
		return nil, fmt.Errorf("no objects found for resource %s.%s/%s", resource, group, version)
	}

	var namespaces []string
	for ns := range objStore {
		if len(objStore[ns]) > 0 {
			namespaces = append(namespaces, ns)
		}
	}

	return namespaces, nil
}

// Shutdown 关闭客户端
func (c *Client) Shutdown() {
	close(c.stopChan)
}
