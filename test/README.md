# 🧪 测试目录结构

本目录包含了 CRDs Objects Browser 项目的所有测试相关文件，包括测试脚本、HTML测试页面和测试报告。

## 📁 目录结构

```
test/
├── README.md                    # 本文件，测试目录说明
├── scripts/                     # 测试脚本
│   └── test-performance-fix.sh  # 性能修复验证脚本
├── html/                        # HTML测试页面
│   ├── test-frontend-fix.html   # 前端修复测试页面
│   ├── debug-frontend.html      # 前端调试页面
│   ├── debug.html               # 系统调试页面
│   └── layout-optimization-test.html  # 布局优化测试页面
└── reports/                     # 测试报告（自动生成）
    └── (测试报告将在此生成)
```

## 📖 文件说明

### 🔧 测试脚本 (scripts/)

#### test-performance-fix.sh
性能修复验证脚本，用于验证性能优化和弃用API修复的效果。

**功能特性：**
- ✅ 服务器健康状态检查
- ⚡ API性能测试（首次请求 vs 缓存命中）
- 🚫 弃用API过滤验证
- 📊 资源对象API性能测试
- 🌐 前端页面加载检查
- 💾 缓存状态监控
- 📈 性能评估和建议

**使用方法：**
```bash
# 确保服务器运行在 localhost:8080
cd test/scripts
chmod +x test-performance-fix.sh
./test-performance-fix.sh
```

### 🌐 HTML测试页面 (html/)

#### test-frontend-fix.html
前端修复测试页面，用于验证前端数据显示修复效果。

**测试内容：**
- 前端数据流测试
- API响应验证
- 数据结构检查
- 错误处理测试

#### debug-frontend.html
前端调试页面，提供详细的前端调试功能。

**调试功能：**
- 实时数据监控
- API调用追踪
- 错误日志显示
- 性能指标统计

#### debug.html
系统级调试页面，提供全面的系统状态监控。

**监控功能：**
- 系统健康检查
- API性能监控
- 缓存状态查看
- 资源使用统计

#### layout-optimization-test.html
布局优化测试页面，用于验证界面优化效果。

**测试功能：**
- 侧边栏自适应宽度验证
- 折叠和拖拽功能测试
- 响应式布局检查
- 性能优化效果验证

## 🚀 快速开始

### 1. 运行性能测试
```bash
# 启动服务器
go run cmd/main.go

# 在新终端运行测试
cd test/scripts
./test-performance-fix.sh
```

### 2. 访问调试页面
```bash
# 系统调试页面
curl http://localhost:8080/debug

# 前端调试页面  
curl http://localhost:8080/debug-frontend

# 前端修复测试页面
curl http://localhost:8080/test-fix

# 布局优化测试页面
curl http://localhost:8080/test-layout
```

### 3. 查看测试结果
测试脚本会输出彩色的测试结果，包括：
- 🟢 成功的测试项
- 🟡 需要注意的项目
- 🔴 失败的测试项

## 📊 性能基准

### API响应时间基准
- **优秀**: < 1秒
- **良好**: 1-3秒  
- **需改进**: > 3秒

### 缓存性能基准
- **优秀**: < 0.5秒
- **良好**: 0.5-1秒
- **需改进**: > 1秒

## 🔍 故障排除

### 常见问题

#### 1. 服务器未运行
```bash
# 错误信息：❌ 服务器未运行，请先启动服务器
# 解决方案：
go run cmd/main.go
```

#### 2. 端口冲突
```bash
# 如果8080端口被占用，可以修改端口
export PORT=8081
go run cmd/main.go
```

#### 3. 权限问题
```bash
# 如果脚本无法执行
chmod +x test/scripts/*.sh
```

## 📝 添加新测试

### 1. 添加测试脚本
```bash
# 在 scripts/ 目录下创建新脚本
touch test/scripts/test-new-feature.sh
chmod +x test/scripts/test-new-feature.sh
```

### 2. 添加HTML测试页面
```bash
# 在 html/ 目录下创建新页面
touch test/html/test-new-feature.html
```

### 3. 更新文档
记得更新本README文件，添加新测试的说明。

## 🔗 相关链接

- [性能修复报告](../docs/troubleshooting/PERFORMANCE_FIX_REPORT.md)
- [前端修复报告](../docs/troubleshooting/FRONTEND_FIX_REPORT.md)
- [性能优化指南](../docs/development/PERFORMANCE_OPTIMIZATION.md)
- [项目主页](../README.md)

## 📈 测试指标

测试脚本会收集以下指标：
- API响应时间
- 缓存命中率
- 资源数量统计
- 错误率统计
- 内存使用情况

这些指标有助于监控系统性能和发现潜在问题。 