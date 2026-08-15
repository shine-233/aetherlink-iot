# tptodb 生成代码

## 目录定位

本目录保存外部 `tp_to_db` gRPC 协议生成的 Go 代码，属于协议兼容层，不是普通业务源码目录。

## 文件说明

- `tp_to_db.pb.go`：protobuf 消息结构和序列化相关代码。
- `tp_to_db_grpc.pb.go`：gRPC 客户端/服务端接口和方法路径定义。

## 维护注意事项

- 不要手工编辑本目录 generated 文件。
- 如果需要改协议名称、字段或方法，必须先修改 `.proto` 源文件，再用固定版本工具重新生成。
- 当前外部 wire name 仍可能被部署环境依赖，重命名需要兼容计划。

## 审查记录与重构建议

- 问题描述：仓库内暂未在本目录显式记录 proto 来源和生成命令。
- 改进方案：补充 `proto/` 源文件或生成脚本，并记录 protoc、protoc-gen-go、protoc-gen-go-grpc 版本。
- 实施步骤：确认协议来源后添加生成文档，再在 CI 中增加 generated diff 检查。
- 预期效果：减少手工修改生成代码导致的协议不一致。

## 验证建议

- generated 文件变更后至少运行 `go test ./third_party/grpc/tptodb_client/... -count=1`。
- 更可靠的验证需要连接兼容版本的外部 gRPC 服务。
