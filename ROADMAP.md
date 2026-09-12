# AetherLink IoT 平台下一阶段路线图

> 版本：2026-09-10
>
> 当前证据基线：`main@30fd899b3dde1920115abd7e44eb2565ae7c46b3`，工作树 materially dirty；没有同 revision、hash-bound 的 canonical runtime archive，Phase 0 与发布就绪均为 unknown。
>
> 目标：以 ThingsBoard（CE/PE/Cloud/Edge/Gateway/生态产品）和 ThingsPanel（社区版及官网宣称的企业扩展）为参照，把 AetherLink 从“功能已合入”推进到“可部署、可运维、可扩展、可证明”。
>
> 本文是执行基线，不是历史交付日志。每个任务都给出交付物、框架代码位置、依赖和验收门禁，便于交给其他模型实现。

## 1. 状态口径

### 1.1 已实现的源码基线（不自动等同当前运行期完成）

- 接入：MQTT/HTTP、Modbus、CoAP/LwM2M、SNMP、OPC UA；GMQTT Broker、ACL、持久会话和 uplink 管道。
- 核心模型：设备、产品/模板、遥测、属性、命令、设备影子队列、资产树、租户父子层级、基础 Casbin RBAC。
- 自动化：DAG 规则链、Vue Flow 编辑器、阈值/映射/Webhook/命令/告警节点；场景自动化已有基础能力。
- 数据与运维：PostgreSQL/TimescaleDB 三态门控、遥测统计、告警、OTA 任务模型、Redis 限流、Casbin watcher、白标配置。
- 产品切片：CSV 预注册 API、行业模板种子、模板导入/导出 MVP、边缘遥测转发/缓存/命令和模板下发、移动端 H5 MVP、AI 遥测查询和告警分析。
- 质量：当前迁移源码连续至 `82.sql` / `VERSION_NUMBER=82`；部分历史协议 E2E、双实例 watcher/限流、Timescale on/off/auto、OIDC/Keycloak 曾验证，但旧归档只作 historical，不能证明当前工作树。

### 1.2 部分完成（不得写成完整能力）

- 3D：设备详情 GLB 预览、遥测驱动材质/旋转和 WebGL 降级；这不等于完整 SCADA。
- 看板：基础 Native Board/ThingsVis 路径可用；native-board-provider.ts 中项目增删改和自定义 canvasConfig/nodes/dataSources/variables 仍返回 unsupported。
- OTA：进度消费、逐设备失败重试和取消已有；持久 rollout worker、强制灰度批次、暂停/恢复、旧 attempt 隔离、回滚和审计未闭环。
- 影子：离线缓存和上线投递已实现；当前 broker publish 仍被记为 delivered，缺少设备 ACK identity、超时/退避、terminal failure 与 duplicate/late ACK 隔离。
- CSV 预注册：API、service 和 UI 基础已实现；真实浏览器 file chooser、逐行可见错误、一次性凭证下载/重载、脱敏导出解析、跨租户拒绝和 cleanup 证据待补。
- 规则链：基础 engine 与 trace/query 已有；节点 timeout/backoff、failure edge、DLQ、replay side-effect safety、draft/published version 和 rollback 未闭环。
- 报表/分析：遥测统计、导出和进程内定时报表局部实现；当前扫描器没有跨副本 claim/lease/fencing，执行与 SMTP 投递没有不可变 run/outbox 记录，手动运行也没有可查询的异步结果，因此仍不构成持久任务调度闭环。Trendz 级分析工作台属于后续 P2，不与本轮 P0 报表可靠性混称。
- 边缘：数据转发、断线缓存、RPC/实体下发已有；节点注册、健康、版本、冲突和远程运维不足。
- 移动端：登录、设备列表、最新遥测和 H5 构建；命令、告警、影子、推送、正式 Android/iOS 发布未完成。
- RBAC：路由登记和基础角色矩阵已激活；仍需按产品定义持续收紧并维护正/负向矩阵。

### 1.3 明确缺失

- 通用 Entity Relations（设备/资产/客户/网关任意关系）；当前只有 roadmap 设计 scaffold，没有 numbered migration、DAL/service/API/Casbin/UI 生产链路。
- 规则链可靠性剩余项：节点级超时、退避、失败分支、死信、回放安全和版本发布；基础 Trace 已有，不能再列为完全缺失。
- 可扩展 Widget/SCADA 工业画布、变量和联动体系。
- OTA 完整状态机和回滚；模板市场升级/回滚/签名/依赖。
- 边缘节点生命周期和云边配置同步冲突解决。
- 统一协议插件 SDK；按需扩展 CAN/BACnet/BLE/LoRaWAN。
- 生产部署、TLS/MQTTS、公网接入、备份恢复、压测和故障演练的可重复证据。

## 2. 竞品能力边界

ThingsBoard CE 覆盖设备/资产/客户实体、遥测、MQTT/HTTP/CoAP、Dashboard、Rule Engine、告警和基础 Gateway；PE/Cloud/Edge 公开介绍进一步强调移动端、白标、SLA/集群、边缘管理、审计、报表等商业能力。TBMQ 和 Trendz 是独立生态产品，不应假设 CE 自带。

ThingsPanel 社区仓库可见 MQTT/HTTP/Modbus、物模型、看板、规则、OTA 以外的部分前端/移动能力；官网还宣称企业级插件、行业方案、移动端、数据网关等扩展，但付费授权矩阵不完整。对 ThingsPanel 的结论必须分为“官网宣称”和“开源仓库可验证”，不能将宣传语当作已实现源码。

产品决策：优先补可靠性闭环、实体关系、SCADA/Widget 扩展、规则链运维、边缘运维和移动控制；不在近期复制完整 TBMQ、Trendz、600+ Widget 或多地域 SaaS 计费体系。

## 3. 阶段路线

### 3.0 当前优先门禁：可信证据生产

在 P0.1 之前先完成两项测量系统修复：

1. runner 从启动时创建唯一 run-scoped staging 目录，所有 Mocha/Playwright/endpoint/page/provenance/summary 直接写入该目录；禁止扫描共享 `automation_tests/reports/` 拼装 canonical archive。
2. 报告关闭后生成 `aetherlink.automation.archive.v1`，绑定 command、interval、exit code、strict mode、evidence kind、Git revision/dirty diff、cleanup/redaction、exact module/case outcomes 和 SHA-256；staging 校验成功后用同文件系统 atomic rename 发布。
3. backend/GMQTT capability mapping 必须使用 repository-relative file + exact Go test function + stable evidence ID + semantic anchor；同文件存在任意 `func Test` 不再算 traceability。
4. canonical producer、inspector negative controls 和 exact identity 门禁通过前，不运行或引用新 full API/E2E 结果来提升 readiness。

当前状态：canonical producer 已完成 run-scoped staging、partial diagnostic、report hash、cleanup truth、manifest redaction、inspector-backed validation、manifest-last 和 same-filesystem atomic rename；2026-09-10 的 producer/policy/inspector focused contracts 为 61 passing，但这只是静态 harness 证据。exact backend/GMQTT identity 与 production-reachable placeholder/no-op gate 已于 2026-09-11 实现并实测通过（详见 `docs/validation/P0.6-P0.7-evidence.md`）：capability mapping 的 26 条 goEvidence 均带 repository-relative file + exact Go test function + stable evidence ID + semantic anchor，其中 GMQTT 条目覆盖 `mqtt-broker/plugin/aetherlink/hooks_test.go` 等精确函数；placeholder gate 从入口做 BFS 可达性分析，实测 cataloged 2123 / reachable 1708 / violations 0，且契约测试含负向对照（能检出全部四类 false-success 族，非空转）。实测门禁：`00_go_test_evidence_contract` 11 passing、`00_go_test_runtime_contract` 8 passing、`00_production_placeholder_gate_contract` 5 passing、`00_coverage_contract` capability mapping 相关 4 项 passing。因此阶段 3.0 的测量系统修复已闭环，但**这仍是静态 harness 证据**，不含真实服务/浏览器运行期证据，故不得据此提升 Phase 0 或发布就绪。

## P0：证据与生产闭环（发布前置）

### P0.1 发布同步和部署门禁

