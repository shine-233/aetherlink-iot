# edgemqttbroker —— 边缘验证 broker 与本地规则执行器（TB-21 scoped v1）

`cmd/edgemqttbroker` 是边缘转发/实体下发 E2E 用的最小 MQTT broker（无鉴权/保留消息，非生产 broker），
同时承载**边缘本地规则执行器**的可选接线（ROADMAP TB-21 scoped v1）。

## 边缘本地规则执行器（可选开关，默认关闭）

云端通过 edge_sync 下发规则链 Graph 快照（`service/edge_sync.go` 投递到
`devices/command/{gateway_device_number}`，信封含 `revision` 修订号）。本开关让 broker
充当最小边缘消费方，在**断云期间本地求值并产生本地告警事件**：

```bash
go build -o edgemqttbroker.exe ./cmd/edgemqttbroker
./edgemqttbroker.exe -addr 127.0.0.1:18883 -edgerules \
  -edgerules-alarm-log edge-alarms.jsonl \
  -edgerules-telemetry "aetherlink/edge/+/+" \
  -edgerules-snapshot-prefix "devices/command/"
```

| flag | 默认 | 说明 |
| --- | --- | --- |
| `-edgerules` | `false` | 总开关（默认关闭，关闭时零行为变化） |
| `-edgerules-telemetry` | `aetherlink/edge/+/+` | 遥测主题过滤器，对照 `internal/edgeforward` 转发主题 `{prefix}/{type}/{device_id}` |
| `-edgerules-snapshot-prefix` | `devices/command/` | 快照命令主题前缀，对照 mqtt 配置 `commands.publish_topic` |
| `-edgerules-alarm-log` | 空（仅日志） | 本地告警事件 JSONL 追加文件 |

另有 `-pub-addr`（默认取 `-addr`）：`-pub` 注入模式实际连接的 broker 地址，便于向独立 broker 发快照/遥测。

### v1 语义边界（诚实边界）

- **节点子集**：只支持 `trigger.telemetry`（入口）、`filter.threshold`（条件）、`action.alarm`（动作）。
  云端 28 种节点类型中其余类型装载通过但**求值时阻断其路径**（fail-closed：告警绝不跨过
  未求值的条件节点），不支持的类型在装载日志中通过 `unsupported` 暴露。
- **阈值语义**：对照云端 `rule_chain_engine.go` 逐条一致——点位缺失/非数值=不通过；
  六种算子 `> >= < <= == !=`；数值字符串可参与比较。op 白名单与 NaN/Inf 在装载期强制（比云端更严）。
- **告警语义**：对照 `internal/calcfield` 的简化版——单节点单条件（`遥测key 比较阈值`），
  无 govaluate 表达式、无严重度阶梯升级、无自动清除、无拓扑传播。severity L/M/H（默认 H）。
- **重连去重**：快照信封 `revision`（对照 `service/edge_sync_revision.go` 单调递增）驱动四态——
  未变（重连/重放同一任务）跳过不重复执行；回退跳过并告警（边缘超前于云端，异常）；
  升级整图替换重装载；非法（<1）拒收保留当前版本。
- **re-trigger 去重**：`action.alarm` 的 `retrigger_dedup_ms` 窗口内同 resource+node+device
  不重复落事件（对照云端 action.alarm 的 node+device 窗口语义）。
- **本地告警**：只产生本地事件（结构体 + JSONL/日志），**不与云端告警收敛去重**——
  断云期间本地动作的云端去重收敛属 TB-21 后续批次（需活栈演练）。
- `trigger.device_online` 等其余触发类型不驱动遥测求值；`failure` 边结构保留但 v1 不走。

### 生产边缘运行时

本 broker 仅是验证栈演示。生产边缘代理应直接 import `aetherlink-iot/backend/internal/edgerules`
（零 DB/零 broker 依赖，全部核心逻辑有 Go 单测），按同样形状的主题投喂
`Executor.LoadSnapshot` / `Executor.ProcessTelemetry`，并实现 `edgerules.EventSink` 接入本地存储。
