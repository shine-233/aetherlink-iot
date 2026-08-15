# 通知服务组件

`components` 存放通知服务配置表单。

## 目录职责

- `email.vue` 管理邮件服务配置和测试发送，并挂载共享的 `@/components/business/email-template-manager.vue`；该页面面向 `SYS_ADMIN`，因此模板管理作用域为系统默认模板。
- `short-message.vue` 管理短信服务配置。
- `push-notification.vue` 管理推送服务配置。

## 维护注意

- 各服务字段差异较大，不要为复用牺牲清晰的接口映射。
- 保存后需要保持用户反馈和配置重新加载策略一致。
- 邮件模板组件同时复用于租户管理员的告警通知组页面；不要在页面层传入 `tenant_id`，系统/租户作用域必须继续由后端 claims 判定。
- 模板预览只渲染白名单纯文本变量，不会发送邮件；SMTP 凭据、收件人解析和模板 CRUD 是三条独立边界。