交付物：同步 ThingsPanel-Go 远端版本注入/Release 提交；统一 preflight:release；目标服务器 HTTPS/TLS、MQTTS、公网 MQTT、backup/restore 验收报告。

框架代码：

    scripts/roadmap/preflight-release.ps1
    scripts/roadmap/validate-deploy.ps1
    scripts/roadmap/backup-restore.ps1
    docs/validation/release-<date>.md

依赖：部署环境、证书、Redis/Postgres/Timescale、真实 DNS 或等价隔离栈。

门禁：全新库迁移通过；TLS cookie 为 Secure；MQTTS 设备可上报/下发；恢复后设备、模板、告警、遥测计数一致；缺少真实环境时状态保持 pending。

### P0.2 设备影子 ACK 闭环

交付物：desired/reported/ack 状态机、超时、重试、取消、过期；API+MQTT+UI E2E。

框架代码：

    type ShadowMessage struct { ID, DeviceID string; Payload json.RawMessage; Status string; Attempts int; ExpiresAt time.Time }
    type ShadowService interface {
      Enqueue(ctx context.Context, deviceID string, payload json.RawMessage, ttl time.Duration) (ShadowMessage, error)
      OnDeviceOnline(ctx context.Context, deviceID string) error
      Ack(ctx context.Context, messageID string) error
      ExpireAndRetry(ctx context.Context, now time.Time) error
    }

门禁：离线下发返回 202/pending；上线后仅投递一次；设备 ACK 后为 delivered；无 ACK 按退避重试并最终 expired/failed；跨租户访问 404/403；浏览器队列状态与 API 一致。

实现状态（2026-09-11，源码层完成；运行时证据待统一验证）：

- 修正了一个虚假成功：**旧实现 dispatch 成功即写 `delivered`，把"已下发"当成"设备已确认"**。
  现引入 `sent`（已下发待 ACK），只有设备 ACK 才转 `delivered` 并写 `ack_at`。
- 迁移 `84.sql`：新增 `attempts` / `sent_at` / `ack_at` / `next_attempt_at` / `last_error`，
  状态词表扩展为 `pending|sent|delivered|failed|expired|canceled`（带 CHECK 约束），
  并回填历史 `delivered` 行的 `ack_at`，不谎称这些行曾等待过 ACK。
- 退避重试：未 ACK 时按 30s→60s→120s（上限 10min）指数退避，超过 `ShadowMaxAttempts=3` 转 `failed`；
  TTL 是硬终止，不因重试而延长，到期转 `expired`。
- 新增端点 `POST /api/v1/device/shadow/:deviceId/:msgId/ack`（只有 pending/sent 可确认，终态行拒绝）。
- cron 与上线钩子都会先跑 `ExpireAndRetryShadowMessages()` 再做投递，避免重复投递。
- 未改 `device_shadow.gen.go`（生成产物，策略上保留不手改）；`attempts` 由 DAL 事务内读-改-写。
- 定向证据：`backend/internal/model/device_shadow_test.go`（5 例，状态词表/终态/退避上限/时间点单调性）全部通过。
- MQTT 侧 ACK 上报入口：新增消息类型 `shadow_ack`（`uplink/bus.go`），路由到响应链路；
  `ResponseUplink.processShadowAck` 解析 `{"shadow_id":"<id>","result":0}`。
  **`result` 非 0 表示设备收到但处理失败——不确认送达，保留 sent 等退避重试**，
  绝不把"设备明确报错"写成 delivered。注意必须在 message_id 校验之前处理，
  否则影子 ACK 会因缺少命令 message_id 被直接丢弃。
- UI 队列状态展示：`device-shadow.vue` 新增 sent/failed 状态与 `attempts`、`ack_at` 两列；
  sent 用中性色以区别 delivered 的绿色；`ack_at` 为空即表示尚未确认，
  不用 `delivered_at` 顶替（前者是确认时间，后者是送达时间）。四语 locale 已同步。
- API 层 E2E 定义：`automation_tests/tests/27_shadow_messages.test.js` 已改为
  **上线后先断言 sent，再显式 ACK 才断言 delivered + ack_at**；
  并新增"终态行不可被确认"的负向用例（原测试把旧的 delivered 假成功写死了，已修正）。
- 仍未执行：上述 API 用例需真实后端 + broker 才能跑，尚未运行；
  UI 的真实浏览器证据与 MQTT 端到端联调同样待统一验证阶段执行。

**2026-09-12 更新：数据库层运行期证据已取得**（`docs/validation/P0.2-shadow-ack-evidence.md`，
常驻用例 `internal/dal/device_shadow_postgres_test.go`，缺 DSN 则 Skip）：
下发后状态必须是 `sent` 而非 `delivered`；设备 ACK 后才转 `delivered` 且 `ack_at` 非空
（不得用 `delivered_at` 顶替）；终态行重复 ACK 被拒；重试耗尽转 `failed`；TTL 到期转 `expired`。
**本项仍 don=false**：真实 MQTT `shadow_ack` 上报的端到端与浏览器证据仍需活栈。

### P0.3 OTA 状态机

交付物：进度消费、批次暂停/恢复/取消、失败重试、灰度、回滚和报告。

框架代码：

    type OTAStatus string
    const ( OTAPending OTAStatus = "pending"; OTARunning OTAStatus = "running"; OTASuccess OTAStatus = "success"; OTAFailed OTAStatus = "failed"; OTAExpired OTAStatus = "expired"; OTACanceled OTAStatus = "canceled" )
    type OTAProgressEvent struct { JobID, DeviceID string; Percent int; Status OTAStatus; Error string; At time.Time }
    func (s *OTAService) ConsumeProgress(ctx context.Context, e OTAProgressEvent) error
    func (s *OTAService) RetryFailed(ctx context.Context, jobID string, limit int) error

门禁：状态转移非法即拒绝；同一事件幂等；失败设备可筛选重试；回滚产生新审计事件；至少一条真实设备/broker 或协议 stub E2E。

实现状态（2026-09-11，部分完成——仅状态机与暂停/恢复）：

- 查清了一个易混淆点：既有的 `commandJobEventResumed` 是 **worker 故障恢复（recovery）语义**，
  不是用户暂停后的恢复。P0.3 要求的"批次暂停/恢复"此前**完全没有实现**（全库无 `paused`）。
  新增用户语义事件 `paused` / `unpaused`，与 recovery 的 `resumed` 明确区分，
  避免把"故障自愈"粉饰成"人工恢复"。
- 新增 `backend/internal/service/fleet_command_job_state_machine.go`：
  集中声明合法状态转移表（`scheduled/running/paused` 及终态），
  **非法转移一律返回 CodeOpDenied 并带上双向状态，绝不静默成功**；终态不可"复活"。
- 新增 `PauseFleetCommandJob` / `ResumeFleetCommandJob`：
  暂停清 `next_dispatch_at` 使 worker 停止领取（派发只取 running/scheduled，故暂停真正生效）；
  恢复置 `next_dispatch_at=now` 并立即触发一次派发。暂停态重复调用幂等，不产生第二次事件。
- 定向证据：`fleet_command_job_state_machine_test.go` 5 例通过（合法转移、终态不可复活、
  非法转移被拒、暂停态不可派发、错误不静默）。
- 进度消费（已补）：`fleet_command_job_progress_rollback.go` 提供 `ConsumeFleetCommandJobProgress`。
  **幂等令牌刻意不含上报时间**——否则设备重传一次进度就变成两条事件，可用事件数量伪造推进速度。
  终态/已取消批次拒绝进度上报（给死掉的批次写进度等于伪造进展）。
- 回滚（已补）：`RollbackFleetCommandJob` 只允许对已结束（completed/partially_failed/failed）批次执行；
  **回滚创建新批次而不是把原批次改回去**，原批次历史保持只读，
  并在原批次与新批次双向留下 `rollback` 审计事件以便追溯。
- 仍未完成：真实设备/broker 或协议 stub 的 E2E。
  另：进度事件目前只落事件表，**尚未回写明细行的状态/百分比**（需迁移加列，且磁盘已满无法验证），
  因此本项**仍不算完成**。

**2026-09-12 更新：上文"灰度/金丝雀、报告导出未完成"已过时，两者均已实现并有单测**

