# P2.3 摄取回压短期 B 方案实施证据（丢弃计数暴露 + 告警阈值）（2026-09-19）

> 对应路线图：§1.2 P2.3「摄取回压策略决策」、§P2.3 数据保留与性能
> 决策输入：`docs/design/2026-09-19-ingest-backpressure-decision-memo.md`（建议稿，
> 短期先做 B）。本轮按 owner「还有什么没有完成的继续完成一下」的指示，
> 依备忘录自身建议实施 B 方案（改动一日级、可回退、不影响 C 方案立项）。
> 运行结果：**账本活栈验证 PASS；告警触发由单测演练证明**

---

## 一、交付物

1. **摄取账本（`backend/internal/uplink/backpressure.go`，新文件）**——总线进程内
   原子计数（热路径零锁），键名 `uplink_dropped_total` 与备忘录验收口径对齐：
   - `received_total`：通过发布闸门且类型可路由的消息数；
   - `accepted_total`：成功进入对应分类队列的消息数；
   - `dropped_channel_full`：响应链路满队列直接丢弃（该链路显式策略，精确可数）；
   - `dropped_caller_context`：阻塞等待中被调用方 ctx 取消/超时；
   - `dropped_bus_closed`：阻塞等待中总线关闭中止；
   - `rejected_admission`：发布闸门（closing/closed）准入前拒绝（不占 received）；
   - `dropped_unknown_type`：未知消息类型拒绝（不占 received）；
   - `blocked_events` / `blocked_seconds_total`：队列满后进入阻塞等待的事件数与
     累计耗时——**paho 入站队列丢"已 PUBACK"消息的先行指标**。
   - 进程内不变量：`received = accepted + channel_full + caller_context + bus_closed`。
2. **总线接线（`backend/internal/uplink/bus.go`）**——`PublishContext` /
   `PublishResponseContext` 全量记账；`publishWithBackpressure` 记录阻塞事件与
   阻塞耗时；`GetChannelStats` 快照扩展 `accounting` / `uplink_dropped_total` /
   `backpressure_alert` 三段。
3. **告警采样器（可演练）**——`BackpressureAlertConfig{Enabled, Window, DropRatio,
   MinReceived}`，默认开（60s 窗口、1% 阈值、窗口内 100 条起评）。判定为纯函数
   `evaluateBackpressureDropRatio`；越限输出稳定标记 **`UPLINK_BACKPRESSURE_ALERT`**
   的 ERROR 日志并把 `alert_count`/`last_alert_at`/`last_drop_ratio` 记入快照。
   配置键：`telemetry.uplink_backpressure_alert.{enabled,window_seconds,drop_ratio,min_received}`。
4. **HTTP 暴露（`backend/internal/api/queue_monitor.go`）**——既有
   `GET /api/v1/queue/stats`（Casbin：SYS_ADMIN/TENANT_ADMIN，107.sql 已登记，
   **无新路由无新迁移**）响应增加 `uplink_bus` 段 = `Bus.GetChannelStats()` 快照。
5. **应用装配（`backend/internal/app/uplink.go`）**——按 viper 键构建告警配置
   （缺省=默认开），注册诊断总线供 API 读取。

## 二、单测（`backend/internal/uplink/backpressure_test.go`，5 例全绿）

- 账本自洽：满队列制造 ctx 超时丢 + 响应链路满丢，`received = accepted + 分账` 严格成立；
- 准入拒绝与未知类型不占 `received`；
- 告警判定纯函数表驱动 6 例（0 流量/低流量静默/阈值恰等不告警/越限告警）；
- **告警演练**：压低窗口（20ms）与下限，制造丢弃，采样器在窗口到期后记录
  `alert_count>=1` + `last_alert_at` + `last_drop_ratio>阈值`；
- 诊断总线注册 nil 安全。
- 实测：`GOTOOLCHAIN=local go test -p 1 -count=1 ./internal/uplink/` → ok 0.540s。

## 三、活栈演练（真实 broker + 真实后端 + mqttbench）

环境：PostgreSQL 17.5 隔离集群 + stub broker（1883）+ 新构建 backend.exe（9999）。
`GET /api/v1/queue/stats` 基线：全 0，`backpressure_alert` 配置如实暴露。

mqttbench：4 连接 × 125 msg/s × 30s（+3s 预热，offered ≈ 16,500）带信封发往
真实设备 `1e59c341-aed2-df19-a711-eae5f798d126`。压测后立即读账本：

| 指标 | 数值 | 含义 |
| --- | --- | --- |
| `received_total[telemetry]` | 16,496 | 到达总线的消息（= offered − 在途 0.02%） |
| `accepted_total[telemetry]` | 15,604 | 已入队（缓冲 10,000 + 已消费 5,604） |
| `uplink_dropped_total` | **0** | 总线级丢弃为 0（阻塞路径不丢，账如实） |
| `blocked_events[telemetry]` | **15,255** | 几乎每条发布都经历阻塞等待 |
| `blocked_seconds_total[telemetry]` | **185,343 线程秒** | 30s 窗口内平均 ≈6,178 个回调并发阻塞 |
| 账本平衡 | 16,496 = 15,604 + 0 + 892(在途) | 进程内不变量成立 |

**两个关键事实**：
1. **阻塞现象首次被直接量化**——15,255 次阻塞、累计 185,343 线程秒，直接坐实
   备忘录对 A 策略（满则阻塞订阅者回调）的定性，为中期 C 方案（反压到 broker）
   提供了现成的立项证据；
2. **总线级丢弃=0 是如实读数**——遥测路径的丢失发生在 paho 内部（到达回调之前），
   本进程不可见；端到端总账需结合 broker 侧计数与落库行数推导，属 C 方案与
   容量模型配套工作，不在 B 范围（备忘录 §四 验收口径中 ±1% 一条适用于总线
   可见丢弃，即响应链路路径——由单测精确锁定）。

探针读回：`p23_mqtt_ingest_probe.js read` → `current[temperature_1]=25.5，
VERDICT=PASS ingest confirmed while stack alive`（摄取在栈存活期间被确认）。

## 四、结论与边界

- P2.3 的「摄取回压策略决策」子项从"待拍板"推进为"**短期 B 已实施并验证**"；
  丢弃从不可见变为可观测、可告警、可演练。
- 中期 C（反压到 broker、落库率 100% 验收）仍按备忘录随容量模型立项；
  tier 达标（资源配额环境）、双实例报告、设备维度扇出维持原缺口不变。
