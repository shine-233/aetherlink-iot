# AetherLink Device Autotest

## 功能定位

`aetherlink-device-autotest` 是 AetherLink IoT 外部设备接入自动化测试工具，面向真实 MQTT broker、平台 API 和 PostgreSQL 环境，验证设备接入链路是否符合接入规范。

它覆盖两类设备模型：

- `direct`：直连设备，直接按照设备协议上报遥测、属性、事件，并响应平台下行。
- `gateway`：网关设备，模拟网关、多级子网关和子设备的嵌套上报与下行响应。

这套目录更接近“集成测试支撑工程”而不是普通库代码：`internal` 中封装协议、设备、平台访问和断言工具，`tests` 中的用例依赖真实外部环境执行。

## 目录关系

```text
aetherlink-device-autotest/
|-- cmd/
|   `-- autotest/
|       `-- main.go
|-- internal/
|   |-- config/
|   |-- device/
|   |-- platform/
|   |-- protocol/
|   `-- utils/
|-- tests/
|   |-- direct/
|   `-- gateway/
|-- config-community.yaml
`-- config-gateway-community.yaml
```

核心调用链如下：

1. `cmd/autotest/main.go` 读取 YAML 配置并初始化日志。
2. `internal/config` 解析配置并做启动前字段校验。
3. `internal/device/factory.go` 根据 `device_type` 创建直连或网关设备实例。
4. `internal/device` 负责 MQTT 连接、主题订阅、消息缓存、上报和响应。
5. `internal/protocol` 负责把测试数据转换为平台协议期望的 JSON payload。
6. `internal/platform` 负责调用平台 API 触发下行、查询 PostgreSQL 验证落库结果。
7. `internal/utils` 提供 topic 生成、脱敏测试数据、日志与断言工具。
8. `tests/direct` 与 `tests/gateway` 组合上述能力形成外部集成用例。

## 子目录职责

| 路径 | 主要职责 |
| --- | --- |
| `cmd/autotest` | CLI 入口，适合做设备接入冒烟验证。 |
| `internal/config` | YAML 配置模型、默认值和必填项校验。 |
| `internal/device` | 设备接口、直连实现、网关实现、设备工厂、拓扑模型。 |
| `internal/platform` | 平台 HTTP API 客户端与 PostgreSQL 验证查询。 |
| `internal/protocol` | 直连/网关消息体构造器。 |
| `internal/utils` | MQTT topic、测试数据、校验工具、日志工具。 |
| `tests/direct` | 直连设备真实环境集成测试。 |
| `tests/gateway` | 网关与多层拓扑真实环境集成测试。 |
## 关键文件关系

- `internal/config/config.go` 与根目录 YAML 本地配置必须同步，否则启动校验和文档会漂移。
- `internal/utils/mqtt_topics.go` 中的 topic 模板被 `internal/device/*` 和测试用例共同依赖，是协议契约核心。
- `internal/protocol/direct_builder.go` 与 `internal/protocol/gateway_builder.go` 决定 MQTT payload 结构，必须与项目根目录的接入规范、验证说明和数据库口径保持一致。
- `internal/platform/api_client.go` 发起平台下行；`internal/device/*` 订阅对应下行 topic 并回传响应；`internal/platform/db_client.go` 最终验证平台侧持久化结果。
- `internal/device/topology.go` 与 `internal/config/config.go` 都描述了网关拓扑，当前存在重复建模，需要审查两边是否持续一致。

## 静态审查重点

### 协议契约

- 检查 `internal/utils/mqtt_topics.go` 的 topic 是否与项目根目录接入规范和真实平台配置保持一致。
- 检查 `internal/protocol` 输出字段是否与平台消费协议一致，尤其是 `result`、`message`、`ts`、`method` 以及网关嵌套字段名。
- 检查直连和网关事件上报是否对同一“事件”概念使用了不同 payload 约定。

### 配置与拓扑

- 检查 `internal/config/config.go` 的 `Validate` 是否覆盖不同设备类型真正需要的最小字段。
- 检查 `internal/device/topology.go` 与 `internal/config` 的网关配置模型是否已经发生字段漂移。
- 检查本地配置文件是否能完整表达 README 与测试用例依赖的场景。

### 稳定性与可测性

- 检查 `internal/device/direct_device.go` 与 `internal/device/gateway_device.go` 中重复的连接、订阅、缓存和 topic 匹配逻辑是否一致。
- 检查 `GetReceivedMessages` 的轮询与超时策略是否足够稳健，是否可能因为历史消息未清理导致误判。
- 检查 `internal/platform/db_client.go` 中“取最新一条记录”的查询是否可能误匹配旧数据。
- 检查 `internal/utils/validator.go` 中仍偏弱的数值归一化、事件结构解析和 topic 契约边界是否会影响断言可信度。

## 已知重构建议

1. 将 `direct_device.go` 与 `gateway_device.go` 的公共 MQTT 行为抽成共享适配层，减少双实现漂移。
2. 合并 `internal/device/topology.go` 与 `internal/config/config.go` 中重复的拓扑结构定义。
3. 为 `internal/protocol` 和 `internal/utils/mqtt_topics.go` 增加契约 fixture 测试或表驱动校验，降低协议回归风险。
4. 为 `internal/platform/db_client.go` 的查询增加时间窗口或 `message_id` 级过滤，减少历史数据误判。
5. 继续扩展 `internal/utils/validator.go` 的表驱动测试，覆盖更多 topic 模板、数值归一化和事件结构负例。

## 文档使用建议

- 阅读源码时先看本 README，再按 `internal/config -> internal/device -> internal/protocol -> internal/platform -> tests` 的顺序进入。
- 做静态审查时，优先对照项目根目录的接入规范、验证文档和数据库表说明，确认 topic、payload、字段命名和数据库查询口径一致。
- 这套目录依赖真实外部环境，不能仅凭 `go test` 通过就断言业务逻辑和平台契约没有问题。
