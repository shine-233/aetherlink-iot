# federation protos

## 目录定位

这个目录保存 federation 插件的 protobuf 源定义，覆盖节点握手、事件流、成员管理和集群加入等跨节点接口。

## 文件用途

- `federation.proto`：定义 federation 事件、握手、成员状态和服务接口。
- `proto_gen.sh`：调用 `protoc` 生成 Go、grpc-gateway 和 Swagger 产物。

## 生成物、示例、测试数据边界

`.proto` 是人工维护的跨节点通信契约；生成的 Go 文件输出到上级 `plugin/federation`，Swagger JSON 输出到 `plugin/federation/swagger`。跨节点协议字段号和 oneof 结构需要保持兼容，不能通过直接修改生成物来变更协议。

## 审查发现

- 目录此前缺少 README，协议源和产物关系不够清楚。
- federation 协议涉及集群成员和事件复制，字段变更风险高于普通管理 API。
- 本轮不向 proto 文件追加中文业务注释，避免伪造协议语义。

## 重构建议

- 为协议变更增加兼容性清单，特别关注字段号保留、oneof 扩展和旧节点互通。
- 将 `protoc` 版本与 grpc-gateway 插件版本固定到统一生成入口。

## 验证建议

- 修改 proto 后运行 `proto_gen.sh` 并检查 Go/Swagger 产物。
- 结合 federation 单元测试或多节点示例启动，验证握手、事件流和 join 行为。
