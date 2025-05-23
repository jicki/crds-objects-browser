# 🚀 Kubernetes CRD 对象浏览器

<div align="center">

![Kubernetes](https://img.shields.io/badge/kubernetes-%23326ce5.svg?style=for-the-badge&logo=kubernetes&logoColor=white)
![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![Vue.js](https://img.shields.io/badge/vuejs-%2335495e.svg?style=for-the-badge&logo=vuedotjs&logoColor=%234FC08D)
![Element Plus](https://img.shields.io/badge/Element%20Plus-409EFF?style=for-the-badge&logo=element&logoColor=white)

一个现代化的Kubernetes集群资源浏览器，专为查看和管理CRD（自定义资源定义）对象而设计

[功能特性](#-功能特性) • [快速开始](#-快速开始) • [部署方式](#-部署方式) • [技术架构](#-技术架构)

</div>

## 📸 界面预览

### 🎯 主要功能
- 📊 **智能资源分类** - 自动区分K8s核心资源和CRD资源
- 🔍 **强大搜索功能** - 支持资源名称、命名空间、状态的实时搜索
- 📱 **响应式设计** - 现代化UI，支持各种屏幕尺寸
- 🏷️ **版本管理** - 清晰显示资源的不同API版本
- 📈 **资源监控** - 实时显示Pod的Request/Limits信息

## ✨ 功能特性

### 🎨 用户界面
- **🌈 现代化设计** - 基于Element Plus的美观界面
- **📱 响应式布局** - 完美适配桌面和移动设备
- **🎯 直观导航** - 树形结构展示资源层次关系
- **🔄 实时更新** - 自动刷新资源状态信息

### 🔍 资源管理
- **📋 全面支持** - 支持所有K8s核心资源和CRD资源
- **🏷️ 版本展示** - 智能显示单版本和多版本资源
- **📊 状态监控** - 实时显示资源运行状态
- **🔍 高级搜索** - 多维度搜索和过滤功能

### 🚀 特色功能
- **📦 Pod资源详情** - 显示容器的CPU/内存Request和Limits
- **🌐 命名空间搜索** - 支持命名空间的快速搜索和过滤
- **📈 对象计数** - 显示每个命名空间的资源对象数量
- **🎯 状态过滤** - 按资源状态（正常/异常/处理中）快速过滤
- **📄 详情查看** - 支持查看完整的YAML配置
- **📋 一键复制** - 快速复制资源配置到剪贴板

### 🛡️ 安全特性
- **🔐 RBAC支持** - 遵循Kubernetes RBAC权限控制
- **🔒 安全访问** - 支持ServiceAccount和kubeconfig认证
- **🛡️ 只读模式** - 仅提供查看功能，确保集群安全

## 🚀 快速开始

### 📋 前提条件

- **Go 1.19+** - 后端开发环境
- **Node.js 16+** - 前端构建环境
- **Kubernetes 1.20+** - 目标集群版本
- **kubectl** - 集群访问工具

### 🏃‍♂️ 本地运行

1. **克隆项目**
```bash
git clone https://github.com/your-org/crds-objects-browser.git
cd crds-objects-browser
```

2. **构建前端**
```bash
cd ui
npm install
npm run build
cd ..
```

3. **启动服务**
```bash
# 使用默认kubeconfig
go run cmd/server/main.go

# 或指定kubeconfig路径
go run cmd/server/main.go --kubeconfig=/path/to/kubeconfig
```

4. **访问应用**
```
🌐 http://localhost:8080
```

### 🐳 Docker 运行

```bash
# 构建镜像
docker build -t crds-browser:latest .

# 运行容器
docker run -d \
  --name crds-browser \
  -p 8080:8080 \
  -v ~/.kube/config:/root/.kube/config:ro \
  crds-browser:latest
```

## 🚀 部署方式

### ☸️ Kubernetes 部署

#### 方式一：使用 kubectl

```bash
# 应用部署清单
kubectl apply -f deploy/kubernetes.yaml

# 检查部署状态
kubectl get pods -l app=crds-browser

# 端口转发访问
kubectl port-forward svc/crds-browser 8080:80
```

### 🔧 配置选项

#### 环境变量

| 变量名 | 描述 | 默认值 |
|--------|------|--------|
| `PORT` | 服务端口 | `8080` |
| `KUBECONFIG` | kubeconfig文件路径 | `~/.kube/config` |
| `LOG_LEVEL` | 日志级别 | `info` |
| `REFRESH_INTERVAL` | 资源刷新间隔(秒) | `30` |

#### 命令行参数

```bash
go run cmd/server/main.go \
  --port=8080 \
  --kubeconfig=/path/to/config \
  --log-level=debug \
  --refresh-interval=60
```

## 🏗️ 技术架构

### 📊 架构图

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Vue.js 前端   │────│   Go 后端服务   │────│ Kubernetes API  │
│                 │    │                 │    │                 │
│ • Element Plus  │    │ • Gin Framework │    │ • client-go     │
│ • Vuex Store    │    │ • REST API      │    │ • CRD Discovery │
│ • Vue Router    │    │ • WebSocket     │    │ • Resource List │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 🛠️ 技术栈

#### 后端技术
- **🔷 Go 1.19+** - 高性能后端语言
- **🌐 Gin** - 轻量级Web框架
- **☸️ client-go** - Kubernetes官方客户端
- **📡 WebSocket** - 实时数据推送
- **🔍 Discovery API** - 动态资源发现

#### 前端技术
- **💚 Vue.js 3** - 现代前端框架
- **🎨 Element Plus** - 企业级UI组件库
- **🗃️ Vuex** - 状态管理
- **🛣️ Vue Router** - 路由管理
- **📦 Webpack** - 模块打包工具

#### 基础设施
- **🐳 Docker** - 容器化部署
- **☸️ Kubernetes** - 容器编排平台
- **📊 Prometheus** - 监控指标收集
- **📈 Grafana** - 可视化监控面板

### 📁 项目结构

```
crds-objects-browser/
├── 📁 cmd/                    # 应用程序入口
│   └── 📁 server/            # 服务器主程序
│       └── 📄 main.go        # 程序入口点
├── 📁 pkg/                   # 核心业务逻辑
│   ├── 📁 api/               # REST API服务
│   │   ├── 📄 server.go      # HTTP服务器
│   │   └── 📄 handlers.go    # 请求处理器
│   ├── 📁 k8s/               # Kubernetes客户端
│   │   ├── 📄 client.go      # K8s客户端封装
│   │   └── 📄 discovery.go   # 资源发现逻辑
│   └── 📁 models/            # 数据模型定义
│       └── 📄 types.go       # 类型定义
├── 📁 ui/                    # 前端Vue项目
│   ├── 📁 public/            # 静态资源
│   ├── 📁 src/               # 源代码
│   │   ├── 📁 components/    # 可复用组件
│   │   ├── 📁 views/         # 页面视图
│   │   │   ├── 📄 ResourcesLayout.vue  # 资源布局
│   │   │   └── 📄 ResourceDetail.vue   # 资源详情
│   │   ├── 📁 store/         # Vuex状态管理
│   │   ├── 📁 router/        # 路由配置
│   │   └── 📁 assets/        # 静态资源
│   ├── 📄 package.json       # 依赖配置
│   └── 📄 vue.config.js      # Vue配置
├── 📁 deploy/                # 部署配置
│   ├── 📄 kubernetes.yaml    # K8s部署清单
│   ├── 📄 docker-compose.yml # Docker Compose
│   └── 📁 helm/              # Helm Chart
├── 📄 Dockerfile             # Docker构建文件
├── 📄 Makefile              # 构建脚本
├── 📄 go.mod                # Go模块定义
└── 📄 README.md             # 项目文档
```

## 🎯 使用指南

### 🔍 浏览资源

1. **选择资源类型** - 在左侧树形菜单中选择要查看的资源
2. **切换命名空间** - 使用顶部的命名空间选择器
3. **搜索过滤** - 使用搜索框快速定位资源
4. **查看详情** - 点击"详情"按钮查看完整配置

### 📊 监控功能

- **状态过滤** - 按资源状态筛选（正常/异常/处理中）
- **实时更新** - 资源状态自动刷新
- **资源统计** - 显示各类资源的数量统计

### 🔧 高级功能

- **Pod资源监控** - 查看容器的CPU/内存配置
- **版本管理** - 支持多版本API资源
- **批量操作** - 支持批量查看和导出

## 🤝 贡献指南

我们欢迎所有形式的贡献！请查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解详细信息。

### 🐛 问题报告

如果您发现了bug或有功能建议，请：

1. 检查 [Issues](https://github.com/your-org/crds-browser/issues) 是否已存在相关问题
2. 创建新的Issue，详细描述问题或建议
3. 提供复现步骤和环境信息

### 💡 功能请求

我们很乐意听到您的想法！请通过Issue告诉我们您希望看到的新功能。

## 📄 许可证

本项目采用 [MIT 许可证](LICENSE) - 详情请查看LICENSE文件。

## 🙏 致谢

感谢以下开源项目的支持：

- [Kubernetes](https://kubernetes.io/) - 容器编排平台
- [Vue.js](https://vuejs.org/) - 渐进式JavaScript框架
- [Element Plus](https://element-plus.org/) - Vue 3组件库
- [Gin](https://gin-gonic.com/) - Go Web框架
- [client-go](https://github.com/kubernetes/client-go) - Kubernetes Go客户端

---

<div align="center">

**⭐ 如果这个项目对您有帮助，请给我们一个Star！**

Made with ❤️ by [Your Team](https://github.com/your-org)

</div> 