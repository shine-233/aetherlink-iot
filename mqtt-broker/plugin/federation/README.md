# Federation 插件维护说明

**警告：这是实验性功能，尚未经过生产环境验证。**

Federation 是 gmqtt broker 的实验性集群机制，目标是在多个 broker 节点之间同步订阅、取消订阅、保留消息和发布事件，让多个节点在一定范围内“像一个 broker 一样”协作。它不是完整的分布式 MQTT 会话实现，维护时必须保留以下限制说明：

1. 持久会话不能从另一个节点恢复。
2. 使用相同 client id 的客户端可以同时连接到不同节点，当前实现不会跨节点踢除重复客户端。

根因是会话状态仍保存在本地节点，未在节点之间共享。本文档面向维护者，重点说明目录定位、关键文件关系、配置与运行示例、REST API、实现边界，以及后续代码审查和重构建议。

## 目录定位

当前目录：`mqtt-broker/plugin/federation`

该目录维护 federation 插件本身，职责包括：

- 通过 Serf 维护节点成员关系。
- 通过内部 gRPC 服务完成节点间握手、事件流传输和 ack。
- 通过 grpc-gateway 暴露成员管理 REST API。
- 维护本地订阅索引和远端 federation 订阅树，用于跨节点路由消息。
- 在 broker hook 中捕获订阅、取消订阅、消息到达、会话终止和遗嘱发布等事件。

相邻目录说明：

- `examples/`：本地多节点示例配置。当前实际文件为 `node1_config.yml`、`node2_config.yml`、`join_node3_config.yml`。
- `protos/`：federation 和 membership 的 protobuf/gRPC/HTTP 映射定义。
- `swagger/`：由 proto 生成的 REST API swagger 描述。

## 关键文件关系

- `config.go`：插件配置结构、默认值、YAML 反序列化和地址校验。维护配置字段时要同步本文档和 examples。
- `federation.go`：插件生命周期、gRPC 服务实现、成员管理 API、peer 认证、会话状态、订阅状态和消息转换。
- `membership.go`：消费 Serf 成员事件，处理节点加入、更新、失败和离开后的 peer 生命周期。
- `peer.go`：远端 peer 连接、握手、事件队列、重连、事件重放和 ack 处理。
- `hooks.go`：将 broker 本地事件转换成 federation 事件，并在消息到达时根据 federation 订阅树决定跨节点转发。
- `protos/federation.proto`：Membership REST API 与 Federation gRPC 流的源契约。
- `federation*.pb.go`、`federation*.pb.gw.go`、`*_grpc.pb.go`、`*_mock.go`：生成代码或测试 mock。除非重新生成，不要手工改生成文件。
- `*_test.go`：当前插件单元测试覆盖配置、peer、hook、membership 和 federation 行为。本文档维护不要求运行这些测试。

## 配置说明

配置挂载在 broker 配置的 `plugins.federation` 下，示例见 `examples/*.yml`。

```yaml
plugins:
  federation:
    node_name: node1
    peer_secret: "change-me"
    fed_addr: :8901
    advertise_fed_addr: :8901
    gossip_addr: :8902
    advertise_gossip_addr: :8902
    retry_join:
      - 127.0.0.1:8912
    retry_interval: 5s
    retry_timeout: 1m
    snapshot_path:
    rejoin_after_leave: false
```

字段维护要点：

