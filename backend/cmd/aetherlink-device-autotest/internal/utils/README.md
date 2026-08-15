# internal/utils

## 目录职责

提供自动测试共享工具，包括测试数据生成、MQTT topic 构建、日志初始化和数据库/API 结果校验。

## 文件关系

`mqtt_topics.go` 被设备实现和测试用例共同使用。`validator.go` 校验数据库查询结果和平台响应。`data_builder.go` 为 CLI 冒烟和部分测试生成脱敏测试数据。`logger.go` 提供可选全局 logger。

## 重点文件

| 文件 | 说明 |
| --- | --- |
| `mqtt_topics.go` | 直连和网关 topic 模板。 |
| `validator.go` | 遥测、属性、事件、响应和时间戳校验。 |
| `data_builder.go` | 脱敏测试数据与 message_id 生成。 |
| `logger.go` | zap logger 初始化。 |

## 审查建议

topic 和 validator 是测试可信度的核心。`ParseMessageFromTopic` 现在按当前 MQTT 下行 topic 契约提取 `message_id`，后续继续为数值类型归一化、事件结构解析和更多 topic 模板增加表驱动测试。
