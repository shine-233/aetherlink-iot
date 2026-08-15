# RDI Composable 组合层

## 目录职责

`frontend/src/views/device/details/modules/rdi/composables` 将 RDI 操作视图中的配置、遥测、历史、命令和分享逻辑拆成可复用、可测试的组合函数。

## 文件关系

- `useRdiConfig.ts`
  - 提供配置状态和保存能力。
- `useRdiTelemetry.ts`
  - 管理实时字段和展示数据。
- `useRdiHistory.ts`
  - 管理历史查询。
- `useRdiCommands.ts`
  - 管理命令动作。
- `useRdiShare.ts`
  - 管理分享链接和 shared-with-me 状态。

## 静态审查建议

- 问题：Composable 层如果混入 route 或 UI 细节，后续就会和面板壳层重新耦合。
- 改进：继续保持输入输出稳定，把业务状态和 UI 细节分开记录。
- 预期效果：RDI 操作视图更容易局部重构，也更方便补聚焦测试。
