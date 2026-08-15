# tests/gateway

## 目录职责

验证网关设备、多层拓扑、嵌套 payload、平台下行和数据库落库闭环。

## 文件关系

`testmain_test.go` 负责环境门禁。遥测、属性和事件上报测试依赖 `internal/protocol/gateway_builder.go` 构造嵌套结构；下行测试依赖 `internal/utils.NewGatewayMQTTTopics` 接收平台消息并通过 `internal/platform` 查询日志。

## 重点文件

| 文件 | 说明 |
| --- | --- |
| `telemetry_test.go` | 网关和子层级遥测上报。 |
| `telemetry_control_test.go` | 网关和子层级遥测控制下行。 |
| `attribute_publish_test.go` | 网关和子层级属性上报。 |
| `attribute_set_test.go` | 网关和子层级属性设置下行。 |
| `event_test.go` | 网关和子层级事件上报。 |
| `command_test.go` | 网关和子层级命令下发。 |
| `multilayer_independent_test.go` | 多层拓扑缺字段和组合边界。 |
| `testmain_test.go` | `AUTOTEST_EXTERNAL` 和数据库可达性门禁。 |
| `buildtag_boundary_test.go` | 无 build tag 时保留包边界。 |

## 审查建议

重点核对配置拓扑是否真实覆盖目标层级。若配置缺少子网关或子设备，相关分支不会执行，不能把该次测试结果解释为完整网关矩阵覆盖。
