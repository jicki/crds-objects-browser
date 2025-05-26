# 📚 文档目录结构

本目录包含了 CRDs Objects Browser 项目的所有文档，按功能和用途进行分类组织。

## 📁 目录结构

```
docs/
├── README.md                    # 本文件，文档目录说明
├── development/                 # 开发相关文档
│   ├── PERFORMANCE_OPTIMIZATION.md      # 性能优化指南
│   ├── INFORMER_IMPLEMENTATION_SUMMARY.md  # Informer实现总结
│   └── INFORMER_OPTIMIZATION.md         # Informer优化文档
├── deployment/                  # 部署相关文档
│   ├── QUICK_START_OPTIMIZATION.md      # 快速启动优化
│   ├── VERSION_GUIDE.md                 # 版本管理指南
│   └── docker-tag-format.md             # Docker标签格式说明
└── troubleshooting/            # 故障排除文档
    ├── PERFORMANCE_FIX_REPORT.md        # 性能修复报告
    └── FRONTEND_FIX_REPORT.md           # 前端修复报告
```

## 📖 文档分类说明

### 🔧 开发文档 (development/)
包含开发过程中的技术文档、架构设计和实现细节：

- **PERFORMANCE_OPTIMIZATION.md**: 详细的性能优化策略和实现方案
- **INFORMER_IMPLEMENTATION_SUMMARY.md**: Kubernetes Informer机制的实现总结
- **INFORMER_OPTIMIZATION.md**: Informer性能优化的具体措施

### 🚀 部署文档 (deployment/)
包含部署、配置和版本管理相关的文档：

- **QUICK_START_OPTIMIZATION.md**: 快速启动和部署优化指南
- **VERSION_GUIDE.md**: 版本管理和发布流程说明
- **docker-tag-format.md**: Docker镜像标签格式规范

### 🔍 故障排除 (troubleshooting/)
包含问题诊断、修复报告和解决方案：

- **PERFORMANCE_FIX_REPORT.md**: 性能问题修复的详细报告
- **FRONTEND_FIX_REPORT.md**: 前端问题修复的详细报告

## 🎯 如何使用这些文档

### 对于开发者
1. 首先阅读 `development/` 目录下的文档了解架构和实现
2. 参考性能优化文档进行代码优化
3. 遇到问题时查看 `troubleshooting/` 目录的修复报告

### 对于运维人员
1. 查看 `deployment/` 目录了解部署和配置
2. 使用快速启动指南进行部署
3. 参考版本指南进行版本管理

### 对于用户
1. 查看主目录的 README.md 了解项目概况
2. 遇到问题时查看故障排除文档
3. 参考部署文档进行安装配置

## 📝 文档维护

- 所有文档使用 Markdown 格式编写
- 文档应保持最新，与代码同步更新
- 新增功能或修复问题时，应同时更新相关文档
- 文档命名使用大写字母和下划线，便于识别

## 🔗 相关链接

- [项目主页](../README.md)
- [更新日志](../CHANGELOG.md)
- [贡献指南](../CONTRIBUTING.md)
- [测试文档](../test/README.md) 