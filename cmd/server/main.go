package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jicki/crds-objects-browser/pkg/api"
	"github.com/jicki/crds-objects-browser/pkg/k8s"
)

var (
	Version   = "dev"
	GitCommit = "none"
	BuildTime = "unknown"
)

type VersionInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildTime string `json:"buildTime"`
	GoVersion string `json:"goVersion"`
	StartTime string `json:"startTime"`
}

func main() {
	startTime := time.Now().Format(time.RFC3339)

	// 命令行参数
	var kubeconfig string
	var port string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file. If not specified, in-cluster config will be used")
	flag.StringVar(&port, "port", "8080", "Server port")
	flag.Parse()

	// 初始化 Kubernetes 客户端
	k8sClient, err := k8s.NewClient(kubeconfig)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// 创建 API 服务器
	server := api.NewServer(k8sClient, port)

	// 设置路由
	r := gin.Default()

	// 配置 CORS
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	r.Use(cors.New(config))

	// 版本信息 API
	r.GET("/version", func(c *gin.Context) {
		c.JSON(200, VersionInfo{
			Version:   Version,
			GitCommit: GitCommit,
			BuildTime: BuildTime,
			GoVersion: "go1.24",
			StartTime: startTime,
		})
	})

	// 启动服务器
	go func() {
		log.Printf("Starting server on port %s", port)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("Shutting down server...")
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
