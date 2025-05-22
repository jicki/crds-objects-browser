# Kubernetes CRD 对象浏览器

这是一个用于浏览Kubernetes集群中自定义资源(CRD)的工具。通过简洁的Web界面，您可以查看集群中所有CRD资源的对象实例。

## 功能特性

- 展示集群中所有自定义资源(CRD)类型
- 按资源组和版本组织资源类型
- 支持按命名空间筛选资源对象
- 显示资源对象的状态信息
- 提供资源对象的详细视图

## 技术栈

- 后端: Golang
- 前端: Vue.js 3 + Element Plus
- Kubernetes: client-go

## 项目结构

```
.
├── cmd/              # 应用程序入口点
│   └── server/       # 服务器入口
├── pkg/              # 可复用的包
│   ├── api/          # API服务器
│   ├── k8s/          # Kubernetes客户端
│   └── models/       # 数据模型
├── ui/               # 前端Vue项目
│   ├── public/       # 静态资源
│   └── src/          # 源代码
│       ├── components/  # 组件
│       ├── views/       # 视图
│       ├── store/       # Vuex状态
│       ├── router/      # Vue路由
│       └── assets/      # 资源文件
└── deploy/           # 部署清单
```

## 构建和运行

### 前提条件

- Go 1.18+
- Node.js 14+
- npm 6+
- Kubernetes集群访问权限

### 本地开发

1. 克隆仓库:

```bash
git clone https://github.com/jicki/crds-objects-browser.git
cd crds-objects-browser
```

2. 安装依赖并构建前端:

```bash
cd ui
npm install
npm run build
cd ..
```

3. 启动后端服务:

```bash
# 使用本地kubeconfig
go run cmd/server/main.go --kubeconfig=$HOME/.kube/config
```

4. 访问 http://localhost:8080

### Docker构建

```bash
docker build -t crds-objects-browser:latest .
```

### Kubernetes部署

```bash
kubectl apply -f deploy/kubernetes.yaml
```

## 许可证

MIT 