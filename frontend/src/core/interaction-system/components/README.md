# interaction-system 组件目录

## 目录定位

`frontend/src/core/interaction-system/components` 提供可视化编辑器交互配置相关的 Vue 组件，是交互系统面向用户配置界面的主要呈现层。

## 文件用途

- `InteractionCardWizard.vue` 负责交互卡片编辑向导。
- `InteractionPreview.vue` 负责交互效果预览。
- `InteractionTemplatePreview.vue` 负责模板详情预览。
- `InteractionTemplateSelector.vue` 负责模板选择。
- 对应 `.test.ts` 文件覆盖组件契约和关键交互分支。

## 维护边界

本目录只处理交互配置 UI、用户输入和预览呈现。交互执行、注册表和业务状态写入应留在 core 服务或 manager 层，不应直接沉入组件内部。

## 审查发现

组件和测试并列放置，边界清楚。主要风险是向导组件继续累积模板解析、校验和状态写入逻辑，导致 UI 层承担过多编排责任。

## 重构建议

后续可以把模板归一化、校验和默认值生成抽成纯函数，再由组件调用，便于用小单测覆盖复杂分支。

## 验证建议

改动组件逻辑时运行本目录相关 Vitest，并对可视化编辑器中的新增、编辑、预览、模板导入流程做手工 smoke 验证。
