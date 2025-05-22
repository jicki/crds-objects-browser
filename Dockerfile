# 构建前端
FROM node:18-alpine AS ui-builder
WORKDIR /app
COPY ui/package*.json ./
RUN npm install
COPY ui/ .
RUN npm run build

# 构建后端
FROM golang:1.24-alpine AS go-builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o browser cmd/server/main.go

# 最终镜像
FROM alpine:3.19
WORKDIR /app
COPY --from=go-builder /app/browser .
COPY --from=ui-builder /app/dist ./ui/dist

# 创建必要的目录
RUN mkdir -p /root/.kube && \
    mkdir -p /app/ui/dist && \
    chown -R nobody:nobody /app && \
    chmod -R 755 /app

USER nobody
EXPOSE 8080
CMD ["./browser"] 