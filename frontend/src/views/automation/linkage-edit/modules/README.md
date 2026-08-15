# 联动编辑模块说明

## 目录职责

这个目录承载联动规则编辑页的核心子模块，包含前提编辑、动作编辑，以及围绕前提编辑器拆分出的状态与生命周期 helper。

## 关键文件

- `edit-premise.vue`：前提条件编辑器父组件，负责条件组 UI 编排、事件参数面板装配和对外暴露表单能力。
- `edit-action.vue`：动作组与动作项编辑面板。
- `PremiseScheduleConditionEditor.vue`：时间条件子面板，承接周期、时段和失效时间等输入。
- `premise-device-condition-state.ts`：设备条件来源类型、设备分组/设备配置查询、焦点与下拉交互状态。
- `premise-trigger-lifecycle.ts`：前提编辑器的触发参数加载、回显回填、初始条件创建和启动流程编排。
- `premise-trigger-param-state.ts`：触发参数选择态、事件参数 UI 状态、回显归一和输入校验。
- `premise-trigger-param-options.ts`：触发参数选项的加载、格式化与状态选项补位。
- `premise-event-param-conditions.ts`：事件参数条件行的字段、操作符和值的增删与联动。
- `premise-localized-condition-options.ts`：前提编辑器里的本地化条件选项与轻量派生值封装，集中维护状态、比较符和时间条件下拉数据。
- `premise-schedule-condition-state.ts`：时间/周期条件的默认字段和选项构建。
- `premise-edit-premise-state.ts`：前提回显数据预处理、初始化条件生成和 locale 刷新辅助。

## 维护提示

- 新增设备条件时，优先沿现有拆分把“来源状态”“参数选项”“触发链路编排”分别放到对应 helper，不要重新塞回 `edit-premise.vue`。
- 调整状态、比较符或周期/星期/失效时间这类本地化选项时，优先改 `premise-localized-condition-options.ts`，避免在父组件里重复声明 `computed`。
- `edit-premise.vue` 仍是父级编排层，调整 `props`、`emit`、`defineExpose` 合约时要同步检查上下游。
- 当前拆分已经覆盖设备来源、触发参数和时间条件；后续若继续收敛，可优先看事件参数面板和服务条件分支。
