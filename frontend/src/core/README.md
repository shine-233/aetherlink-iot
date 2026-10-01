# 前端核心模块

`frontend/src/core` 保存位于页面之下、通用工具之上的前端核心引擎。

## 文件夹定位

- `script-engine/`：脚本编辑、执行和辅助能力。

## 维护关系

- 新模块必须能从 `src/main.ts` 静态或动态导入到达，否则 `pnpm check:reachability` 失败。原 `data-architecture/`、`interaction-system/` 因不可达已于 2026-10 删除；看板数据绑定在 `components/thingsvis/` 与 `service/visualization-provider/`。

- core 模块通常承载持久化配置格式，字段删除或重命名前必须有迁移证据。
- 页面和组件应尽量通过稳定 API 调用 core 能力，不应直接依赖内部临时状态。
- 大文件重构应先抽纯 helper，再补 focused tests，避免一次性改动 UI 与数据合同。

## 审查建议

- 问题：核心模块常包含历史兼容字段、复杂 helper 和跨页面副作用。
- 改进：重构前记录数据合同，保持导入导出兼容测试，并避免无迁移删除 alias 字段。
- 预期效果：让核心引擎的维护更安全，减少隐藏回归。
