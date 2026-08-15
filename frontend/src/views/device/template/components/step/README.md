# Thing Model Wizard Steps

## 目录职责

存放物模型新增/编辑向导的各步骤组件，包括基础信息、模型定义、遥测/属性/事件/命令编辑、Web/App 图表配置、自定义命令/控件和完成页。

## 文件关系

- `add-info.vue` 负责物模型基础信息创建或更新，并把物模型 ID 传给后续步骤。
- `model-definition.vue` 聚合遥测、属性、事件、命令四类物模型列表和增删改弹窗。
- `add-edit-test.vue`、`add-edit-attributes.vue`、`add-edit-events.vue`、`add-edit-commands.vue` 分别处理物模型子项表单。
- `enum-info.vue` 为枚举/布尔等附加信息提供表格编辑。
- `web-chart-config.vue`、`app-chart-config.vue`、`widget-preset-config.vue` 处理 ThingsVis 图表配置和物模型预设。
- `custom-commands.vue`、`custom-controls.vue` 维护物模型级自定义命令和自定义控制。
- `complete.vue` 展示最终物模型 JSON。
- `model-definition-table-columns.ts` 集中物模型定义步骤的表格列定义。

## 重点文件

- `model-definition.vue`: 物模型四类数据的聚合调度中心。
- `web-chart-config.vue`: Web 图表配置保存、平台字段抽取和物模型字段归一化核心。
- `app-chart-config.vue`: App 图表配置分支，和 Web 配置共享较多业务概念。
- `widget-preset-config.vue`: 物模型字段到 ThingsVis widget 预设的桥接点。
- `add-info.vue`: 物模型 ID 生成和向导继续的前置步骤。

## 审查建议

- 修改物模型 API 参数时，四类模型组件和 `model-definition.vue` 的分页查询要一起检查。
- 修改图表配置字段时，必须同时检查 Web/App 分支以及预设组件。
- 当前多个步骤使用宽松 `any` 和响应式对象拷贝，重构时建议先抽出物模型类型和共享表单适配器。
- 不要在步骤组件里直接扩散跨模块服务逻辑，新增 API 语义应优先沉到 service 或专用 composable。
