# Device Config Module Tests

## 目录职责

`frontend/src/views/device/config/modules/__tests__` 覆盖设备配置模块组件的前端行为测试，主要验证表单初始化、弹窗开关、物模型市场数据流、安装/发布事件和错误提示。

## 文件关系

- 每个测试文件对应上级目录中的同名 `.vue` 组件。
- 测试通过组件 mount、API mock 和 Naive UI 轻量 stub 验证组件公开状态与事件。
- 物模型市场列表测试会串联登录门控、安装调用、详情打开和成功事件，是市场流程的主要前端回归点。
- 发布确认测试依赖配置详情读取和发布 API mock，用于保护发布 payload 与 token 检查。

## 重点文件

- `config-modal.test.ts`: 配置表单默认值、标题切换、模板加载、关闭重置和新增/编辑提交。
- `market-template-list.test.ts`: 搜索分页、详情抽屉、登录门控、安装成功、重复安装和 401 清理。
- `publish-confirm-modal.test.ts`: 发布表单、默认名称、配置详情预填、无 token 错误和发布成功事件。
- `market-login-modal.test.ts`: 登录弹窗重置、市场登录调用、token 写入和跳转注册。
- `market-template-card.test.ts` 与 `market-template-drawer.test.ts`: 模板展示、默认封面、详情加载和安装事件。

## 审查建议

新增组件状态时优先断言用户可见结果、emit 事件和 API payload，不要只断言内部变量存在。若组件改用新的市场 API 或 token 存储键，需要同时检查对应 API wrapper 测试和父组件集成路径。
