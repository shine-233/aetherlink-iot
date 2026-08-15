# Thing Model Wizard Step Tests

## 目录职责

覆盖物模型向导各步骤的初始化、API 调用、表单默认值、模型列表、自定义命令/控件、图表配置保存和 ThingsVis widget 预设行为。

## 文件关系

- `add-info.test.ts` 覆盖模板基础信息步骤。
- `model-definition.test.ts` 覆盖物模型四类列表聚合和步骤组件映射。
- `add-edit-*.test.ts` 覆盖遥测、属性、事件、命令编辑弹窗。
- `web-chart-config.test.ts`、`app-chart-config.test.ts`、`widget-preset-config.test.ts` 覆盖图表配置和预设保存。
- `custom-commands.test.ts`、`custom-controls.test.ts` 覆盖模板级自定义能力。
- `complete.test.ts` 覆盖完成页模板详情读取。

## 重点文件

- `web-chart-config.test.ts`: 图表配置保存和平台字段清洗的关键回归。
- `widget-preset-config.test.ts`: 物模型字段生成 widget 预设的关键回归。
- `model-definition.test.ts`: 四类物模型 API 查询边界的关键回归。

## 审查建议

- 改 API 查询参数时，优先检查 `model-definition.test.ts` 和对应 `add-edit-*` 测试。
- 改 ThingsVis 配置保存时，保留虚拟设备字段清洗断言。
- 新增步骤组件时，补齐初始化、前后步骤事件和失败/空数据分支。
