# P0.6 持久化报表执行与 SMTP 事实语义 运行期证据

> 日期：2026-09-16
> 对标基准：ThingsBoard 报表中心（Reporting Center）与持久化调度投递体系
> 状态：**已全面闭环 (done)**
> 质量判定：`npm run typecheck` 0 错误；前端定向 vitest **20/20 100% 全部通过**；后端 Go 单测 **100% 全部通过**；自动化 API 契约测试（`37_report_schedule.test.js`）**11/11 100% 全部通过**。

---

## 一、交付物全景

### 1. 数据库持久化与两阶段调度模型（83.sql）
- `report_schedules`：调度定义主表，包含 Cron 表达式、时区、Lookback 窗口、收件人列表、关联设备与遥测 Key、版本号（Revision）；
- `report_schedule_runs`：生成执行实例表，通过唯一 `claim_token` 与 `lease_until` 实现分布式租约竞争，防止多节点并发抢占；
- `report_schedule_deliveries`：Outbox 投递表，生成完成后原子写入不可变邮件信封与快照 Payload，解耦数据生成与网络投递两阶段。

### 2. 后端核心执行引擎与 SMTP 状态机
- `backend/internal/app/report_schedule_worker.go`：
  - 常驻调度 Worker，包含 `initialize schedules`、`dispatch schedules`、`claim generations`、`claim deliveries`、`recover expired leases` 五个生命周期步骤；
  - 启动阶段自动注册至 `Application` 服务管理器（`WithReportScheduleWorker`）；
  - 支持通过配置文件（`reports.worker.*`）灵活调节扫描轮询周期（`interval`）、租约时长（`lease_timeout`）、最大重试次数（`max_attempts`）与重试退避底数（`retry_base_delay`）。
- `backend/internal/dal/report_run.go` & `report_delivery.go`：
  - 实现了基于 `clock_timestamp()` 的排它锁定（`SKIP LOCKED`）领任务与续约机制；
  - 实现了可配置的指数退避重试延迟算法（`reportRetryDelay`）；
  - 严格支持 CAS 乐观锁版本更新与 `Idempotency-Key` 幂等防重重放。
- `backend/internal/service/report_run_processor.go` & `report_smtp.go`：
  - 遥测范围抽取与 CSV 安全转义，防公式注入，限定 25MB 与 100,000 行保护内存；
  - 精准映射 SMTP 交付边界三态模型：
    - `accepted`：对方 SMTP 服务器明确接受（以终态 succeeded 归档）；
    - `failed`：前置网络不可达或认证失败（进入重试队列或以终态 failed 归档）；
    - `ambiguous`：在 DATA 发送过程中断开连接，状态不可知，归档为 ambiguous 并标记 `duplicate_delivery_risk = true`，禁止盲目重投引发垃圾邮件风暴。

### 3. 前端报表调度管理工作台
- `frontend/src/views/visualization/report/index.vue`（914 行全功能工作台）：
  - 调度配置列表：查看、新增、编辑、版本受控更新、删除（带活动任务保护检测）；
  - 快捷手动执行（Run Now）：携带防重 Idempotency-Key，立即提交生成；
  - 运行记录侧滑抽屉与详细看板：
    - 列表展示总体状态（queued/running/succeeded/failed/ambiguous）、生成阶段、投递阶段、尝试次数、时间窗口等；
    - 失败/模糊态一键重试（Retry Run）；
  - `frontend/src/views/visualization/report/useSelectedReportRunPoll.ts`：
    - 专用状态轮询 Hook，对活跃态（pending/processing/retrying）自动退避轮询，进入终态后自动静默停止，杜绝无效前端长轮询；
  - `frontend/src/views/visualization/report/report-model.ts`：
    - 纯逻辑格式化与状态推导，包括颜色标识、状态文案、重试资格判定。

---

## 二、测试验证矩阵

### 1. 前端类型与单元测试
- `npm run typecheck`: 0 错误
- `npx vitest run src/views/visualization/report`: 3 files / 20 tests 全部通过

### 2. 后端核心服务单测
- `go test ./internal/app -run Report`: PASS (10/10 tests)
- `go test ./internal/service -run Report`: PASS (all tests)
- `go test ./router/apps ./internal/api ./internal/model -run Report`: PASS (all tests)

### 3. 全链路 API 契约与运行期自动化验证（37_report_schedule）
- 模块：`report-schedule` (37_report_schedule.test.js)
- 耗时：51.57s
- 结果：11/11 用例 100% 全部通过 (passed)