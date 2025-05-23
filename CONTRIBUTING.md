# 🤝 贡献指南

感谢您对 Kubernetes CRD 对象浏览器项目的关注！我们欢迎所有形式的贡献，包括但不限于：

- 🐛 Bug 报告
- 💡 功能建议
- 📝 文档改进
- 🔧 代码贡献
- 🧪 测试用例

## 📋 开始之前

在开始贡献之前，请确保您已经：

1. ⭐ 给项目点了 Star
2. 🍴 Fork 了项目到您的 GitHub 账户
3. 📖 阅读了项目的 README.md
4. 🔍 检查了现有的 Issues 和 Pull Requests

## 🐛 报告 Bug

如果您发现了 bug，请按照以下步骤报告：

### 1. 检查现有 Issues
首先搜索 [现有 Issues](https://github.com/your-org/crds-browser/issues) 确认问题是否已被报告。

### 2. 创建 Bug 报告
如果问题尚未被报告，请创建新的 Issue 并包含以下信息：

```markdown
## Bug 描述
简洁明了地描述遇到的问题

## 复现步骤
1. 进入 '...'
2. 点击 '....'
3. 滚动到 '....'
4. 看到错误

## 期望行为
描述您期望发生的行为

## 实际行为
描述实际发生的行为

## 环境信息
- OS: [例如 macOS 12.0]
- 浏览器: [例如 Chrome 95.0]
- Kubernetes 版本: [例如 v1.22.0]
- 项目版本: [例如 v1.0.0]

## 截图
如果适用，请添加截图来帮助解释问题

## 额外信息
添加任何其他有关问题的信息
```

## 💡 功能建议

我们很乐意听到您的想法！请按照以下格式提交功能建议：

```markdown
## 功能描述
简洁明了地描述您想要的功能

## 问题背景
描述这个功能要解决的问题

## 解决方案
描述您希望的解决方案

## 替代方案
描述您考虑过的其他替代方案

## 额外信息
添加任何其他有关功能请求的信息
```

## 🔧 代码贡献

### 开发环境设置

1. **Fork 项目**
```bash
# 克隆您的 fork
git clone https://github.com/YOUR_USERNAME/crds-browser.git
cd crds-browser

# 添加上游仓库
git remote add upstream https://github.com/your-org/crds-browser.git
```

2. **安装依赖**
```bash
# 后端依赖
go mod download

# 前端依赖
cd ui
npm install
cd ..
```

3. **创建功能分支**
```bash
git checkout -b feature/your-feature-name
```

### 开发规范

#### 🔷 Go 代码规范
- 遵循 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- 使用 `gofmt` 格式化代码
- 使用 `golint` 检查代码质量
- 添加适当的注释和文档
- 编写单元测试

```bash
# 格式化代码
go fmt ./...

# 检查代码
golint ./...

# 运行测试
go test ./...
```

#### 💚 Vue.js 代码规范
- 遵循 [Vue.js 风格指南](https://vuejs.org/style-guide/)
- 使用 ESLint 检查代码质量
- 使用 Prettier 格式化代码
- 组件名使用 PascalCase
- 文件名使用 kebab-case

```bash
cd ui

# 检查代码
npm run lint

# 格式化代码
npm run format

# 运行测试
npm run test
```

#### 📝 提交信息规范
使用 [Conventional Commits](https://www.conventionalcommits.org/) 格式：

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

类型包括：
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `style`: 代码格式化
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建过程或辅助工具的变动

示例：
```
feat(ui): 添加命名空间搜索功能

- 支持实时搜索过滤
- 显示命名空间对象数量
- 优化下拉框样式

Closes #123
```

### Pull Request 流程

1. **确保代码质量**
```bash
# 运行所有检查
make check

# 或者手动运行
go fmt ./...
go test ./...
cd ui && npm run lint && npm run test
```

2. **更新文档**
- 更新 README.md（如果需要）
- 添加或更新注释
- 更新 API 文档（如果适用）

3. **创建 Pull Request**
- 使用清晰的标题和描述
- 引用相关的 Issues
- 添加截图（如果是 UI 变更）
- 确保 CI 检查通过

4. **PR 模板**
```markdown
## 变更类型
- [ ] Bug 修复
- [ ] 新功能
- [ ] 文档更新
- [ ] 代码重构
- [ ] 其他

## 变更描述
简洁明了地描述您的变更

## 相关 Issues
Closes #(issue number)

## 测试
- [ ] 已添加测试用例
- [ ] 所有测试通过
- [ ] 手动测试通过

## 截图
如果是 UI 变更，请添加截图

## 检查清单
- [ ] 代码遵循项目规范
- [ ] 自我审查了代码
- [ ] 添加了必要的注释
- [ ] 更新了相关文档
- [ ] 没有引入新的警告
```

## 🧪 测试

### 运行测试

```bash
# 后端测试
go test ./...

# 前端测试
cd ui
npm run test

# 端到端测试
npm run test:e2e
```

### 编写测试

- **单元测试**: 测试单个函数或组件
- **集成测试**: 测试组件间的交互
- **端到端测试**: 测试完整的用户流程

## 📚 文档贡献

文档同样重要！您可以帮助改进：

- README.md
- API 文档
- 代码注释
- 用户指南
- 开发者文档

## 🎯 项目路线图

查看我们的 [项目路线图](https://github.com/your-org/crds-browser/projects) 了解未来的开发计划。

## 💬 社区

- 💬 [Discussions](https://github.com/your-org/crds-browser/discussions) - 一般讨论和问答
- 🐛 [Issues](https://github.com/your-org/crds-browser/issues) - Bug 报告和功能请求
- 📧 [Email](mailto:maintainers@your-org.com) - 私人联系

## 🏆 贡献者

感谢所有贡献者！

<a href="https://github.com/your-org/crds-browser/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=your-org/crds-browser" />
</a>

## 📄 许可证

通过贡献代码，您同意您的贡献将在 [MIT 许可证](LICENSE) 下授权。

---

再次感谢您的贡献！🎉 