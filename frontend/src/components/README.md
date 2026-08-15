# 前端组件目录

`frontend/src/components` 保存 AetherLink IoT 前端跨页面复用的 Vue 组件。

## 文件夹定位

- 表格、选择器、筛选器、表单、可视化编辑器、ThingsVis 和通用布局组件放在这里。
- `visualization-provider/` 只负责把 service 层选定的 provider 渲染成页面组件；provider 选择、ID 和数据访问仍归 `src/service/visualization-provider/` 管理。
- 面向单一业务页面的组件应尽量通过 props/events 暴露边界，避免直接耦合路由或全局 store。
- 设备选择、列表页面、遥测历史筛选、网格工具等子模块由各自 README 继续说明。

## 文件关系

- 页面层 `views/` 通常组合这里的组件完成业务流程。
- 复杂组件应把 API 调用放到页面、composable 或 service wrapper 中，组件本身专注展示和交互。
- ThingsVis 组件属于外部嵌入合同边界，修改时要同时检查兼容文档和 focused tests。

## 审查建议

- 问题：共享组件容易悄悄变成单业务专用组件，导致复用和测试困难。
- 改进：明确组件所有权，保持 props/events 稳定，并为复杂状态补组件级测试。
- 预期效果：减少跨页面复用风险，方便 GitHub 读者理解组件边界。
