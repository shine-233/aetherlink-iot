# 系统管理目录说明

## 目录定位

`frontend/src/views/management` 是后台治理类页面集合，主要承接“账号、角色、权限、通知、系统设置、API 凭据”这类高权限、低频但高风险的操作界面。和设备管理、可视化编辑这类日常业务页面不同，这里的页面更强调配置正确性、权限边界、批量操作反馈和审计可读性。

从当前真实结构看，这个目录不是单一页面集合，而是一组“管理子域”：

- 身份与权限域：`user`、`role`、`auth`
- 系统配置域：`setting`、`notification`
- 接入与凭据域：`api`

目录下大量页面都采用了 `index.vue + components/modules + __tests__` 的组织方式，说明这些管理页已经具备一定组件化基础，但也暴露出不少“弹窗、表格列、权限树”重复模式，后续适合继续抽象。

## 真实结构概览

| 路径 | 主要职责 | 结构特点 |
| --- | --- | --- |
| `api/` | 管理 API Key 或接入凭据的列表、新增、编辑、启停 | `index.vue` + `modules/table-action-modal.vue` + 页面测试 |
| `auth/` | 管理权限元素、授权项或接口级权限资源 | `index.vue` + `components/table-action-modal.vue` + 页面测试 |
| `notification/` | 管理邮件、短信、推送通知等发送通道配置 | `index.vue` + `components/email.vue`、`short-message.vue`、`push-notification.vue` |
| `role/` | 管理角色、授权和列配置等高权限操作 | `index.vue` + `modules/*`，包含权限编辑和列设置 |
| `setting/` | 管理账号资料、邮箱、品牌配置、数据清理、功能开关等系统级设置 | `index.vue` + 多个分块组件 |
| `user/` | 管理后台用户、密码修改、列配置、用户表格动作 | `index.vue` + `components/*` |

## 模块关系与业务分层

### 1. 用户、角色、权限是一条完整的授权链

`user`、`role`、`auth` 三个目录不是孤立页面，而是后台权限体系的三个层次：

- `user` 负责“谁可以进入系统”
- `role` 负责“这类人拥有什么权限集合”
- `auth` 负责“权限资源本身如何定义或配置”

因此这三类页面的风险并不在单个表单，而在它们的联动关系。如果只改某一个页面的字段展示，却不核对对应角色授权或权限树展示逻辑，很容易出现“界面能配、实际不生效”或者“角色边界与页面文案不一致”的问题。

### 2. `setting` 与 `notification` 共同影响系统级运营配置

`setting` 目录下已经拆出多块独立组件：

- `account-profile-setting.vue`
- `account-email-setting.vue`
- `branding-setting.vue`
- `data-clear-setting.vue`
- `function-setting.vue`
- `warning-email-setting.vue`

这说明“系统设置”并不是单表单，而是多个配置子域共存。与此同时，`notification` 也在管理邮件、短信、推送通道。两者之间存在明显边界：

- `setting` 更偏平台自身配置
- `notification` 更偏消息发送能力配置

后续如果新增告警通知策略、品牌邮件模板、短信签名等能力，要先判定它属于“系统基础配置”还是“通知能力配置”，否则目录会越来越混乱。

### 3. `api` 是凭据治理入口，安全等级高

`api/modules/table-action-modal.vue` 的存在说明该页面不仅展示列表，还包含高风险凭据操作。API Key、Access Token、密钥启停这类能力通常涉及复制、查看、重置、禁用等动作，必须把权限控制、二次确认、敏感值脱敏和失败反馈视为一等公民，而不是普通 CRUD 页面。

## 重点文件与阅读建议

### `api`

- `api/index.vue`
  API 凭据管理主页面，建议优先确认列表筛选、启停状态和凭据创建入口是否集中在这里。
- `api/modules/table-action-modal.vue`
  这是最可能承载新增、编辑、重置等敏感动作的弹窗组件，后续任何安全相关改动都应优先审查这里。

### `auth`

- `auth/index.vue`
  权限资源管理主入口，适合重点查看权限树、表格动作和新增编辑流程。
- `auth/components/table-action-modal.vue`
  若权限项字段、绑定范围、显示文案发生变更，这里通常是第一落点。

### `role`

- `role/index.vue`
  角色管理主视图，通常会承接列表、搜索、授权入口和高权限批量操作。
