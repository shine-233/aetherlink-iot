# 后台用户组件

`components` 存放后台用户管理页面的表单和列设置组件。

## 目录职责

- `table-action-modal.vue` 负责用户新增编辑表单。
- `edit-password-modal.vue` 负责用户密码修改。
- `column-setting.vue` 负责表格列展示配置。

## 维护注意

- 地址、区号和时区字段存在较多转换逻辑，改动时优先补充聚焦测试。
- 列设置只影响前端展示，不应混入用户实体提交逻辑。