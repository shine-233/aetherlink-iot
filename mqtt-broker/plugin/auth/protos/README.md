# auth protos

## 目录定位

这个目录保存 auth 插件账号管理 API 的 protobuf 源定义，是账号列表、查询、更新和删除接口的契约来源。

## 文件用途

- `account.proto`：定义账号请求、响应、账号结构和 `AccountService` HTTP/gRPC 映射。
- `proto_gen.sh`：调用 `protoc` 生成 Go、grpc-gateway 和 Swagger 产物。

## 生成物、示例、测试数据边界

`.proto` 是人工维护源文件；生成的 Go 文件输出到上级 `plugin/auth`，Swagger JSON 输出到 `plugin/auth/swagger`。接口变更必须从 proto 开始，不能只改生成物。

## 审查发现

- 目录缺少 README，账号 API 的生成链路此前需要读脚本才能确认。
- 生成脚本依赖 `$GOPATH` 和 grpc-gateway 的 include 目录，版本漂移可能影响输出。
- proto 文件已有英文接口说明，本轮不对 proto 内容追加伪业务注释。

## 重构建议

- 将生成命令纳入统一工程脚本，并固定 `protoc` 插件版本。
- 账号接口涉及认证数据，后续字段变更应同步补充兼容性和安全审查说明。

## 验证建议

- 修改 `account.proto` 后运行 `proto_gen.sh` 并检查 Go/Swagger 产物。
- 结合 auth 插件单元测试或集成测试确认账号增删改查仍符合预期。