- `role/modules/edit-permission-modal.vue`
  角色授权核心弹窗，是权限边界最敏感的位置之一。
- `role/modules/column-setting.vue`
  暗示页面支持列表列可配，后续如果更多管理页都有类似需求，可以考虑统一抽象。

### `setting`

- `setting/index.vue`
  系统设置总入口，负责组合多个设置分块。
- `setting/components/branding-setting.vue`
  与品牌、Logo、视觉识别直接相关，往往还会联动全局主题或系统配置 store。
- `setting/components/function-setting.vue`
  一般承载系统级开关，最需要补充禁用态、说明文案和风险提示。
- `setting/components/data-clear-setting.vue`
  数据清理动作天然高风险，需重点关注确认流程和结果反馈。

### `user`

- `user/index.vue`
  用户管理总入口，通常连接搜索、表格、列配置和密码操作。
- `user/components/table-action-modal.vue`
  用户新增编辑入口，字段校验、默认值和权限范围通常集中于此。
- `user/components/edit-password-modal.vue`
  密码操作常见风险点，需关注明文暴露、错误反馈和提交节流。
- `user/components/column-setting.vue`
  说明用户管理页已经有可配置列能力，可作为全目录表格能力抽象样本。

## 当前目录呈现出的共性模式

- 多数管理页都是“主表格 + 弹窗动作 + 局部设置组件”的后台经典结构。
- 很多目录都已经补了 `README.md` 和 `__tests__`，说明项目希望把页面说明、组件说明和测试一起沉淀。
- `table-action-modal.vue` 在多个子目录重复出现，表明新增编辑类弹窗模式高度相似。
- `column-setting.vue` 至少在 `user` 与 `role` 中都存在，说明表格列偏好已经是共性需求。

## 维护注意事项

- 这里的大部分页面都带权限属性，不能把它们按普通 CRUD 页面对待。任何字段、按钮、弹窗的改动都要先问清楚“谁能看、谁能改、失败后怎么提示”。
- 新增后台配置前，先判断它属于 `management` 还是其他业务目录，避免和设备管理、租户上下文初始化、可视化配置目录发生职责重叠。
- 表格类页面改动时，要同步留意列配置、分页筛选、搜索条件和弹窗默认值是否仍一致。
- `setting`、`notification`、`api` 中涉及敏感配置的页面，维护时要优先关注脱敏展示、二次确认和不可逆操作提示。
- 已存在的 `__tests__` 结构说明这些页面适合继续走“页面主流程 + 组件细粒度”测试组织，不建议后续把复杂交互重新塞回单文件页面中。

## 建议的重构方向

### 1. 抽象后台表格与动作弹窗骨架

当前 `api`、`auth`、`role`、`user` 都出现了“列表页 + table-action-modal”的稳定模式。建议逐步提炼统一的管理页骨架，例如：

- 列表查询区域约定
- 表格操作列插槽约定
- 新增编辑弹窗的提交流程约定
- 通用权限按钮禁用逻辑

预期效果：

- 降低多个目录重复维护相似表单和表格逻辑的成本
- 减少新增一个管理页时复制粘贴旧页面的冲动
- 更容易统一错误提示、按钮文案和权限校验行为

### 2. 把权限敏感逻辑与展示逻辑继续拆层

像 `role/modules/edit-permission-modal.vue`、`api/modules/table-action-modal.vue`、`user/components/edit-password-modal.vue` 这类组件，往往同时混杂字段展示、接口提交、权限判断和错误反馈。建议逐步把字段定义、表单默认值、权限判断和请求编排拆到独立组合函数或 helper 中。

预期效果：

- 组件职责更清晰
- 容易补充单测和静态审查
- 降低一个弹窗改动影响整页行为的概率

### 3. 为高风险操作建立统一交互标准

`data-clear-setting.vue`、密码修改、API Key 启停或重置、本地品牌配置覆盖这类动作，都应该有统一的风险提示和失败反馈策略。当前目录结构已经能看出风险分散在多个子域中，建议后续整理一套管理端交互规范。

预期效果：

- 减少不同页面对同类风险操作的处理标准不一致
- 提升 GitHub 开源前的可维护性和可审查性
- 让新成员更容易判断一个高风险操作页面该具备哪些最基本的保护措施
