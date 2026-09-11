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

当前状态：canonical producer 已完成 run-scoped staging、partial diagnostic、report hash、cleanup truth、manifest redaction、inspector-backed validation、manifest-last 和 same-filesystem atomic rename；2026-09-10 的 producer/policy/inspector focused contracts 为 61 passing，但这只是静态 harness 证据。exact backend/GMQTT identity 与 production-reachable placeholder/no-op gate 尚未完成，因此阶段 3.0 仍为 `partial`，不得提升 Phase 0 或发布就绪。

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
- 仍未完成：灰度/金丝雀、报告导出，以及真实设备/broker 或协议 stub 的 E2E。
  另：进度事件目前只落事件表，**尚未回写明细行的状态/百分比**（需迁移加列，且磁盘已满无法验证），
  因此本项**仍不算完成**。

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
- 未含：全局 `ai.llm.api_key`（yaml 配置）仍为明文，属配置级密钥管理，不在本项“静态加密（落库）”范围内。

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

### P1.4 移动端控制与通知

交付物：命令、影子、告警确认、OTA 状态、Dashboard 查看、FCM/APNs 抽象、离线缓存、Android/iOS 构建。

框架代码：

    interface MobileApi { listDevices(): Promise<Device[]>; sendCommand(id: string, c: Command): Promise<CommandResult>; ackAlarm(id: string): Promise<void>; getShadow(id: string): Promise<Shadow>; }
    interface PushProvider { register(token: string): Promise<void>; send(message: PushMessage): Promise<void>; }

门禁：角色权限与 Web 端一致；弱网重试不重复命令；推送失败可重试并可审计；至少 Android/H5 一条完整业务 E2E。

### P1.5 边缘运维

交付物：节点注册/证书、心跳健康、版本兼容、配置/模板/规则下发状态、冲突解决、远程升级和回滚。

框架代码：

    type EdgeNode struct { ID, TenantID, Version, Status string; LastSeen time.Time; Capabilities []string }
    type SyncJob struct { ID, NodeID, Resource, Version, State string; Conflict *Conflict }
    func (m *Manager) Reconcile(ctx context.Context, node EdgeNode) error

门禁：断云自治不丢本地数据；重连后按版本同步；冲突进入人工可见状态；节点离线和升级失败产生告警。

### P1.6 模板市场产品化

交付物：浏览/搜索/行业打包下载、导入冲突预览、签名、依赖检查、升级/回滚和审计。

门禁：租户幂等；坏签名/坏依赖拒绝；升级可回滚；导入不产生孤儿租户数据；所有动作有审计记录。

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
