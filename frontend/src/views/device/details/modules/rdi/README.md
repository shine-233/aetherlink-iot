# RDI 设备详情支持

## 目录职责

`frontend/src/views/device/details/modules/rdi` 保存 RDI 设备详情操作视图依赖的 composable、常量和专用业务支持，是设备详情中最具协议与业务契约敏感性的一层。

## 文件关系

- `../RdiDeviceOperationsView.vue`
  - 直接消费本目录下的 composable 和常量。
- `composables/*`
  - 分别管理配置、遥测、历史、命令和分享逻辑。
- `constants/rdi-labels.ts`
  - 提供 RDI 字段标签和展示文案来源。

## 静态审查建议

- 问题：RDI 字段、命令和分享语义直接连接前后端契约，随意改动容易同时影响展示、控制和权限路径。
- 改进：继续保持常量、composable 和面板之间的职责说明清晰，并优先做小步静态收敛。
- 预期效果：RDI 操作视图相关改动更容易按“常量 / 状态 / 命令 / 分享”分层审查。
