package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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
	var (
		kubeconfig string
		port       string
		debug      bool
	)

	// 获取默认的 kubeconfig 路径
	if home := os.Getenv("HOME"); home != "" && os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfig, "Path to kubeconfig file. If not specified, in-cluster config will be used")
	flag.StringVar(&port, "port", "8080", "Server port")
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.Parse()

	// 设置 gin 模式
	if debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建 Kubernetes 客户端
	k8sClient, err := k8s.NewClient(kubeconfig)
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}
	defer k8sClient.Shutdown()

	// 创建 API 服务器
	server := api.NewServer(k8sClient, port)

	// 添加版本信息路由
	server.Router().GET("/version", func(c *gin.Context) {
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
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