- **灰度/金丝雀执行面**：`internal/service/ota_rollout_governance_apply.go`（`ApplyRolloutGovernance`，
  含 `applyDispatchBatch` / `applyAbort` / `applyTimeout` / `applyComplete` 四个决策分支）
  + `internal/dal/ota_rollout_governance.go` + `internal/api/ota.go`。
  关键设计：金丝雀设备的选取是**确定性的**（同一批在多次治理间顺序一致，便于比对）——
  若每次随机取样，"限速下发"会退化成"一次性全推"，金丝雀就失去意义。
- **报告导出**：`internal/service/fleet_command_job_report.go`
  （`GetFleetCommandJobReport` + `FormatFleetCommandJobReportCSV` + `sanitizeReportCell`）。
- 单测：`ota_rollout_governance_apply_test.go`、`ota_rollout_governance_test.go`、
  `ota_rollout_governance_preview_test.go`、`fleet_command_job_report_test.go`。
- **缺口**：两者均**无运行期证据文档**（无 `docs/validation/` 对应条目），
  灰度治理亦未接真实设备/broker 的 E2E。按 §4 口径，仍属"代码在、运行期证据缺失"。

### P0.4 场景与 Flow 语义

交付物：开始/结束时间、时区、过期、停止其他 Flow、定时器触发的统一语义。

框架代码：

    type ExecutionWindow struct { StartsAt, ExpiresAt *time.Time; Timezone string }
    func (e Engine) CanRun(now time.Time, w ExecutionWindow) bool
    func (e Engine) StopConflictingFlows(ctx context.Context, deviceID, flowID string) error

门禁：边界时间表驱动测试；重复触发幂等；停止动作可审计；服务重启后调度不丢任务。

实现状态（2026-09-11，部分完成——执行窗口与冲突停止语义；此前 `ExecutionWindow` /
`StopConflictingFlows` 在全库完全不存在）：

- 新增 `backend/internal/service/scene_execution_window.go`：
  - `ExecutionWindow{StartsAt, ExpiresAt, Timezone}` 与 `FlowEngine.CanRun(now, w)`。
    区间语义为**左闭右开 `[starts_at, expires_at)`**，避免同一时刻被两个窗口同时命中。
  - **时区非法一律 fail closed**（返回 `ErrInvalidExecutionTimezone`），
    不静默按 UTC 兜底——那会让窗口边界整体偏移，属于伪造可执行性。空时区才按 UTC。
  - `FlowTriggerKey` 提供重复触发幂等：按 `(flow, device, 秒级时刻)` 生成键，
    吸收定时器亚秒抖动，避免一次触发被放大成多次。
  - `StopConflictingFlows` 通过可注入的 `FlowRunRegistry` / `FlowAuditSink` 停止同设备上的
    其他运行中 Flow；**任一侧缺失即拒绝执行，禁止静默停止**，且每次停止都留审计事件。
- 定向证据：`scene_execution_window_test.go` 7 例通过，含门禁要求的**边界时间表驱动测试**
  （前/恰在起点/窗口内/前 1ns/恰在终点/过期后/无上界/无下界/完全无界共 11 行）。
- 仍未完成：定时器触发的持久化与"服务重启后调度不丢任务"、
  与真实场景引擎（automate_telemetry_scene_execution / scene.go）的接线，以及真实 E2E。
  本项**不算完成**。

### P0.5 CSV 浏览器 E2E

交付物：上传、校验错误展示、批量建档、一次性凭证下载、脱敏导出和清理。

门禁：真实浏览器选择文件；坏行逐行反馈；下载文件可解析；凭证只出现一次；跨租户产品不可选。

实现状态（2026-09-11，部分完成——仅导入校验层有可运行证据）：

- 已存在：CSV 导入（`buildFilePreRegisterRows` / `readPreRegisterImportCSV`），
  表头严格校验为 `device_number,name`，坏行带 `csv_row` 反馈，跨租户产品校验
  （`validatePreRegisterProductTenant`）已具备。
- 本轮补的定向证据：`device_pre_register_csv_test.go` 3 例通过——
  路径穿越 / 绝对路径 / 非 csv / 越出白名单目录一律拒绝；表头顺序错、缺列、空文件被拒；
  单元格仅裁剪首尾空白、内部空格保留。
- 仍缺失（本项**不算完成**）：
  - 真实浏览器选择文件的 E2E（需前端 + 后端 + 数据库的活栈，未执行）；
  - **一次性凭证下载**（"凭证只出现一次"）在代码里没有独立机制，
    目前只是沿用批量创建的 username 形态，不满足门禁；
  - **脱敏导出与清理完全没有实现**（`device_pre_register.go` 中无任何 export 逻辑）。

**2026-09-12 更新：上面的"完全没实现"已过时。** 导出与清理均已接线并完成 Casbin 登记：
- 导出走 Excel，`utils.MaskVoucher` 脱敏、按租户过滤、分批 5000、上限 20 万行
  （`device_preregister_export.go`）；列头已声明"已脱敏，完整凭证仅创建时可见"。
- 清理执行面 `device_preregister_cleanup.go`，分流逻辑由 `device_pre_register_export.go` 的
  `classifyPreRegisterCleanup` 提供（注入使用）：已激活设备永不删除、跨租户 fail closed、空批次幂等。
- `preRegister/cleanup` 路由此前**从未登记 Casbin**，而 `casbin.route-audit-mode` 默认 fail-fast，
  缺登记会让后端在启动期直接拒绝启动——已由迁移 `90.sql` 补登记（同批次的 export 在 63.sql）。
- 运行期证据：`docs/validation/P0.5-cleanup-execution-evidence.md`（真实删除路径，9 例全过）。
**本项仍记为 partial**：真实浏览器 file chooser E2E 需活栈。

### P0.6 持久化报表执行与 SMTP 事实语义

交付物：`83.sql`、显式 IANA 时区与 `next_run_at`、乐观 revision、不可变 `report_schedule_runs`、一对一 `report_schedule_deliveries` outbox、数据库时间驱动的 slot materialization、`SKIP LOCKED` claim、UUID fencing token、lease 续租/恢复/最终尝试收口、手动与子重试幂等、固定报表窗口、租户级 run history/detail、精确 HTTP 202/Location，以及管理员报表工作台。

事实边界：SMTP 仅提供 at-least-once 尝试。`accepted` 只表示 SMTP 服务器接受消息，不表示收件人最终送达；可能已接受但客户端未收到确定响应的结果必须终止为 `ambiguous`，只能由管理员显式创建带重复投递风险提示的不可变子 run，禁止静默自动重发。调度停机期间只合并为一个有用 occurrence，并记录 bounded misfire evidence，不生成无界补跑积压。

门禁：同一 scheduled slot 在并发副本中至多落一个 run；过期/错误 token 不能续租或结算；生成失败不得创建“成功”报表，任一遥测查询失败或行/字节上限都使生成失败；generation success 与 delivery outbox 插入同一 fenced transaction；过期 delivery lease 进入 `ambiguous`；手动运行不改变 recurring cadence；重复 Idempotency-Key 同形状重放原结果、异形状冲突；跨租户 ID 表现为 not found；软删除保留历史且存在 active work 时拒绝；前端独立呈现 generation/delivery 状态和 SMTP 风险。

部署约束：这是版本 82→83 的协调切换。先停止并 drain 全部 v82 backend，再应用迁移 83 并启动 v83 lifecycle worker；禁止 v82 cron scanner 与 v83 durable worker 重叠，失败时只允许 roll-forward。

### P0.7 AI 凭证静态加密

交付物：AI provider API key 的 envelope encryption、密钥版本、轮换与不可逆 API 掩码；现有公共 HTTPS safe-egress、DNS 重验、IP pinning、禁代理/禁重定向、origin-bound Authorization 和请求/响应上限保持不变。

门禁：数据库与日志不出现明文密钥；缺失/错误主密钥 fail closed；旧密文可在轮换窗口读取并可重加密；创建/更新/读取/调用、跨租户拒绝和网络错误脱敏都有定向证据。

实现状态（2026-09-11，源码层完成；运行时证据待统一验证）：

