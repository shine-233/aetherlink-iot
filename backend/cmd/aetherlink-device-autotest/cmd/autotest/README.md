# cmd/autotest

## 目录职责

提供 `aetherlink-device-autotest` 的可执行入口，适合人工或脚本触发基础设备接入冒烟验证。

## 文件关系

`main.go` 是入口文件，依赖 `internal/config`、`internal/device` 和 `internal/utils`。它只发布基础数据，不直接查询数据库，也不执行 `tests/direct` 或 `tests/gateway` 的断言。

## 重点文件

| 文件 | 说明 |
| --- | --- |
| `main.go` | 解析 `-config` 和 `-mode`，连接设备，订阅 topic，并执行 telemetry、attribute、event 或 all 模式。 |

## 审查建议

优先检查 CLI 是否能清楚暴露失败原因。若要提升自动化可信度，建议把执行逻辑拆成 runner 并补充不依赖真实外部环境的单元测试。
