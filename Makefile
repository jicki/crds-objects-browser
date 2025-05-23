# Kubernetes CRD 对象浏览器 Makefile

# 项目信息
PROJECT_NAME := crds-objects-browser
VERSION := $(shell git describe --tags --always --dirty | sed 's/-g[a-f0-9]*-dirty//' | sed 's/-g[a-f0-9]*//')
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
build-go: ## 构建Go后端
	@echo "🔧 构建Go后端..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "$(LDFLAGS)" \
		-o bin/$(PROJECT_NAME) \
		cmd/main.go

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
		-v ~/.kube/config:/root/.kube/config:ro \
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

.PHONY: informer-stats
informer-stats: ## 查看Informer缓存统计
	@echo "📈 查看Informer缓存统计..."
	curl -s http://localhost:8080/api/cache/stats | jq .

.PHONY: benchmark
benchmark: ## 运行性能基准测试
	@echo "⚡ 运行性能基准测试..."
	@echo "测试直接API调用 vs Informer缓存性能..."
	@for i in {1..10}; do \
		echo "测试轮次 $$i:"; \
		time curl -s http://localhost:8080/api/crds/apps/v1/deployments/objects > /dev/null; \
	done

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

# 发布相关
.PHONY: release
release: clean build docker-build docker-push ## 发布新版本
	@echo "🎉 发布版本 $(VERSION) 完成!"
	@echo "Docker镜像: $(IMAGE_NAME):$(IMAGE_TAG)"

# 信息显示
.PHONY: version
version: ## 显示版本信息
	@echo "项目: $(PROJECT_NAME)"
	@echo "版本: $(VERSION)"
	@echo "提交: $(COMMIT)"
	@echo "构建时间: $(BUILD_TIME)"
	@echo "镜像: $(IMAGE_NAME):$(IMAGE_TAG)"

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