- 新增 `backend/pkg/secrets`：AES-256-GCM 信封加密，密文格式 `aenv1.<keyID>.<base64(nonce||ciphertext)>`。
  主密钥取自 `secrets.master_keys.<keyID>`（base64 的 32 字节），当前版本由 `secrets.active_key_id` 指定。
- AAD 绑定租户：把 A 租户的密文行搬到 B 租户必然认证失败，无法冒充可用凭证。
- 写入路径（创建/更新）先封装再落库；主密钥缺失、非法或长度错误一律 fail closed，绝不降级为明文。
- 读取路径（详情/列表/调用）解密；遗留明文行在迁移窗口内仍可读，调用时自动重写到当前主密钥（自愈式轮换）。
- 出参只出不可逆掩码（前 4 位 + `****`，不足 4 位全掩码）；解密失败时直接全掩码，不回显密文。
- 定向证据：`backend/pkg/secrets/envelope_test.go`（8 例）与 `backend/internal/service/ai_model_secret_test.go`（5 例）全部通过。
- 配置文件 `conf.yml` / `conf-dev.yml` / `conf.example.yml` 只写入占位符，默认未配置即 fail closed；
  生产部署须通过环境变量注入主密钥，配置文件中不得出现真实密钥。
- 未含：全局 `ai.llm.api_key`（yaml 配置）仍为明文，属配置级密钥管理，不在本项"静态加密（落库）"范围内。

**2026-09-12 更新：数据库层运行期证据已取得**（`docs/validation/P0.7-secret-encryption-evidence.md`，
常驻用例 `internal/service/ai_model_secret_postgres_test.go`，缺 DSN 则 Skip）：
断言落到存储层——直接 `SELECT api_key` 后确认**库内不含明文**且为信封格式
（只断言"调用了加密"发现不了降级明文落库）；跨租户搬运密文解不开（AAD 绑定）；
出参掩码不回显明文；主密钥缺失时 `Seal` fail closed；遗留明文可读且标记 `needsReseal`。
**本项仍记为 partial**：生产环境主密钥注入与"日志无明文"未验证。

## P1：平台核心竞争力

### P1.1 通用 Entity Relations

交付物：设备/资产/客户/网关关系表、方向/类型/元数据、CRUD/查询、租户 Scope、看板和权限集成。

框架代码：

    CREATE TABLE entity_relations (
      id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id varchar(36) NOT NULL,
      from_type varchar(32) NOT NULL, from_id varchar(36) NOT NULL,
      relation_type varchar(64) NOT NULL, to_type varchar(32) NOT NULL, to_id varchar(36) NOT NULL,
      metadata jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now(),
      UNIQUE (tenant_id, from_type, from_id, relation_type, to_type, to_id)
    );

    CreateRelation(ctx context.Context, r Relation) error

实现状态（2026-09-11，部分完成——模型、校验与迁移已落地；CRUD 与看板集成未做）：

- 新增迁移 `backend/sql/85.sql`：`entity_relations` 表。
  唯一约束包含 `tenant_id`（跨租户关系不会互相覆盖），
  **CHECK 直接拒绝自环**，并按起点/终点各建一条索引支持正查与反查。
- 新增 `backend/internal/model/entity_relation.go`：`EntityRelation` 模型与集中校验。
  要点：
  - **关系是有向的**：`relation_type` 不隐含对称，反向必须显式写入，
    不允许由查询层"脑补"出来（提供 `IsReverseOf` 便于提示而非自动生成）。
  - **实体类型走受控白名单**（device/asset/customer/gateway），
    拒绝任意字符串，防止关系图语义漂移。
  - 关系类型长度与元数据大小均有上限，超限拒绝而非静默截断。
- 定向证据：`entity_relation_test.go` 6 例通过（必填、白名单、自环、超限、反向判定）。
- 未完成（本项**不算完成**）：DAL/Service 的 CRUD 与查询、租户 Scope 守卫、
  看板与权限集成，以及迁移 85 的实际执行（磁盘已满，无法验证）。
    ListRelations(ctx context.Context, q RelationQuery) ([]Relation, error)
    DeleteRelation(ctx context.Context, id string) error

门禁：成环策略明确；父租户可按 Scope 查询子租户，子租户不能越权；删除实体有关系保护或级联策略；API/UI/E2E 四面一致。

### P1.2 规则链可靠性

交付物：节点超时、指数退避、失败分支、死信、单消息 Trace、输入回放、草稿/发布版本和审计。

框架代码：

    type NodePolicy struct { Timeout time.Duration; MaxAttempts int; Backoff time.Duration; DeadLetter bool }
    type TraceEvent struct { ExecutionID, NodeID, Status string; Input, Output json.RawMessage; Error string; At time.Time }
    func (e *Engine) Execute(ctx context.Context, chain Chain, msg Message) (ExecutionResult, error)
    func (e *Engine) Replay(ctx context.Context, executionID string) error

门禁：失败节点进入 DLQ；重试次数和延迟可观测；同一消息 Trace 可串联；发布版本可回滚；回放不重复生产副作用（除非显式确认）。

实现状态（2026-09-11，部分完成——仅节点级执行策略；失败分支、回放与版本化未做）：

- 新增 `backend/internal/service/rule_chain_node_policy.go`：节点可用 `config.policy`
  声明 `timeout_ms` / `max_attempts` / `backoff_ms` / `dead_letter` / `retry_safe`。
- **有副作用节点默认禁止重试**：`action` / `external` 分类（设备命令、Webhook、告警）
  以及注册表外的未知类型，即使配置 `max_attempts>1` 也强制只执行一次，
  除非显式声明 `retry_safe: true`。重试一次设备命令等于两次下发、重试一次 Webhook
  等于两次业务投递——静默重试会把重复副作用伪装成成功。
- 节点级独立超时：此前只有整链 `ruleChainExecTimeout=10s`，单节点无法限制。
  超时值非法（0 或超过整链上限）一律拒绝，不静默兜底。
- 指数退避 `Backoff * 2^(n-1)`，单次上限 5s；等待必须响应上下文取消，
  **剩余时间不足整链 deadline 时直接放弃重试**，绝不睡过超时窗口。
- 终局失败下沉死信（默认开启，可 `dead_letter: false` 关闭）；落点
  `ruleChainDeadLetterSink` 为注入点，未接线时旁路，不改变既有行为。
  死信刻意不含消息载荷，沿用 trace 的审计最小化约定。
- 重试次数与累计退避进入聚合错误文案（`attempts=N, backoff=Nms`）以满足
  "重试次数和延迟可观测"；trace 表未加列，避免为可观测性引入新迁移。
- 定向证据：`rule_chain_node_policy_test.go` 11 例通过（默认值、9 种非法策略拒绝、
  无副作用节点重试到上限、有副作用节点不重试、retry_safe 放行、未知类型按有副作用处理、
  节点超时生效、退避响应取消、死信字段完整、死信可关闭、成功不产生死信）。
  既有 `-run RuleChain` 用例无回归。
- **失败分支（已补）**：`RuleChainEdge` 增加 `kind` 字段（`success` / `failure`），
  **空值等价 success**，存量 graph JSON 行为完全不变。成功路径只走 success 边，
  `Successors` 已排除 failure 边；节点失败时改走 `FailureSuccessors`。
  - 被失败分支接管后，该错误**不再计入聚合 errs**（表示已被下游处理），
    但 trace 与死信照常记录——失败分支只接管流向，**不抹除失败事实**。
  - 失败分支自身失败时其错误照常冒泡，不会被吞掉。
  - 失败事实以 `rc_failed_node` / `rc_error` 注入下游 metadata，
    只带错误文本不带原始载荷（沿用审计最小化约定）。
  - 非法边类型一律拒绝，不静默按 success 兜底。
  - 定向证据：`rule_chain_failure_edge_test.go` 8 例通过（非法边类型拒绝、
    三种合法 kind、成功路径跳过失败边、失败分支接管、失败分支自身错误冒泡、
    无失败分支时保持既有行为、metadata 承载失败事实、无 kind 存量边行为不变）。
