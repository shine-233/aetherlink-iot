# 应用插件组件测试说明

本目录覆盖应用插件接入相关弹窗与动态表单，确保服务注册、服务配置和表单 schema 解析保持稳定。

## 测试覆盖目标

- `form.test.ts` 覆盖 input、select、table 等动态表单元素和空 schema。
- `serviceConfigModal.test.ts` 覆盖接入协议/接入服务配置弹窗、配置解析、关闭重置和提交。
- `serviceModal.test.ts` 覆盖新增/编辑服务弹窗的默认值、回填、关闭和创建/更新提交。

## 维护规则

- 新增表单元素类型、校验规则或服务配置字段时，需同步补充 schema 和提交 payload 断言。
- 弹窗测试只 mock API 和 Naive UI，不应依赖真实服务配置。

## 已知缺口

- 尚未覆盖复杂嵌套 schema、远端配置解析失败提示和多服务类型权限边界。
