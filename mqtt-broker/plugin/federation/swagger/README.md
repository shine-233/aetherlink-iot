# federation swagger

## 目录定位

这个目录保存 federation 插件 HTTP 网关接口的 Swagger JSON，主要用于集群管理相关 API 文档和调试。

## 文件用途

- `federation.swagger.json`：由 `federation.proto` 生成的 federation API 文档。

## 生成物、示例、测试数据边界

本目录内容是生成文档产物，来源是 `plugin/federation/protos/federation.proto`。不要直接手改 JSON；协议或路由变化应从 proto 修改开始并重新生成。

## 审查发现

- 当前目录只有生成产物，缺少维护边界说明。
- federation 接口和跨节点协议强相关，Swagger 只能说明 HTTP 映射，不能替代多节点行为验证。

## 重构建议

- 在发布文档时把 Swagger 产物和多节点验证说明分开维护。
- 增加生成产物一致性检查，避免 proto 更新后忘记提交 Swagger。

## 验证建议

- 用 JSON 校验工具确认 `federation.swagger.json` 格式有效。
- 修改 federation API 后重新生成，并结合多节点示例或相关 Go 测试验证行为。
