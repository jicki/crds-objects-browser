package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jicki/crds-objects-browser/pkg/api"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	var kubeconfig string
	var port string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	flag.StringVar(&port, "port", "8080", "Port to run the server on")
	flag.Parse()

	// 初始化klog
	klog.InitFlags(nil)
	flag.Set("v", "2") // 设置日志级别

	klog.Info("Starting CRDs Objects Browser with Informer optimization")

	// 创建Kubernetes配置
	config, err := createKubeConfig(kubeconfig)
	if err != nil {
		log.Fatalf("Failed to create kube config: %v", err)
	}

	// 创建API服务器
	server, err := api.NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// 设置优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器
	go func() {
		klog.Infof("Server starting on port %s", port)
		if err := server.Run(":" + port); err != nil {
			klog.Errorf("Server failed to start: %v", err)
			cancel()
		}
	}()

	// 等待关闭信号
	select {
	case sig := <-sigChan:
		klog.Infof("Received signal %v, shutting down gracefully", sig)
	case <-ctx.Done():
		klog.Info("Context cancelled, shutting down")
	}

	// 优雅关闭
	klog.Info("Shutting down server...")
	server.Shutdown()

	// 等待一段时间让所有goroutine完成
	time.Sleep(2 * time.Second)
	klog.Info("Server shutdown complete")
}

// createKubeConfig 创建Kubernetes配置
func createKubeConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		// 使用指定的kubeconfig文件
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %v", err)
		}
		return config, nil
	}

	// 尝试使用集群内配置
	config, err := rest.InClusterConfig()
	if err != nil {
		// 如果不在集群内，尝试使用默认的kubeconfig
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %v", err)
		}

		kubeconfigPath := fmt.Sprintf("%s/.kube/config", home)
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from default kubeconfig: %v", err)
		}
		return config, nil
	}

	return config, nil
}