- **输入回放与副作用显式确认（已补）**：`backend/internal/service/rule_chain_replay.go`。
  - `ruleChainReplayRecorder` 为可注入记录落点，**默认不接线**（nil 即旁路、热路径零开销）。
    回放必然要留存输入，这与 trace 的"审计最小化"不是一回事：trace 只记事实、replay 记输入，
    因此默认状态不留存任何载荷，只有运维显式接入持久化时才产生第二份数据。
  - 回放**不沿图继续遍历**：后继节点各有自己的记录，跟着边走会把下游重复执行 N 遍。
  - **副作用闸门**：命中 `action`/`external` 及未知类型节点时，未显式确认即**整体拒绝**，
    一个节点都不执行——绝不放行到一半才发现有副作用。确认后放行，并在 metadata 打
    `rc_replay` / `rc_replay_of`，使重放产生的副作用可被追溯、不与首次执行混淆。
  - **节点类型漂移拒绝重跑**：记录是按旧类型语义捕获的，换类型后旧输入不再适用。
  - 回放必须带来源执行 ID，否则无法审计。
  - 定向证据：`rule_chain_replay_test.go` 10 例通过（缺来源 ID 拒绝、空记录拒绝、
    未确认副作用整体拦截且零执行、确认后执行、无副作用节点免确认、类型漂移拒绝、
    节点缺失上报、重放标记、未接线旁路、记录捕获到输入）。
- 仍未完成（本项**不算完成**）：草稿/发布版本与回滚、真实链路 E2E。
  回放持久化未接数据库（迁移号位 87/88/89 已被占用，回放持久化需另行排号）。

**2026-09-12 更新：草稿/发布版本与回滚已落地（上文"版本化未做"已过时）**

- 语义层 `backend/internal/service/rule_chain_version.go`：版本单向 `draft -> published`；
  published 只读、不可重复发布；**回滚产生新草稿而非回写原版本**，原历史保持只读，
  两侧各留一条审计事件；版本号单调递增且由已有最大版本推导；**图哈希参与版本身份**，
  内容未变不产生空版本。定向证据 9 例通过。
- 持久化迁移 `backend/sql/93.sql`：`rule_chain_versions` 表。关键设计是
  **部分唯一索引保证一条链同一时刻只有一个 published**——把不变式交给数据库，
  不做应用层"先查再写"（高并发必漏判）。status 受 CHECK 约束，
  `rolled_back_from` 记录回滚来源便于双向追溯。
- 端点接线（model / dal / service 编排 / api / router）：
  `GET /api/v1/rule-chains/:id/versions`、`POST /api/v1/rule-chains/:id/versions`、
  `POST /api/v1/rule-chains/versions/publish`、`POST /api/v1/rule-chains/versions/rollback`。
  三条新路由已在 93.sql 登记 Casbin——缺登记会让后端在启动期 fail-fast 拒绝启动。
- 编排函数统一带 `Record` 后缀（`PublishRuleChainVersionRecord` /
  `RollbackRuleChainVersionRecord`）：避免与纯语义函数同名，**Go 没有重载**，同名会编译失败。
- 运行期证据：`docs/validation/P1.2-rulechain-version-evidence.md`。真实 PostgreSQL 验证 7 项全过，
  含"第二条 published 被部分唯一索引拒绝"（`rule_chain_versions_single_published_idx`），
  并已固化为常驻用例 `internal/service/rule_chain_version_postgres_test.go`（缺 DSN 则 Skip，不假通过）。
- 仍缺：真实链路 E2E（需活栈）。**因此 P1.2 整体仍记为 partial 而非 done。**

### P1.3 Widget 与 SCADA 基础层

交付物：多项目/多看板、Widget 注册、数据源绑定、变量、工业符号、实时控制、版本发布；明确 3D 仅为一种 Widget。

框架代码：

    export interface WidgetDefinition {
      type: string; version: string; schema: JSONSchema7
      render: (ctx: WidgetRenderContext) => VNode
      commands?: CommandDefinition[]
    }
    export interface DashboardProvider {
      list(projectId: string): Promise<Dashboard[]>
      publish(id: string, version: number): Promise<void>
      rollback(id: string, version: number): Promise<void>
    }

门禁：项目 CRUD 不再返回 unsupported；画布保存/加载可往返；遥测断线有状态；控制命令有权限、确认和审计；3D/WebGL 降级不影响 2D 看板。

实现状态（2026-09-11，**后端内核已完成，尚未接线到 HTTP/前端，本项不算完成**）：

- 迁移 `backend/sql/88.sql`：`scada_projects`（多项目容器，此前根本没有"项目"这一层，
  所以项目 CRUD 只能返回 unsupported）、`scada_documents`（画布，草稿/发布/归档三态）、
  `scada_document_versions`（发布快照，不可变）、`scada_control_audits`。
  `global.VERSION_NUMBER` 已提到 88。
- 模型 `internal/model/scada.go` + DAL `internal/dal/scada.go`：
  - 保存走**条件更新**（`WHERE current_version = ? AND status <> 'ARCHIVED'`），
    由 RowsAffected 判定成败；RowsAffected=0 时服务层**再查一次**来区分
    "版本冲突 / 已归档 / 不存在"，不猜。
  - `published_version` 可空：NULL 表示从未发布，与"发布了第 0 版"是两种事实。
  - 画布 JSON 超限拒绝，不静默截断（截断后无法解析，等于存一份永久损坏的画布）。
- 服务 `internal/service/scada_document.go`：项目 CRUD、画布往返、乐观并发、发布、
  回滚。**回滚不改历史**——把目标版本内容写成新的草稿版本，且不自动发布，
  与 fleet 批次回滚保持同一语义。
- 服务 `internal/service/widget_registry.go`：Widget 按 (type, version) 注册，
  能力必须显式声明（空能力按"没想清楚"拒绝，不静默当 2D）。
  3D 降级是**逐个 Widget** 的：无 WebGL 时 3D Widget 转 degraded，2D Widget 全部照常可用，
  未注册 Widget 转 unknown，两者都不阻断整块看板加载。
- 服务 `internal/service/scada_control.go`：控制命令依次过
  「存在性 → 归档终态 → Widget/命令已注册 → 权限 → 二次确认」，
  确认令牌由 HMAC 绑定 (租户, 文档, Widget, 命令, 操作人, 过期时间)，密钥缺失即 fail closed。
  **审计先于执行落库（pending），写不进去就拒绝执行**；被拒绝的命令同样留痕。
- 服务 `internal/service/scada_telemetry_link.go`：链路状态机。
  未连接时数据一律判为陈旧，且**不参考最后一帧有多新**——断线后继续把最后一帧
  当实时值显示，正是本项目要消灭的假成功。
- 证据：服务层定向用例全通过；`scada_postgres_test.go` 4 例在**真实 PostgreSQL** 通过
  （复合唯一约束、jsonb 往返、乐观并发、审计 pending→success、状态 CHECK）。
- HTTP 接口与路由**已接线**（`internal/api/scada.go` + `router/apps/scada.go`）：
  项目 CRUD、文档 CRUD/保存/发布/回滚/归档/版本/审计、控制命令与确认令牌签发，
  共 19 条端点，并由 `router/apps/scada_routes_test.go` 在真实 Gin 引擎上校验注册结果
  （静态契约测试只解析源码，证明不了端点真的挂上了）。
  租户一律由 claims 推导，非系统管理员指定其他租户明确拒绝而非静默降级。
  **控制相关端点在服务未接线时 fail closed**（ScadaControl 为 nil 即报错），
  接口存在不等于能力可用。
- 前端画布编辑器**已建**（`frontend/src/views/visualization/scada-editor/`）：
  - `scada-model.ts`（纯模型）+ `scada-model.test.ts` **24 例通过**，覆盖：
    画布解析**失败即阻断保存**（不退化成空画布，否则一保存就覆盖真实内容）、
    序列化往返、版本冲突与归档错误的分别识别、
    遥测陈旧判定（断开状态下最后一帧再新也算陈旧）、3D 降级不影响 2D、
    未知命令默认要求确认（fail closed）。
  - `index.vue`：项目/文档管理、画布编辑、带版本号保存、冲突与归档分别提示、
    发布/回滚/归档、陈旧横幅、降级提示、控制命令确认流程。
  - API 客户端 `src/service/api/scada.ts`；i18n 四个语种各 34 键；
    路由已注册（`visualization_scada-editor`）。前端 typecheck 干净。
