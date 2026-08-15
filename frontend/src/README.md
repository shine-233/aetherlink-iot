# 前端源码目录

`frontend/src` 保存 AetherLink IoT Web 客户端的 Vue 3 应用源码，是页面、服务、路由、状态管理、可视化集成和核心前端引擎的主目录。

## 文件夹定位

- `views/`：路由页面，例如仪表盘、设备管理、自动化、告警和系统管理。
- `service/`：封装前后端 API 调用和请求合同。
- `store/`：Pinia 状态模块，维护认证、路由、主题、应用状态和领域数据。
- `router/`：路由注册、路由守卫和路由元数据。
- `components/`：跨页面共享组件和 ThingsVis 集成组件。
- `core/`：数据架构、交互系统、脚本引擎等复杂前端能力。
- `utils/`、`hooks/`、`constants/`、`enum/`、`typings/`、`config/`：可复用基础设施。
- `locales/`、`theme/`、`styles/`、`assets/`：国际化、主题、样式和静态资源。

## 关键关系

- `main.ts` 负责挂载 Vue、插件、路由、状态、样式和应用级 provider。
- `router/` 与 `service/api/` 共同构成 UI 流程和后端接口之间的主合同。
- `components/thingsvis/` 将嵌入式 ThingsVis 大屏与平台设备、告警、遥测 API 连接起来。
- `core/data-architecture/` 和 `core/interaction-system/` 包含可视化编辑器配置逻辑，重构前必须先补合同说明和 focused tests。

## 审查与重构建议

- 问题：部分高价值文件同时包含 UI 状态、传输适配、兼容逻辑和数据整形，审查成本高。
- 改进：先补文件级说明，再逐步抽取 transport adapter、纯数据 mapper 和兼容 helper。
- 实施步骤：记录当前合同，补可见行为和 API 参数测试，一次只抽一个 helper，并同步对应目录 README。
- 预期效果：降低 ThingsVis、data-architecture 和自动化页面的回归风险，让 GitHub 审阅边界更清晰。

## 文档标准

每个 Vue、TypeScript、JavaScript 手写源文件应包含简短文件头或模块级注释，说明用途、核心逻辑、重要输入输出、副作用和维护注意事项。生成类型、样式和明显资源文件可由目录 README 统一说明。
