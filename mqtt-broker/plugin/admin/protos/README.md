# admin protos

## 目录定位

这个目录保存 admin 插件对外管理 API 的 protobuf 源定义，是生成 gRPC、grpc-gateway 和 Swagger 文档的源头。

## 文件用途

- `client.proto`：客户端列表、查询和断开连接接口定义。
- `publish.proto`：管理端向 broker 发布消息的接口定义。
- `subscription.proto`：订阅列表、过滤、订阅和取消订阅接口定义。
- `proto_gen.sh`：调用 `protoc` 生成 Go、grpc-gateway 和 Swagger 产物。

## 生成物、示例、测试数据边界

`.proto` 文件是人工维护的接口契约；生成的 Go 文件输出到上级 `plugin/admin` 目录，Swagger JSON 输出到 `plugin/admin/swagger`。不要直接手改生成产物来改变接口，应先修改 `.proto` 再重新生成。

## 审查发现

- 当前目录此前缺少 README，生成入口和输出位置不够直观。
- `proto_gen.sh` 依赖 `$GOPATH` 下的 grpc-gateway 相关 include 路径，环境不一致时容易生成失败。
- proto 内部已有接口注释，本轮不额外给 proto 文件伪造中文业务注释。

## 重构建议

- 后续可把生成命令纳入 Makefile 或脚本入口，统一检查 `protoc`、插件版本和 include 路径。
- 可在接口变更时补充兼容性说明，明确字段号不可复用、HTTP 路由不可随意破坏。

## 验证建议

- 修改 `.proto` 后运行本目录的 `proto_gen.sh`，再检查上级 Go 产物和 `../swagger` 是否同步。
- 修改生成脚本后建议运行 admin 插件相关 Go 测试，并比对生成产物 diff。
