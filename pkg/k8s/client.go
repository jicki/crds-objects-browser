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
					time.Hour*24,
					"",
					nil,
				)

				informer := factory.ForResource(gvr).Informer()

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

		// 10分钟检查一次新的CRDs
		time.Sleep(10 * time.Minute)
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
		return nil, fmt.Errorf("error discovering preferred resources: %v", err)
	}

	var crds []CRDResource

	for _, list := range resources {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}

		for _, r := range list.APIResources {
			// 排除内置资源
			if isInternalResource(r.Name, gv.Group) {
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

// 检查是否为内置资源
func isInternalResource(name, group string) bool {
	internalGroups := []string{"", "apps", "batch", "extensions", "core"}
	for _, g := range internalGroups {
		if g == group {
			return true
		}
	}
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
