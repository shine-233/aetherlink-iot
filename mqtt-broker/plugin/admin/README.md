# Admin 管理插件

`mqtt-broker/plugin/admin` 提供 broker 管理 API，通过 gRPC 与 grpc-gateway 暴露客户端、订阅和消息发布管理能力，并附带轻量管理页面。

## 目录定位

- 暴露 broker 管理 API，包括客户端列表/断开、订阅查询/变更和外部发布消息。
- 通过 broker hooks 镜像客户端与订阅状态，供管理 API 查询。
- 在同一个 HTTP gateway 上提供轻量登录和 dashboard 页面。
- 保存 Admin API 的 protobuf、Swagger 和生成代码。

## 关键文件关系

- `admin.go`：插件注册、API 注册、HTTP handler 装配和 broker 依赖初始化入口。
- `config.go`：HTTP/gRPC 默认监听地址和配置校验。
- `hooks.go`：在 broker 会话或订阅变化时更新 `store.go`。
- `client.go`、`subscription.go`、`publish.go`：手写 RPC 服务逻辑。
- `web.go`：内置管理 UI、环境变量凭证校验和 session cookie。
- `utils.go`：分页、gRPC 错误和有序 `Indexer`；`auth` 插件也复用该索引器。
- `protos/`、`swagger/`、`*.pb.go`、`*.pb.gw.go`、`*_grpc.pb.go`：API 契约和生成物。

## API 文档

本仓库内置 Swagger 位于：

- `swagger/client.swagger.json`
- `swagger/subscription.swagger.json`
- `swagger/publish.swagger.json`

不要再链接上游仓库的 swagger；发布前应以本仓当前生成物为准。

## 示例

管理面支持可选共享密钥认证：在 `plugins.admin.http_auth_secret` 配置非空密钥后，所有 admin HTTP API 请求必须携带匹配的 `X-Admin-Secret` 头（内置管理页登录后的会话 cookie 也可访问，保证 dashboard 可用）；密钥为空时保持既有行为，仅依赖 `api.http`/`api.grpc` 网络边界保护，此时绑定非回环地址会在启动时打印告警。

列出客户端：

```bash
curl -H "X-Admin-Secret: <secret>" 127.0.0.1:8083/v1/clients
```

按 topic 过滤订阅：

```bash
curl -H "X-Admin-Secret: <secret>" '127.0.0.1:8083/v1/filter_subscriptions?filter_type=1,2,3&match_type=1&topic_name=/a'
```

通过管理 API 发布消息：

```bash
curl -X POST -H "X-Admin-Secret: <secret>" 127.0.0.1:8083/v1/publish -d '{"topic_name":"a","payload":"test","qos":1}'
```

## 维护注意事项

- 该插件是高权限管理面，任何默认监听地址、认证、session、CORS 或发布 API 变更都需要安全审查。
- `http_auth_secret` 是管理面的应用层共享密钥边界；修改其语义（请求头名、会话回退、告警条件）必须同步 `config.go`、`http_auth.go` 与部署文档。
- `Indexer` 本身不做内部同步，调用方必须保证并发访问安全。
- 修改 topic、QoS、session 或订阅语义前，要确认不会破坏 MQTT 协议行为。
- 修改 `.proto` 后必须重新生成 Go gateway 文件和 Swagger，不要手工改生成物。

## 代码审查与重构建议

- 问题：Admin 插件同时承载外部 API、内存状态镜像和 broker 变更能力，容易把展示查询和可变操作混在一起。
- 改进方案：把只读查询、状态镜像、变更命令和 UI handler 分层审查，并为 publish/subscription 变更补 focused tests。
- 实施步骤：先固定 API 契约和鉴权边界，再拆分可测试 helper，最后用 gRPC/REST focused 用例验证。
- 预期效果：管理面权限边界更清晰，外部系统集成和发布审查更稳。
