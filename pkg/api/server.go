package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jicki/crds-objects-browser/pkg/k8s"
)

// Server 表示API服务器
type Server struct {
	k8sClient  *k8s.Client
	router     *gin.Engine
	httpServer *http.Server
	port       string
	isReady    atomic.Bool
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
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%s", port),
			Handler: router,
		},
	}

	// 注册路由
	server.registerRoutes()

	// 5秒后将 isReady 设置为 true
	go func() {
		time.Sleep(5 * time.Second)
		server.isReady.Store(true)
	}()

	return server
}

// Router 返回 gin 路由器
func (s *Server) Router() *gin.Engine {
	return s.router
}

// registerRoutes 注册API路由
func (s *Server) registerRoutes() {
	// 健康检查端点
	s.router.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 就绪检查端点
	s.router.GET("/readyz", func(c *gin.Context) {
		if s.isReady.Load() {
			c.String(http.StatusOK, "ok")
		} else {
			c.String(http.StatusServiceUnavailable, "not ready")
		}
	})

	// API 路由组
	api := s.router.Group("/api")
	{
		api.GET("/crds", s.GetCRDs)
		api.GET("/namespaces", s.GetNamespaces)
		api.GET("/crds/:group/:version/:resource/namespaces", s.GetAvailableNamespaces)
		api.GET("/crds/:group/:version/:resource/objects", s.GetCRDObjects)
	}

	// 处理根路径
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/ui/index.html")
	})

	// 处理 favicon.ico
	s.router.GET("/favicon.ico", func(c *gin.Context) {
		c.File("./ui/dist/favicon.ico")
	})

	// 静态文件服务
	s.router.Static("/ui", "./ui/dist")

	// 处理前端路由
	s.router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// 如果是以 /ui 开头的路径
		if strings.HasPrefix(path, "/ui/") {
			// 尝试提供静态文件
			filePath := "./ui/dist" + strings.TrimPrefix(path, "/ui")
			if _, err := os.Stat(filePath); err == nil {
				c.File(filePath)
				return
			}
			// 如果文件不存在，返回 index.html（用于支持前端路由）
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
	return s.httpServer.ListenAndServe()
}

// Shutdown 关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
