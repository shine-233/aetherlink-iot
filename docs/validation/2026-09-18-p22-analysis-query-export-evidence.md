# P2.2 分析查询与导出运行期证据（analysis query & export on live stack）

> 日期：2026-09-19（活栈操作与取证时间 2026-09-18 深夜 ~ 19 日凌晨）
> 对应路线图：§1.2 P2.2（Trendz 类轻量分析）
> 结论：**P2.2 全部交付面具备真实 API 运行期证据，状态由 `partial` 翻 `done`**。

## 一、本次补齐的缺口

P2.2 此前唯一缺口：分析查询（`POST /api/v1/telemetry/analysis`）与导出
（`POST /api/v1/telemetry/analysis/export`）**只有 Go 单测，没有任何活栈 API 契约测试**。
原状态行里"剩余约束来自 P0.6 durable execution"已随 P0.6 闭环（2026-09-16）失效。

落地过程中发现并修复两处真实缺陷：

1. **`aggregate=last` 是 API 校验承诺了但热路径从未兑现的能力**：
   `service.validateTelemetryAnalysisAggregate` 允许 `last`，但
   `dal.GetTelemetryDatasAggregate` 的 switch 没有 `last` 分支，运行期必然报
   `不支持的聚合函数: last`。修复：新增 `telemetryAggregateLastQuery`
   （`array_agg(number_v ORDER BY ts_sec DESC)[1]` 取桶内最新样本），与冷层
   rollup 的 `last` 列语义一致；`GetQueryString1` 与分派 switch 同步补 `last`。
2. **`TelemetryAggregateResult` 缺 json tag**：Go 字段名 `Value/OK/Aggregate`
   会原样序列化，破坏全站 snake_case 线契约。补 tag 为 `value/ok/aggregate`。
   该结构此前无 HTTP 消费方（55 号用例里 `devResult.current.avg` 是被 `if`
   守卫吞掉的软断言，从未生效），无破坏面。

## 二、活栈环境

| 组件 | 版本/形态 | 状态 |
| --- | --- | --- |
| PostgreSQL | 17.5，隔离集群 `C:\Users\Zz\al_pg_fresh`，`127.0.0.1:55433`，库 `aetherlink_go99` | `pg_ctl start`（-p 55433），后端启动时自动迁移至 111 |
| Redis | 本机 6379 服务实例，db 1 | 运行中 |
| MQTT | `automation_tests/scripts/local_mqtt_broker.js`（**测试替身**，非生产 broker） | 127.0.0.1:1883 |
| 后端 | `go run . -config configs/conf-localdev.yml`，`AETHERLINK_TIMESCALE_MODE=off`，`GOTOOLCHAIN=local` | `GET /health` → `{"code":200}` |
| 测试账号 | `automation_tests/.env.local`（09-13 生成，显式 `set -a` 导入） | 登录成功 |

## 三、新增契约测试与实测结果

`automation_tests/tests/60_telemetry_analysis.test.js`，**8/8 全绿（1s）**：

```text
  P2.2 telemetry analysis query & export [60_telemetry_analysis]
    ✔ returns one numeric aggregate per device with the snake_case wire contract
    ✔ supports the aggregate=last contract that the hot path previously rejected
    ✔ returns undefined percent change with a reason when the baseline window has no data
    ✔ reports an unreadable device per-row without failing the whole comparison
    ✔ rejects an unsupported aggregate, an empty device list and a reversed window
    ✔ exports the analysis as CSV and reports the server-side file path
    ✔ defaults the export format to xlsx when format is omitted
    ✔ rejects an unsupported export format
  8 passing (1s)
```

判定要点（每条都是先写断言再让代码兑现，不是看着结果写断言）：

- **线契约严格锁形状**：`current` 必须恰好是 `{value, ok, aggregate}` 三键
  （`have.all.keys`），单点 25.5 的均值必须 `closeTo(25.5, 0.001)`；
- **last 用例是"承诺→兑现"证据**：修复前该请求在运行期 100% 失败，修复后返回
  桶内最新样本 25.5——不是恒真断言；
- **未定义百分比永不伪装成数字**：基线窗口无数据时 `percent_change` 缺位、
  `delta` 缺位、`percent_change_reason = 'insufficient data in one or both periods'`；
- **单设备失败不中断多设备对比**：不可读设备返回逐行 `device not readable`，
  可读设备行照常返回数值；
- **导出契约**：`format=csv` → `.csv` 路径；缺省 → `xlsx`；`pdf` → `100002`。

回归：`55_units_conversion.test.js` + `40_telemetry_anomaly.test.js` 合跑 **23 passing（2s）**，
确认 json tag 与 last 修复未破坏相邻面。

## 四、清理动作

- 契约测试 `after()` 调用 `seed.cleanup()` 回收播种设备（无残留）；
- 导出文件写入 `backend/files/export/`（已 gitignore，不入库）；
- stub broker / 后端进程为会话内后台任务，取证后随会话结束退出。

## 五、仍未验证（如实）

- 分析查询的**冷层回落路径**（`telemetry_rollups` 整窗冷数据）在活栈上未被本套件
  覆盖——需要早于降采样边界的历史数据，属 P2.3 数据保留场景，由其压测/容量环境验证；
- `same_period_last`（同比）只验证了参数通路（环比窗口语义共用同一基线推导函数），
  自然周期回退的数值断言仍由 Go 单测覆盖。
