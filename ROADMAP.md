# AetherLink IoT 平台下一阶段路线图

> 版本：2026-09-07
>
> 目标：以 ThingsBoard（CE/PE/Cloud/Edge/Gateway/生态产品）和 ThingsPanel（社区版及官网宣称的企业扩展）为参照，把 AetherLink 从“功能已合入”推进到“可部署、可运维、可扩展、可证明”。
>
> 本文是执行基线，不是历史交付日志。每个任务都给出交付物、框架代码位置、依赖和验收门禁，便于交给其他模型实现。

## 1. 状态口径

### 1.1 已完成（有源码和测试/运行期证据）

- 接入：MQTT/HTTP、Modbus、CoAP/LwM2M、SNMP、OPC UA；GMQTT Broker、ACL、持久会话和 uplink 管道。
- 核心模型：设备、产品/模板、遥测、属性、命令、设备影子队列、资产树、租户父子层级、基础 Casbin RBAC。
- 自动化：DAG 规则链、Vue Flow 编辑器、阈值/映射/Webhook/命令/告警节点；场景自动化已有基础能力。
- 数据与运维：PostgreSQL/TimescaleDB 三态门控、遥测统计、告警、OTA 任务模型、Redis 限流、Casbin watcher、白标配置。
- 产品切片：CSV 预注册 API、行业模板种子、模板导入/导出 MVP、边缘遥测转发/缓存/命令和模板下发、移动端 H5 MVP、AI 遥测查询和告警分析。
- 质量：新库迁移 1..66 已验证；部分真实协议 E2E、双实例 watcher/限流、Timescale on/off/auto、OIDC/Keycloak 已验证。

### 1.2 部分完成（不得写成完整能力）

- 3D：设备详情 GLB 预览、遥测驱动材质/旋转和 WebGL 降级；这不等于完整 SCADA。
- 看板：基础 Native Board/ThingsVis 路径可用；native-board-provider.ts 中项目增删改和自定义 canvasConfig/nodes/dataSources/variables 仍返回 unsupported。
- OTA：任务创建、下发、进度模型已有；进度消费、失败重试、暂停/恢复、回滚未闭环。
- 影子：离线缓存和上线投递已实现；真实 broker ACK、超时/重试和 UI 全链路待验收。
- 报表/分析：遥测统计和导出局部实现；尚无 Trendz 级分析工作台、定时报表和任务调度闭环。
- 边缘：数据转发、断线缓存、RPC/实体下发已有；节点注册、健康、版本、冲突和远程运维不足。
- 移动端：登录、设备列表、最新遥测和 H5 构建；命令、告警、影子、推送、正式 Android/iOS 发布未完成。
- RBAC：路由登记和基础角色矩阵已激活；仍需按产品定义持续收紧并维护正/负向矩阵。

### 1.3 明确缺失

- 通用 Entity Relations（设备/资产/客户/网关任意关系）。
- 规则链节点级超时、重试、退避、死信、Trace、回放、版本发布。
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

### P0.3 OTA 状态机

交付物：进度消费、批次暂停/恢复/取消、失败重试、灰度、回滚和报告。

框架代码：

    type OTAStatus string
    const ( OTAPending OTAStatus = "pending"; OTARunning OTAStatus = "running"; OTASuccess OTAStatus = "success"; OTAFailed OTAStatus = "failed"; OTAExpired OTAStatus = "expired"; OTACanceled OTAStatus = "canceled" )
    type OTAProgressEvent struct { JobID, DeviceID string; Percent int; Status OTAStatus; Error string; At time.Time }
    func (s *OTAService) ConsumeProgress(ctx context.Context, e OTAProgressEvent) error
    func (s *OTAService) RetryFailed(ctx context.Context, jobID string, limit int) error

门禁：状态转移非法即拒绝；同一事件幂等；失败设备可筛选重试；回滚产生新审计事件；至少一条真实设备/broker 或协议 stub E2E。

### P0.4 场景与 Flow 语义

交付物：开始/结束时间、时区、过期、停止其他 Flow、定时器触发的统一语义。

框架代码：

    type ExecutionWindow struct { StartsAt, ExpiresAt *time.Time; Timezone string }
    func (e Engine) CanRun(now time.Time, w ExecutionWindow) bool
    func (e Engine) StopConflictingFlows(ctx context.Context, deviceID, flowID string) error

门禁：边界时间表驱动测试；重复触发幂等；停止动作可审计；服务重启后调度不丢任务。

### P0.5 CSV 浏览器 E2E

交付物：上传、校验错误展示、批量建档、一次性凭证下载、脱敏导出和清理。

门禁：真实浏览器选择文件；坏行逐行反馈；下载文件可解析；凭证只出现一次；跨租户产品不可选。

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

交付物：多设备对比、聚合/同比环比、基础异常、CSV/Excel、定时报表和权限/分享。报表后端当前仅部分实现，完成前不得标记为完整。

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

立即执行：P0.1 → P0.2 → P0.3 → P0.4 → P0.5。P0 未通过前，不宣称生产发布就绪。

随后执行：P1.1 → P1.2 → P1.3 → P1.4 → P1.5 → P1.6，按客户最先需要的设备关系、规则可靠性和可视化控制能力排序。

最后执行：P2 生态/分析/性能，P3 商业化和多地域能力。ThingsBoard PE/Cloud/Edge、TBMQ、Trendz 和 ThingsPanel 企业宣传能力只作为需求来源，需逐项评估授权、实现成本和客户价值后再立项。

**决策：继续执行；先做 P0 真实闭环，再扩展 P1 平台能力。**
