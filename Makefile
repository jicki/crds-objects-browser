.PHONY: all build clean test run docker-build docker-run k8s-deploy ui-build ui-dev

# 变量定义
APP_NAME = crds-objects-browser
APP_VERSION = 0.1.0
DOCKER_IMAGE = $(APP_NAME):$(APP_VERSION)
DOCKER_IMAGE_LATEST = $(APP_NAME):latest

GO_FILES = $(shell find . -name "*.go" -type f -not -path "./vendor/*")
BUILD_DIR = build
BINARY = $(BUILD_DIR)/$(APP_NAME)

# 默认目标
all: clean build

# 构建项目
build: ui-build
	@echo "==> 构建 Go 后端..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BINARY) ./cmd/server

# 清理构建产物
clean:
	@echo "==> 清理构建目录..."
	@rm -rf $(BUILD_DIR)
	@rm -rf ui/dist
	@echo "==> 清理完成"

# 运行测试
test:
	@echo "==> 运行 Go 测试..."
	@go test -v ./...

# 本地运行服务
run: ui-build
	@echo "==> 启动服务器..."
	@go run cmd/server/main.go

# 构建Docker镜像
docker-build: ui-build
	@echo "==> 构建 Docker 镜像..."
	@docker build -t $(DOCKER_IMAGE) .
	@docker tag $(DOCKER_IMAGE) $(DOCKER_IMAGE_LATEST)
	@echo "==> 构建镜像完成: $(DOCKER_IMAGE)"

# 运行Docker容器
docker-run: docker-build
	@echo "==> 启动 Docker 容器..."
	@docker run -p 8080:8080 -v $(HOME)/.kube/config:/root/.kube/config $(DOCKER_IMAGE_LATEST)

# 部署到Kubernetes
k8s-deploy: docker-build
	@echo "==> 部署到 Kubernetes..."
	@kubectl apply -f deploy/kubernetes.yaml
	@echo "==> 部署完成"

# 构建前端
ui-build:
	@echo "==> 构建 Vue 前端..."
	@cd ui && npm install
	@cd ui && npm run build
	@echo "==> 前端构建完成"

# 启动前端开发服务
ui-dev:
	@echo "==> 启动前端开发服务器..."
	@cd ui && npm run serve

# 安装Go依赖
deps:
	@echo "==> 安装 Go 依赖..."
	@go mod tidy
	@echo "==> Go 依赖安装完成"

# 显示帮助
help:
	@echo "可用命令:"
	@echo "  make build          - 构建项目"
	@echo "  make clean          - 清理构建产物"
	@echo "  make test           - 运行测试"
	@echo "  make run            - 本地运行服务"
	@echo "  make docker-build   - 构建Docker镜像"
	@echo "  make docker-run     - 运行Docker容器"
	@echo "  make k8s-deploy     - 部署到Kubernetes"
	@echo "  make ui-build       - 构建前端"
	@echo "  make ui-dev         - 启动前端开发服务"
	@echo "  make deps           - 安装Go依赖" 