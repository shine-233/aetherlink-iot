# auth swagger

## 目录定位

这个目录保存 auth 插件账号管理 API 的 Swagger JSON，用于描述 HTTP 网关暴露的账号管理接口。

## 文件用途

- `account.swagger.json`：由 `account.proto` 生成的账号管理接口文档。

## 生成物、示例、测试数据边界

本目录内容属于生成文档产物，来源是 `plugin/auth/protos/account.proto`。不要直接手改 JSON 来表达业务变化，应修改 proto 并重新生成。

## 审查发现

- 当前目录只有生成 JSON，缺少来源和维护规则说明。
- Swagger 与 proto 的一致性依赖人工重新生成，目录内没有自动漂移检测。

## 重构建议

- 在 CI 或本地脚本中增加 proto 变更后的 Swagger 产物检查。
- 对外发布前可增加统一的 API 文档聚合步骤，而不是在生成 JSON 内手写说明。

## 验证建议

- 用 JSON 校验工具确认 `account.swagger.json` 格式有效。
- 修改账号 API 后重新生成并抽查 `/v1/accounts` 相关路由是否与实现一致。
