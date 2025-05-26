# Kubernetes CRD 对象浏览器 Makefile

# 项目信息
PROJECT_NAME := crds-objects-browser
VERSION := $(shell git describe --tags --always --dirty | sed 's/-g[a-f0-9]*-dirty//' | sed 's/-g[a-f0-9]*$$//')
COMMIT := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Docker相关
DOCKER_REGISTRY ?= docker.io
DOCKER_NAMESPACE ?= jicki
IMAGE_NAME := $(DOCKER_REGISTRY)/$(DOCKER_NAMESPACE)/$(PROJECT_NAME)
IMAGE_TAG := $(VERSION)

# Go相关
GO_VERSION := 1.21
GOOS ?= linux
GOARCH ?= amd64
CGO_ENABLED ?= 0

# 构建标志
LDFLAGS := -w -s \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildTime=$(BUILD_TIME)

# 默认目标
.DEFAULT_GOAL := help

# 帮助信息
.PHONY: help
help: ## 显示帮助信息
	@echo "CRDs Objects Browser with Informer Optimization"
	@echo ""
	@echo "可用命令:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# 开发相关
.PHONY: dev
dev: ## 启动开发环境
	@echo "🚀 启动开发环境..."
	@echo "📦 构建前端..."
	cd ui && npm install && npm run build
	@echo "🔧 启动后端服务器..."
	go run cmd/main.go -v=4

.PHONY: dev-fast
dev-fast: ## 快速启动开发环境（跳过前端构建）
	@echo "⚡ 快速启动开发环境..."
	go run cmd/main.go -v=4

.PHONY: dev-debug
dev-debug: ## 启动调试模式（详细日志）
	@echo "🐛 启动调试模式..."
	go run cmd/main.go -v=6 -logtostderr=true

.PHONY: dev-ui
dev-ui: ## 启动前端开发服务器
	@echo "🎨 启动前端开发服务器..."
	cd ui && npm run dev

# 构建相关
.PHONY: build
build: build-ui build-go ## 构建完整项目

.PHONY: build-ui
build-ui: ## 构建前端
	@echo "📦 构建前端..."
	cd ui && npm install && npm run build

.PHONY: build-go
build-go: ## 构建Go后端（Linux版本）
	@echo "🔧 构建Go后端..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "$(LDFLAGS)" \
		-o bin/$(PROJECT_NAME) \
		cmd/main.go

.PHONY: build-local
build-local: build-ui ## 构建本地版本（自动检测操作系统）
	@echo "🔧 构建本地版本..."
	CGO_ENABLED=0 go build \
		-ldflags "$(LDFLAGS)" \
		-o bin/$(PROJECT_NAME)-local \
		cmd/main.go

.PHONY: build-optimized
build-optimized: build-ui ## 构建性能优化版本
	@echo "⚡ 构建性能优化版本..."
	CGO_ENABLED=0 go build \
		-ldflags "$(LDFLAGS) -X main.optimized=true" \
		-gcflags="-m -l" \
		-o bin/$(PROJECT_NAME)-optimized \
		cmd/main.go

.PHONY: run
run: build-local ## 构建并运行本地版本
	@echo "🚀 启动应用..."
	./bin/$(PROJECT_NAME)-local -port=8080

# 测试相关
.PHONY: test
test: ## 运行测试
	@echo "🧪 运行测试..."
	go test -v ./...

.PHONY: test-coverage
test-coverage: ## 运行测试并生成覆盖率报告
	@echo "📊 生成测试覆盖率报告..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

.PHONY: test-race
test-race: ## 运行竞态检测测试
	@echo "🏁 运行竞态检测测试..."
	go test -race -v ./...

.PHONY: test-bench
test-bench: ## 运行基准测试
	@echo "⚡ 运行基准测试..."
	go test -bench=. -benchmem ./...

# 代码质量
.PHONY: lint
lint: ## 运行代码检查
	@echo "🔍 运行代码检查..."
	golangci-lint run

