FROM node:20-alpine AS ui-builder

WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm install
COPY ui ./
RUN npm run build

FROM golang:1.24-alpine AS backend-builder

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_TIME

WORKDIR /app
COPY VERSION ./
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /app/ui/dist /app/ui/dist
RUN VERSION=$(cat VERSION) && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o crds-objects-browser ./cmd/server

FROM alpine:3.19

# 安装必要的软件包
RUN apk --no-cache add ca-certificates nginx tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    rm -rf /var/cache/apk/*

WORKDIR /app

# 配置 nginx
COPY deploy/nginx.conf /etc/nginx/nginx.conf
COPY deploy/default.conf /etc/nginx/http.d/default.conf

# 复制应用程序文件
COPY --from=backend-builder /app/crds-objects-browser .
COPY --from=ui-builder /app/ui/dist /usr/share/nginx/html
COPY VERSION ./

# 创建必要的目录并设置权限
RUN mkdir -p /var/log/nginx /var/run/nginx && \
    chown -R nginx:nginx /var/log/nginx /var/run/nginx /usr/share/nginx/html

# 复制启动脚本
COPY deploy/start.sh /start.sh
RUN chmod +x /start.sh

ENV KLOG_V=0
ENV KLOG_LOGTOSTDERR=true

EXPOSE 80 8080
ENTRYPOINT ["/start.sh"] 