- `node_name`：节点唯一标识，默认使用主机名。跨节点必须唯一。
- `peer_secret`：federation gRPC peer 调用共享密钥。当前服务端在 `Hello` 和 `EventStream` 上要求 `peer_secret` 元数据匹配；为空会导致 peer 认证失败。示例配置当前未显式设置该字段，维护或实测集群时应补齐一致密钥。
- `fed_addr`：内部 federation gRPC 监听地址，默认端口 `8901`。
- `advertise_fed_addr`：向其他节点广播的 federation gRPC 地址。多机部署时应使用其他节点可路由地址。
- `gossip_addr`：Serf gossip 监听地址，同时用于 UDP/TCP gossip，默认端口 `8902`。
- `advertise_gossip_addr`：向其他节点广播的 gossip 地址。多机部署时应使用其他节点可路由地址。
- `retry_join`：启动时尝试加入的其他节点 gossip 地址；缺省端口为 `8902`。
- `retry_interval`：重试加入间隔，默认 `5s`，必须大于 0。
- `retry_timeout`：加入超时时间，默认 `1m`，必须大于 0。
- `snapshot_path`：传递给 Serf snapshot，用于记住历史节点并避免重放旧 user event。
- `rejoin_after_leave`：传递给 Serf `RejoinAfterLeave`。默认 `false`，节点显式 leave 后不会自动重新加入，除非收到新的 join 或重启后按配置加入。

## 本地运行示例

以下命令只说明如何使用当前 examples 文件，不代表本文档维护时需要启动 broker。示例路径以当前目录 `mqtt-broker/plugin/federation` 为基准。

启动 node1：

```bash
gmqttd start -c ./examples/node1_config.yml
```

启动 node2：

```bash
gmqttd start -c ./examples/node2_config.yml
```

node1 和 node2 的示例配置通过 `retry_join` 指向彼此的 gossip 地址。启动后可以用 `mosquitto_pub/sub` 做跨节点消息路由冒烟验证：

```bash
mosquitto_sub -t topicA -h 127.0.0.1 -p 1884
```

```bash
mosquitto_pub -t topicA -m 123 -h 127.0.0.1 -p 1883
```

如果 federation、端口、peer 认证和本地 broker 均正常，订阅端应收到：

```text
123
```

启动第三个节点但暂不通过 `retry_join` 自动加入：

```bash
gmqttd start -c ./examples/join_node3_config.yml
```

当前 examples 中 node3 的 MQTT/API/federation/gossip 端口分别为 `1885`、`8283/8284`、`8931`、`8932`。

## REST API 示例

REST API 源契约位于 `protos/federation.proto`，swagger 输出位于 `swagger/federation.swagger.json`。以下示例假设 node1 HTTP API 监听 `127.0.0.1:8083`，node3 gossip 地址是 `127.0.0.1:8932`。

让 node1 请求 node3 加入 federation：

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"hosts":["127.0.0.1:8932"]}' \
  http://127.0.0.1:8083/v1/federation/join
```

成功响应通常为空 JSON：

```json
{}
```

查询成员：

```bash
curl http://127.0.0.1:8083/v1/federation/members
```

示例响应：

```json
{
  "members": [
    {
      "name": "node1",
      "addr": "127.0.0.1:8902",
      "tags": {
        "fed_addr": "127.0.0.1:8901"
      },
      "status": "STATUS_ALIVE"
    },
    {
      "name": "node2",
      "addr": "127.0.0.1:8912",
      "tags": {
        "fed_addr": "127.0.0.1:8911"
      },
      "status": "STATUS_ALIVE"
    },
    {
      "name": "node3",
      "addr": "127.0.0.1:8932",
      "tags": {
        "fed_addr": "127.0.0.1:8931"
      },
      "status": "STATUS_ALIVE"
    }
  ]
}
```

看到 3 个成员且状态为 `STATUS_ALIVE`，表示这 3 个节点在 Serf 成员视图中均处于存活状态。

优雅离开当前节点：

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{}' \
  http://127.0.0.1:8083/v1/federation/leave
```

强制移除失败节点：

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"node_name":"node3"}' \
  http://127.0.0.1:8083/v1/federation/force_leave