- **控制服务已装配进启动流程**（`internal/app/scada_mobile_wiring.go` + `main.go`）：
  - `service.AssembleScadaControl` 注入内置 Widget 注册表、二次确认签发器（密钥取自
    `scada.control.confirmation_secret` / `GOTP_SCADA_CONTROL_CONFIRMATION_SECRET`）
    与真实下发执行器；执行器委托 `CommandData.CommandPutMessageWithTracking`。
  - **下发必须携带真实 claims**：命令通道在没有 claims 参数时会跳过设备写权限校验
    （`command_data.go: ensureCommandWriteAccess`），因此 `ControlExecution.ActorClaims`
    为空时执行器直接拒绝执行。已有用例锁住这条闸与「凭证一路传到执行器」。
  - 密钥未配置**不阻断启动**，但打 warn：签发不出令牌，所有需要确认的命令被拒。
    未启用 SCADA 的部署不该被一个用不到的密钥挡在门外，代价是日志里必须看得见。
  - 内置 Widget 与前端 `WIDGET_REGISTRY` 的一致性由
    `TestBuiltinWidgetRegistryMatchesFrontend` 兜住（该用例直接解析前端源码，
    并已用「改前端版本号 → 用例失败」做过负向对照）。
- 未完成（**明确不算完成**）：
  - **画布拖拽未接线**（`grid-layout-plus` 现无调用方证据，不盲接；界面上未做假入口）
  - Widget 注册表仍是前后端各一份常量（有一致性测试兜底，但未改为前端从后端拉取）
  - 内置 Widget 的 `schema` 是最小合法 JSON 对象，**尚未定义真实配置字段**，
    因此后端目前不校验画布里单个 Widget 的配置内容
  - 工业符号库未做、3D 无实际 Widget 渲染
  - **无浏览器/运行时 E2E**（模型层有单测，界面未经真实浏览器验证）
  - **未做真实下发联调**：执行器接到真实命令通道，但没有在连着 broker 的环境里
    验证过一条命令真的到达设备

### P1.4 移动端控制与通知

交付物：命令、影子、告警确认、OTA 状态、Dashboard 查看、FCM/APNs 抽象、离线缓存、Android/iOS 构建。

框架代码：

    interface MobileApi { listDevices(): Promise<Device[]>; sendCommand(id: string, c: Command): Promise<CommandResult>; ackAlarm(id: string): Promise<void>; getShadow(id: string): Promise<Shadow>; }
    interface PushProvider { register(token: string): Promise<void>; send(message: PushMessage): Promise<void>; }

门禁：角色权限与 Web 端一致；弱网重试不重复命令；推送失败可重试并可审计；至少 Android/H5 一条完整业务 E2E。

实现状态（2026-09-11，**推送与幂等内核已完成，移动端本身未开始，本项不算完成**）：

- 迁移 `backend/sql/88.sql`：`push_device_registrations`（令牌登记，复合唯一含租户/用户/平台/令牌）、
  `push_deliveries`（投递与重试审计）。
- 模型 `internal/model/push.go` + DAL `internal/dal/push.go`：
  - 令牌登记走数据库 `ON CONFLICT` upsert，不做"先查再插"；重复登记不产生第二行，
    否则一次推送会被放大成 N 条。
  - 终态 `dead` 与可重试的 `failed` 分开表达；终态不得携带 `next_attempt_at`，
    否则重试器会把已放弃的投递重新捞起，把失败伪装成"还在路上"。
  - 捞取重试必须同时限定 status 与 `next_attempt_at <= now`，避免重试节奏被击穿。
- 服务 `internal/service/push_provider.go`：`PushProvider` 契约（FCM/APNs 适配器位），
  **无可用 Provider 时报错而非静默成功**；重试有上限，用尽转 dead；
  每次尝试都计 `attempt_count` 并留 `last_error`，投递历史即审计。
- 服务 `internal/service/mobile.go`：移动端能力聚合。
  - **能力矩阵由实际接线决定**：依赖没注入就报 false，调用即失败——
    先报 true 再说会让移动端展示一堆点了就报错的功能。
    `OfflineCache` 恒为 false：那是客户端能力，服务端报 true 等于替客户端撒谎。
  - 弱网幂等：命令必须带幂等键，命中已完成键**直接返回原收据且不再下发**；
    下发失败**释放键**以保证可重试；同一键换参数必须拒绝
    （否则第二条命令返回第一条的结果，用户以为生效其实没有）。
  - 幂等存储为接口，默认实现是进程内的（重启即失），跨实例强幂等需换成 DB/Redis 实现。
- 证据：服务层定向用例全通过；`scada_postgres_test.go` 中推送 2 例在**真实 PostgreSQL** 通过
  （upsert 不产生重复行、failed→dead 终态、终态不再被捞起、状态 CHECK 生效）。
- HTTP 接口与路由**已注册**（`internal/api/mobile.go` + `router/apps/mobile.go`）：
  能力矩阵、推送登记/撤销、幂等命令下发；命令强制 `Idempotency-Key` 头。
  **服务未接线时全部 fail closed**（`service.GroupApp.Mobile` 为 nil 即报错），
  唯独能力矩阵接口返回全 false——那正是"未接线"的如实声明，供客户端隐藏入口。
- **移动端服务已装配进启动流程**（与 SCADA 同一个 `WithScadaMobileWiring`）：
  命令、设备列表、告警、影子均已接到真实实现；OTA / 看板 / 推送投递仍未接线。
- **设备列表 / 告警 / 影子已接线，且复用既有归属过滤**（`internal/service/mobile_adapters.go`）：
  - 归属过滤**不自己实现**，一律委托既有判定：`applyDeviceListOwnerFilterForClaims`
    （设备）、`GetAlarmHisttoryListByPage` 内部的 `deviceOwnerUserIDFilterForClaims`（告警）、
    `ensureTelemetryDeviceReadAccess` / `ensureAlarmHistoryWriteAccess`（影子与告警确认）。
    自己拼 `WHERE owner_user_id = ?` 会在管理员处漏数据、在普通用户处把"没配归属的设备"全放出去。
  - **依赖契约收完整 `*utils.UserClaims` 而不是 (tenantID, userID)**：归属过滤由
    `claims.Authority` 决定，光有 userID 拼不出这个判断。HTTP 接口也因此**不收 `tenant_id` 入参**——
    允许调用方指定租户等于把过滤开关交出去。
  - 端点：`GET /mobile/devices`、`GET /mobile/alarms`、`POST /mobile/alarms/:id/ack`、
    `GET|PUT /mobile/devices/:id/shadow`，由 `router/apps/scada_routes_test.go` 在真实 Gin 引擎上校验。
  - 分页夹紧（默认 20，上限 100）：`page_size` 直接下推会让 0 变成"不限量"。
  - **影子语义如实说明**：本项目的影子是**离线命令队列**（`device_shadow_messages`），
    不是自由格式的 desired/reported 文档。`GET` 返回影子消息队列视图，
    `PUT` 提交 `{"method":..., "params":...}` 命令载荷（在线即下发、离线入队）；
    非 JSON 载荷直接拒绝，不静默存一份设备侧解析不了的字节。
  - `MobileDeviceSummary` 用 `warn_status`（"Y"/"N"）而非 `alarm_count`：设备列表查询
    不 join 告警表，造一个 0/1 的"条数"会让移动端把布尔标记当数量展示。
- 证据：`TestMobileDeviceListOwnershipFilterOnPostgres` /
  `TestMobileDeviceListHidesUnownedDevicesFromTenantUser` 在**真实 PostgreSQL** 通过
  （普通用户 A/B 各只见自己的设备、租户管理员见全部、其它租户见 0 台、
  无归属设备对普通用户不可见），并已用「移除归属过滤 → 用例失败」
  做过负向对照，确认断言真的在卡这条规则。
  告警与影子的过滤直接复用既有实现，其校验由既有告警/影子用例覆盖，未另造一套。
