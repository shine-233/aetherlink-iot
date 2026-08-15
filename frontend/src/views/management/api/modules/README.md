# API 密钥模块

`modules` 存放 API Key 管理页面的弹窗组件。

## 目录职责

- `table-action-modal.vue` 负责 API Key 新增编辑表单和提交。
- `__tests__/` 覆盖弹窗表单初始化、校验和提交分支。

## 维护注意

- 弹窗通过父页面控制可见性与刷新，不直接维护列表。
- 新增字段时同步接口 payload、校验规则和测试数据。