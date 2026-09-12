# P2 / P3 现状核查（2026-09-12）

方法：不读 ROADMAP 的"实现状态"文字下结论，一律以 **grep 调用点 + 编译 + 跑测试** 为准。
判据只有两条：

1. **有没有消费方** —— 函数存在但无人调用，等于不存在。
2. **是否可达** —— API 存在但未注册路由，等于不可达；未登记 Casbin，等于启动即失败。

本机环境：无 Docker、无网络、无真实 PostgreSQL。需要运行期的证据一律标记为 pending。

---

## 结论速览

| 项 | 判定 | 依据 |
|---|---|---|
| P2.1 协议插件 SDK | **pending** | 接口只在死包 `internal/roadmap` 里，零 import |
| P2.2 Trendz 轻量分析 | **partial** | 聚合/多设备/CSV 已实现且已接线；同比环比、异常、Excel、分享未做 |
| P2.3 数据保留与性能 | **partial** | 保留策略**真执行**（cron 每日 2 点）；降采样/分层/缓存/压测为 0 |
| P3 商业化 | **pending** | 无 License 代码；多数条目本就不是代码 |

---

## 最重要的发现：`internal/roadmap` 是一个死包

```
backend/internal/roadmap/
├── contracts.go   # 声明 ProtocolAdapter / AnalyticsService 等接口
├── skeletons.go   # 30 个 ErrNotImplemented 骨架（UnwiredProtocolAdapter 等）
├── types.go / doc.go / schema/
```

- `go build ./internal/roadmap/` **通过**（`rc=0`）。
- 但 `grep -rn "internal/roadmap" --include="*.go" .` **除自身外零命中** —— 没有任何代码 import 它。

这是本项目最大的假完成信号：**接口声明 + 可编译 + 零消费方**。
它会让人以为"契约已经定了、只差实现"，实际上连一个调用者都没有。

**处理建议**：要么真正接入生产代码，要么删除。保留现状会让后续每次评估都高估进度。
（本次核查未删除——破坏性操作，交由决策。）

---

## P2.1 协议插件 SDK

ROADMAP 交付物：插件 manifest、配置 Schema、点表、凭证映射、指标、版本兼容、签名；
按客户需求接入 CAN/BACnet/BLE/LoRaWAN。

- `ProtocolAdapter` 接口：仅存在于死包 `internal/roadmap/contracts.go:49`。
- 配套的 `UnwiredProtocolAdapter`（`skeletons.go:79`）方法体直接 `return ErrNotImplemented`。
- 全仓无第二处实现该接口。

**但基础设施是真实的**：

- `internal/protocolgw/`（708 行，含 `gateway_test.go`）**已接线** ——
  `internal/app/application.go:32` 持有 `*protocolgw.Gateway`，
  `internal/app/coap_gateway.go` 构造 `DefaultConfig()` / `NewTelemetryBridge()`。
- `internal/service/protocol_plugin.go`、`internal/api/protocol_plugin.go`、
  `internal/dal/device_protocol_plugin.go` 均存在。

判定：**有真实网关基础，但没有 ROADMAP 所定义的插件 SDK**。
CAN/BACnet/BLE/LoRaWAN 属"按客户需求接入"，不应在没有真实设备时伪造适配器 —— 记 pending。

---

## P2.2 Trendz 类轻量分析

ROADMAP 交付物：多设备对比、聚合/同比环比、基础异常、CSV/Excel、权限和分享。

**比文档描述更完整**（文档未写这部分）：

- `internal/service/telemetry_statistic.go:33`
  `GetTelemetrServeStatisticData` —— 单设备聚合，`req.IsExport` 为真时走 `exportToCSV`。
  聚合窗口 `aggregate_window`、聚合函数 `aggregate_function`、时间范围均已有校验。
- `internal/service/telemetry_statistic.go:430`
  `GetTelemetryStatisticDataByDeviceIds` —— **多设备聚合**，
  **已接线**到 `internal/api/telemetry_data.go:456`（`ServeStatisticDataByDeviceId`）。
  入参 `GetTelemetryStatisticByDeviceIdReq.DeviceIds []string`。

**确实缺失**：

| 能力 | 命中 |
|---|---|
| 同比 / 环比（PeriodOverPeriod） | 0 个文件 |
| 基础异常检测（Anomaly） | 0 个文件 |
| 分析的 Excel 导出 | excelize 仅用于报表与预注册导出，未用于分析 |
| 分享 | 21 处均为"设备分享"概念，非分析分享 |
| `AnalyticsService` 正式接口 | 仅在死包中 |

判定：**partial**。核心聚合与多设备对比已可用，分析层的高级能力未开始。

---

## P2.3 数据保留与性能

ROADMAP 交付物：保留策略、降采样、冷热分层、查询缓存、基准压测、容量模型和告警。

**保留策略是真执行的**（这条曾被怀疑，已验证到底）：

```
main.go: app.WithCronService()
  → internal/app/cron_service.go:23  croninit.CronInit()
    → initialize/croninit/cron.go:53  c.AddFunc("0 2 * * *", ...)
      → service.GroupApp.CleanSystemDataByCron()
        → internal/service/datapolicy.go:69
```

`CleanSystemDataByCron` 遍历数据策略，跳过非法的 `retention_days` 并记 warn（不静默吞掉）。
配置面 `internal/api/data_policy.go` 与路由 `router/apps/datapolicy.go`（PUT/GET）均已注册。

**缺失**：降采样、冷热分层、查询缓存、基准压测、容量模型 —— 全为 0。

门禁要求"至少单实例和双实例报告""压测不使用假数据" —— **本机无此环境，记 pending**。

---

## P3 商业化与长期能力

- **许可证**：全仓 `License` 零命中（非测试代码）。
- **计费 / 配额**：`quota` 仅见于 `fleet_command_job_dispatch_quota.go` 等派发配额，
  非通用计费配额。
- **签名 / 供应链扫描**：仅 `device_template_market_integrity.go`（P1.6 模板市场），
  无第三方插件签名与供应链扫描。
- 其余条目（多地域高可用演练、RPO/RTO、滚动升级、移动端商店发布、桌面运维工具、
  生态市场运营）属基础设施与运营范畴，不是代码任务。

判定：**pending**。

---

## 回归状态

```
ok  internal/service   6.884s
ok  internal/api       5.561s
ok  internal/dal       5.897s
ok  internal/app       6.712s
ok  router             3.709s
```

前端（本次未改动）：`src/views/scada` 38 例通过。

---

## 仍未验证（需要真实环境）

- 多设备聚合在真实时序数据量下的表现（需 PostgreSQL + Timescale/分区）
- 保留策略的真实删除效果（需运行 cron 并观察数据）
- P2.3 的全部性能门禁
- P3 全部条目
