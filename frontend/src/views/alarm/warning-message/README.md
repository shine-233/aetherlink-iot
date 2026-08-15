# 告警消息

`warning-message` 目录负责告警消息列表、告警配置和新增编辑弹窗。

## 目录职责

- `index.vue` 管理告警消息页面主体和组件组合。
- `components/alarm-configuration.vue` 处理告警配置、确认、重置和历史查询。
- `components/new-information.vue` 处理告警消息列表与启停编辑。
- `components/pop-up.vue` 处理告警消息新增编辑弹窗。
- `components/alarm-configuration.helpers.ts` 提供告警字段映射辅助函数。

## 维护注意

- 告警严重级别、类型和动作字段需保持与后端枚举一致。
- 启停、确认和重置操作必须保留明确反馈，避免误判告警状态。