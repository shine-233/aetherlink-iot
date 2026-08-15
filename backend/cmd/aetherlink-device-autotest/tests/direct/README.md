# tests/direct

## 目录职责

验证直连设备与平台之间的 MQTT 上报、API 下行、设备响应和数据库落库闭环。

## 文件关系

`testmain_test.go` 负责环境门禁。其他测试通过 `internal/device` 创建设备，通过 `internal/platform` 下发 API 或查询数据库，通过 `internal/utils` 校验结果。

## 重点文件

| 文件 | 说明 |
| --- | --- |
| `telemetry_test.go` | 遥测上报和遥测控制。 |
| `attribute_test.go` | 属性上报和属性设置响应。 |
| `event_test.go` | 事件上报和事件响应格式。 |
| `command_test.go` | 命令下发、设备响应和命令日志。 |
| `testmain_test.go` | `AUTOTEST_EXTERNAL` 和数据库可达性门禁。 |
| `buildtag_boundary_test.go` | 无 build tag 时保留包边界。 |

## 审查建议

优先检查每个用例是否有唯一数据标识和严格时间窗口。当前部分查询取最新记录，运行环境若残留历史数据，可能影响断言可信度。
