# tptodb gRPC 兼容客户端

## 目录定位

本目录封装后端访问外部 `tp_to_db` gRPC 服务的客户端。由于外部协议仍保留历史命名，当前代码以兼容为优先，不直接重写生成符号。

## 文件说明

- `doc.go`：说明包级协议边界和 generated 文件维护要求。
- `init.go`：读取配置、建立 gRPC 连接并暴露 `TelemetryQueryClient`。
- `grpc_tptodb/`：protobuf 生成代码目录，不建议手工编辑。

## 依赖关系

- 本地遥测存储是默认核心路径；`grpc.tptodb_type` 为空、`NONE` 或本地数据库类型时不会建立外部连接。
- 仅当 `grpc.tptodb_type` 显式设为 `TSDB`、`KINGBASE` 或 `POLARDB` 时，应用才读取 `grpc.tptodb_server` 并初始化本客户端。
- 被后端遥测、属性历史查询等业务间接调用。
- 依赖 `google.golang.org/grpc`，当前连接使用 insecure credentials，因此只适合受信内网。
- 外部 endpoint 缺失或客户端初始化失败会作为启动错误返回，不会触发进程级 panic。

## 审查记录与后续建议

- 已完成：外部客户端改为显式启用；默认本地部署不依赖 `tp_to_db`；初始化失败通过 `error` 返回。
- 已完成：初始化使用带超时的阻塞拨号；重复初始化仅在新连接可用后替换旧连接；`Close` 幂等释放连接并清空包级状态。
- 待改进：当前仍仅支持受信内网的 insecure credentials，且全局 client 不利于业务测试隔离。
- 建议后续增加显式可选 TLS 配置和接口注入，同时保留 generated wire contract；不要在未配置证书的现有私有部署中强制启用 TLS。

## 验证建议

- 修改初始化逻辑后运行 `go test ./internal/app ./internal/dal ./third_party/grpc/tptodb_client/... -count=1`。
- 协议升级需配合真实 gRPC 服务做联调验证。
