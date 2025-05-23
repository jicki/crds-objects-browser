# Docker 标签格式优化

## 📋 概述

本文档说明了如何修改Docker构建标签格式，使其更加简洁和易读。

## 🔧 修改内容

### 修改前
```bash
VERSION := $(shell git describe --tags --always --dirty)
# 生成的标签: v0.2.7-1-ge48d77b-dirty
```

### 修改后
```bash
VERSION := $(shell git describe --tags --always | sed 's/-g[a-f0-9]*.*$$//' | sed 's/-dirty$$//')
# 生成的标签: v0.2.7-1
```

## 🎯 效果对比

| 修改前 | 修改后 |
|--------|--------|
| `v0.2.7-1-ge48d77b-dirty` | `v0.2.7-1` |
| `v0.2.7-1-ge48d77b` | `v0.2.7-1` |
| `v0.2.7` | `v0.2.7` |

## 📝 标签格式说明

新的标签格式包含以下信息：
- **主版本号**: `v0.2.7` - 最近的Git标签
- **提交次数**: `-1` - 自最近标签以来的提交次数（如果有）

去除的信息：
- **Git哈希**: `-ge48d77b` - Git提交的短哈希
- **Dirty标记**: `-dirty` - 工作目录有未提交的更改

## 🚀 使用方法

### 1. 查看当前版本
```bash
make info
```

### 2. 构建Docker镜像
```bash
make docker-build
```

### 3. 查看构建的镜像
```bash
docker images | grep crds-browser
```

### 4. 自定义标签
如果需要使用自定义标签，可以设置环境变量：
```bash
DOCKER_TAG=custom-tag make docker-build
```

## 🔍 技术细节

### sed命令解释
```bash
git describe --tags --always | sed 's/-g[a-f0-9]*.*$$//' | sed 's/-dirty$$//'
```

1. `git describe --tags --always`: 获取Git描述信息
2. `sed 's/-g[a-f0-9]*.*$$//'`: 移除Git哈希及其后的所有内容
3. `sed 's/-dirty$$//'`: 移除末尾的"-dirty"标记

### 正则表达式说明
- `-g[a-f0-9]*.*$$`: 匹配"-g"开头，后跟十六进制字符，直到行尾
- `-dirty$$`: 匹配行尾的"-dirty"

## 📊 优势

1. **简洁性**: 标签更短，更易读
2. **一致性**: 无论工作目录是否干净，标签格式一致
3. **可读性**: 专注于版本信息，去除技术细节
4. **兼容性**: 保持语义版本控制的核心信息

## 🔄 回滚方法

如果需要恢复到原来的格式，修改Makefile中的VERSION定义：
```bash
VERSION := $(shell git describe --tags --always --dirty)
```

## 📚 相关命令

```bash
# 查看构建信息
make info

# 构建Docker镜像
make docker-build

# 推送Docker镜像
make docker-push

# 运行Docker容器
make docker-run

# 停止Docker容器
make docker-stop

# 清理Docker镜像
make clean
```

## 🎉 总结

通过这个简单的修改，Docker标签从冗长的`v0.2.7-1-ge48d77b-dirty`变成了简洁的`v0.2.7-1`，提高了可读性和一致性，同时保留了版本控制的核心信息。 