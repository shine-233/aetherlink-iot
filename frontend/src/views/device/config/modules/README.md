# 设备配置入口子模块

## 目录职责

`frontend/src/views/device/config/modules` 保存设备配置入口页的局部组件，覆盖配置新增/编辑、物模型市场列表、物模型详情、市场登录和发布确认等流程。

## 文件关系

- `config-modal.vue`
  - 服务于设备配置创建和编辑流程。
- `market-template-list.vue`
  - 组合模板搜索、分页、安装、登录状态和详情抽屉。
- `market-template-card.vue`、`market-template-drawer.vue`
  - 承载物模型卡片展示与详情查看/安装事件。
- `market-login-modal.vue`
  - 管理物模型市场登录。
- `publish-confirm-modal.vue`
  - 管理配置发布确认流程。
- `__tests__`
  - 存放这些模块的测试资源。

## 当前静态审查结论

- 问题：物模型市场登录、配置编辑和发布确认共处一层，状态机耦合风险较高。
- 改进：继续明确 token、配置 ID、物模型字段和安装/发布事件在父子组件之间的传递边界。
- 预期效果：设备配置入口页的市场链路更容易维护，也更容易分层验证。