.PHONY: fmt
fmt: ## 格式化代码
	@echo "✨ 格式化代码..."
	go fmt ./...
	goimports -w .

# 清理
.PHONY: clean
clean: ## 清理构建文件
	@echo "🧹 清理构建文件..."
	rm -rf bin/
	rm -rf ui/dist/
	rm -f coverage.out coverage.html
	rm -f cpu.prof mem.prof trace.out

# Docker相关
.PHONY: docker-build
docker-build: ## 构建Docker镜像
	@echo "🐳 构建Docker镜像..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(IMAGE_NAME):$(IMAGE_TAG) \
		-t $(IMAGE_NAME):latest \
		.

.PHONY: docker-push
docker-push: ## 推送Docker镜像
	@echo "📤 推送Docker镜像..."
	docker push $(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(IMAGE_NAME):latest

.PHONY: docker-run
docker-run: ## 运行Docker容器
	@echo "🏃 运行Docker容器..."
	docker run -d \
		--name $(PROJECT_NAME) \
		-p 8080:8080 \
		-v $(HOME)/.kube/config:/root/.kube/config:ro \
		$(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: docker-stop
docker-stop: ## 停止Docker容器
	@echo "⏹️ 停止Docker容器..."
	docker stop $(PROJECT_NAME) || true
	docker rm $(PROJECT_NAME) || true

# Kubernetes部署
.PHONY: k8s-deploy
k8s-deploy: ## 部署到Kubernetes
	@echo "☸️ 部署到Kubernetes..."
	@echo "使用镜像: $(IMAGE_NAME):$(IMAGE_TAG)"
	sed 's|IMAGE_PLACEHOLDER|$(IMAGE_NAME):$(IMAGE_TAG)|g' k8s/deployment.yaml | kubectl apply -f -
	kubectl apply -f k8s/

.PHONY: k8s-undeploy
k8s-undeploy: ## 从Kubernetes卸载
	@echo "🗑️ 从Kubernetes卸载..."
	kubectl delete -f k8s/ || true

.PHONY: k8s-logs
k8s-logs: ## 查看Kubernetes日志
	@echo "📋 查看应用日志..."
	kubectl logs -f deployment/crds-objects-browser -n crds-browser

.PHONY: k8s-status
k8s-status: ## 查看Kubernetes状态
	@echo "📊 查看应用状态..."
	kubectl get all -n crds-browser

# 性能监控和优化
.PHONY: informer-stats
informer-stats: ## 查看Informer缓存统计
	@echo "📈 查看Informer缓存统计..."
	@curl -s http://localhost:8080/api/cache/stats | jq . || echo "请确保应用正在运行"

.PHONY: cache-status
cache-status: ## 查看缓存状态
	@echo "💾 查看缓存状态..."
	@curl -s http://localhost:8080/api/cache/status | jq . || echo "请确保应用正在运行"

.PHONY: performance-stats
performance-stats: ## 查看性能统计
	@echo "⚡ 查看性能统计..."
	@curl -s http://localhost:8080/api/performance/stats | jq . || echo "请确保应用正在运行"

.PHONY: monitor
monitor: ## 实时监控应用状态
	@echo "📊 实时监控应用状态..."
	@while true; do \
		clear; \
		echo "=== CRDs Objects Browser 实时监控 ==="; \
		echo "时间: $$(date)"; \
		echo ""; \
		echo "缓存状态:"; \
		curl -s http://localhost:8080/api/cache/status | jq . 2>/dev/null || echo "无法连接到应用"; \
		echo ""; \
		echo "性能统计:"; \
		curl -s http://localhost:8080/api/performance/stats | jq . 2>/dev/null || echo "无法获取性能统计"; \
		echo ""; \
		echo "按 Ctrl+C 退出监控"; \
		sleep 5; \
	done

.PHONY: benchmark
benchmark: ## 运行性能基准测试
	@echo "⚡ 运行性能基准测试..."
	@echo "测试直接API调用 vs Informer缓存性能..."
	@echo "正在测试 deployments 资源..."
	@for i in {1..10}; do \
		echo "测试轮次 $$i:"; \
		time curl -s http://localhost:8080/api/crds/apps/v1/deployments/objects > /dev/null 2>&1; \
	done
	@echo ""
	@echo "正在测试快速接口..."
	@for i in {1..10}; do \
		echo "快速接口测试轮次 $$i:"; \
		time curl -s http://localhost:8080/api/crds/apps/v1/deployments/objects/fast > /dev/null 2>&1; \
	done

.PHONY: load-test
load-test: ## 运行负载测试
	@echo "🔥 运行负载测试..."
	@echo "并发请求测试..."
	@for i in {1..20}; do \
		curl -s http://localhost:8080/api/crds/apps/v1/deployments/objects > /dev/null & \
	done; \
	wait; \
	echo "负载测试完成"

.PHONY: profile-cpu
profile-cpu: ## CPU性能分析
	@echo "🔍 启动CPU性能分析..."
	@echo "访问 http://localhost:8080/debug/pprof/profile?seconds=30 进行30秒CPU分析"
	@echo "或运行: go tool pprof http://localhost:8080/debug/pprof/profile"

.PHONY: profile-mem
profile-mem: ## 内存性能分析
	@echo "🧠 启动内存性能分析..."
	@echo "访问 http://localhost:8080/debug/pprof/heap 进行内存分析"
	@echo "或运行: go tool pprof http://localhost:8080/debug/pprof/heap"

.PHONY: profile-trace
profile-trace: ## 执行跟踪分析
	@echo "🔬 启动执行跟踪分析..."
	@echo "访问 http://localhost:8080/debug/pprof/trace?seconds=10 进行10秒跟踪"
	@echo "或运行: go tool trace trace.out"

# 健康检查
.PHONY: health-check
health-check: ## 健康检查
	@echo "🏥 执行健康检查..."
	@echo "健康状态:"
	@curl -s http://localhost:8080/healthz || echo "健康检查失败"
	@echo ""
	@echo "就绪状态:"
	@curl -s http://localhost:8080/readyz || echo "就绪检查失败"
	@echo ""
	@echo "存活状态:"
	@curl -s http://localhost:8080/livez || echo "存活检查失败"

.PHONY: stress-test
stress-test: ## 压力测试
	@echo "💪 运行压力测试..."
	@echo "启动100个并发请求..."
	@for i in {1..100}; do \
		(curl -s http://localhost:8080/api/crds > /dev/null &); \
	done; \
	wait; \
	echo "压力测试完成，检查应用状态..."
	@make health-check

# 依赖管理
.PHONY: deps
deps: ## 安装依赖
	@echo "📦 安装Go依赖..."
	go mod tidy
	go mod download
	@echo "📦 安装前端依赖..."
	cd ui && npm install

.PHONY: deps-update
deps-update: ## 更新依赖
	@echo "🔄 更新Go依赖..."
	go get -u ./...
	go mod tidy
	@echo "🔄 更新前端依赖..."
	cd ui && npm update

.PHONY: deps-check
deps-check: ## 检查依赖安全性
	@echo "🔒 检查依赖安全性..."
	go list -json -m all | nancy sleuth
	cd ui && npm audit

# 调试和故障排除
.PHONY: debug-informers
debug-informers: ## 调试Informer状态
	@echo "🐛 调试Informer状态..."
	@echo "活跃的Informers:"
	@curl -s http://localhost:8080/api/cache/stats | jq '.resourceStats | keys[]' || echo "无法获取Informer状态"
	@echo ""
	@echo "同步状态:"
	@curl -s http://localhost:8080/api/cache/stats | jq '.syncStatus' || echo "无法获取同步状态"

.PHONY: debug-memory
debug-memory: ## 调试内存使用
	@echo "🧠 调试内存使用..."
	@curl -s http://localhost:8080/debug/pprof/heap > mem.prof
	@go tool pprof -text mem.prof | head -20
	@echo "详细内存分析已保存到 mem.prof"

.PHONY: debug-goroutines
debug-goroutines: ## 调试Goroutine状态
	@echo "🔄 调试Goroutine状态..."
	@curl -s http://localhost:8080/debug/pprof/goroutine?debug=1 | head -50

.PHONY: logs-tail
logs-tail: ## 实时查看日志
	@echo "📋 实时查看应用日志..."
	@tail -f server.log 2>/dev/null || echo "日志文件不存在，请确保应用正在运行"

# 版本信息
.PHONY: version
version: ## 显示版本信息
	@echo "项目: $(PROJECT_NAME)"
	@echo "版本: $(VERSION)"
	@echo "提交: $(COMMIT)"
	@echo "构建时间: $(BUILD_TIME)"
	@echo "Go版本: $(shell go version)"

# 完整的CI/CD流程
.PHONY: ci
ci: deps lint test build ## 完整的CI流程

.PHONY: cd
cd: ci docker-build docker-push ## 完整的CD流程

# 发布相关
.PHONY: release
release: clean build docker-build docker-push ## 发布新版本
	@echo "🎉 发布版本 $(VERSION) 完成!"
	@echo "Docker镜像: $(IMAGE_NAME):$(IMAGE_TAG)"

# 开发辅助
.PHONY: install-hooks
install-hooks: ## 安装Git钩子
	@echo "🪝 安装Git钩子..."
	@cp scripts/pre-commit .git/hooks/
	@chmod +x .git/hooks/pre-commit

.PHONY: generate
generate: ## 生成代码
	@echo "🔄 生成代码..."
	@go generate ./...

# 文档相关
.PHONY: docs
docs: ## 生成文档
	@echo "📚 生成文档..."
	@godoc -http=:6060 &
	@echo "文档服务器启动在 http://localhost:6060"

.PHONY: docs-ui
docs-ui: ## 生成前端文档
	@echo "📚 生成前端文档..."
	@cd ui && npm run docs

# 安全检查
.PHONY: security
security: ## 运行安全检查
	@echo "🔒 运行安全检查..."
	@gosec ./...
	@cd ui && npm audit

# 性能测试
.PHONY: bench
bench: ## 运行性能测试
	@echo "⚡ 运行性能测试..."
	@go test -bench=. -benchmem ./...

# 数据库相关（如果需要）
.PHONY: migrate
migrate: ## 运行数据库迁移
	@echo "🗄️ 运行数据库迁移..."
	@# 添加迁移命令

# 备份相关
.PHONY: backup
backup: ## 备份配置
	@echo "💾 备份配置..."
	@tar -czf backup-$(shell date +%Y%m%d-%H%M%S).tar.gz deploy/ ui/src/ pkg/

# 开发快捷命令
.PHONY: quick-start
quick-start: deps build-ui ## 快速启动（适合首次运行）
	@echo "🚀 快速启动应用..."
	go run cmd/main.go

.PHONY: restart
restart: build-ui ## 重启应用
	@echo "🔄 重启应用..."
	pkill -f "$(PROJECT_NAME)" || true
	sleep 1
	go run cmd/main.go &

# 监控和调试
.PHONY: monitor
monitor: ## 监控应用状态
	@echo "📊 监控应用状态..."
	@echo "应用健康状态:"
	@curl -s http://localhost:8080/healthz || echo "应用未运行"
	@echo ""
	@echo "Informer缓存统计:"
	@curl -s http://localhost:8080/api/cache/stats | jq . || echo "无法获取缓存统计"

.PHONY: debug
debug: ## 启动调试模式
	@echo "🐛 启动调试模式..."
	go run cmd/main.go -v=6 -kubeconfig=~/.kube/config

# 安装开发工具
.PHONY: install-tools
install-tools: ## 安装开发工具
	@echo "🛠️ 安装开发工具..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest