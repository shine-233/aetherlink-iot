# admin swagger

## 目录定位

这个目录保存 admin 插件管理 API 的 Swagger JSON，供 HTTP 网关文档、接口调试和外部客户端对接使用。

## 文件用途

- `client.swagger.json`：客户端管理接口文档。
- `publish.swagger.json`：管理端发布消息接口文档。
- `subscription.swagger.json`：订阅管理接口文档。

## 生成物、示例、测试数据边界

这些 JSON 是由 `plugin/admin/protos/*.proto` 通过 `proto_gen.sh` 生成的接口文档产物。维护时不要直接手写业务含义，应回到 proto 源文件修改后重新生成。

## 审查发现

- 当前目录只有生成产物，缺少来源说明，容易被误认为是人工维护文件。
- Swagger 文件未在本目录提供生成命令，实际入口在相邻 `protos` 目录。

## 重构建议

- 在统一生成脚本中加入产物新旧 diff 检查，避免 proto 与 Swagger 漂移。
- 如果未来发布 OpenAPI 文档，可增加版本号和生成时间记录，但不要污染生成 JSON 内容。

## 验证建议

- 修改 proto 后重新生成 Swagger，并用 JSON 校验工具确认文件格式有效。
- 对外发布前抽查 HTTP 路由、请求体和响应结构是否与 gateway 实际行为一致。
