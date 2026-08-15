# Data Architecture Components

## 目录职责

存放数据源配置、轮询、合并策略、HTTP 表单和设备参数选择相关 Vue 组件，是 data-architecture 面向用户编辑配置的主要 UI 层。

## 文件关系

- `common/` 提供 HTTP 步骤、动态参数编辑器和导入导出面板等共享组件。
- `device-selectors/` 负责设备、指标、属性等参数选择，并把结果交给动态参数编辑器或 HTTP 配置。
- `modals/` 包装完整配置弹窗流程。
- 顶层合并策略和轮询组件会产出执行器直接消费的配置字段。

## 重点文件

- `DataSourceMergeStrategyEditor.vue`：完整版合并策略编辑入口。
- `DataSourceMergeStrategyEditorSimple.vue`：轻量合并策略编辑入口。
- `ComponentPollingConfig.vue`：组件级数据源轮询配置。
- `common/DynamicParameterEditor.vue`：动态参数编辑核心组件，体量和风险都较高。

## 审查建议

- UI 改动不能只看展示效果，还要确认 emit payload 和持久化字段是否兼容。
- 大组件拆分前先补足参数、设备选择和导入导出的契约测试。
- 避免在组件中复制执行器或导入导出规则，持久化行为应下沉到工具或服务层。