- **OTA 状态已接线**（`GET /mobile/devices/:id/ota`）：
  - 先过 `ensureTelemetryDeviceReadAccess`（与影子/遥测同一道闸，含归属判定），
    再以**设备的租户**查 OTA 明细。
  - 新查询 `dal.LatestOTAUpgradeDetailForDevice`：**`ota_upgrade_task_details` 自身没有
    tenant_id 列**，租户要经 `task → package.tenant_id` 两级关联才拿得到。
    只按 device_id 查会跨租户泄漏升级进度，故必须走包路径过滤。
  - 无升级记录返回 `none`（不是报错）；未知状态码返回 `unknown`（不猜，猜错会把失败显示成成功）。
- **看板列表已接线**（`GET /mobile/dashboards`）：复用 `Board.GetBoardListByPage`。
  看板**没有归属列**，是租户级共享资产，可见性由既有 `resolveBoardListTenant` 按角色裁决
  （SYS_ADMIN 全量 / TENANT_ADMIN 本租户 / 其余拒绝）。这里刻意**不加** owner 过滤——
  无字段可依，且会改掉"看板是共享资产"的既有语义。
- **推送投递已可接线**：实现了 FCM HTTP v1 Provider（`internal/service/push_provider_fcm.go`）
  与 APNs Provider（`internal/service/push_provider_apns.go`）。
  - 服务账号 JWT（RS256）换 OAuth2 令牌，令牌缓存到过期前 60 秒。
  - **错误分可重试与终态**：429/5xx 与换票失败=可重试；4xx（除 429）与空令牌=终态，
    由 `PushService.settleTerminal` 直接置 dead，不再浪费重试预算——
    令牌失效重试一万次也不会成功，还会把这个事实埋进最后一次 last_error。
  - `AssemblePush(PushWiringConfig{FCM, APNs})` **按凭据分别注册**：某个渠道没配齐就
    不注册该渠道；**两个都没配**返回 nil 服务（不是空壳），能力矩阵报 `push=false`；
    配了但凭据非法则**阻断启动**，不静默降级成"不发推送"。
    `ErrFCMNotConfigured` / `ErrAPNSNotConfigured` 是"没配"，不是错误，不阻断启动。
  - **APNs（iOS）已实现**：ES256 签名 JWT（`kid` 头）作 Bearer，**强制 HTTP/2**
    （APNs 只在 HTTP/2 上服务，走 HTTP/1.1 会被拒），`apns-topic` / `apns-push-type` 齐备，
    自定义字段放在 `aps` 之外（放进 `aps` 会被 Apple 静默丢弃），非字符串值转字符串
    （与 FCM `data` 同一约束）。错误分类同 FCM：429/5xx 可重试，400/403/410/413 终态，
    其中 410 = 令牌对该 topic 已失效，直接置 dead。
- 证据：
  - FCM：httptest 起假 OAuth2/FCM 端点，覆盖成功、429/5xx 可重试、404/403/400/401 终态、
    空令牌终态、换票失败可重试、令牌缓存（3 次发送只换票 1 次）、请求体断言
    （含 `data` 非字符串值转字符串），共 14 例通过。
  - APNs：httptest 起 **HTTP/2** 假端点（断言 `r.Proto == "HTTP/2.0"`，否则测的是不存在的协议路径），
    覆盖配置校验（缺字段逐个报、RSA 密钥冒充 EC 密钥须拒）、仅支持 iOS、请求头与路径、
    载荷形状（自定义字段在 `aps` 外、数值转字符串）、429/500/503 可重试、410/400/403/413 终态、
    空令牌终态、JWT 复用（含 `kid`/`alg` 解码校验）、双渠道/单渠道/凭据非法阻断/都没配四种装配组合，
    共 16 例通过。
  - OTA / 看板：真实 PostgreSQL 上验租户边界（别租户同名设备的 `succeeded` 记录不泄漏）、
    归属闸（同租户普通用户读不到他人设备的 OTA 状态）、看板只见本租户且普通用户被拒。
    **负向对照**：摘掉 `p.tenant_id` 条件 → 用例失败（读到别租户的 succeeded），已还原复测。
  - **移动端接口级 E2E**（`backend/router/apps/mobile_e2e_test.go`，8 例通过）：
    真实 Gin 引擎 + 真实统一响应中间件 + 真实 PostgreSQL，走 HTTP → 路由 → handler →
    service → DAL → 库全链路。覆盖能力矩阵、设备列表、影子写入读回与非法载荷拒绝、
    OTA（无记录 `none` → 有记录 `upgrading`）、看板（管理员可见 / 普通用户按既有规则被拒）、
    命令缺幂等键被拒、推送令牌登记/重复登记不增行/撤销/撤销不存在报 404。
    **负向对照**：把能力矩阵的 `push` 改成硬编码 true → 用例失败，已还原复测。
  - 统一响应中间件对业务错误**也返回 HTTP 200**（错误码在 body 的 `code`），
    所以这套 E2E 一律断言 `code` 而不是 HTTP 状态；只看 HTTP 200 会把错误当成通过。
- 顺带修掉的既有缺陷（由上述 E2E 暴露，非本项功能，但如实记录）：
  - `UnsubscribePush`：登记 id 是 uuid 列，**非 uuid 的输入原样进 SQL** 会让 PG 抛 22P02，
    而该错误经 `CodeDBError` 把驱动原文（`SQLSTATE`）带回了客户端。改为先判 uuid，统一按 404 返回。
  - 告警历史的读/写两条路径：查不到记录（`gorm.ErrRecordNotFound`）被当成
    `101001 数据库错误` 返回，响应里带 `sql_error: "record not found"`。
    改成 `404 alarm history not found`——"没有这条告警"不是系统故障，
    混在一起会让客户端把它判成可重试的错误反复重试。**该改动影响 Web 端告警接口（同一条路径）**。
- 未完成（**明确不算完成**）：
  - **FCM / APNs 都未与真实 Firebase / Apple 联调过**。httptest 覆盖了各条分支，
    但真实通道的 4xx 错误码分布、`data` 字段限制、APNs sandbox 与生产的 topic 差异
    需拿到真实凭据后补一次验证。
    **凭据就位的验证入口已经写好**（`internal/service/push_provider_live_test.go`）：
    默认 SKIP，设置 `AETHERLINK_FCM_LIVE_*` / `AETHERLINK_APNS_LIVE_*` 后即跑真机通道，
    每个渠道都同时验"有效令牌必须成功"与"无效令牌必须失败且判终态"两件事。
    在此之前**不得**声称推送已验证。
  - **Android/iOS 构建未做**（无客户端工程，本仓库目前只有后端）
  - **真机业务 E2E 未做**（需 Android/iOS 客户端 + 真实推送通道）。
    上面那套是**接口级** E2E：链路真实但没有客户端参与，不能顶替门禁里的
    「至少 Android/H5 一条完整业务 E2E」。
  - 进程内幂等存储**重启即失**，跨实例强幂等需换成 DB/Redis 实现同一接口
  - 告警列表透传既有动态投影（`[]map[string]interface{}`），未为移动端另造强类型 DTO

### P1.5 边缘运维

交付物：节点注册/证书、心跳健康、版本兼容、配置/模板/规则下发状态、冲突解决、远程升级和回滚。

框架代码：

    type EdgeNode struct { ID, TenantID, Version, Status string; LastSeen time.Time; Capabilities []string }
    type SyncJob struct { ID, NodeID, Resource, Version, State string; Conflict *Conflict }
    func (m *Manager) Reconcile(ctx context.Context, node EdgeNode) error

门禁：断云自治不丢本地数据；重连后按版本同步；冲突进入人工可见状态；节点离线和升级失败产生告警。

实现状态（2026-09-11，部分完成——仅治理决策层；节点注册/证书与 Reconcile 未做）：

