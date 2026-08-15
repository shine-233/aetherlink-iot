# 前端服务层

`frontend/src/service` 保存 API wrapper、请求基础设施和前后端合同类型。

## 文件夹定位

- `api/`：按 auth、route、device、RDI、dashboard、plugin、system 等领域组织产品 API。
- `request/`：维护共享请求客户端、token 注入、语言请求头、base URL 规则和全局错误处理。
- `visualization-provider/`：可视化 provider 的唯一契约与 adapter seam；`provider-ids.ts` 集中维护稳定的 provider/project ID，具体 provider 实现在同目录内。

## 关键关系

- 页面、store 和集成组件应调用 service wrapper，不应临时创建独立 HTTP 客户端。
- `visualization-provider.ts` 只保留历史导入兼容门面；新的 provider 类型、ID 或行为必须进入 `visualization-provider/`，不能在门面文件中重建第二套实现。
- 后端响应 envelope 的归一化逻辑集中在这里，修改会影响大多数 UI 流程。
- 鉴权、语言、401/session 过期处理是平台级行为，变更时需要重点测试。

## 审查建议

- 问题：大型 API wrapper 容易隐藏前后端合同漂移。
- 改进：按领域维护 endpoint 文档，超大 wrapper 拆分为更小模块，并从 index 保留兼容导出。
- 预期效果：后端 API 变化更容易追踪到前端调用点和测试。
