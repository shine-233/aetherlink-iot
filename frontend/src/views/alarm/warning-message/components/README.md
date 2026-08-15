# 告警消息组件

`components` 存放告警消息页面的配置、列表、弹窗和辅助函数。

## 目录职责

- `alarm-configuration.vue` 管理告警历史、确认、重置和配置表格。
- `new-information.vue` 管理告警消息列表、启停和编辑入口。
- `pop-up.vue` 管理告警新增编辑表单。
- `alarm-configuration.helpers.ts` 集中告警字段解析和展示映射。

## 维护注意

- 辅助函数应保持纯函数优先，便于覆盖边界输入。
- 弹窗组件通过事件通知父页面刷新，不直接承担页面级列表状态。