```

`force_leave` 适合清理 Serf 视图中的 failed 成员；如果目标节点实际仍存活，它可能再次加入。

## 实现边界

### 节点间通信

节点既是 federation gRPC client，也是 federation gRPC server。核心内部 RPC：

```proto
service Federation {
  rpc Hello(ClientHello) returns (ServerHello) {}
  rpc EventStream(stream Event) returns (stream Ack) {}
}
```

- `Hello`：建立事件流前的握手，携带 session id，并返回是否需要 clean start 以及下一个期望 event id。
- `EventStream`：双向流，client 发送订阅、取消订阅和消息事件，server 处理成功后返回 ack。
- peer 元数据：调用方发送 `node_name`，并在配置了 `peer_secret` 时发送同名元数据；服务端要求本地 `peer_secret` 非空且匹配。

### 会话状态

federation 事件按“至少一次”语义传输，接近 MQTT QoS 1 的处理思路。client 和 server 通过 session id 关联状态。

client 侧 session 状态包括：

- 已发送但尚未 ack 的事件。
- 等待发送的事件。

server 侧 session 状态包括：

- session 是否存在。
- server 愿意接受的下一个 event id。
- 最近已见事件的 LRU，用于降低重复事件影响。

当前 session 状态只在内存中维护。client 启动时生成随机 session id；重连或发现新节点时先 `Hello` 握手。若 server 返回 `clean_start=true`，client 会重放当前本地订阅和保留消息作为全量状态；若返回 `clean_start=false`，client 从 `next_event_id` 开始继续发送增量事件。

### 订阅树和消息分发

每个节点维护两类订阅视图：

- 本地订阅树：由 gmqtt core 维护，保存连接到本节点的客户端订阅。
- federation 订阅树：由 federation 插件维护，保存远端节点同步过来的订阅，订阅者标识使用远端 `node_name`。

本地客户端订阅或取消订阅时，节点先更新本地索引，再把必要事件广播给 peers。收到远端订阅或取消订阅事件时，只更新 federation 订阅树。

消息分发流程：

- 保留消息会广播给所有 peers，用于更新远端保留消息状态。
- 非保留消息根据 federation 订阅树匹配远端节点。
- 共享订阅会在本地节点和远端节点之间选择一个目标，避免同一共享组被重复投递。
- 如果消息只应投递到远端共享订阅，当前节点可能通过 `Drop` 和 `IterationOptions` 调整本地投递行为。

### 成员管理

成员关系由 Serf 维护：

- `retry_join` 和 REST `Join` 最终调用 Serf join。
- `Leave` 调用 Serf leave，使其他节点看到 graceful leave 而非 failed。
- `ForceLeave` 调用 Serf failed-node 清理。
- 成员事件由 `membership.go` 转换成 peer 创建、替换、关闭和订阅树清理。

### 当前不保证的能力

- 不保证跨节点持久会话迁移。
- 不保证跨节点唯一 client id。
- 不保证生产级分布式 MQTT 语义完整性。
- federation 内部 gRPC 当前使用明文传输；如要引入 TLS，需要配置迁移和兼容方案。
- 事件队列是有界内存队列，溢出、重连和 clean start 需要重点回归。
- 文档示例是本地开发示例，不等同生产安全配置。

## 代码审查与重构建议

维护 federation 相关代码时优先审查以下风险：

- `peer_secret` 是否在所有 peer gRPC 路径中一致传递和校验，避免空密钥或错误密钥被误当成可用配置。
- `Hello`、`EventStream`、ack、重连、队列溢出和 clean start 的组合是否会重复投递、漏投递或卡住。
- Serf 成员更新时是否正确替换旧 peer、关闭旧 stream，并清理远端订阅树。
- 共享订阅、保留消息和遗嘱消息在跨节点场景下是否符合当前插件预期。
- `force_leave`、`leave` 和节点重启场景是否与 `snapshot_path`、`rejoin_after_leave` 一致。
- examples 与 `config.go` 字段是否同步，尤其是安全默认值。

推荐重构方向：

- 将 peer 认证、session 管理、事件队列、成员事件处理拆成更深的内部模块，降低 `federation.go` 和 `peer.go` 的职责密度。
- 为事件队列溢出、clean start 全量重放、共享订阅路由和 retained replay 增加更明确的契约测试。
- 将 examples 升级为可直接说明安全默认值的维护样例，避免无密钥示例被复制到真实环境。
- 如需修改 REST/gRPC 契约，应先改 `protos/federation.proto`，再重新生成 pb/gateway/swagger/mock，并把生成命令记录到维护文档。
