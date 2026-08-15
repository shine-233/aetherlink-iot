# MQTT 控制包编解码包

`mqtt-broker/pkg/packets` 负责 MQTT 控制包的线协议编解码、固定头处理、可变头/负载解析、MQTT v5 属性读写以及测试用 mock。它处在 broker 网络入口和业务会话逻辑之间，任何行为变化都可能影响客户端互操作性。

## 包定位

- 提供 `Packet` 接口、控制包类型注册、固定头读写和公共二进制读写 helper。
- 覆盖 MQTT 3.x 与 MQTT 5 的连接、发布、订阅、确认、心跳、断开和认证控制包。
- 面向 broker 协议层使用，不承载设备业务语义、ACL 决策或会话存储逻辑。

## 主要文件

- `packets.go`: 公共接口、控制包类型、固定头、剩余长度和基础读写工具。
- `properties.go`: MQTT v5 属性集合的编码、解码和属性合法性支撑。
- `connect.go` / `connack.go`: 客户端连接握手和服务端连接确认。
- `publish.go`: PUBLISH 数据通路控制包，处理主题、QoS、包标识符、属性和负载。
- `puback.go` / `pubrec.go` / `pubrel.go` / `pubcomp.go`: QoS 1/2 发布确认链路。
- `subscribe.go` / `suback.go`: 订阅请求与订阅确认。
- `unsubscribe.go` / `unsuback.go`: 取消订阅请求与取消订阅确认。
- `pingreq.go` / `pingresp.go`: keepalive 心跳请求与响应。
- `disconnect.go` / `auth.go`: MQTT 5 断开和增强认证相关控制包。
- `*_test.go`: 各控制包的样例解析、编码往返和畸形输入覆盖。
- `packets_mock.go`: GoMock 生成的 `Packet` 接口 mock，仅用于测试替身。

## MQTT 控制包关系

- 握手链路: `CONNECT -> CONNACK`，决定协议版本、认证输入、会话恢复和连接属性。
- 数据发布链路: `PUBLISH` 是主数据入口；QoS 1 通过 `PUBACK` 完成确认，QoS 2 通过 `PUBREC -> PUBREL -> PUBCOMP` 完成四步握手。
- 订阅链路: `SUBSCRIBE -> SUBACK` 返回每个主题过滤器的订阅结果。
- 退订链路: `UNSUBSCRIBE -> UNSUBACK` 返回取消订阅处理结果。
- 保活链路: `PINGREQ -> PINGRESP` 用于 keepalive 存活探测。
- 会话结束/认证链路: `DISCONNECT` 传递断开原因和属性，`AUTH` 支撑 MQTT 5 增强认证交换。

## 审查发现

- 该目录是线协议敏感层，主要风险不在代码风格，而在字节级兼容性、剩余长度、属性长度、固定头标志和原因码合法性。
- 多个控制包存在相似的原因码、属性、包标识符和确认类编码逻辑，当前实现可读但重复度较高。
- 测试已经覆盖常见 round trip 和部分畸形输入，但协议边界仍建议继续补强，尤其是固定头标志、属性长度、QoS 状态流转和大负载场景。
- `packets_mock.go` 是生成文件，当前仅补充维护说明；不应在其中承载手写业务逻辑。

## 重构建议

- 将固定头解析、剩余长度校验和控制包类型注册拆成更独立的小模块，降低审查成本。
- 为 PUBACK/PUBREC/PUBREL/PUBCOMP 抽取确认类控制包通用 helper，减少重复编码。
- 将 MQTT v5 属性元数据表与二进制读写流程分离，方便单独校验属性合法性。
- 将 SUBSCRIBE/UNSUBSCRIBE 的主题过滤器校验沉淀为共享 helper，并明确两者语义差异。
- 保持生成 mock 的生成流程可追溯，接口拆分后同步更新 mock，而不是手写补丁。

## 验证建议

- 每次改动后至少执行 `gofmt -w pkg/packets` 和 `go test ./pkg/packets -count=1`。
- 对控制包行为变更补充字节级 golden 样例、畸形包样例和编码/解码往返测试。
- 对 PUBLISH、SUBSCRIBE 和 QoS 2 确认链路优先增加边界测试，因为它们直接影响数据通路和消息投递语义。
- 对 MQTT v5 属性变更增加属性长度、重复属性、未知属性和控制包适用性测试。
- 如改动固定头或剩余长度逻辑，应同时运行更广的 broker 协议测试，避免跨控制包回归。
