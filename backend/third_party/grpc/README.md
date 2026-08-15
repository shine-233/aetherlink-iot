# 第三方 gRPC 适配

## 目录定位

本目录保存后端访问外部 gRPC 服务的兼容适配代码。当前主要包含 `tptodb_client`，仅在显式选择 TSDB、KINGBASE 或 POLARDB 时查询外部遥测/属性数据；默认本地存储路径不依赖该服务。

## 文件夹关系

- `tptodb_client`：对外部 `tp_to_db` gRPC 服务的客户端初始化和兼容封装。
- `tptodb_client/grpc_tptodb`：由 protobuf 生成的 gRPC 数据结构和客户端代码。

## 审查记录与重构建议

- 问题描述：生成代码和手写初始化代码放在同一树下，维护者容易误改 generated 文件。
- 改进方案：补充生成来源、命令和禁止手改说明，并在 README 中明确兼容边界。
- 实施步骤：找回或补齐 `.proto` 源文件，增加生成脚本，再把生成代码目录标记为只读维护面。
- 预期效果：降低协议漂移风险，提升外部服务升级时的可追溯性。

## 验证建议

- 修改手写初始化代码后运行 `go test ./third_party/grpc/tptodb_client -count=1`。
- 修改生成协议前必须在联调环境验证服务端兼容性。
