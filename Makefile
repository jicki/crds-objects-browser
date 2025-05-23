# Kubernetes CRD 对象浏览器 Makefile

# 变量定义
APP_NAME := crds-browser
# 获取简洁的版本号，只保留主版本号和提交次数
VERSION := $(shell git describe --tags --always | sed 's/-g[a-f0-9]*.*$$//' | sed 's/-dirty$$//')
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION := $(shell go version | awk '{print $$3}')
GIT_COMMIT := $(shell git rev-parse HEAD)

# Docker 相关
DOCKER_REGISTRY ?= your-registry.com
DOCKER_IMAGE := $(DOCKER_REGISTRY)/$(APP_NAME)
DOCKER_TAG ?= $(VERSION)

# 构建标志
LDFLAGS := -X main.Version=$(VERSION) \
           -X main.BuildTime=$(BUILD_TIME) \
           -X main.GoVersion=$(GO_VERSION) \
           -X main.GitCommit=$(GIT_COMMIT)

# 默认目标
.DEFAULT_GOAL := help

# 帮助信息
.PHONY: help
help: ## 显示帮助信息
	@echo "Kubernetes CRD 对象浏览器 - 构建工具"
	@echo ""
	@echo "可用命令:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# 开发相关
.PHONY: dev
dev: ## 启动开发环境
	@echo "🚀 启动开发环境..."
	@make ui-dev &
	@make server-dev

.PHONY: server-dev
server-dev: ## 启动后端开发服务器
	@echo "🔧 启动后端服务器..."
	@go run cmd/server/main.go --log-level=debug

.PHONY: ui-dev
ui-dev: ## 启动前端开发服务器
	@echo "🎨 启动前端开发服务器..."
	@cd ui && npm run serve

# 构建相关
.PHONY: build
build: ui-build server-build ## 构建完整应用

.PHONY: server-build
server-build: ## 构建后端服务器
	@echo "🔨 构建后端服务器..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags "$(LDFLAGS)" \
		-o bin/$(APP_NAME) \
		cmd/server/main.go

.PHONY: ui-build
ui-build: ui-deps ## 构建前端资源
	@echo "🎨 构建前端资源..."
	@cd ui && npm run build

.PHONY: ui-deps
ui-deps: ## 安装前端依赖
	@echo "📦 安装前端依赖..."
	@cd ui && npm install

# 测试相关
.PHONY: test
test: test-go test-ui ## 运行所有测试

.PHONY: test-go
test-go: ## 运行Go测试
	@echo "🧪 运行Go测试..."
	@go test -v -race -coverprofile=coverage.out ./...

.PHONY: test-ui
test-ui: ## 运行前端测试
	@echo "🧪 运行前端测试..."
	@cd ui && npm run test:unit

.PHONY: test-e2e
test-e2e: ## 运行端到端测试
	@echo "🧪 运行端到端测试..."
	@cd ui && npm run test:e2e

.PHONY: coverage
coverage: test-go ## 生成测试覆盖率报告
	@echo "📊 生成覆盖率报告..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

# 代码质量
.PHONY: lint
lint: lint-go lint-ui ## 运行所有代码检查

.PHONY: lint-go
lint-go: ## 运行Go代码检查
	@echo "🔍 检查Go代码..."
	@golangci-lint run ./...

.PHONY: lint-ui
lint-ui: ## 运行前端代码检查
	@echo "🔍 检查前端代码..."
	@cd ui && npm run lint

.PHONY: format
format: format-go format-ui ## 格式化所有代码

.PHONY: format-go
format-go: ## 格式化Go代码
	@echo "✨ 格式化Go代码..."
	@go fmt ./...
	@goimports -w .

.PHONY: format-ui
format-ui: ## 格式化前端代码
	@echo "✨ 格式化前端代码..."
	@cd ui && npm run format

.PHONY: check
check: format lint test ## 运行所有检查（格式化、代码检查、测试）

# Docker相关
.PHONY: docker-build
docker-build: ## 构建Docker镜像
	@echo "🐳 构建Docker镜像..."
	@docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		.

.PHONY: docker-push
docker-push: ## 推送Docker镜像
	@echo "📤 推送Docker镜像..."
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@docker push $(DOCKER_IMAGE):latest

.PHONY: docker-run
docker-run: ## 运行Docker容器
	@echo "🏃 运行Docker容器..."
	@docker run -d \
		--name $(APP_NAME) \
		-p 8080:8080 \
		-v ~/.kube/config:/root/.kube/config:ro \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-stop
docker-stop: ## 停止Docker容器
	@echo "🛑 停止Docker容器..."
	@docker stop $(APP_NAME) || true
	@docker rm $(APP_NAME) || true

# Kubernetes部署
.PHONY: k8s-deploy
k8s-deploy: ## 部署到Kubernetes
	@echo "☸️ 部署到Kubernetes..."
	@kubectl apply -f deploy/kubernetes.yaml

.PHONY: k8s-delete
k8s-delete: ## 从Kubernetes删除
	@echo "🗑️ 从Kubernetes删除..."
	@kubectl delete -f deploy/kubernetes.yaml

.PHONY: k8s-logs
k8s-logs: ## 查看Kubernetes日志
	@echo "📋 查看应用日志..."
	@kubectl logs -l app=$(APP_NAME) -f

.PHONY: k8s-port-forward
k8s-port-forward: ## 端口转发
	@echo "🔗 设置端口转发..."
	@kubectl port-forward svc/$(APP_NAME) 8080:80

# 清理
.PHONY: clean
clean: ## 清理构建文件
	@echo "🧹 清理构建文件..."
	@rm -rf bin/
	@rm -rf ui/dist/
	@rm -rf ui/node_modules/
	@rm -f coverage.out coverage.html
	@docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) 2>/dev/null || true
	@docker rmi $(DOCKER_IMAGE):latest 2>/dev/null || true

# 依赖管理
.PHONY: deps
deps: deps-go ui-deps ## 安装所有依赖

.PHONY: deps-go
deps-go: ## 安装Go依赖
	@echo "📦 安装Go依赖..."
	@go mod download
	@go mod tidy

.PHONY: deps-update
deps-update: ## 更新依赖
	@echo "🔄 更新Go依赖..."
	@go get -u ./...
	@go mod tidy
	@echo "🔄 更新前端依赖..."
	@cd ui && npm update

# 工具安装
.PHONY: tools
tools: ## 安装开发工具
	@echo "🔧 安装开发工具..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest

# 发布相关
.PHONY: release
release: check docker-build docker-push ## 发布新版本

.PHONY: tag
tag: ## 创建Git标签
	@echo "🏷️ 创建Git标签..."
	@git tag -a $(VERSION) -m "Release $(VERSION)"
	@git push origin $(VERSION)

# 信息显示
.PHONY: info
info: ## 显示构建信息
	@echo "📋 构建信息:"
	@echo "  应用名称: $(APP_NAME)"
	@echo "  版本: $(VERSION)"
	@echo "  构建时间: $(BUILD_TIME)"
	@echo "  Go版本: $(GO_VERSION)"
	@echo "  Git提交: $(GIT_COMMIT)"
	@echo "  Docker镜像: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# 监控相关
.PHONY: logs
logs: ## 查看应用日志
	@echo "📋 查看应用日志..."
	@docker logs -f $(APP_NAME) 2>/dev/null || echo "容器未运行"

.PHONY: status
status: ## 检查应用状态
	@echo "📊 检查应用状态..."
	@curl -s http://localhost:8080/health || echo "应用未运行"

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

.PHONY: all
all: clean deps check build docker-build ## 完整构建流程 