- 新增 `backend/internal/service/edge_governance.go`（纯决策函数，零迁移）：
  - `ClassifyEdgeNodeHealth`：按最后心跳判定 online/degraded/offline/**unknown**。
    **心跳为 nil、零值或晚于当前时间一律判 unknown**——时钟异常不得被乐观地当成"刚上报过"。
  - `CheckEdgeVersionCompatibility`：点分数字版本比较，**版本串为空或含非数值段一律判不兼容**。
    无法判断兼容就不允许下发，避免在边缘把节点刷成砖。
  - `DetectEdgeSyncConflict`：**同一资源若已有一份内容不同的在途快照即判冲突**，只检测上报、
    **绝不自动合并或自动覆盖**——否则边缘最终状态取决于消息到达顺序，出问题无法归因。
    内容一致视为幂等重发，不算冲突；**在途快照解析失败按"内容不同"处理**，
    宁可升级为人工确认也不静默放行。
- 接线：`CreateEdgeSync` 落库前先跑冲突闸门，命中即返回带 `conflict=true` 与在途任务 ID 的
  参数错误。**查询在途任务失败同样直接报错**——无法确认无冲突就不允许下发（fail closed）。
- 定向证据：`edge_governance_test.go` 11 例通过（9 行健康判定时间表、两种"不得乐观"场景、
  10 行版本兼容表、冲突判定/幂等/跨资源/跨类型/缺 ID/解析失败升级/跳过 nil/载荷解析）。
- 仍未完成（本项**不算完成**）：节点注册与证书、断云自治与重连后按版本同步
  （当前 `edgeSyncPayload.Version` 是**快照格式版本**且恒为 1，没有同步修订号，
  无从判断"边缘已拿到哪一版"）、远程升级回滚、Reconcile 编排，以及真实边缘节点联调。

### P1.6 模板市场产品化

交付物：浏览/搜索/行业打包下载、导入冲突预览、签名、依赖检查、升级/回滚和审计。

门禁：租户幂等；坏签名/坏依赖拒绝；升级可回滚；导入不产生孤儿租户数据；所有动作有审计记录。

实现状态（2026-09-11，部分完成——签名、依赖自洽与冲突预览；升级/回滚与审计记录未做）：

- 现状：市场目录与打包导出已有（`MarketCatalog` / `ExportMarketBundle`），
  但 `MarketBundle` 此前**没有签名、没有依赖声明、没有版本**，包在租户间流转时
  既无法验真也无法预判导入后果。
- 新增 `backend/internal/service/device_template_market_integrity.go`（纯逻辑，零迁移）：
  - `MarketBundle` 增加 `digest` / `signature` / `signed_key_id`（均 `omitempty`，
    老包解析不受影响）。摘要覆盖**除签名三字段外的规范 JSON**，否则无法验签。
  - `SignMarketBundle` / `VerifyMarketBundle`：HMAC-SHA256，**摘要与签名都用常量时间比较**，
    避免通过响应时间侧信道推断。验签按"未签名 → 密钥缺失 → 摘要不符 → 签名不符"逐级拒绝。
  - **签名密钥与 P0.7 的加密主密钥分开**（`market.bundle_signing_keys`），
    签名与加密不共用同一把钥匙；密钥需 base64 且不小于 32 字节，短密钥与非法 base64 一律拒绝。
  - `CheckMarketBundleDependencies`：包内自洽检查——模板名缺失、包内重名
    （导入后互相覆盖，最终状态取决于顺序）、模板 `type_key` 与包声明不符、`count` 与实际条数不符。
  - `PreviewMarketBundleImport`：**只读**，不落库不建模板，只回答"导入会发生什么"，
    区分 create / overwrite / blocking；**已存在模板必须显式列为覆盖项**，
    阻断项非空即不应导入。
  - 接线：`ExportMarketBundle` 出包即签名。**未配置签名密钥一律拒绝出包**
    （与 P0.7 一致：默认未配置即 fail closed）——这意味着**打包导出端点在配置
    签名密钥前不可用**，属刻意行为变更。
  - 配置占位符已写入 `conf.yml` / `conf-dev.yml` / `conf.example.yml` 的 `market` 段。
- 定向证据：`device_template_market_integrity_test.go` 11 例通过（未配置密钥拒绝出包、
  短密钥与非法 base64 拒绝、签名验签往返、未签名拒绝导入、篡改内容摘要失配、
  换密钥验签失败、摘要不含签名字段、依赖检查 6 个场景、冲突预览 3 个场景）。
- 仍未完成（本项**不算完成**）：升级/回滚（需版本与快照，待排迁移）、导入动作的审计记录、
  导入接口本身尚未接入打包载荷（当前只有单模板 import 与打包 export），以及真实端到端验证。

## P2：生态、分析与规模

### P2.1 协议插件 SDK

    type ProtocolAdapter interface { ValidateConfig(any) error; Connect(context.Context) error; Discover(context.Context) ([]Device, error); ReadTelemetry(context.Context) (Telemetry, error); WriteCommand(context.Context, Command) error; Health(context.Context) Health; Close() error }

交付物：插件 manifest、配置 Schema、点表、凭证映射、指标、版本兼容和签名；按客户需求接入 CAN/BACnet/BLE/LoRaWAN。

### P2.2 Trendz 类轻量分析

    type AnalysisQuery struct { DeviceIDs []string; Keys []string; From, To time.Time; Granularity string; Aggregations []string }
    func (s *AnalyticsService) Query(ctx context.Context, q AnalysisQuery) (AnalysisResult, error)
    func (s *AnalyticsService) Export(ctx context.Context, q AnalysisQuery, format string) (io.ReadCloser, error)

交付物：多设备对比、聚合/同比环比、基础异常、CSV/Excel、权限和分享。P0.6 先提供可靠的定时报表执行、历史与 SMTP 事实语义；本阶段在该 durable execution contract 上扩展分析查询和展示，不再创建第二套调度系统。

### P2.3 数据保留与性能

交付物：保留策略、降采样、冷热分层、查询缓存、基准压测、容量模型和告警。

门禁：明确 p95/p99、吞吐、数据完整性和降级行为；至少单实例和双实例报告；压测不使用假数据掩盖数据库瓶颈。

## P3：商业化与长期能力

- 多地域/高可用故障演练、RPO/RTO 和滚动升级。
- 计费/配额/审计导出、客户自助开通和商业许可证边界。
- 移动端正式商店发布、桌面运维工具和行业解决方案包。
- 生态市场运营、第三方插件签名和供应链扫描。

## 4. 统一验收门禁

每项任务必须同时提供：

1. 源码交付：迁移、DAL/service/API、前端或 CLI、配置和文档。
2. 契约测试：成功、失败、越权、幂等、超时和降级路径。
3. 运行证据：真实依赖或明确隔离 stub；记录命令、版本、日志、数据和清理结果。
4. 四面一致：API/OpenAPI、后端权限、UI 行为、自动化 E2E 对齐。
5. 状态标记：done 仅限已运行证明；partial 表示代码存在但关键闭环缺失；pending 表示等待环境或实现。

禁止用路由可访问、单元测试绿色、覆盖率数字或页面截图单独宣称功能完成。

## 5. 给实现模型的统一提示

    你在 AetherLink IoT 仓库实现一个路线图任务。先阅读 AGENTS.md、ROADMAP.md、相关迁移/API/UI/测试，确认现状和未提交改动。
    只修改任务边界内文件，沿用现有 Go/Vue/SQL/Redis 模式，不引入无消费方的新抽象。
    先补失败、越权、幂等和超时测试，再实现代码；迁移必须可重跑且兼容全新库。
    完成后运行最小定向测试，再运行受影响的 release/API/UI/E2E 门禁。
    在 docs/validation/ 写证据：命令、版本、结果、日志关键行、清理动作和仍未验证的部分。
    最终报告严格分为 done / partial / pending，不把静态检查当作运行期闭环。

## 6. 当前执行顺序

立即执行：canonical archive producer → exact backend/GMQTT identity 与 placeholder gate → DataItemFetcher failure algebra 收口 → P0.6 持久化报表执行 → P0.7 AI 凭证静态加密 → P0.1 → P0.2 → P0.3 → P0.4 → P0.5。当前 durable report P0 已进入实现批次；P0 未通过前，不宣称生产发布就绪。

随后执行：P1.1 → P1.2 → P1.3 → P1.4 → P1.5 → P1.6，按客户最先需要的设备关系、规则可靠性和可视化控制能力排序。

最后执行：P2 生态/分析/性能，P3 商业化和多地域能力。ThingsBoard PE/Cloud/Edge、TBMQ、Trendz 和 ThingsPanel 企业宣传能力只作为需求来源，需逐项评估授权、实现成本和客户价值后再立项。

**决策：继续执行；先做 P0 真实闭环，再扩展 P1 平台能力。**
