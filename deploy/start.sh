#!/bin/sh

# 错误处理
set -e

# 日志函数
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# 检查必要的目录
check_directories() {
    log "检查必要的目录..."
    for dir in /var/log/nginx /var/run/nginx /usr/share/nginx/html; do
        if [ ! -d "$dir" ]; then
            log "创建目录: $dir"
            mkdir -p "$dir"
        fi
    done
}

# 启动 nginx
start_nginx() {
    log "启动 nginx..."
    nginx -t && nginx || {
        log "nginx 启动失败"
        exit 1
    }
}

# 启动后端服务
start_backend() {
    log "启动后端服务..."
    cd /app
    /app/crds-objects-browser --port 8080 || {
        log "后端服务启动失败"
        exit 1
    }
}

# 主函数
main() {
    log "开始启动服务..."
    check_directories
    start_nginx
    start_backend
}

# 运行主函数
main 