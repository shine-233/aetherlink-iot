# internal/config

## 目录职责

定义设备自动测试工具的配置结构，负责从 YAML 文件加载 MQTT、设备、网关、数据库、API 和测试参数。

## 文件关系

`config.go` 被 CLI、设备工厂、API client、DB client 和集成测试共同使用。`device_type` 决定 `internal/device/factory.go` 创建直连设备还是网关设备。

## 重点文件

| 文件 | 说明 |
| --- | --- |
| `config.go` | 配置结构体、Load、Validate 和 PostgreSQL DSN 生成。 |

## 审查建议

重点检查默认值、必填字段、本地配置和真实测试环境是否一致。网关拓扑字段与 `internal/device/topology.go` 存在重复定义，修改时要同步审查。
