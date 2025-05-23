# 版本标签管理指南

## 当前版本格式

当前版本: `v0.3.4-1`

## 版本格式说明

- **格式**: `v{主版本}.{次版本}.{修订版本}-{构建号}`
- **示例**: `v0.3.4-1`, `v0.3.5-1`, `v1.0.0-1`
- **构建号**: 用于区分同一版本的不同构建

## 如何创建新版本

### 1. 提交更改
```bash
git add .
git commit -m "feat: 新功能描述"
```

### 2. 创建版本标签
```bash
# 创建新的版本标签
git tag v0.3.5-1

# 或者创建带注释的标签
git tag -a v0.3.5-1 -m "Release v0.3.5-1: 功能描述"
```

### 3. 构建Docker镜像
```bash
make docker-build
```

### 4. 推送标签（可选）
```bash
git push origin v0.3.5-1
```

## 版本号规则

- **主版本号**: 重大架构变更或不兼容的API变更
- **次版本号**: 新功能添加，向后兼容
- **修订版本号**: Bug修复，向后兼容
- **构建号**: 同一版本的不同构建，通常从1开始

## 示例版本演进

```
v0.3.4-1  -> 当前版本
v0.3.4-2  -> 同版本的第二次构建
v0.3.5-1  -> 新功能版本
v0.4.0-1  -> 较大功能更新
v1.0.0-1  -> 正式发布版本
```

## Docker镜像标签

每次构建会创建两个标签：
- `jicki/crds-objects-browser:v0.3.4-1` - 具体版本
- `jicki/crds-objects-browser:latest` - 最新版本

## 自动化版本管理

Makefile中的VERSION变量会自动从Git标签获取版本信息：
```makefile
VERSION := $(shell git describe --tags --always --dirty | sed 's/-g[a-f0-9]*-dirty//' | sed 's/-g[a-f0-9]*$$//')
```

这确保了版本信息的一致性和自动化。

## 健康检查端点

应用提供了标准的Kubernetes健康检查端点：

### 端点说明

- **`/healthz`** - 健康检查端点
  - 用途：基本健康状态检查
  - 返回：服务基本状态信息

- **`/readyz`** - 就绪检查端点
  - 用途：检查服务是否准备好接收流量
  - 返回：服务就绪状态，包括缓存初始化状态

- **`/livez`** - 存活检查端点
  - 用途：检查服务是否仍在运行
  - 返回：服务存活状态

### 使用示例

```bash
# 检查服务健康状态
curl http://localhost:8080/healthz

# 检查服务就绪状态
curl http://localhost:8080/readyz

# 检查服务存活状态
curl http://localhost:8080/livez
```

### Kubernetes配置

在Kubernetes部署中使用健康检查：

```yaml
livenessProbe:
  httpGet:
    path: /livez
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5

startupProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
```

这些端点确保了应用在Kubernetes环境中的可靠性和可观测性。 