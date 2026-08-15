# internal/protocol

## 目录职责

构建设备自动测试发布到 MQTT 的 JSON payload，区分直连设备扁平结构和网关设备嵌套结构。

## 文件关系

`message_builder.go` 定义统一接口。`direct_builder.go` 服务直连设备实现，`gateway_builder.go` 服务网关设备实现和多层拓扑测试。

## 重点文件

| 文件 | 说明 |
| --- | --- |
| `message_builder.go` | payload builder 接口。 |
| `direct_builder.go` | 直连 telemetry、attribute、event、response 构建。 |
| `gateway_builder.go` | 网关嵌套 payload 与辅助构造函数。 |

## 审查建议

优先对照项目根目录接入规范、验证文档和本地脱敏协议资料核对字段名和结构层级。建议用 golden JSON fixture 固定直连与网关输出，避免 map 结构悄悄偏离协议。
