# 构建前端
FROM node:18-alpine AS ui-builder
WORKDIR /app
COPY ui/package*.json ./
RUN npm install
COPY ui/ .
RUN npm run build

# 构建后端
FROM golang:1.23-alpine AS go-builder
WORKDIR /app

# 复制Go模块文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用（使用新的main.go路径）
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o browser cmd/main.go

# 最终镜像
FROM alpine:3.19

# 安装必要的工具
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 复制构建的二进制文件和前端资源
COPY --from=go-builder /app/browser .
COPY --from=ui-builder /app/dist ./ui/dist

# 创建必要的目录和设置权限
RUN mkdir -p /root/.kube && \
    mkdir -p /app/ui/dist && \
    chmod +x /app/browser

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/cache/stats || exit 1

EXPOSE 8080

# 启动应用（以root用户运行以访问kubeconfig）
CMD ["./browser"] 