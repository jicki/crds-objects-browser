package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jicki/crds-objects-browser/pkg/k8s"
)

// Server 表示API服务器
type Server struct {
	k8sClient *k8s.Client
	router    *gin.Engine
	httpSrv   *http.Server
	port      string
}

// NewServer 创建新的API服务器
func NewServer(k8sClient *k8s.Client, port string) *Server {
	router := gin.Default()

	// 配置CORS
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "POST", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	router.Use(cors.New(config))

	server := &Server{
		k8sClient: k8sClient,
		router:    router,
		port:      port,
	}

	// 注册路由
	server.registerRoutes()

	return server
}

// Router 返回 gin 路由器
func (s *Server) Router() *gin.Engine {
	return s.router
}

// registerRoutes 注册API路由
func (s *Server) registerRoutes() {
	// 获取所有CRD资源
	s.router.GET("/api/crds", s.GetCRDs)

	// 获取所有命名空间
	s.router.GET("/api/namespaces", s.GetNamespaces)

	// 获取指定CRD的所有对象
	s.router.GET("/api/resources/:group/:version/:resource", s.GetCRDObjects)

	// 获取指定CRD可用的所有命名空间
	s.router.GET("/api/resources/:group/:version/:resource/namespaces", s.GetAvailableNamespaces)

	// 静态文件服务
	s.router.Static("/ui", "./ui/dist")
	s.router.NoRoute(func(c *gin.Context) {
		c.File("./ui/dist/index.html")
	})
}

// GetCRDs 处理获取所有CRD资源的请求
func (s *Server) GetCRDs(c *gin.Context) {
	crds, err := s.k8sClient.GetCRDs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, crds)
}

// GetNamespaces 处理获取所有命名空间的请求
func (s *Server) GetNamespaces(c *gin.Context) {
	namespaces, err := s.k8sClient.GetNamespaces()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, namespaces)
}

// GetCRDObjects 处理获取指定CRD所有对象的请求
func (s *Server) GetCRDObjects(c *gin.Context) {
	group := c.Param("group")
	version := c.Param("version")
	resource := c.Param("resource")
	namespace := c.Query("namespace")

	objects, err := s.k8sClient.ListCRDObjects(group, version, resource, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, objects)
}

// GetAvailableNamespaces 处理获取指定CRD可用的所有命名空间的请求
func (s *Server) GetAvailableNamespaces(c *gin.Context) {
	group := c.Param("group")
	version := c.Param("version")
	resource := c.Param("resource")

	namespaces, err := s.k8sClient.GetAllAvailableNamespaces(group, version, resource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, namespaces)
}

// Start 启动服务器
func (s *Server) Start() error {
	s.httpSrv = &http.Server{
		Addr:    fmt.Sprintf(":%s", s.port),
		Handler: s.router,
	}
	return s.httpSrv.ListenAndServe()
}

// Shutdown 关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}
