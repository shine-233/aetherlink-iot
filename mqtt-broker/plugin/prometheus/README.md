# Prometheus 指标插件

`mqtt-broker/plugin/prometheus` 提供 broker 运行指标导出能力，默认通过 `127.0.0.1:8082/metrics` 暴露 Prometheus 格式数据，并提供轻量 dashboard。

## 目录定位

- 读取 broker runtime stats 并转换为 Prometheus 指标。
- 提供 `/metrics` HTTP exporter。
- 提供一个只读 dashboard 页面，读取 metrics endpoint 展示核心指标。
- 保持插件被动：正常情况下只读取统计信息，不包裹或修改 broker hooks。

## 关键文件关系

- `prometheus.go`：插件注册、HTTP server、collector、dashboard template、统计到指标的转换。
- `config.go`：监听地址和 metrics path 默认配置。
- `hooks.go`：显式返回空 hook wrapper，证明插件不改变 broker 行为。
- `prometheus_test.go`：覆盖指标输出契约、collector 注册回滚、监听失败和 reload 行为。
- `hooks_test.go`：验证插件保持被动。

## 指标名前缀

当前代码中的指标名前缀是 `gmqtt_`，例如 `gmqtt_clients_connected_total`。这是运行时兼容契约，不应只为品牌统一而改名。若未来迁移到 `aetherlink_` 前缀，需要提供双写或兼容窗口，并同步 dashboard、告警规则和外部监控配置。

## 主要指标

| 指标名 | 类型 | 标签 |
| --- | --- | --- |
| `gmqtt_clients_connected_total` | Counter | 无 |
| `gmqtt_clients_disconnected_total` | Counter | 无 |
| `gmqtt_messages_dropped_total` | Counter | `qos`, `type` |
| `gmqtt_messages_inflight_current` | Gauge | 无 |
| `gmqtt_messages_queued_current` | Gauge | 无 |
| `gmqtt_messages_received_total` | Counter | `qos` |
| `gmqtt_messages_sent_total` | Counter | `qos` |
| `gmqtt_packets_received_bytes_total` | Counter | `type` |
| `gmqtt_packets_received_total` | Counter | `type` |
| `gmqtt_packets_sent_bytes_total` | Counter | `type` |
| `gmqtt_packets_sent_total` | Counter | `type` |
| `gmqtt_sessions_created_total` | Counter | 无 |
| `gmqtt_sessions_terminated_total` | Counter | `reason` |
| `gmqtt_sessions_active_current` | Gauge | 无 |
| `gmqtt_sessions_inactive_current` | Gauge | 无 |
| `gmqtt_subscriptions_current` | Gauge | 无 |
| `gmqtt_subscriptions_total` | Counter | 无 |

Packet 指标的 `type` 是有限 MQTT 控制包集合，包括 MQTT 5 的 `AUTH`。过期会话通过 `gmqtt_sessions_terminated_total{reason="expired"}` 表示；QoS inflight 超时丢弃通过 `gmqtt_messages_dropped_total{type="inflight_expired"}` 表示。

## 维护注意事项

- 指标名、label 和类型属于外部监控契约，修改前必须写迁移说明。
- collector 注册失败要能回滚，否则 reload 后可能无法重新注册。
- dashboard 改动应和指标契约改动分开审查。
- 新增指标前要确认它来自稳定 runtime stats，而不是临时调试值。

## 代码审查与重构建议

- 问题：监控插件看似只读，但指标名变更会影响外部告警和仪表盘。
- 改进方案：把 collector 注册、stats 转换和 dashboard 读取拆成独立验证点。
- 实施步骤：先用 fixture 固定当前指标输出，再新增或迁移指标，最后更新 dashboard 和告警文档。
- 预期效果：可观测性接口更稳定，发布后监控回归风险更低。
