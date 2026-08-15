# internal/device

## 功能定位

本目录负责“模拟设备”这一层：为测试提供统一 `Device` 抽象，并分别实现直连设备与网关设备的 MQTT 行为。

这里是测试代码与外部 MQTT 协议之间的边界，topic、连接、订阅、消息缓存和响应回传都集中在这一层。

## 文件关系

| 文件 | 作用 |
| --- | --- |
| `device.go` | 定义统一设备接口和接收消息模型。 |
| `factory.go` | 按 `device_type` 分派到直连或网关实现。 |
| `direct_device.go` | 直连设备 MQTT 连接、发布、订阅、缓存和 topic 匹配。 |
| `gateway_device.go` | 网关设备 MQTT 连接、发布、订阅、缓存和拓扑感知日志。 |
| `topology.go` | 网关、多级子网关和子设备拓扑模型。 |

实际依赖关系：

- `factory.go` 依赖 `config.Config` 做设备类型分派。
- `direct_device.go` 与 `gateway_device.go` 都依赖 `internal/protocol` 生成 payload。
- 两种设备实现都依赖 `internal/utils/mqtt_topics.go` 生成 topic。
- `tests/direct` 与 `tests/gateway` 只应该依赖 `Device` 接口，而不是直接依赖具体实现细节。

## 静态审查建议

### 直连/网关双实现是否漂移

- 比较两份实现里的连接参数、超时、订阅集合、日志和缓存行为是否一致。
- 特别关注修 bug 时是否只修了一边。

### topic 匹配与消息缓存

- `matchTopic` 只实现了最小通配语义，审查时要确认足够覆盖当前测试用例依赖的 topic 模式。
- `GetReceivedMessages` 采用轮询读取缓存，若历史消息没有及时清理，可能影响测试稳定性。

### 拓扑模型重复

- `topology.go` 与 `internal/config/config.go` 都在描述网关拓扑。
- 如果字段未来发生变更，应优先合并建模，避免测试配置与运行结构不一致。

## 重构建议

1. 抽出共享 MQTT 基类或适配层，收敛重复连接和缓存逻辑。
2. 让 topic 匹配和消息缓存成为独立可测组件。
3. 合并重复拓扑模型，只保留一份权威定义。
