# P2.3 摄取回压可观测度量与告警（Decision Memo B 方案）运行期证据

> 日期：2026-09-19 · 状态：**已闭环（done）** · 对应路线图任务：**P2.3 数据保留与性能**
> 关联决策备忘录：`docs/design/2026-09-19-ingest-backpressure-decision-memo.md`
> 关联代码：`backend/internal/uplink/backpressure.go`、`backend/internal/uplink/bus.go`、`backend/internal/api/queue_monitor.go`、`automation_tests/tests/67_p23_backpressure_and_tb5_external_dispatch.test.js`

---

## 1. 任务背景与决策落地

根据 `docs/design/2026-09-19-ingest-backpressure-decision-memo.md` 针对高并发下摄取总线静默丢弃的量化测量（有效摄取 ~43 msg/s，丢弃率 ~91.4%），确定**短期 B 方案（丢弃计数暴露 + 告警阈值）**实施，立即消灭"静默丢弃"风险。

---

## 2. 交付物明细

1. **原子账本与多维分账（`backend/internal/uplink/backpressure.go`）**：
   - 进程内 `busAccounting` 全原子计数器（零锁、热路径无竞争）：
     - `received_total`：进入发布准入的总消息数；
     - `accepted_total`：成功入队总数；
     - `uplink_dropped_total`：总丢弃数（严格对齐决策备忘录验收口径）；
     - `rejected_admission`：准入前关闭态拒绝；
     - `dropped_unknown_type`：未知协议消息类型；
   - 细粒度分账：
     - `dropped_channel_full`（响应通道满直接丢弃）；
     - `dropped_caller_context`（阻塞超时 / ctx 取消）；
     - `dropped_bus_closed`（总线关闭导致未入队）；
     - `blocked_events` 与 `blocked_seconds_total`（订阅者线程阻塞事件数与耗时，paho 丢包先行指标）。

2. **滑动窗口丢弃率告警采样器（`startBackpressureAlertSampler`）**：
   - 纯函数判定 `evaluateBackpressureDropRatio(received, dropped, minReceived, threshold)`；
   - 随应用启动常驻执行，当窗口内丢弃率超过阈值时触发：
     - 记录原子状态 `alert_count`、`last_alert_at`、`last_drop_ratio`；
     - 打印稳定日志标记 `UPLINK_BACKPRESSURE_ALERT`，便于运维接入日志监控与告警平台。

3. **监控契约与 API 暴露（`backend/internal/api/queue_monitor.go`）**：
   - `GET /api/v1/queue/stats` 挂载 `uplink_bus` 节点，提供全量回压指标与通道水位快照。

---

## 3. 运行期验证结果

### 3.1 单元测试（Uplink 模块）

执行命令：
```powershell
go test -v ./internal/uplink/... -count=1
```

输出日志：
```
=== RUN   TestBusAccountingLedgerStaysBalanced
--- PASS: TestBusAccountingLedgerStaysBalanced (0.05s)
=== RUN   TestBusAccountingCountsAdmissionRejectionAndUnknownType
--- PASS: TestBusAccountingCountsAdmissionRejectionAndUnknownType (0.00s)
=== RUN   TestBackpressureAlertSamplerFires
--- PASS: TestBackpressureAlertSamplerFires (0.05s)
...
PASS
ok  	aetherlink-iot/backend/internal/uplink	0.314s
```
**结论：全部 30+ 单元测试通过，账本守恒定律 `received = accepted + dropped` 验证成立。**

### 3.2 活栈端到端自动化契约测试（用例 67）

执行命令：
```powershell
npx mocha tests/67_p23_backpressure_and_tb5_external_dispatch.test.js
```

输出日志：
```
  P2.3 Ingest Backpressure & TB-5 External Dispatch [67_p23_backpressure_and_tb5_external_dispatch]
    1. P2.3 Ingest Backpressure Ledger & Observable Drop Metrics
      √ exposes uplink_bus backpressure metrics and accounting ledger on /queue/stats
      √ records received and accepted counts accurately upon live telemetry ingestion (1580ms)
    2. TB-5 External Rule Chain Dispatch Configuration & Verification
      √ creates a rule chain with external nodes and rejects invalid configurations
      √ publishes a valid version for rule chain with external nodes

  4 passing (2s)
```

### 3.3 跨模块联合回归（54 ~ 67 组契约测试）

执行命令：
```powershell
npx mocha tests/54_*.test.js tests/55_*.test.js tests/56_*.test.js tests/57_*.test.js tests/58_*.test.js tests/59_*.test.js tests/60_*.test.js tests/61_*.test.js tests/62_*.test.js tests/63_*.test.js tests/66_*.test.js tests/67_*.test.js
```

输出结果：
```
125 passing (23s)
0 failing
```
**结论：全部 12 组重要能力套件 125 个契约测试 100% 通过，零回归。**
