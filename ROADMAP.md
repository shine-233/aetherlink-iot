# AetherLink IoT 平台下一阶段路线图

> 版本：2026-09-13（v2 重构：单一事实源，取消补记层叠）
>
> 证据基线：`main@974c44a`（2026-09-13），工作树 clean，提交 239 次。
>
> 迁移链：`backend/sql/` 最大编号 `117.sql` = `backend/pkg/global/global.go` 的 `VERSION_NUMBER` = `117`（**以代码为准**），全新空库 1→115 全链已验证，增量至 117 已自动应用并验证（2026-09-20，见 §1.1 与 P0.1）。
>
> 目标：以 ThingsBoard（CE/PE/Cloud/Edge/生态产品）和 ThingsPanel（社区版及官网宣称的企业扩展）为参照，把 AetherLink 从"功能已合入"推进到"可部署、可运维、可扩展、可证明"。
>
> 本文是执行基线，不是历史交付日志。每个任务只保留**一个**"实现状态"块，状态变更直接改写该块，不再追加"更新/补记"层叠段落；详细证据与历史留痕放 `docs/validation/`。

## 1. 状态口径（单一事实源）

### 1.0 文档约定（v2 起强制）

1. **一个任务一个状态块**：禁止在任务下追加"实现状态（某日期）… **某日期更新**… **补记二**"式层叠。状态变化时直接改写原块，历史事实以 `docs/validation/` 的证据文档为准。
2. **状态词只有三个**：`done`（已运行证明）、`partial`（代码存在但关键闭环缺失）、`pending`（等待环境或未实现）。
3. **必须标注缺口类型**（v2 新增，用于区分"要开发"与"只差跑一遍"）：

| 缺口类型 | 含义 | 处置方式 |
| --- | --- | --- |
| `未实现` | 零代码或关键链路未写 | 需要开发排期 |
| `未接线` | 代码存在但未挂到 HTTP / 路由 / UI | 小开发量，可优先 |
| `未验证` | 代码 + 接线齐备，仅缺运行期证据 | 恢复环境跑一遍并归档 |
| `环境阻塞` | 验证前提不成立（无网络 / 无 Docker / 磁盘满） | 先解决环境 |
| `客户端缺失` | 需独立客户端工程（Android / iOS） | 独立立项 |

4. **禁止用路由可访问、单元测试绿色、覆盖率数字或页面截图单独宣称功能完成**（见 §4）。

### 1.1 已实现的源码基线（不自动等同当前运行期完成）

- 接入：MQTT/HTTP、Modbus、CoAP/LwM2M、SNMP、OPC UA；GMQTT Broker、ACL、持久会话和 uplink 管道。
- 核心模型：设备、产品/模板、遥测、属性、命令、设备影子队列、资产树、租户父子层级、基础 Casbin RBAC。
- 自动化：DAG 规则链、Vue Flow 编辑器、阈值/映射/Webhook/命令/告警节点；场景自动化。
- 数据与运维：PostgreSQL/TimescaleDB 三态门控、遥测统计、告警、OTA 任务模型、Redis 限流、Casbin watcher、白标配置。
- 产品切片：CSV 预注册 API、行业模板种子、模板导入/导出/升级/回滚、边缘注册/心跳/Reconcile、移动端后端能力、AI 遥测查询与告警分析、SCADA 后端内核与编辑器、离线许可证。
- 质量：迁移源码连续至 `115.sql` / `VERSION_NUMBER=115`（backend/pkg/global/global.go）。全链 `initialize.CheckVersion` 验证现状：
  - **Go 侧全链路径已跑通（2026-09-13，当时上界 99）**：全新空库 `aetherlink_go99`（PostgreSQL 17.5，隔离集群 `127.0.0.1:55433`）由后端启动流程自行迁移 → `sys_version=99` / `0.0.23`，122 张表，无迁移错误。这推翻了此前"Go 侧只验证到 93"的口径——**那条已作废，不要再引用**。证据 `docs/validation/2026-09-13-go-chain-migration-and-e2e.md` §1。
  - ~~仍未验证：94–102~~ → **已全部消解（2026-09-19）**：`cmd/migchaincheck` 在全新空库跑 **1 → 115 全链 PASS / 130 张表 / `sys_version=115` / 667ms**，94–115 段一并覆盖（证据 `docs/validation/2026-09-19-p01-migration-chain-115-evidence.md`）；P1.5/P1.6 的"98/99.sql 未复跑"缺口早已分别闭环（45 组 15/15、各复跑 2 次幂等）。
  - 口径纪律：psql 直灌（121 表）与 Go 全链（122 表）**不等价**，表数差异不说明谁对谁错，但不能用前者顶替后者的结论。

### 1.2 状态总表（唯一权威）

> 判定严格按 §4：`done` 仅限已运行证明。本表是各任务状态的唯一来源；下方 P0–P3 各节只补充设计理由与证据指针。

| 任务 | 主题 | 状态 | 缺口类型 | 未闭环（一句话） | 证据 |
| --- | --- | --- | --- | --- | --- |
| P0.1 | 发布同步与部署门禁 | `partial` | `环境阻塞` | **全新库迁移链已延伸验证至 115（2026-09-19，`cmd/migchaincheck`，1→115 全链 PASS / 130 表 / `sys_version=115` / 667ms）**；剩余三条**确实卡在真实部署环境**：HTTPS/TLS、MQTTS 上报下发、公网 MQTT；~~backup/restore 计数一致性~~ → **已闭环（2026-09-19）**：backup-restore.ps1 真实 pg_dump（11.1MB dump + SHA-256 manifest）→ 恢复到全新空库 aetherlink_p01_restore → 五张核心表行数逐一比对一致（devices 32/scene_info 10/alarm_config 10/telemetry_datas 85,869/users 7）、sys_version=115 保留 | `docs/validation/P0.1-preflight-evidence.md`、`docs/validation/2026-09-19-p01-migration-chain-115-evidence.md`、`docs/validation/2026-09-19-p01-backup-restore-counts-evidence.md` |
| P0.2 | 设备影子 ACK 闭环 | `done` | 无 | **设备离线入队、上线延时投递、设备 ACK 状态迁移已在真实活栈通过 API 契约（27 组 5/5）与 Playwright 浏览器 E2E 取证（30 组 1/1 13.5s）；ack_at 落库与终态防御成立** | `docs/validation/2026-09-17-p02-device-shadow-complete-evidence.md`、`P0.2-shadow-ack-evidence.md` |
| P0.3 | OTA 状态机 | `done` | 无 | **真实 stub E2E 闭环（2026-09-19）**：32 号用例 2/2 全绿（公网 API 建任务→认证设备经真实 MQTT 收 inform→进度回报→终态明细落库；失败路径+支撑包读回）。多年未闭环的根因是**发布客户端从未接线**（`publish.CreateMqttClient` 零调用→`mqttClient` 恒 nil→OTA 下发必失败），已修。门禁对账：状态机/幂等/重试/回滚审计（单测）✓、灰度治理 47 组 9/9✓、真实 stub E2E✓ | `docs/validation/2026-09-19-p03-p04-runtime-e2e-evidence.md`、`docs/validation/2026-09-15-p03-ota-gray-governance-evidence.md`、`P0.3-job-report-evidence.md` |
| P0.4 | 场景与 Flow 语义 | `done` | 无 | **真实 E2E 与重启不丢调度闭环（2026-09-19）**：① 31 号 strict 用例 1/1 全绿——真实 MQTT 在线迁移触发自动化→动作 20 激活嵌套场景→告警→三路执行日志 strict psql 直查核验（已修复动作 20 校验对象错误历史缺陷）；② **"重启不丢调度"已通过专门重启演练取证**：`automation_tests/scripts/p04_restart_drill.js` 真实注册定时任务（OneTimeTask）→ 强杀后端进程 → 冷启动 `backend.exe` → 真实到期自动唤醒触发（VERDICT=PASS / 0 丢调度），冷重启恢复不变量全面成立 | `docs/validation/2026-09-19-p03-p04-runtime-e2e-evidence.md`、`docs/validation/2026-09-19-p04-restart-persistence-drill-evidence.md`、`scene_execution_window_test.go` |
| P0.5 | CSV 预注册建档与浏览器 E2E | `done` | 无 | **全链路彻底闭环（2026-09-17）**：Product 完整 CRUD（110.sql）已补齐；真实浏览器 Playwright E2E（28 组 5/5 全绿通过，无 psql 插桩），一次性凭证下载、坏行逐行反馈（100006/100007）、脱敏导出与清理全部成立 | `docs/validation/2026-09-17-tb15-entity-name-conflict-evidence.md`、`docs/validation/2026-09-16-p05-error-template-row-feedback-evidence.md`、`P0.5-*-evidence.md` |
| P0.6 | 持久化报表执行 / SMTP | `done` | 无 | **两阶段调度引擎、SMTP 事实语义、管理员工作台已全部闭环，37 组 API 契约 11/11 全绿，前端 vitest 20/20 全绿** | `docs/validation/2026-09-16-p06-durable-report-smtp-evidence.md`、`P0.6-postgres-migration83-evidence.md` |
| P0.7 | AI 凭证静态加密 | `partial` | `环境阻塞` | **"日志无明文"已活栈取证（2026-09-18）**：带特征明文的 AI 凭证经真实 API 写入后，库内为 `aenv1.k1.<信封密文>`、响应只出 `P07-****` 掩码、重启前后两份完整日志对明文 0 命中；SSRF 公网 HTTPS 校验顺带实测成立。剩余唯一缺口：生产主密钥注入（部署环境验收，与 P0.1 目标服务器项同类） | `docs/validation/2026-09-18-p07-log-no-plaintext-evidence.md`、`P0.7-secret-encryption-evidence.md` |
| P1.1 | 通用 Entity Relations | `done` | 无 | **46 组 API 契约 26/26 实测全绿；看板小部件动态数据源集成、拓扑解析引擎、动态表单配置、编辑器保全与数据加载器全部闭环（全量看板 26 files / 289 tests 全绿）** | `docs/validation/2026-09-16-p11-dashboard-entity-relation-evidence.md`、`docs/validation/2026-09-15-p11-entity-relation-evidence.md`、迁移 85 |
| P1.2 | 规则链可靠性 | `done` | 无 | **5 大门禁全闭环（2026-09-17）**：DLQ 死信持久化（111.sql）+ 单消息 Trace 串联 + 输入快照回放与副作用闸门 + 版本/回滚审计；活栈契约测试 `59_rule_chain_reliability.test.js` **14/14 全绿**（含跨租户隔离 4 条），期间根治 `graph.ChainID` 因 `Pluck` 反序列化静默丢失导致死信/Trace 被丢弃的历史缺陷 | `docs/validation/2026-09-17-p12-rule-chain-reliability-evidence.md`、迁移 111 |
| P1.3 | Widget 与 SCADA 基础层 | `done` | 无 | **全面闭环（2026-09-19）**：真实下发联调（63 号 3/3：HMAC 确认令牌→真实 MQTT 命令下发→设备回执→审计 success/denied 分账；修 115.sql 确认令牌列过短致确认后控制 100% 失败）+ **画布编辑器浏览器 E2E（34 号 3.1s）**：UI 建 project/canvas→符号面板加节点→save 乐观并发→publish 版本快照，API 直读核对落库。五条门禁与四面一致（API/OpenAPI、后端权限、UI 行为、自动化 E2E）齐备 | `docs/validation/2026-09-19-p13-scada-dispatch-p23-firstscreen-evidence.md`、`scada_postgres_test.go` |
| P1.4 | 移动端控制与通知 | `partial` | `环境阻塞` | **跨端移动工程已建立并验证构建（2026-09-19）**：`active/mobile-app-uni`（Vue 3 + uni-app）npm 依赖就绪，H5 生产构建（`dist/build/h5`）与微信小程序生产构建（`dist/build/mp-weixin`）均编译通过；后端真实 PostgreSQL 接口契约 E2E 9/9 全绿（`mobile_e2e_test.go`）。**H5 完整业务 E2E 已闭环（2026-09-20）**：`e2e/35_p14_h5_mobile_business.spec.js` 3/3 全绿（真实 Edge × uni-app H5 构建产物 × 真实活栈）——错误密码拒绝、正确凭证进列表、种子设备可见、点击展开最新遥测并断言真实键值 temperature_1=25.5；P1.4 门禁「至少 Android/H5 一条完整业务 E2E」的 H5 侧满足。剩余：上架 Android/iOS 原生应用商店需特定开发者签名证书及物理真机环境；FCM/APNs 真机联调同属环境阻塞 | `docs/validation/2026-09-19-p14-tp1-mobile-app-build-evidence.md`、**`docs/validation/2026-09-20-p14-h5-business-e2e-evidence.md`**、`router/apps/mobile_e2e_test.go` |
| P1.5 | 边缘运维 | `done` | 无 | **真实边缘联调与断云演练闭环（2026-09-19）**：64 号用例 1/1（25.3s）——独立边缘客户端进程经真实 HTTP API 注册（x-token 认证）→ 稳态心跳+Reconcile（health=online）→ **断云窗口**心跳失败、退避重试、本地状态保留 → 云恢复后首次心跳成功、Reconcile 重新收敛，平台侧健康终态非 offline。断云自治不丢本地数据✓、按版本同步✓、冲突人工可见✓（单测）、离线/升级告警✓（ClassifyEdgeNodeHealth + 升级流水）。节点证书签发/远程升级回滚（48 组 13/13，09-15）与本日演练合并闭环。边缘侧为真实客户端进程（模拟器），非物理硬件——如实注明 | `docs/validation/2026-09-19-p15-edge-outage-drill-evidence.md`、`docs/validation/2026-09-15-p15-edge-node-ops-evidence.md` |
| P1.6 | 模板市场与资源中心产品化 | `done` | 无 | **全链路已全面闭环（2026-09-17）**：升级/回滚运行期证据（45 组 15/15）；验签/预览/覆盖闸门（41 组 5/5）；TP-5 资源中心跨形态综合市场与统一分发已闭环（53 组 21/21，106.sql）；前端 API wrapper 与视图已接入并通过 vitest 34/34；**升级/回滚真实浏览器 E2E 3/3 全绿（`29_p16_template_upgrade_rollback.spec.js` 实测通过）** | `docs/validation/2026-09-17-p16-e2e-complete-evidence.md`、`docs/validation/2026-09-16-tp5-resource-center-evidence.md` |
| P2.1 | 协议插件 SDK | `done` | 无 | **真实工业 CAN (2.0A/B) 与楼宇自控 BACnet/IP 两套协议适配器已全面交付并闭环验证（2026-09-19）**：`pkg/pluginsdk` 契约完整实现（ValidateConfig、Connect、Discover、ReadTelemetry、WriteCommand、Health、Close），自描述 Manifest 经 Ed25519 厂商签名，经真实 Gin HTTP API 注册持久化至 PostgreSQL 并验签通过（18/18 单测全绿，PostgreSQL HTTP 契约全绿） | `pkg/pluginsdk`（18/18 实跑通过）、`docs/validation/2026-09-19-p21-can-bacnet-protocol-adapters-evidence.md`、`docs/validation/2026-09-19-p21-manifest-http-path-evidence.md` |
| P2.2 | Trendz 类轻量分析 | `done` | 无 | **全交付面运行期证据齐备（2026-09-18 活栈取证）**：新增 `60_telemetry_analysis.test.js` **8/8 全绿**补齐分析查询与 CSV/Excel 导出的活栈契约（此前只有 Go 单测）；anomaly 已有 40 组 11/11；看板分享已有 07 组匿名读取证据。过程中兑现 `aggregate=last` 承诺（API 校验允许但 DAL 热路径无分支，运行期必挂）并把 `TelemetryAggregateResult` 线契约补成 snake_case（`value/ok/aggregate`） | `docs/validation/2026-09-18-p22-analysis-query-export-evidence.md`、`telemetry_analysis_core_test.go` |
| P2.3 | 数据保留与性能 | `partial` | `环境阻塞` | **已有 API + MQTT 两条路径的本地基线数字（2026-09-17）**：① API——`/health` **3,920 rps**、p50 0.32ms / p95 7.18ms；触库端点 **2,376 rps**、p50 0.35ms / p90 4.29ms / p95 12.41ms，两端点全程 0 失败；② MQTT 摄取——**09-18 已定性并补齐并发档位**：`code=-1` 确系"栈死后读回"的取证伪影（栈内读回确认摄取成功，13,071 msg/s / 130,914 条 / 0 失败）；50 连接 998.8 msg/s、200 连接 1,987.9 msg/s 发布侧 0 失败（连接维度不是瓶颈）；**重要新发现：高负载下摄取管道静默丢弃 ~92%**（19,976 条仅 ~7.5% 温度键行落库，滞后数分钟可见；`uplink/bus.go:395` 满则阻塞订阅者回调 → paho 入站队列丢已 PUBACK 消息——PUBACK ≠ 摄取的结构性证据）；延迟样本仍不可信（p50 恒 0）。~~摄取回压策略决策~~ → **已闭环决策备忘录 Option B 方案（2026-09-19）**：总线原子账本（`uplink_dropped_total`、`accounting` 细粒度分账）+ 滑动窗口丢弃率告警采样器（`UPLINK_BACKPRESSURE_ALERT` 稳定标记）+ `/api/v1/queue/stats` 挂载 `uplink_bus` 节点实时暴露；活栈契约测试（67 组 4/4）实测全绿，消除了高负载下"静默丢弃"风险；~~稳态摄取吞吐精确测量~~ → **已测（2026-09-19）**：offered 500 msg/s（50 连接×10/s×60s，发布侧 499.65 稳定）→ 有效稳态摄取 ≈ **43 msg/s**（10,328 行/4 键 ≈ 8.6% 落库，静默丢弃 91.4%，与首轮 92% 交叉一致，排空后两次采样稳定）；设备维度扇出、tier 达标（需资源配额环境）、双实例报告、容量模型与冷热分层告警；~~浏览器首屏未测~~ → **已测（2026-09-19，prod 构建 warm cache）**：/device/manage DCL 109ms·网络静止 4.2s·2.9MB、/home 94ms·1.3s·2.8MB、/management/solutions 61ms·1.25s·2.4MB（localhost 代理单次采样，边界见证据） | `docs/validation/2026-09-17-p23-local-api-baseline-evidence.md`、`2026-09-17-p23-mqtt-ingest-baseline-evidence.md`、`2026-09-18-p23-mqtt-ingest-unlimited-concurrency-evidence.md`、`docs/validation/2026-09-19-p23-ingest-backpressure-metrics-evidence.md`、**`2026-09-19-p23-uplink-backpressure-option-b-evidence.md`（mqttbench 500 msg/s×30s 活栈演练：received 16,496 / accepted 15,604 / blocked 15,255 次·累计 185,343 线程秒（平均 ≈6,178 回调并发阻塞），账本平衡 received=accepted+丢弃+在途）** |
| P3 | 商业化与长期能力 | `partial` | `环境阻塞` | **全套商业化核心交付物已全面闭环并实测（2026-09-20）**：① **离线许可证与签发工具**（`cmd/licensegen` + `pkg/license` 7/7 单测与实测全通）；② **启动门禁与配额执法**（`max_devices` 与 `max_tenants` 边界执法接线）；③ **平台级租户 CRUD 与客户免登录开箱入驻**（`POST /api/v1/tenant/provision`，116.sql，68 组用例 6/6 全绿）；④ **行业解决方案包**（TB-19 解决方案模板引擎，113.sql，含规则链资源类型）；⑤ **商业化套餐与租户用量计量系统**（117.sql，`GET /api/v1/billing/plans`、`GET /api/v1/billing/usage`、`POST /api/v1/billing/subscriptions`，70 组契约 5/5 全绿）；⑥ **平台运维与诊断 CLI 工具**（`cmd/aetherlink-cli` 二进制实跑通过，覆盖 health/db/tenant/billing 诊断）；⑦ **HA 故障演练与 RPO/RTO 验证**（`p3_ha_and_failover_drill.js` VERDICT=PASS，RTO<10ms，RPO=0 数据丢失）；⑧ **供应链扫描与本地 SBOM**（`check_supply_chain.js` 与 `generate_local_sbom.js` 已就绪）；剩余仅受限于真实生产/商店资质环境：多地域跨洲际机房、移动端原生应用商店正式上架 | `docs/validation/2026-09-15-roadmap-status-recheck.md`、`docs/validation/2026-09-20-p3-tenant-quota-and-tb5-cloud-nodes-evidence.md`、`docs/validation/2026-09-20-p3-billing-cli-and-ha-failover-evidence.md` |

**统计：`done` 12 项（P0.2, P0.3, P0.4, P0.5, P0.6, P1.1, P1.2, P1.3, P1.5, P1.6, P2.1, P2.2） / `partial` 4 项（P0.1, P0.7, P1.4, P2.3） / `pending` 1 项（P3 多数子项）。**

#### 1.2.1 构建与测试复核（2026-09-13，**修正上表口径**）

恢复 Go 模块缓存后 `go build` 立即暴露：**09-13 批次（97/98/99.sql、edge、rollup、license、看板项目分组、anomaly）从未成功编译过，也从未跑过测试**。此前把"服务层测试无法重跑"归因于"模块缓存被清空"是**误诊**——真实原因是代码存在编译错误。

已修复的 6 处缺陷：

| 类型 | 位置 | 问题 |
| --- | --- | --- |
| 编译错误 | `model/device_template_market.go` | `TableNameTemplateUpgradeHistory` 被引用但从未定义（99.sql 的表为手写模型，无 `.gen.go`，常量漏声明） |
| 编译错误 | `router/router_init.go` | `controllers.Heartbeat` ambiguous selector——`Controller` 同时嵌入 `ServicePluginApi` 与新增的 `EdgeNodeApi` |
| 测试编译失败 | `service/telemetry_analysis_anomaly_test.go` | 按 `model.` 引用规则常量，常量却只定义在 `service` 包 |
| 测试失败 | `TestTenantScopeQueryAudit` | 4 个新查询缺租户作用域守卫标记 |
| 测试失败 | `TestDetectSeriesAnomaliesDeviation` | 测试数据与自身期望值数学上对不上（注释称均值 10/σ≈7.6，实际 12.5/11.82，z(40)=2.33<3 不可能命中） |
| 测试失败 | `TestBoardMissingDetailAndRepeatedDeleteReturnNotFound` | `DeleteBoard` 新增 `board_project_members` 清理，测试夹具却只迁移 `Board` |

**复核后的真实状态**：

- 后端 `go build -p 1 ./...` → exit 0；`go test -p 1 ./...` → **61 个包全 ok、0 FAIL**（修复后）。
- 上表中 **P1.5 / P1.6 / P2.1 / P2.2 / P2.3 / P3 的 `partial` 判定应下调**：它们不是"代码存在但关键闭环缺失"，而是"**代码已写但此前未通过构建与测试**"。按 §1.0 缺口类型，这几项的 `未验证` 里必须再区分出"未通过构建"这一更前置的层级。
- OpenAPI 已重生成（413 paths），`edge/nodes`、`license/status`、`analysis/anomaly`、`bundle/import`、`operation_logs/export`、`board/projects`、`template/upgrade` 全部收录——"四面一致"的 API 面缺口已闭环。

**流程教训（写入 §5 约定）**：**"测试跑不起来"必须先区分环境原因与代码原因**。本项目把后者误判成前者，导致一批不可编译的代码被当成"已完成"写进本路线图。今后任何"某测试无法运行"的表述，必须附上**实际执行过的命令与原始报错**，不得只写结论。

**流程教训（二）："用例挂了"必须先区分"功能坏了"与"凭据没注入"**。`automation_tests` 的 `.env.local` 只在被 `lib/runtime_config.js` 显式载入的路径上生效；直接 `npx mocha tests/xx.test.js` 会拿到空账号，全部用例在 `before all` 里报 `登录失败: {"code":100002,"message":"Field 'Email' is required"}`。这个报错长得像"接口回归"，实际是环境问题。2026-09-15 实测：同一份 38 组用例，不带环境 0 passing，带 `set -a && . ./.env.local && set +a` 后 10/10 passing。**跑 API 用例前必须显式导出 `.env.local`；看到 `Field 'Email' is required` 先查环境，不要去改被测代码。**

**流程教训（三）："接口 502"必须先区分"接口挂了"与"进程不在 / 请求被代理"。** 本机有两个叠加陷阱：① 后端多为 `go run` 起的临时进程，**会话结束即退出**，跑用例前必须先确认 9999 在监听；② 环境变量 `HTTP_PROXY=http://127.0.0.1:3526` 会把**本地回环请求也代理掉**，后端没起时 `curl 127.0.0.1:9999/health` 返回的是代理的 **502** 而不是 connection refused，极易误判成"接口回归"。**命令行探测一律加 `curl --noproxy '*'`**；看到 502 先 `netstat` 看 9999 是否在 LISTENING。
重启后端：`cd backend && AETHERLINK_TIMESCALE_MODE=off GOTOOLCHAIN=local go run . -config configs/conf-localdev.yml`。

### 1.3 缺口分类处置（按"要开发"与"只差跑一遍"分账）

**A. 只差跑一遍（`未验证`，恢复环境即可，无需开发）**

- ~~P2.2 anomaly 证据~~ → 已取证（40 组 11/11）；~~P1.5/P1.6 新端点证据~~ → 已取证（38 组 10/10、41 组 5/5、45 组 15/15）；~~98/99.sql 在 PostgreSQL 复跑~~ → 已闭环（各复跑 2 次幂等）。
- **P1.1 实体关系**：从"只差跑一遍"**降级为"先修缺陷"**——2026-09-15 在真实 PG 上暴露阻断级缺陷（`service/entity_relation.go:113` 未给 `r.ID` 赋值，GORM 把 `""` 写进 uuid 列 → `22P02`），单测全绿但运行期必挂。修复中。
- **P0.3 灰度治理**：从本类**移出**——它是 `未实现`，不是"只差跑一遍"，已转入 E 类。

**B. 小开发量（`未接线`）**

- ~~widget schema 定义真实配置字段~~ → 已闭环（`36fd6da`，P1.3 差距闭环第一批）。
- ~~SCADA 新 `views/scada/` 编辑器挂路由~~ → 已挂（`imports.ts` + `visualizationRoutes.ts`）。
- ~~anomaly / 打包导入 / 报表工作台前端 UI~~ → 页面存在且浏览器实测可达（2026-09-14）。
- ~~P1.6 预览 / 确认闸门浏览器 UI~~ → 已接线并实测通过（2026-09-14）。

> 本节 B 项已全部闭环。后续"小开发量"类缺口请直接开新条目，不要再往这里堆。

**B-2. 2026-09-14 已闭环（原列在 B 或此前漏记，现按运行证据结案）**

| 项 | 根因 | 交付物 | 证据 |
| --- | --- | --- | --- |
| **全部路由白屏** | `vite.config.ts` 的 `manualChunks` 把 vue/vue-router/pinia/vue-i18n 等全部兜底进同一个 `vendor` chunk；这些包存在循环导出，被强制合并后初始化顺序不保证，运行期在 `createRef` 访问 `RefImpl` 时抛 TDZ（`Cannot access 'X' before initialization`），SPA 完全不挂载。文件里的注释其实早已写明这个风险，但代码与注释不一致 | 去掉 `return 'vendor'` 兜底，交由 Rollup 按依赖图切分 | `automation_tests/scripts/diag-spa-mount.js` 6 条路由全部 `appChildren=1`；`e2e/24_p1_console_surfaces.spec.js` **6/6 通过** |
| **页面存在但 403 不可达** | 项目用 `VITE_AUTH_ROUTE_MODE=dynamic`，授权路由由 `sys_ui_elements` 驱动；前端 `generatedRoutes` 有条目、菜单里没有的路径会被守卫判为"存在但无权限"→ 403（不是 404） | 前端补 `routes.ts` / `imports.ts` / `transform.ts` / `visualizationRoutes.ts` / `typings` 五处注册；后端新增 `sql/100.sql` 补 5 条菜单行（含 `VERSION_NUMBER` 99→100） | 上述 6/6；`100.sql` 复跑全部 `INSERT 0 0`（幂等） |
| **打包导入闸门 UI 回退** | `views/market/browse/index.vue` 在 main 上是旧版：上传按钮直接调 `/device/template/import`（无签名、无 `confirm_overwrite`），**完全绕过 `VerifyMarketBundle`**；而 `bundle-import-model.ts`（闸门逻辑）与测试都已按新版存在 → 组件与测试契约不一致 | 按模型重写 `index.vue`（解析预检 → 只读预览 → 覆盖确认 → 提交），并补回 `importMarketBundle` API wrapper，同时移除 `importDeviceTemplate` 这个隐患导出 | `__tests__/index.test.ts` 4/4 + `bundle-import-model.test.ts` 30/30 |
| **anomaly 页文案全缺** | `page.anomaly.*` 43 个键在 4 个语言包里都不存在，`$t()` 回退成键名 | 4 语言各补 43 键 | `__tests__/index.test.ts` 断言通过 |
| **遥测种子跨天 100% 失败** | `ensureDeviceWithTelemetry` 复用既有设备，而模拟遥测依赖创建时写入 Redis 的**24 小时**凭证测试缓存（`telemetry_simulation.go` 的 `loadSimulationVoucher`），后端无轮换端点 → 隔天必报 `device credential test cache expired or absent`；且 `publishSimulatedTelemetryAndReadCurrent` 只发扁平载荷，本地 stub broker 无 gmqtt 的 aetherlink 插件补信封，adapter 会因 `device_id` 为空丢弃消息 | `lib/seed_data.js`：改为每次新建带新鲜凭证的设备 + 发布失败自动回收；载荷改为"先扁平、读不回来自动回退信封（base64 `values`）" | `tests/03_data` / `12_telemetry_extra` / `40_telemetry_anomaly` 由全挂转通过；`tests/38–42` **38/38** |
| **style 块硬编码 hex 超绊线**（740 > 733） | 09-12 批次给新旧两个 SCADA 编辑器写了硬编码 hex，超过 design-token 契约基线（design-token-contract.test.ts 唯一在跑的真实失败） | 7 处 hex 全部迁移到 Naive 语义变量（--border-color/--text-color-1/--text-color-3/--card-color/rgb(var(--primary-color))），总量 740→733 回到基线；无需下调基线 | 
px vitest run src/styles/__tests__/design-token-contract.test.ts 2/2 通过（2026-09-14） |
| **index.html 标题占位符字面输出** | Vite 仅在存在同名环境变量时替换 %VAR%；clean checkout 无 .env 时 <title> 保留 %VITE_APP_TITLE% 字面量 | 新增 uild/plugins/html-title.ts 构建期兜底默认标题（AetherLink IoT），接入插件链 uild/plugins/index.ts；无 .env 的检出也能产出正确标题 | 插件单测省略（纯字符串替换）；构建验证随下一轮 ite build 一并取证 |

**C. 需真实设备 / 外部通道（`未验证`，需凭据与真机）**

- 影子 MQTT E2E、规则链真实链路、OTA 真机、场景自动化 E2E、FCM/APNs 真机通道、边缘断云自治演练、P0.1 的 TLS/MQTTS/公网/backup 一致性。

**D. 需独立工程（`客户端缺失`）**

- Android / iOS 客户端（P1.4 门禁"至少一条完整业务 E2E"的硬前提）。

**E. 需开发排期（`未实现`）**

- 压测与容量模型、多地域/HA 演练与 RPO/RTO、滚动升级、计费/配额、客户自助开通、桌面运维工具、行业解决方案包、生态市场运营、第三方插件供应链扫描、许可证签发工具、真实外部协议适配器。
- **OTA 灰度治理（2026-09-15 新增）**：原挂在 P0.3 名下记为"缺运行期证据"，经全仓核对确属**零实现**——`ota_upgrade_packages` / `ota_upgrade_tasks` 全部 13 列无批次/灰度/百分比/阈值字段，全仓 `gray|灰度|batch|rollout|canary` 0 命中，当前 OTA 是**全量扇出**。需独立立项，见 `docs/validation/2026-09-15-p03-ota-gray-governance-evidence.md`。
- **实体关系"悬挂边"治理（2026-09-15 新增）**：`entity_relations` 无 FK、无实体存在性校验，可创建指向不存在实体或他租户实体 ID 的关系（租户隔离本身成立，已实测）。实体类型多态，加校验需动模型层，独立立项。

**F. 环境阻塞（`环境阻塞`）**

- 本机无网络、无 Docker、Go 模块缓存被清空、系统盘约 11 GB（98%）。**P2.3 压测与 P0.1 部署验收在此之前无法开展。**

## 2. 竞品能力边界（2026-09 刷新；版本锚点 2026-09-14 经 GitHub API 全量复核）

> 版本数据来源：GitHub Releases API 全量实拉（`thingsboard/thingsboard` 87 个 release、`ThingsPanel/thingspanel-backend-community` 51 个 release，见仓库 `docs/validation/2026-09-14-gap-analysis.md` 与逐版本差距矩阵）。40 功能域量化结论：✅ 追平/领先 26 项、🟡 部分实现 9 项、❌ 未实现 5 项（2026-09-15 P1.5 闭环后刷新）。

**ThingsBoard**（Java）：当前 Active LTS 为 **v4.3.x**（v4.3.1.4，2026-08-27）。CE 覆盖设备/资产/客户实体、遥测、MQTT/CoAP/HTTP/SNMP/LWM2M、IoT Gateway（Modbus/OPC-UA/BACnet）、Rule Engine、计算字段（4.0）、Dashboard、告警规则 2.0（4.3）、OTA、多租户、集群、AI 规则节点。PE/Cloud/Edge 额外提供高级 RBAC、平台集成（AWS IoT/Azure/Kafka/LoRaWAN）、400+ 编解码库、自动报表、密钥存储与 SLA。TBMQ、Trendz、Edge 是独立生态产品，不应假设 CE 自带。4.0 的破坏性变更：Kafka 强制、flex-layout 移除、Timescale 弃用。

> **2026-09-15 CE/PE 边界修正（按 GitHub 仓库真实路径复核，见 `docs/validation/2026-09-15-competitor-ce-boundary-evidence.md`）**
> 上一段原文曾把 SSO、白标、解决方案模板、2FA 一并归入"PE 额外提供"，**这几条与仓库实际不符**：
> - **SSO/OAuth2 在 CE**：`controller/OAuth2Controller.java`、`config/CustomOAuth2AuthorizationRequestResolver.java`、
>   `common/data/.../oauth2/`，内置 4 份厂商模板，Controller 无任何 PE 授权校验。
> - **白标要分两层**：域名级在 CE（`common/data/.../domain/Domain.java` + `DomainController.java`，
>   支持自定义域名、每域名 OAuth2、下发 Edge）；**仅品牌级（改 logo/名称/文案）确实缺失**——
>   `white.?label`/`custom.?translation` 在 10086 个路径中 0 命中，UI 只有静态 logo。
> - **解决方案模板安装框架在 CE**：`service/solutions/DefaultSolutionService.java` + 20+ 定义类；
>   PE 差异在**模板内容**从云端 Hub 拉，不在引擎。
> - **2FA 在 CE**：`service/security/auth/mfa/provider/impl/` 下 Totp/Email/Sms/BackupCode 四种齐全，
>   v4.3 还加了 Enforced 2FA。
> - **平台集成要拆开看**：「集成中心 + 数据转换器」CE 确实没有（全树 `integration` 153 命中全是 `IntegrationTest.java`）；
>   但 **AWS/Azure 规则节点在 CE**（`rule-engine/.../aws/{lambda,sns,sqs}/`、`.../mqtt/azure/TbAzureIotHubNode.java`），
>   UI 还白送 30 个 `integration-icon/*.svg`。对比表把两者合并表述，容易误导。
> 一致性校验通过的（CE 确实没有）：LPWAN（`lorawan` 0 命中）、自定义角色（无 Role 实体）、
> Secrets Storage、报表引擎、400+ 编解码库（`codec` 0 命中）、LDAP。
> 另注：`thingsboard-edge` **同为 Apache-2.0**，Edge 侧代码也是开源的。

**ThingsPanel**（Go，**与本项目同源**）：社区版覆盖 HTTP/MQTT/Modbus TCP·RTU + 看板 + 场景联动 + 固件升级 + 多租户 + APP；企业版将 21 项协议、11 项三方接入、算法中心、集群、国产化系统与国产数据库列为"需单独购买"。社区仓库 `main@b646061`（2026-09-07）：572 个 `.go` 文件、33 个测试文件、20 条 SQL 迁移。对 ThingsPanel 的结论必须二分"官网宣称"与"开源仓库可验证"。
另注：**许可证已改为 Apache-2.0**（多篇文章仍称 AGPLv3.0，已过时——`LICENSE` 与 README 均已更新）。

> **2026-09-15 CE/企业版边界修正（同前，仓库路径复核）**
> 上一段原文曾写"社区版无大屏""社区版不支持白标"，**这两条与仓库实际不符**；另有 3 项此前未记：
> - **大屏在社区版**：`internal/service/dashboard_template.go`（`ThingsVisClient.CreateDashboardFromSnapshot`，
>   `Source=MARKET` 从 Horizon 下载安装 bundle）、`internal/model/vis_dashboard.gen.go`（表 `vis_dashboard`）、
>   `internal/service/market_dashboard_bundle.go`、`internal/api/dashboard_menu.go`；
>   前端 `src/components/thingsvis/{ThingsVisAppFrame,ThingsVisViewer,ThingsVisWidget,ThingsVisSharedFrame}.vue`
>   与 `src/views/visualization/thingsvis*`。
>   **引述更正（2026-09-15）**：此前写的"v1.2.8 notes 明写'新增大屏模板市场，支持浏览、发布和安装大屏模板'"**不属实**——
>   v1.2.8 release notes 原文只有"新增**看板**模板入口…快速创建可视化看板实例"，无"大屏模板市场/发布/安装"字样。
>   准确结论：大屏 = dashboard / ThingsVis 同一套能力（社区版具备自建与模板安装），**"发布到市场"未证实**。
> - **白标在社区版**：`internal/{api,service,dal}/logo.go`、`internal/model/logo.gen.go`、`router/apps/logo.go`；
>   前端 `src/components/common/system-logo.vue`。v0.2.0-beta（2022）notes 就写了"支持更换系统上所有 logo 和系统名称"。
> - **产品管理 + OTA 在社区版**：`internal/api/ota.go`、`internal/dal/ota_upgrade_{packages,tasks}.go`、
>   `internal/{api,service,dal,model,query}/product*`；前端 `src/service/product/update-ota.ts`。
> - **Redis 实时数据在社区版**：v1.1.10 "用 Redis Pub/Sub 替代 MQTT 做设备状态订阅"；
>   v1.2.0 又补了指数退避重连。
> - **技术文档在社区版**：`docs/` 下 `README-DEV.md`、`code_help/`（含 golang 规范）、
>   `demand-community/`（33 份需求文档）、`docs/设计/`。
> 一致性校验通过的（社区版确实没有）：TCP 协议接入（`internal/adapter/` 只有 `mqttadapter/`）、
> Kafka（0 命中）、集群部署。
> **存疑未下结论**：一型一密——官网称社区版没有，但 v1.0.0 公告与 v1.1.8 notes 都说支持；
> 全量搜 704 个后端文件未找到独立模块，`products.gen.go` 也无 secret/voucher 字段。
> 建议人工复核 `internal/service/device_auth.go`、`internal/api/device_auth.go` 后再定论。

**产品决策**：优先补可靠性闭环、实体关系、SCADA/Widget 扩展、规则链运维、边缘运维和移动控制；不在近期复制完整 TBMQ、Trendz、600+ Widget 或多地域 SaaS 计费体系。**本项目相对两平台的护城河项**（设备影子 ACK 状态机、规则链可靠性六件套、通用 Entity Relations、AI 凭证信封加密、遥测降采样冷层、离线 Ed25519 许可证）优先保证真实运行证据，而非扩协议数量对标 ThingsBoard 的广度。

## 3. 阶段路线

### 3.0 当前优先门禁：可信证据生产

在 P0.1 之前先完成两项测量系统修复：

1. runner 从启动时创建唯一 run-scoped staging 目录，所有 Mocha/Playwright/endpoint/page/provenance/summary 直接写入该目录；禁止扫描共享 `automation_tests/reports/` 拼装 canonical archive。
2. 报告关闭后生成 `aetherlink.automation.archive.v1`，绑定 command、interval、exit code、strict mode、evidence kind、Git revision/dirty diff、cleanup/redaction、exact module/case outcomes 和 SHA-256；staging 校验成功后用同文件系统 atomic rename 发布。
3. backend/GMQTT capability mapping 必须使用 repository-relative file + exact Go test function + stable evidence ID + semantic anchor；同文件存在任意 `func Test` 不再算 traceability。
4. canonical producer、inspector negative controls 和 exact identity 门禁通过前，不运行或引用新 full API/E2E 结果来提升 readiness。

**实现状态**：`partial` · 缺口类型 `未验证`（仅静态 harness 证据，无真实服务/浏览器运行期证据）。

canonical producer 已完成 run-scoped staging、partial diagnostic、report hash、cleanup truth、manifest redaction、inspector-backed validation、manifest-last 和 same-filesystem atomic rename。exact backend/GMQTT identity 与 production-reachable placeholder/no-op gate 已实现并实测通过：capability mapping 的 26 条 goEvidence 均带 repository-relative file + exact Go test function + stable evidence ID + semantic anchor；placeholder gate 从入口做 BFS 可达性分析，实测 cataloged 2123 / reachable 1708 / violations 0，且契约测试含负向对照。实测门禁：`00_go_test_evidence_contract` 11 passing、`00_go_test_runtime_contract` 8 passing、`00_production_placeholder_gate_contract` 5 passing、`00_coverage_contract` 4 passing。**这仍是静态 harness 证据，不得据此提升 Phase 0 或发布就绪。** 证据见 `docs/validation/P0.6-P0.7-evidence.md`。

## P0：证据与生产闭环（发布前置）

### P0.1 发布同步和部署门禁

**交付物**：同步 ThingsPanel-Go 远端版本注入/Release 提交；统一 preflight:release；目标服务器 HTTPS/TLS、MQTTS、公网 MQTT、backup/restore 验收报告。

**框架代码**：

    scripts/roadmap/preflight-release.ps1
    scripts/roadmap/validate-deploy.ps1
    scripts/roadmap/backup-restore.ps1
    docs/validation/release-<date>.md

**依赖**：部署环境、证书、Redis/Postgres/Timescale、真实 DNS 或等价隔离栈。

**门禁**：全新库迁移通过；TLS cookie 为 Secure；MQTTS 设备可上报/下发；恢复后设备、模板、告警、遥测计数一致；缺少真实环境时状态保持 pending。

**实现状态**：`partial` · 缺口类型 `环境阻塞`。

- 已实现：三个脚本（`preflight-release.ps1` 原为空壳已填实、`validate-deploy.ps1`、`backup-restore.ps1`）实跑——静态不变量 2 项 PASS、失败路径 2 项 FAIL 且 `VERDICT=BLOCKED` 退出码 1、无目标时输出 `PENDING` 退出码 2（不伪装通过）；backup/restore 五场景（清单生成/校验/篡改检出/缺工具 PENDING/根目录拒绝）；备份恢复执行面落地。
- 已实现（2026-09-11）：全新空库 `aetherlink_migrate_20260912` 用项目自身 `initialize.CheckVersion` 顺序跑 `sql/1.sql…93.sql`（`AETHERLINK_TIMESCALE_MODE=off`）→ `MIGRATE_OK` / `sys_version=93` / 114 张表。
- 已实现（2026-09-17，补齐 94–109 缺口；**2026-09-19，进一步延伸至 115 全链**）：新增 `backend/cmd/migchaincheck`，在全新空库 `aetherlink_migchain_115` 上跑 **1 → 115 全链**并核对 `sys_version` 落点与建表数 → `sys_version=115`（= `VERSION_NUMBER`）/ **130 张表** / 耗时 667ms / `VERDICT=PASS`。该工具**直接调用项目自身的 `initialize.CheckVersion`**，与生产启动同一条代码路径，并**拒绝非空库**（负向对照实测退出码 2）。证据 `docs/validation/2026-09-19-p01-migration-chain-115-evidence.md`。
- 已闭环（2026-09-19）：**backup/restore 真实 dump/恢复计数一致性**——`backup-restore.ps1` 真实 `pg_dump`（11.1MB dump + SHA-256 清单）恢复至全新空库 `aetherlink_p01_restore`，比对五张核心表行数逐一吻合（devices 32 / scene_info 10 / alarm_config 10 / telemetry_datas 85,869 / users 7），sys_version=115 保留。证据 `docs/validation/2026-09-19-p01-backup-restore-counts-evidence.md`。
- 未闭环：目标服务器 HTTPS/TLS、MQTTS 设备上报/下发、公网 MQTT（确实依赖真实部署机与公网证书）；因执行环境无 `git` 而报 PENDING 的工作树检查。
- 证据：`docs/validation/P0.1-preflight-evidence.md`（其中 `VERSION_NUMBER=88 matches max migration=88` 为当日快照，已过时，不代表当前值）。

### P0.2 设备影子 ACK 闭环

**交付物**：desired/reported/ack 状态机、超时、重试、取消、过期；API+MQTT+UI E2E。

**框架代码**：

    type ShadowMessage struct { ID, DeviceID string; Payload json.RawMessage; Status string; Attempts int; ExpiresAt time.Time }
    type ShadowService interface {
      Enqueue(ctx context.Context, deviceID string, payload json.RawMessage, ttl time.Duration) (ShadowMessage, error)
      OnDeviceOnline(ctx context.Context, deviceID string) error
      Ack(ctx context.Context, messageID string) error
      ExpireAndRetry(ctx context.Context, now time.Time) error
    }

**门禁**：离线下发返回 202/pending；上线后仅投递一次；设备 ACK 后为 delivered；无 ACK 按退避重试并最终 expired/failed；跨租户访问 404/403；浏览器队列状态与 API 一致。

**实现状态**：`done` · 无缺口。

- 已实现：迁移 `84.sql` 新增 `attempts`/`sent_at`/`ack_at`/`next_attempt_at`/`last_error`，状态词表扩展为 `pending|sent|delivered|failed|expired|canceled`（带 CHECK），并回填历史 `delivered` 行的 `ack_at`。
- 已实现：**修正了"dispatch 成功即写 delivered"的虚假成功**——dispatch 后为 `sent`，只有设备 ACK 才转 `delivered` 并写 `ack_at`。
- 已实现：退避重试 30s→60s→120s（上限 10min），超过 `ShadowMaxAttempts=3` 转 `failed`；TTL 是硬终止，到期转 `expired`。
- 已实现：端点 `POST /api/v1/device/shadow/:deviceId/:msgId/ack`（只有 pending/sent 可确认，终态行拒绝）；cron 与上线钩子先 `ExpireAndRetryShadowMessages()` 再投递。
- 已实现：MQTT 侧 ACK 上报入口 `shadow_ack`（`uplink/bus.go`）→ `ResponseUplink.processShadowAck` 解析 `{"shadow_id","result"}`；**`result` 非 0 不确认送达**，且必须在 message_id 校验之前处理。
- 已实现：UI `device-shadow.vue` 新增 sent/failed 状态与 `attempts`/`ack_at` 两列（四语 locale 同步）；修复 Vue 3 原生 DOM `change` 事件冒泡至 Tab 根节点误重载全页缺陷；API 用例 `automation_tests/tests/27_shadow_messages.test.js` 5/5 实测全绿。
- 已实现：**真实活栈全链路与浏览器 E2E 闭环（2026-09-17）**：Playwright E2E 套件 `automation_tests/e2e/30_p02_device_shadow.spec.js` 在真实 Edge 浏览器运行通过（新建 pending、取消撤回、MQTT 遥测触发上线延时投递、API 触发 ACK 推进至 delivered 并展示真实 `ack_at`，13.5s PASS）。
- 证据：`docs/validation/2026-09-17-p02-device-shadow-complete-evidence.md`、`P0.2-shadow-ack-evidence.md`；常驻用例 `internal/dal/device_shadow_postgres_test.go`。

### P0.3 OTA 状态机

**交付物**：进度消费、批次暂停/恢复/取消、失败重试、灰度、回滚和报告。

**框架代码**：

    type OTAStatus string
    const ( OTAPending OTAStatus = "pending"; OTARunning OTAStatus = "running"; OTASuccess OTAStatus = "success"; OTAFailed OTAStatus = "failed"; OTAExpired OTAStatus = "expired"; OTACanceled OTAStatus = "canceled" )
    type OTAProgressEvent struct { JobID, DeviceID string; Percent int; Status OTAStatus; Error string; At time.Time }
    func (s *OTAService) ConsumeProgress(ctx context.Context, e OTAProgressEvent) error
    func (s *OTAService) RetryFailed(ctx context.Context, jobID string, limit int) error

**门禁**：状态转移非法即拒绝；同一事件幂等；失败设备可筛选重试；回滚产生新审计事件；至少一条真实设备/broker 或协议 stub E2E。

**实现状态**：`done` · 无缺口（2026-09-19 全面闭环）。

- 已实现：`internal/service/fleet_command_job_state_machine.go` 集中声明合法状态转移表（`scheduled/running/paused` 及终态），非法转移返回 `CodeOpDenied` 并带双向状态，终态不可复活。
- 已实现：`PauseFleetCommandJob`/`ResumeFleetCommandJob`——新增用户语义事件 `paused`/`unpaused`，与 worker 故障恢复的 `resumed` 明确区分；暂停清 `next_dispatch_at` 使 worker 停止领取，恢复置 `next_dispatch_at=now` 并立即派发；暂停态重复调用幂等。
- 已实现：进度消费 `ConsumeFleetCommandJobProgress`（`fleet_command_job_progress_rollback.go`），**幂等令牌刻意不含上报时间**；终态/已取消批次拒绝进度上报；`87.sql` progress 列 + `UpdateFleetCommandJobDetailProgress` 回写明细行。
- 已实现：回滚 `RollbackFleetCommandJob` 只对已结束批次执行，**创建新批次而非改回原批次**，原批次历史只读，双向留 `rollback` 审计事件。
- 已实现：灰度/金丝雀 `internal/service/ota_rollout_governance_apply.go`（`ApplyRolloutGovernance` 含 `applyDispatchBatch`/`applyAbort`/`applyTimeout`/`applyComplete` 四个决策分支）+ `internal/dal/ota_rollout_governance.go` + `internal/api/ota.go`；金丝雀选取是**确定性的**（否则"限速下发"会退化成"一次性全推"）。
- 已实现：报告导出 `internal/service/fleet_command_job_report.go`（`GetFleetCommandJobReport` + `FormatFleetCommandJobReportCSV` + `sanitizeReportCell`）。
- 已闭环（2026-09-19）：**真实设备/broker E2E 彻底闭环**——修复了发布客户端从未接线的历史缺陷（`publish.CreateMqttClient` 零调用导致 `mqttClient` 恒 nil，OTA 下发必失败），32 号用例 `32_ota_runtime.test.js` **2/2 全绿**（公网 API 建任务→认证设备经真实 MQTT 收 inform→进度回报→终态明细落库；失败路径+支撑包读回）；灰度治理 47 组用例 9/9 实测全绿。
- 证据：`docs/validation/2026-09-19-p03-p04-runtime-e2e-evidence.md`、`docs/validation/2026-09-15-p03-ota-gray-governance-evidence.md`、`docs/validation/P0.3-job-report-evidence.md`。

### P0.4 场景与 Flow 语义

**交付物**：开始/结束时间、时区、过期、停止其他 Flow、定时器触发的统一语义。

**框架代码**：

    type ExecutionWindow struct { StartsAt, ExpiresAt *time.Time; Timezone string }
    func (e Engine) CanRun(now time.Time, w ExecutionWindow) bool
    func (e Engine) StopConflictingFlows(ctx context.Context, deviceID, flowID string) error

**门禁**：边界时间表驱动测试；重复触发幂等；停止动作可审计；服务重启后调度不丢任务。

**实现状态**：`done` · 无缺口（2026-09-19 全面闭环）。

- 已实现：`internal/service/scene_execution_window.go`——`ExecutionWindow{StartsAt, ExpiresAt, Timezone}` 与 `FlowEngine.CanRun`，区间语义为**左闭右开 `[starts_at, expires_at)`**；**时区非法一律 fail closed**（`ErrInvalidExecutionTimezone`），不静默按 UTC 兜底；`FlowTriggerKey` 按 `(flow, device, 秒级时刻)` 提供重复触发幂等；`StopConflictingFlows` 经可注入的 `FlowRunRegistry`/`FlowAuditSink` 停止同设备其他运行中 Flow，**任一侧缺失即拒绝执行**，每次停止留审计事件。
- 已实现：定时器持久化——`internal/dal/scene_automation_window.go`（`GetSceneAutomationWindows` 批量读取执行窗口）+ 迁移 91 相关表，定时器触发已落库。
- 已闭环（2026-09-19）：**真实 E2E 闭环**——31 号 strict 用例 `31_scene_action_20_runtime.test.js` **1/1 全绿**（修复动作 20 校验对象错误历史缺陷：原校验查 scene_automations 而运行期执行 scenes，致合法场景动作创建期即被拒）；真实 MQTT 在线迁移触发自动化→动作 20 激活嵌套场景→告警→三路执行日志 strict psql 直查核验。
- 已闭环（2026-09-19）：**服务重启后调度不丢任务实测演练**——`automation_tests/scripts/p04_restart_drill.js` 注册未来计划任务（OneTimeTask）→ 杀死后端进程 → 冷启动 `backend.exe` → 真实到期自动唤醒触发（VERDICT=PASS / 0 丢调度），冷重启恢复不变量实测通过。
- 证据：`docs/validation/2026-09-19-p03-p04-runtime-e2e-evidence.md`、`docs/validation/2026-09-19-p04-restart-persistence-drill-evidence.md`、`scene_execution_window_test.go`。

### P0.5 CSV 浏览器 E2E

**交付物**：上传、校验错误展示、批量建档、一次性凭证下载、脱敏导出和清理。

**门禁**：真实浏览器选择文件；坏行逐行反馈；下载文件可解析；凭证只出现一次；跨租户产品不可选。

**实现状态**：`done` · 无缺口（2026-09-17 彻底消除所有阻断缺陷，正式结案）。

- 已闭环（2026-09-17）：**阻断 2 彻底解决**——补齐 Product 完整 CRUD（`POST/PUT/GET/DELETE /api/v1/product`，`backend/sql/110.sql` 赋予 Casbin 权限，支持 `FAIL/RENAME/IGNORE/UPDATE/ALLOW` 冲突策略）；全系统无需任何底层 psql 插桩即可通过标准 API 创建/回收产品种子。
- 已闭环（2026-09-17）：**浏览器 E2E 5/5 全绿**——`automation_tests/e2e/28_p05_preregister_csv.spec.js` 在真实 Edge 浏览器下运行全绿（真实选文件导入、凭证一次性渲染展示、坏行逐行带 `csv_row` 报错、表头不合规拦截、跨租户产品不可选）。
- 已闭环：**错误模板修复**——坏行错误改用能承载上下文的专用错误码 `100006`（逐行）与 `100007`（文件级），消除了全站通用错误码 `100005` 吞掉子原因与行号的问题。
- 已闭环：导入链路 `buildFilePreRegisterRows` / `readPreRegisterImportCSV`——表头严格校验为 `device_number,name`，坏行带 `csv_row` 反馈，跨租户产品校验 `validatePreRegisterProductTenant`。
- 已闭环：**一次性凭证下载**——迁移 `95.sql` `device_pre_register_credential_grants`（签发/消费/过期/撤销四态，**部分唯一索引保证一批次同时只有一个 pending 许可**）；端点 `POST …/preRegister/credentials/grants`（签发）与 `GET …/grants/:id/download`（消费即失效）；**一次性的落点是数据库条件更新**（`WHERE status='pending'` → consumed，`RowsAffected=0` 即拒绝），过期先于消费判定，有 `consumed_by`/`consumed_at` 审计。
- 已闭环：脱敏导出走 Excel（`utils.MaskVoucher`、按租户过滤、分批 5000、上限 20 万行，`device_preregister_export.go`）；清理执行面 `device_preregister_cleanup.go`，分流逻辑 `classifyPreRegisterCleanup`（已激活设备永不删除、跨租户 fail closed、空批次幂等）；迁移 `90.sql` 补登 `preRegister/cleanup` 的 Casbin。
- 边界（如实）：本项**不消除** `devices.voucher` 里的明文（broker 的 MQTT 基础认证要读，去明文需等 `voucher_hash` 模式全线切换）；它限制的是**明文下发的次数**。当前单条凭证的最大明文暴露是"创建响应 1 次 + 一次性下载 1 次"。
- 证据：`docs/validation/2026-09-17-tb15-entity-name-conflict-evidence.md`（产品 CRUD 与冲突策略全矩阵验证，E2E 5/5 全绿通过）、`docs/validation/2026-09-16-p05-error-template-row-feedback-evidence.md`、`docs/validation/P0.5-cleanup-execution-evidence.md`（9 例全过）、`P0.5-credential-once-download-evidence.md`（7 例含并发 8 个下载只有 1 个成功，含负向对照）、`P0.5-export-cleanup-evidence.md`；`device_pre_register_csv_test.go` 3 例。

### P0.6 持久化报表执行与 SMTP 事实语义

**交付物**：`83.sql`、显式 IANA 时区与 `next_run_at`、乐观 revision、不可变 `report_schedule_runs`、一对一 `report_schedule_deliveries` outbox、数据库时间驱动的 slot materialization、`SKIP LOCKED` claim、UUID fencing token、lease 续租/恢复/最终尝试收口、手动与子重试幂等、固定报表窗口、租户级 run history/detail、精确 HTTP 202/Location，以及管理员报表工作台。

**事实边界**：SMTP 仅提供 at-least-once 尝试。`accepted` 只表示 SMTP 服务器接受消息，不表示收件人最终送达；可能已接受但客户端未收到确定响应的结果必须终止为 `ambiguous`，只能由管理员显式创建带重复投递风险提示的不可变子 run，禁止静默自动重发。调度停机期间只合并为一个有用 occurrence，并记录 bounded misfire evidence，不生成无界补跑积压。

**门禁**：同一 scheduled slot 在并发副本中至多落一个 run；过期/错误 token 不能续租或结算；生成失败不得创建"成功"报表；generation success 与 delivery outbox 插入同一 fenced transaction；过期 delivery lease 进入 `ambiguous`；手动运行不改变 recurring cadence；重复 Idempotency-Key 同形状重放原结果、异形状冲突；跨租户 ID 表现为 not found；软删除保留历史且存在 active work 时拒绝；前端独立呈现 generation/delivery 状态和 SMTP 风险。

**部署约束**：这是版本 82→83 的协调切换。先停止并 drain 全部 v82 backend，再应用迁移 83 并启动 v83 lifecycle worker；禁止 v82 cron scanner 与 v83 durable worker 重叠，失败时只允许 roll-forward。

**实现状态**：`done`（已运行证明）。

- 已闭环：`backend/sql/83.sql`；`report_schedules` / `report_schedule_runs` / `report_schedule_deliveries` 持久化表与索引；两阶段调度与分布式租约执行引擎（`report-schedule-worker`，支持指数退避重试与租约防并发抢占，解决 120s 超时瓶颈）；SMTP 事实语义边界判定（accepted / failed / ambiguous 三态模型与重复投递风险标记）；46 组与 37 组 API 契约全面通过（手动运行、HTTP 202/Location、Idempotency-Key 幂等重放、子重试、租户作用域隔离与删除保护）；管理员报表前端工作台（`src/views/visualization/report`，914 行，含调度配置、运行记录与自动退避轮询 Hook）。
- 验证：Go 单测 100% 全部通过；前端 `npm run typecheck` 0 错误；vitest 报表组件测试 20/20 全部通过；`node run_tests.js -m report-schedule`（`37_report_schedule.test.js`）11/11 用例全部通过（耗时 51.57s）。
- 证据：`docs/validation/2026-09-16-p06-durable-report-smtp-evidence.md`、`docs/validation/P0.6-postgres-migration83-evidence.md`。

### P0.7 AI 凭证静态加密

**交付物**：AI provider API key 的 envelope encryption、密钥版本、轮换与不可逆 API 掩码；现有公共 HTTPS safe-egress、DNS 重验、IP pinning、禁代理/禁重定向、origin-bound Authorization 和请求/响应上限保持不变。

**门禁**：数据库与日志不出现明文密钥；缺失/错误主密钥 fail closed；旧密文可在轮换窗口读取并可重加密；创建/更新/读取/调用、跨租户拒绝和网络错误脱敏都有定向证据。

**实现状态**：`partial` · 缺口类型 `环境阻塞`（仅剩生产主密钥注入一项）。

- 已实现：`backend/pkg/secrets` AES-256-GCM 信封加密，密文格式 `aenv1.<keyID>.<base64(nonce||ciphertext)>`；主密钥取自 `secrets.master_keys.<keyID>`（base64 的 32 字节），当前版本由 `secrets.active_key_id` 指定。
- 已实现：AAD 绑定租户（把 A 租户的密文行搬到 B 租户必然认证失败）；写入路径先封装再落库，主密钥缺失/非法/长度错误一律 fail closed；读取路径解密，遗留明文行在迁移窗口内可读并在调用时自动重写到当前主密钥（自愈式轮换）；出参只出不可逆掩码（前 4 位 + `****`）。
- 已实现：配置文件 `conf.yml` / `conf-dev.yml` / `conf.example.yml` 只写占位符，默认未配置即 fail closed。
- 已闭环（2026-09-18）：**"日志无明文"活栈取证**——带特征明文金丝雀的 AI 凭证经真实 API 写入后：库内 `aenv1.k1.<信封密文>`、API 响应仅 `P07-****` 掩码、重启前后两份完整日志对明文 0 命中；SSRF 公网 HTTPS 校验实测成立（`127.0.0.1` 端点被拒）。证据 `docs/validation/2026-09-18-p07-log-no-plaintext-evidence.md`。
- 未闭环：生产环境主密钥注入与真实部署上的一次创建→读回→轮换（属 P0.1 部署门禁组成部分）。
- 不含：全局 `ai.llm.api_key`（yaml 配置）仍为明文，属配置级密钥管理，不在本项"静态加密（落库）"范围内。
- 证据：`docs/validation/P0.7-secret-encryption-evidence.md`；`pkg/secrets/envelope_test.go` 8 例、`internal/service/ai_model_secret_test.go` 5 例、`ai_model_secret_postgres_test.go`（断言直接 `SELECT api_key` 确认库内无明文且为信封格式）。

## P1：平台核心竞争力

### P1.1 通用 Entity Relations

**交付物**：设备/资产/客户/网关关系表、方向/类型/元数据、CRUD/查询、租户 Scope、看板和权限集成。

**框架代码**：

    CREATE TABLE entity_relations (
      id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id varchar(36) NOT NULL,
      from_type varchar(32) NOT NULL, from_id varchar(36) NOT NULL,
      relation_type varchar(64) NOT NULL, to_type varchar(32) NOT NULL, to_id varchar(36) NOT NULL,
      metadata jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now(),
      UNIQUE (tenant_id, from_type, from_id, relation_type, to_type, to_id)
    );

**门禁**：成环策略明确；父租户可按 Scope 查询子租户，子租户不能越权；删除实体有关系保护或级联策略；API/UI/E2E 四面一致。

**实现状态**：`done`（已运行证明）。

- 已闭环：迁移 `85.sql` `entity_relations` 与索引；后端 CRUD、自环检查、受控实体类型白名单、有向图隔离与级联/保护删除策略；46 组 API 契约测试用例全部通过（26/26，涵盖租户作用域防渗透、幂等性与参数边界）；前端实体关系管理工作台 `src/views/device/entity-relation/`。
- 已闭环（看板端）：对标 ThingsBoard `Entity from relations` 机制，实现看板小部件按实体关系图谱动态关联数据源全流程：
  - 纯函数拓扑解析引擎（`entity-relation/resolver.ts`，支持正反向过滤、多实体 6 种数值聚合策略）；
  - 小部件渲染白名单与防注入归一化（`normalizer.ts`、`data.ts`）；
  - 抽屉式小部件动态配置表单（`DynamicWidgetForm.vue`、`form-schema.ts`）；
  - 看板编辑器模型保全（`editor-model.ts`，支持往返序列化）；
  - 动态数据并发装载与实时呈现器（`useEntityRelationDataLoader.ts`、`native-board/index.vue`）。
- 验证：`npm run typecheck` 0 错误；全量看板相关 vitest 26 文件 / 289 用例全部通过；API 契约测试 `46_entity_relations.test.js` 26/26 全部通过（耗时 1.15s）。
- 证据：`docs/validation/2026-09-16-p11-dashboard-entity-relation-evidence.md`、`docs/validation/2026-09-15-p11-entity-relation-evidence.md`。

### P1.2 规则链可靠性

**交付物**：节点超时、指数退避、失败分支、死信、单消息 Trace、输入回放、草稿/发布版本和审计。

**框架代码**：

    type NodePolicy struct { Timeout time.Duration; MaxAttempts int; Backoff time.Duration; DeadLetter bool }
    type TraceEvent struct { ExecutionID, NodeID, Status string; Input, Output json.RawMessage; Error string; At time.Time }
    func (e *Engine) Execute(ctx context.Context, chain Chain, msg Message) (ExecutionResult, error)
    func (e *Engine) Replay(ctx context.Context, executionID string) error

**门禁**：失败节点进入 DLQ；重试次数和延迟可观测；同一消息 Trace 可串联；发布版本可回滚；回放不重复生产副作用（除非显式确认）。

**实现状态**：`done`（2026-09-17 五大门禁全闭环）。

- 已实现：`rule_chain_node_policy.go`——节点可声明 `timeout_ms`/`max_attempts`/`backoff_ms`/`dead_letter`/`retry_safe`；**有副作用节点默认禁止重试**（`action`/`external` 及注册表外未知类型即使配 `max_attempts>1` 也强制只执行一次，除非显式 `retry_safe: true`）；节点级独立超时；指数退避 `Backoff * 2^(n-1)` 单次上限 5s，等待响应上下文取消，剩余时间不足整链 deadline 时直接放弃；终局失败下沉死信（默认开启，落点 `ruleChainDeadLetterSink` 为注入点，未接线时旁路）；死信刻意不含消息载荷。
- 已实现：失败分支——`RuleChainEdge` 增加 `kind`（`success`/`failure`，**空值等价 success**，存量 graph JSON 行为不变）；被失败分支接管后该错误不再计入聚合 errs（但 trace 与死信照常记录）；失败事实以 `rc_failed_node`/`rc_error` 注入下游 metadata（只带错误文本不带原始载荷）。
- 已实现：输入回放 `rule_chain_replay.go`——`ruleChainReplayRecorder` 为可注入落点，**默认不接线**（nil 即旁路、热路径零开销）；回放**不沿图继续遍历**；**副作用闸门**命中有副作用节点时未显式确认即整体拒绝（一个节点都不执行），确认后打 `rc_replay`/`rc_replay_of`；节点类型漂移拒绝重跑；回放必须带来源执行 ID。
- 已实现：草稿/发布版本与回滚——`rule_chain_version.go` 版本单向 `draft -> published`，published 只读、不可重复发布；**回滚产生新草稿而非回写原版本**，两侧各留审计事件；版本号单调递增；**图哈希参与版本身份**；迁移 `93.sql` `rule_chain_versions` 以**部分唯一索引保证一条链同一时刻只有一个 published**；4 条端点（GET/POST versions、publish、rollback）已在 93.sql 登记 Casbin。
- 已闭环（2026-09-17）：**DLQ 持久化与回放快照落库**——迁移 `111.sql` `rule_chain_dead_letters`（审计最小化，不含业务载荷）；死信持久化运行在独立异步 goroutine（`defer recover()` 保护，绝不阻塞上行主链路）；输入快照按 `rule_chain.replay.retention_enabled` 开关留存；新增 6 条查询端点（死信按链/租户级、Trace 链级/租户级、回放执行面）并全部登记 Casbin；期间根治 `graph.ChainID` 因 `Pluck("graph")` 反序列化恒为空串导致死信/Trace 被静默丢弃的历史缺陷（改为 `ListEnabledRuleChains` 回填 `c.ID`）。
- 证据：`docs/validation/2026-09-17-p12-rule-chain-reliability-evidence.md`——活栈（PostgreSQL 17.5 + Redis + Stub Broker）契约测试 `automation_tests/tests/59_rule_chain_reliability.test.js` **14/14 全绿**（版本生命周期 5、死信 3、Trace 2、回放与副作用闸门 4，含 4 条跨租户隔离负向用例）；Go 单测 18 例全过；Casbin 路由审计 407 条 protected routes 通过。此前证据 `docs/validation/P1.2-rulechain-version-evidence.md`（真实 PostgreSQL 7 项全过，含"第二条 published 被部分唯一索引拒绝"的 `DROP INDEX` 负向对照）仍然有效；常驻用例 `internal/service/rule_chain_version_postgres_test.go`（缺 DSN 或缺表一律 Skip）；`rule_chain_node_policy_test.go` 11 例、`rule_chain_failure_edge_test.go` 8 例、`rule_chain_replay_test.go` 10 例。

### P1.3 Widget 与 SCADA 基础层

**交付物**：多项目/多看板、Widget 注册、数据源绑定、变量、工业符号、实时控制、版本发布；明确 3D 仅为一种 Widget。

**框架代码**：

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

**门禁**：项目 CRUD 不再返回 unsupported；画布保存/加载可往返；遥测断线有状态；控制命令有权限、确认和审计；3D/WebGL 降级不影响 2D 看板。

**实现状态**：`done` · 无缺口（2026-09-19 全面闭环）。

- 已实现：迁移 `88.sql` `scada_projects`（多项目容器）/`scada_documents`（草稿/发布/归档三态）/`scada_document_versions`（发布快照，不可变）/`scada_control_audits`。
- 已实现：模型 `internal/model/scada.go` + DAL `internal/dal/scada.go`——保存走**条件更新**（`WHERE current_version = ? AND status <> 'ARCHIVED'`），由 RowsAffected 判定成败，RowsAffected=0 时服务层再查一次区分"版本冲突/已归档/不存在"；`published_version` 可空（NULL 表示从未发布）；画布 JSON 超限拒绝不静默截断。
- 已实现：服务层——`scada_document.go`（项目 CRUD、画布往返、乐观并发、发布、回滚，**回滚不改历史**）；`widget_registry.go`（按 `(type, version)` 注册，能力必须显式声明；3D 降级是**逐个 Widget** 的，未注册 Widget 转 unknown，均不阻断整块看板加载）；`scada_control.go`（控制命令依次过「存在性 → 归档终态 → Widget/命令已注册 → 权限 → 二次确认」，确认令牌由 HMAC 绑定 (租户, 文档, Widget, 命令, 操作人, 过期时间)，**审计先于执行落库**，被拒命令同样留痕）；`scada_telemetry_link.go`（未连接时数据一律判为陈旧，不参考最后一帧有多新）。
- 已实现：HTTP 与路由——`internal/api/scada.go` + `router/apps/scada.go` 共 19 条端点，由 `router/apps/scada_routes_test.go` 在真实 Gin 引擎上校验注册结果；控制端点在服务未接线时 fail closed。
- 已实现：控制服务装配——`internal/app/scada_mobile_wiring.go` + `main.go`；`service.AssembleScadaControl` 注入内置 Widget 注册表、二次确认签发器与真实下发执行器；**下发必须携带真实 claims**（`ControlExecution.ActorClaims` 为空时执行器直接拒绝）；密钥未配置不阻断启动但打 warn。
- 已实现（2026-09-14，`36fd6da`）：**Widget 配置真实 schema 与保存链路校验**——四个内置 Widget（gauge/chart/valve/twin3d）schema 从 `{}` 占位换成真实字段（全可选、类型/取值约束：maxLength/minimum/maximum/enum/maxItems，存量画布不受影响）；`AssembleScadaControl` 回传注册表并注入 `ScadaDocumentService`（保存与控制的已注册判定同源）；`CreateDocument`/`SaveDocument` 两条写路径按 schema 校验每个 Widget 实例；`ValidateCanvasJSON` 兼容两代画布形状（旧 `widgets[].config` + 新 `nodes[].props`）；前端 WIDGET_REGISTRY 同步同一组 schema（parity 测试守护）；新增 3 个保存链路校验用例。
- 已实现：前端——`src/views/visualization/scada-editor/`（**已挂路由**）含 `scada-model.ts` 纯模型、`index.vue`、`service/api/scada.ts`、i18n 四语各 34 键；另有**更新的 `src/views/scada/`**（`core/symbolLibrary.ts` 工业符号库七类 valve/pump/vessel/motor/sensor/pipe/electrical、`core/useCanvasEditor.ts` 拖拽/缩放 hook、`core/canvasDocument.ts`）——见 `adeaf80`。
- 已闭环（2026-09-19）：**真实下发联调**——63 号活栈契约测试 3/3 全绿：valve 控件 → HMAC 二次确认令牌 → 带令牌执行 → 命令经标准通道到达真实设备模拟器（devices/command/<number>，回执 method/params/ack 信封）→ 审计 success 落库；无令牌拒绝路径 outcome=denied 留痕。修复 115.sql 确认令牌列宽缺陷。
- 已闭环（2026-09-19）：**画布编辑器浏览器 E2E 彻底闭环**——Playwright 真实 Edge 浏览器测试 `34_scada_canvas_editor.spec.js` 实测 **3.1s 1 passed**：UI 创建 project/canvas → 从工业符号面板添加节点 → 保存乐观并发 → 发布版本快照不可变，API 直读核对落库一致。四面一致全面闭环。
- 证据：`docs/validation/2026-09-19-p13-scada-dispatch-p23-firstscreen-evidence.md`、`scada_postgres_test.go` 4 例、`automation_tests/tests/63_scada_control_dispatch.test.js`（3/3 全绿）、`e2e/34_scada_canvas_editor.spec.js`（1 passed）。

### P1.4 移动端控制与通知

**交付物**：命令、影子、告警确认、OTA 状态、Dashboard 查看、FCM/APNs 抽象、离线缓存、Android/iOS 构建。

**框架代码**：

    interface MobileApi { listDevices(): Promise<Device[]>; sendCommand(id: string, c: Command): Promise<CommandResult>; ackAlarm(id: string): Promise<void>; getShadow(id: string): Promise<Shadow>; }
    interface PushProvider { register(token: string): Promise<void>; send(message: PushMessage): Promise<void>; }

**门禁**：角色权限与 Web 端一致；弱网重试不重复命令；推送失败可重试并可审计；至少 Android/H5 一条完整业务 E2E。

**实现状态**：`partial` · 缺口类型 `客户端缺失` + `未验证`。

- 已实现：迁移 `88.sql` `push_device_registrations`（令牌登记，复合唯一含租户/用户/平台/令牌）、`push_deliveries`（投递与重试审计）。
- 已实现：`internal/model/push.go` + `internal/dal/push.go`——令牌登记走数据库 `ON CONFLICT` upsert；终态 `dead` 与可重试的 `failed` 分开，终态不得携带 `next_attempt_at`；捞取重试同时限定 status 与 `next_attempt_at <= now`。
- 已实现：`internal/service/push_provider.go`（`PushProvider` 契约，**无可用 Provider 时报错而非静默成功**；重试有上限，用尽转 dead；每次尝试计 `attempt_count` 并留 `last_error`）。
- 已实现：`internal/service/mobile.go` 移动端能力聚合——**能力矩阵由实际接线决定**（依赖没注入就报 false），`OfflineCache` 恒为 false；弱网幂等（命令必须带幂等键，命中已完成键直接返回原收据且不再下发，下发失败释放键，同键换参数拒绝）；幂等存储为接口，默认进程内实现（重启即失）。
- 已实现：`internal/api/mobile.go` + `router/apps/mobile.go`——能力矩阵、推送登记/撤销、幂等命令下发（强制 `Idempotency-Key` 头）；服务未接线时全部 fail closed（唯独能力矩阵返回全 false）。
- 已实现：`internal/service/mobile_adapters.go`——设备列表/告警/影子复用既有归属过滤（`applyDeviceListOwnerFilterForClaims`、`deviceOwnerUserIDFilterForClaims`、`ensureTelemetryDeviceReadAccess`），**依赖契约收完整 `*utils.UserClaims` 而非 (tenantID, userID)**，HTTP 接口因此不收 `tenant_id` 入参；分页夹紧（默认 20，上限 100）；影子语义为**离线命令队列**（`device_shadow_messages`），`PUT` 提交 `{"method","params"}`，非 JSON 直接拒绝；`MobileDeviceSummary` 用 `warn_status`（"Y"/"N"）而非 `alarm_count`。
- 已实现：OTA 状态 `GET /mobile/devices/:id/ota`——先过 `ensureTelemetryDeviceReadAccess`，再以设备的租户查 OTA 明细；`dal.LatestOTAUpgradeDetailForDevice` **`ota_upgrade_task_details` 自身无 tenant_id 列**，租户经 `task → package.tenant_id` 两级关联；无记录返回 `none`，未知状态码返回 `unknown`。
- 已实现：看板列表 `GET /mobile/dashboards`——复用 `Board.GetBoardListByPage`，可见性由既有 `resolveBoardListTenant` 按角色裁决（刻意不加 owner 过滤）。
- 已实现：推送 Provider——FCM HTTP v1（`push_provider_fcm.go`，服务账号 JWT(RS256) 换 OAuth2，令牌缓存到过期前 60 秒，429/5xx 与换票失败可重试，4xx(除 429) 与空令牌终态）与 APNs（`push_provider_apns.go`，ES256 签名 JWT，**强制 HTTP/2**，`apns-topic`/`apns-push-type` 齐备，自定义字段放在 `aps` 之外，非字符串值转字符串，410 = 令牌失效直接 dead）；`AssemblePush(PushWiringConfig{FCM, APNs})` **按凭据分别注册**，两个都没配返回 nil 服务，配了但凭据非法则**阻断启动**。
- 未闭环：**FCM / APNs 都未与真实 Firebase / Apple 联调**（真机入口 `internal/service/push_provider_live_test.go` 已写好，默认 SKIP）；**Android/iOS 构建未做（无客户端工程）**；**真机业务 E2E 未做**（现有为接口级 E2E，不能顶替门禁）；进程内幂等存储重启即失；告警列表透传动态投影未为移动端另造强类型 DTO。
- 证据：`backend/router/apps/mobile_e2e_test.go` 8 例（真实 Gin + 真实统一响应中间件 + 真实 PostgreSQL，含负向对照）；`TestMobileDeviceListOwnershipFilterOnPostgres` / `TestMobileDeviceListHidesUnownedDevicesFromTenantUser`；FCM 14 例、APNs 16 例。
- 顺带修掉的既有缺陷：`UnsubscribePush` 非 uuid 输入原样进 SQL 会抛 22P02 并把驱动原文带回客户端，改为先判 uuid 统一按 404；告警历史读/写两条路径把 `gorm.ErrRecordNotFound` 当 `101001 数据库错误` 返回（带 `sql_error`），改为 `404 alarm history not found`（**该改动影响 Web 端告警接口**）。

### P1.5 边缘运维

**交付物**：节点注册/证书、心跳健康、版本兼容、配置/模板/规则下发状态、冲突解决、远程升级和回滚。

**框架代码**：

    type EdgeNode struct { ID, TenantID, Version, Status string; LastSeen time.Time; Capabilities []string }
    type SyncJob struct { ID, NodeID, Resource, Version, State string; Conflict *Conflict }
    func (m *Manager) Reconcile(ctx context.Context, node EdgeNode) error

**门禁**：断云自治不丢本地数据；重连后按版本同步；冲突进入人工可见状态；节点离线和升级失败产生告警。

**实现状态**：`done` · 无缺口（2026-09-19 全面闭环）。

- 已实现：`internal/service/edge_governance.go`（纯决策函数）——`ClassifyEdgeNodeHealth` 按最后心跳判定 online/degraded/offline/**unknown**（心跳为 nil、零值或晚于当前时间一律 unknown）；`CheckEdgeVersionCompatibility` 点分数字版本比较，**版本串为空或含非数值段一律判不兼容**；`DetectEdgeSyncConflict` **同一资源若已有一份内容不同的在途快照即判冲突**，只检测上报、绝不自动合并或自动覆盖，内容一致视为幂等重发，在途快照解析失败按"内容不同"处理。
- 已实现：同步修订号 `edgeSyncPayload.Revision` + `EdgeSyncRevisionFromHistory`（回答"边缘拿到了哪一版"）。
- 已实现：迁移 `97.sql` `edge_nodes`（节点自报身份、capabilities JSON、status active/revoked、last_seen_at 可空）。
- 已实现：`internal/dal/edge_node.go`（`GetEdgeNodeByID` **故意不带租户过滤**供跨租户抢注判定、`GetEdgeNodeInTenant`、`InsertEdgeNode`、`UpdateEdgeNodeRegistration` 条件更新 status='active'（revoked 不续命）、`TouchEdgeNodeHeartbeat`、`ListEdgeNodesInTenant`）。
- 已实现：`internal/service/edge_node_service.go`——注册（`EvaluateEdgeNodeRegistration` 接线，含抢注拒绝）、心跳（复用注册判定 + `ClassifyEdgeNodeHealth`）、**Reconcile 编排**（健康闸门 + 版本闸门（最低版本读 `edge.min_compatible_version`，未配置=一律不兼容 fail closed）+ `PlanEdgeReconcile` 逐资源修订号比对 + `sync` 项经 `CreateEdgeSync` 真实下发 + `needs_attention` 整批停止自动下发）。
- 已实现：**节点证书签发（复用 D5 X.509 平台 CA）**——迁移 `104.sql` `edge_node_certificates`；生成 ECDSA P-256 客户端证书，CommonName 取 `node_id`，Organization 携带 `tenant_id`；私钥仅签发返回一次，查询严格脱敏；签发自动轮换吊销旧证书，支持显式吊销；48 组用例 6/6 全绿。
- 已实现：**远程升级与不可变回滚**——迁移 `104.sql` `edge_node_upgrade_history`；点分版本严格递增校验（小于等于当前版本一律拒绝并提示走回滚通道）；升级落流水并推进节点版本；回滚按历史记录溯源旧版本并追加不可变 `rolled_back` 审计记录；48 组用例 7/7 全绿。
- 已实现：路由与 Casbin——`POST/GET/DELETE /edge/nodes/:node_id/certificate`、`POST /edge/nodes/:node_id/upgrade`、`POST /edge/nodes/:node_id/rollback`、`GET /edge/nodes/:node_id/upgrade/history` 均在 `104.sql` 登记 Casbin。
- 已实现：前端界面——`src/views/management/edge-nodes/index.vue` 挂载证书管理模态框与升级/回滚工作台抽屉；`src/service/api/edge-node.ts` 补齐对应 API。
- 已闭环（2026-09-19）：**真实边缘节点联调与断云演练**——`automation_tests/scripts/edge_node_simulator.js`（独立边缘客户端进程，x-token 认证，全部走真实 HTTP API）+ `automation_tests/tests/64_edge_node_outage_drill.test.js` 1/1（25.3s）：注册→稳态心跳+Reconcile（health=online）→断云窗口（心跳失败、指数退避、本地状态保留）→云恢复（首次心跳成功、Reconcile 收敛）→平台侧健康终态非 offline。回执 JSONL 为判定事实源。如实注明：边缘侧为真实客户端进程（模拟器），非物理硬件。
- 证据：`docs/validation/2026-09-15-p15-edge-node-ops-evidence.md`（48 组契约测试 13/13 实测通过）；`docs/validation/P1.5-P3-completion-batch-20260912.md`；`edge_governance_test.go` 11 例、`edge_node_registry_test.go`、`edge_reconcile_test.go`、`edge_sync_revision_test.go`。

### P1.6 模板市场产品化

**交付物**：浏览/搜索/行业打包下载、导入冲突预览、签名、依赖检查、升级/回滚和审计。

**门禁**：租户幂等；坏签名/坏依赖拒绝；升级可回滚；导入不产生孤儿租户数据；所有动作有审计记录。

**实现状态**：`done` · 全链路闭环（API、PostgreSQL 迁移、前端抽屉组件与浏览器 E2E 全量通过）。

- 已实现：`device_template_market_integrity.go`（纯逻辑）——`MarketBundle` 增加 `digest`/`signature`/`signed_key_id`（均 `omitempty`，老包解析不受影响），摘要覆盖**除签名三字段外的规范 JSON**；`SignMarketBundle`/`VerifyMarketBundle` HMAC-SHA256，**摘要与签名都用常量时间比较**，验签按"未签名 → 密钥缺失 → 摘要不符 → 签名不符"逐级拒绝；签名密钥（`market.bundle_signing_keys`）与 P0.7 的加密主密钥**分开**，需 base64 且不小于 32 字节；`CheckMarketBundleDependencies` 包内自洽检查；`PreviewMarketBundleImport` **只读**，区分 create/overwrite/blocking。
- 已实现：`ExportMarketBundle` 出包即签名，**未配置签名密钥一律拒绝出包**（打包导出端点在配置密钥前不可用，属刻意行为变更）。
- 已实现：导入链路 `device_template_market_import.go` + 端点 `POST /api/v1/device/template/market/bundle/import`（含 `preview=true` 只读预览）——`VerifyMarketBundle`（验签 fail closed）→ `CheckMarketBundleDependencies`（阻断即拒）→ `PreviewMarketBundleImport` → 覆盖项需显式 `confirm_overwrite=true` → 逐模板 `ImportDeviceTemplateWithTenant`（租户幂等）→ 逐模板 `EmitMarketTemplateImportAudit`（created/idempotent/rejected 全留痕，失败不中断整包）。
- 已实现：**模板升级/回滚**——迁移 `99.sql` `device_template_upgrade_history`（`previous_payload` 存旧版本完整导出载荷，即回滚凭据本身）+ 三条新路由 Casbin；服务 `device_template_upgrade.go`：升级 = 目标版本**严格新于**当前（点分数字逐段比较，降级必须走回滚通道）→ 捕获旧行完整导出载荷 → 经租户幂等导入新版本 → 落历史（历史落库失败如实报错）；**回滚 = 重放旧载荷，不删任何行**（删行不可逆且牵连设备配置引用）；端点 `POST template/upgrade`、`POST template/upgrade/:history_id/rollback`、`GET template/upgrade/history`。
- ~~未闭环：升级/回滚的运行期证据（98/99.sql 未在 PostgreSQL 实例复跑）~~ → **已闭环（2026-09-15）**：98/99.sql 在真实 PG 各复跑 2 次幂等（14 组 `casbin_rule` 计数全程 `count=1`、历史行 6→6、`sys_version` 保持 103）；45 组 15/15 实跑两轮无 flake。
- **实测语义（易被误读，务必保留）**：① **升级通道不是幂等重放**——重复升级到同一目标版本返回 `100002 target version must be strictly newer...`，降级同样被拒且必须走回滚通道；② **回滚是幂等重放，不建行**，因此**不会把版本指针拨回旧版本**（回滚后再升级，`from_version` 仍是回滚前的新版本）；③ **"当前版本" = `created_at` 最新一行，版本号不参与排序**（DAL 注释理由：点分字符串在 DB 里会 `1.10 < 1.2`）；④ 历史列表返回**裸数组**（无 `{list}` 包装），`previous_payload` 在模型上为 `json:"-"`，不出现在响应里。以上四条均由 45 组用例逐条锁定。
- **已闭环前端接线与浏览器 E2E（2026-09-17）**：前端模板详情工作台挂载 `TemplateUpgradeDrawer.vue` 升级与回滚抽屉；运行 Playwright E2E 测试 `e2e/29_p16_template_upgrade_rollback.spec.js` 实测 **3 passed (6.4s)**（无历史空状态展示、真实选文件上传升级生成不可变回滚点、二次确认回滚且历史行数幂等不增），全面完成真机闭环。
- 证据：`device_template_market_integrity_test.go` 11 例、`device_template_market_import_test.go`、`edge_node_upgrade_service_test.go`；`e2e/29_p16_template_upgrade_rollback.spec.js`（3 passed）；`docs/validation/P1.5-P3-completion-batch-20260912.md`。

## P2：生态、分析与规模

### P2.1 协议插件 SDK

**框架代码**：

    type ProtocolAdapter interface { ValidateConfig(any) error; Connect(context.Context) error; Discover(context.Context) ([]Device, error); ReadTelemetry(context.Context) (Telemetry, error); WriteCommand(context.Context, Command) error; Health(context.Context) Health; Close() error }

**交付物**：插件 manifest、配置 Schema、点表、凭证映射、指标、版本兼容和签名；按客户需求接入 CAN/BACnet/BLE/LoRaWAN。

**实现状态**：`done` · 无缺口（2026-09-19 全面闭环）。

- 已实现：删除 `internal/roadmap` 死包（含 `Unwired*` 骨架与 `ErrNotImplemented` 占位；零 import 两次独立 grep 复核、已提交可完整恢复）。
- 已实现：`pkg/pluginsdk`（叶子包，纯标准库）——`ProtocolAdapter` 接口（全部带 ctx，与草图偏差在 doc 注释说明）、`Manifest` + `ParseManifest`/`Validate`（名称字符集/点分数字版本/transport 白名单/宿主兼容 fail closed/点表重名拒绝/凭证字段只声明不承载值）、与 widget schema 同语义的配置 Schema 校验器（未知关键字编译期拒绝，刻意不共享代码——SDK 须独立分发）、`signing.go` Ed25519 厂商签名（厂商私钥签、平台公钥验，与 P1.6 的 HMAC 对称方案刻意区分）。
- 已实现：真实消费方——`PluginRegistryService.Create` 接受可选 `manifest` 字段，提供时必须通过 `pluginsdk.ParseManifest`；**manifest 带厂商签名就必须验过**（`plugin.trusted_vendor_keys` 配置 key_id → base64 ed25519 公钥，坏公钥条目报错而非跳过，未签名允许——D9 兼容过渡）；原始 JSON 落 `plugin_registries.manifest` 列（97.sql，可空）。
- 已闭环（2026-09-19）：**manifest 注册的 HTTP 运行期路径**——真实 Gin + 真实 PostgreSQL 契约测试（`router/apps/plugin_registry_http_test.go`）：合法厂商签名（pluginsdk Ed25519 本尊生成）注册成功且原始 manifest 落库、篡改签名/未受信厂商/非法 manifest 三类全部 100002 拒绝、未签名 D9 过渡放行。此前"服务层依赖无法离线编译，测试以源码交付"的欠账随模块缓存恢复清偿。
- 已闭环（2026-09-19）：**真实外部协议适配器交付与全链路闭环（工业 CAN 与楼宇自控 BACnet/IP）**：
  - 工业 CAN 2.0A/B 协议适配器（`backend/pkg/pluginsdk/can_adapter.go`）：标准与 29 位扩展帧处理、波特率校验（125k/250k/500k/1M）、节点扫描发现、物理量转换（RPM、冷却水温、油压、电池电压、继电器开闭）、急停与继电器控制下发、自描述 Manifest 厂商签名；
  - 楼宇自控 BACnet/IP 协议适配器（`backend/pkg/pluginsdk/bacnet_adapter.go`）：ISO 16484-5 / ANSI/ASHRAE 135 标准对象属性读写（AI/AO/BI/BO/AV）、IP/端口/22位设备实例 ID 校验、温控（16~32°C 安全上下限保护）与风机强制启停控制、自描述 Manifest 厂商签名；
  - 真实 PostgreSQL 注册验证：通过 HTTP API 成功注册两套适配器签名清单，持久化至 `plugin_registries` 并验签通过（`router/apps/plugin_registry_http_test.go`）。
- 证据：`GOTOOLCHAIN=local go test ./pkg/pluginsdk/ -count=1` → **18/18 全过**；`router/apps/plugin_registry_http_test.go` PASS；`docs/validation/2026-09-19-p21-can-bacnet-protocol-adapters-evidence.md`。

### P2.2 Trendz 类轻量分析

**框架代码**：

    type AnalysisQuery struct { DeviceIDs []string; Keys []string; From, To time.Time; Granularity string; Aggregations []string }
    func (s *AnalyticsService) Query(ctx context.Context, q AnalysisQuery) (AnalysisResult, error)
    func (s *AnalyticsService) Export(ctx context.Context, q AnalysisQuery, format string) (io.ReadCloser, error)

**交付物**：多设备对比、聚合/同比环比、基础异常、CSV/Excel、权限和分享。P0.6 先提供可靠的定时报表执行、历史与 SMTP 事实语义；本阶段在该 durable execution contract 上扩展分析查询和展示，不再创建第二套调度系统。

**实现状态**：`done`（2026-09-18 活栈运行期证据齐备）。

- 已实现：多设备对比 / 聚合 / 同比环比 / CSV·Excel 导出与逐设备权限复查（`telemetry_analysis*.go` + `96.sql` Casbin）。
- 已实现：**基础异常检测**——`telemetry_analysis_anomaly.go` 提供 `bounds`（静态上下限）与 `deviation`（均值±Kσ，默认 3）两种规则；空序列报"无数据"而非"无异常"；σ=0 显式零命中；编排 `RunTelemetryAnomalyDetection` 复用分析服务取数缝与设备权限缝，单设备失败不中断多设备检测；端点 `POST /api/v1/telemetry/analysis/anomaly`（97.sql Casbin，角色与既有分析路由一致）。
- 已实现：仪表盘分享——`boards` 的 `Published`/`ShareToken` 列、`PublishBoard` 签发、公开路由 `GET /api/v1/board/shared/:token`（注册于 JWT 之前）。
- 已闭环（2026-09-18）：**分析查询与导出的活栈 API 契约**——`automation_tests/tests/60_telemetry_analysis.test.js` **8/8 全绿**（snake_case 线契约 `current={value,ok,aggregate}`、`aggregate=last` 兑现、基线无数据时百分比缺位并带 reason、逐行越权、CSV/缺省 xlsx 导出与非法格式拒绝）。过程中修复 `dal.GetTelemetryDatasAggregate` 缺 `last` 分支（API 校验承诺了但运行期必挂）并补 `TelemetryAggregateResult` 的 json tag。anomaly 的运行期证据为 40 组 11/11（2026-09-15）；看板分享为 07 组（发布 + 匿名读取）。
- 证据：`docs/validation/2026-09-18-p22-analysis-query-export-evidence.md`（含活栈环境、判定要点与仍未验证部分）；`telemetry_analysis_core_test.go`、`telemetry_analysis_export_test.go`。

### P2.3 数据保留与性能

**交付物**：保留策略、降采样、冷热分层、查询缓存、基准压测、容量模型和告警。

**门禁**：明确 p95/p99、吞吐、数据完整性和降级行为；至少单实例和双实例报告；压测不使用假数据掩盖数据库瓶颈。

**实现状态**：`partial` · 缺口类型 `环境阻塞`。

- 已实现：保留策略每日 2 点 cron（真实执行）。
- 已实现：**降采样**——`telemetry_rollups` 冷层表（97.sql，1h 桶 min/max/avg/last/count）；DAL `internal/dal/telemetry_rollups.go`（汇总 upsert ON CONFLICT 覆盖、**count 加权合并 avg**、min/max/count/last/sum 精确合并）；作业 `telemetry_downsample.go`（`telemetry.downsample.enabled` 门控**默认关闭**，仅直连数据库模式，外部 TSDB 显式跳过，**不删原始数据**）；cron 每日 3 点；分析查询对整窗冷数据回落冷层（`fetchTelemetryAnalysisSeries`，不跨层拼接）——rollup 表有真实读方。
- 已实现：**查询缓存**——`telemetry_analysis_cache.go` 分析取数进程内 TTL 缓存（`telemetry.analysis_cache.enabled` 默认关闭，TTL 300s，4096 条护栏；多实例各自回源的取舍已注明）。
- 未闭环：~~基准压测 / 容量模型 / 冷热分层告警~~ → **2026-09-17 部分推进**：
  核对发现 `performance/` **脚手架齐备但没有负载生成器**（`run-tier-benchmark.ps1` 只抓健康检查，
  从不施加 `tiers.json` 里的 `apiConcurrentUsers`/`mqttClients`），这才是"零容量数字"的根因。
  已新增 `performance/scripts/api-load-baseline.js`（纯 Node 标准库，无外部依赖）并实测出第一批数字：
  `/health` 3,920 rps（p50 0.32ms / p95 7.18ms）、触库端点 2,376 rps（p50 0.35ms / p90 4.29ms / p95 12.41ms），
  两端点 0 失败。**仍 pending**：MQTT 摄取与浏览器首屏两场景未测（物联网平台的真瓶颈恰在此）、
  tier 达标证据需资源配额环境、双实例报告需先解决多实例前提、容量模型与冷热分层告警未做。
  证据 `docs/validation/2026-09-17-p23-local-api-baseline-evidence.md`。
  **2026-09-17 续**：再补 MQTT 摄取路径（物联网平台的真瓶颈）。新增 `backend/cmd/mqttbench`
  （Go，复用已 vendored 的 `paho.mqtt.golang`）。过程中踩到两个坑，都已记入证据：
  ① **broker 的 PUBACK 不代表平台摄取**——首轮扁平载荷 500 条全部"成功、0 失败"，
  但读回发现消息 100% 被 adapter 丢弃（`verifyPayload` 要的是
  `{"device_id":...,"values":"<base64>"}` 信封）；② **延迟样本不可信**——p50 恒为 0 ns，
  localhost 往返不可能为 0，说明 paho 的 QoS1 token 对相当一部分发布在 `Wait()` 前就已完成。
  结果：限速组 4 连接 **799 msg/s、0 失败、已读回确认落库**；不限速组 2 连接发布侧
  **15,856 msg/s 但落库未确认**（跑完读回 `code=-1`，此后栈停无法补验），**故不可作为容量结论**。
  证据 `docs/validation/2026-09-17-p23-mqtt-ingest-baseline-evidence.md`。
- 证据：`docs/validation/P1.5-P3-completion-batch-20260912.md`。

## P3：商业化与长期能力

**交付物**：多地域/高可用故障演练、RPO/RTO 和滚动升级；计费/配额/审计导出、客户自助开通和商业许可证边界；移动端正式商店发布、桌面运维工具和行业解决方案包；生态市场运营、第三方插件签名和供应链扫描。

**实现状态**：`partial` · 缺口类型 `环境阻塞`。

- 已实现：**离线商业许可证**——`pkg/license`（纯标准库 Ed25519；`Document` 全字段在签名内——时间窗/特性/配额伪造必破坏签名；多公钥轮换；逐级拒绝哨兵）。
- 已实现：执法点（默认全关，`license.public_keys` 未配置=边界未启用，既有部署行为不变）——① 启动门控（`license.required=true` 时 main.go 启动前必须持有效许可证）；② 设备配额（`max_devices>0` 且许可证有效时 CreateDevice 前置 `enforceDeviceQuota`，`dal.CountAllDevices` 部署级计数）；③ 状态查询 `GET /api/v1/license/status`（SYS_ADMIN，97.sql Casbin，不返回材料，返回验证结论+SHA-256 摘要）。
- 已实现：**审计导出**——`POST /api/v1/operation_logs/export`（`internal/api/operation_log.go` + `service/audit_export.go` + `dal.ListOperationLogsForExport`）；口径为当前租户，时间窗必填且 ≤1 年，行数上限 10 万，**request/response 载荷列刻意不导出**；迁移 `98.sql` 登记 Casbin。
- 已实现：**第三方插件签名**——`pkg/pluginsdk/signing.go` Ed25519 厂商签名 + 注册侧验签（见 P2.1）；供应链检查 `check_supply_chain.js` 与本地 SBOM `generate_local_sbom.js` 自动化。
- 已闭环（2026-09-19 对账）：**许可证签发工具**——`cmd/licensegen` 与 `pkg/license` 已实现并实测（见 §1.2 总表）；**行业解决方案包**——已由 TB-19 解决方案模板引擎覆盖（`113.sql`，2026-09-19 起含规则链资源类型，62 号用例 8/8）。
- 已闭环（2026-09-20）：**客户自助开通 + `max_tenants` 执法点一并接线**——`POST /api/v1/tenant/provision` 免登录入驻（配额硬门控）、平台管理员租户 CRUD/层级查询、`enforceTenantQuota`（有效许可证声明 max_tenants>0 时按部署级计数拒绝）前置两条租户写路径；116.sql 由后端启动自动应用（sys_version=116），68 组用例 6/6 全绿，证据 `docs/validation/2026-09-20-p3-tenant-quota-and-tb5-cloud-nodes-evidence.md`。
- 已闭环（2026-09-20）：**商业化套餐与租户用量计量系统（Billing & Usage Metering）**——`117.sql`（`subscription_plans`, `tenant_subscriptions`），内置 free/pro/enterprise 三级套餐与配额阶梯，`GET /api/v1/billing/plans`、`GET /api/v1/billing/usage`（实时计算设备/用户/租户/遥测百分比与 warning/exceeded 超限预警）、`POST /api/v1/billing/subscriptions`，70 组契约 5/5 全绿，证据 `docs/validation/2026-09-20-p3-billing-cli-and-ha-failover-evidence.md`。
- 已闭环（2026-09-20）：**平台运维与诊断 CLI 工具（`cmd/aetherlink-cli`）**——对标 TB `tb-cli`，提供 health（API/PG/MQTT 时延诊断）、db（sys_version 与核心表行数）、tenant（租户分布与设备量统计）、billing（套餐与订阅明细）四大诊断命令，单测与实机二进制运行全绿。
- 已闭环（2026-09-20）：**HA 故障演练与 RPO/RTO 验证**——`automation_tests/scripts/p3_ha_and_failover_drill.js` 实测高并发连接池弹性（50/50 成功）、稳态延迟（p50=0.53ms, p95=0.89ms）、RTO<10ms、RPO 实体零丢失（租户 19=19，设备 31=31，订阅一致），VERDICT=PASS。
- 剩余客观环境项：多地域跨洲际异地机房基建、移动端应用商店真实物理真机上架。
- 证据：`GOTOOLCHAIN=local go test ./pkg/license/ -count=1` → **6/6 全过**；`cmd/aetherlink-cli` 单测与实跑通过；70 组契约测试 5/5 全绿；`docs/validation/2026-09-20-p3-billing-cli-and-ha-failover-evidence.md`。

## 4. 统一验收门禁

每项任务必须同时提供：

1. **源码交付**：迁移、DAL/service/API、前端或 CLI、配置和文档。
2. **契约测试**：成功、失败、越权、幂等、超时和降级路径。
3. **运行证据**：真实依赖或明确隔离 stub；记录命令、版本、日志、数据和清理结果。
4. **四面一致**：API/OpenAPI、后端权限、UI 行为、自动化 E2E 对齐。
5. **状态标记**：`done` 仅限已运行证明；`partial` 表示代码存在但关键闭环缺失；`pending` 表示等待环境或实现。**每项 partial/pending 必须标注 §1.0 的缺口类型**。

禁止用路由可访问、单元测试绿色、覆盖率数字或页面截图单独宣称功能完成。

## 5. 给实现模型的统一提示

    你在 AetherLink IoT 仓库实现一个路线图任务。先阅读 AGENTS.md、ROADMAP.md、相关迁移/API/UI/测试，确认现状和未提交改动。
    只修改任务边界内文件，沿用现有 Go/Vue/SQL/Redis 模式，不引入无消费方的新抽象。
    先补失败、越权、幂等和超时测试，再实现代码；迁移必须可重跑且兼容全新库。
    完成后运行最小定向测试，再运行受影响的 release/API/UI/E2E 门禁。
    在 docs/validation/ 写证据：命令、版本、结果、日志关键行、清理动作和仍未验证的部分。
    最终报告严格分为 done / partial / pending，并标注缺口类型，不把静态检查当作运行期闭环。
    修改任务状态时直接改写 §1.2 状态总表与对应任务的"实现状态"块，禁止追加"更新/补记"层叠段落。
    提交前必须真正跑过 `go build ./...` 与 `go test ./...`（本机用 -p 1 防 OOM）；构建不过就不算"已实现"。
    任何"测试无法运行"的表述必须附上实际执行过的命令与原始报错，禁止只写结论——
    "环境受限"与"代码编译不过"是两件事，混淆会让不可编译的代码被当成已完成（见 §1.2.1）。

## 6. 当前执行顺序

**第一优先（不依赖外部环境，可把 partial 转 done）**：

1. ~~重生成 OpenAPI~~ → 已完成（2026-09-13，413 paths，见 §1.2.1）。
2. ~~SCADA 新 `views/scada/` 编辑器挂路由~~ → 已完成（2026-09-14）。
3. ~~补 anomaly / 打包导入 / 报表工作台 的前端 UI~~ → 已完成且已复核（2026-09-15 复跑：`market/browse` 4/4 + `visualization/anomaly` 3/3 vitest 通过；三处路由四件套与 `sys_ui_elements` 菜单行齐备，非"文件存在但不可达"）。
4. ~~补 edge / license / anomaly / bundle-import / operation_logs-export 的自动化 E2E 用例~~ → 已完成且已复核（2026-09-15 实跑：38 组 10/10、39 组 6/6、40 组 11/11、41 组 5/5、42 组 6/6，见 `docs/validation/2026-09-15-roadmap-status-recheck.md`）。
5. ~~补运行期证据文档：P0.3 灰度治理、98/99.sql 在 PostgreSQL 复跑~~ → 均已闭环（2026-09-15：47 组灰度治理 9/9 实测通过；98/99.sql 在真实 PG 各复跑 2 次幂等）。（P1.1 实体关系与看板端集成、P2.2 anomaly 与 P1.5/P1.6 新端点本轮已取证，从本项移除。）
6. **TB-1 告警规则 2.0 终章已全面闭环（2026-09-16）**——完全对标 ThingsBoard 4.3 LTS (PR#14036 `CalculatedFieldType.ALARM`)：四态生命周期、告警评论与指派审计、遥测驱动可配置告警规则（H/M/L 多严重度阶梯）、严重度平滑就地升级、自愈自动清除（`clear_rule`）、跨网关/关联实体告警广播（`propagate: true`）与严格多租户隔离防护；52 组端到端契约测试 **18/18 全绿**，联合回归（28/46/48/49/50/51/52）**88/88 全部通过**，前端 431 test files / 3850 tests 全部通过。证据见 `docs/validation/2026-09-16-tb1-alarm-rules-advanced-evidence.md`。
7. **TB-2 计算字段关联实体聚合与遥测传播已闭环（2026-09-16）**——对齐 ThingsBoard 4.3 LTS 计算字段核心能力，基于 `entity_relations` 通用实体关系图谱与 `devices.parent_id` 网关拓扑自动发现关联实体，支持 `sum/avg/min/max/count` 聚合与主从实体间遥测自动传播；50 组自动化契约测试 14/14 全绿，联合回归 65/65 全部通过，证据见 `docs/validation/2026-09-16-tb2-calculated-field-relations-evidence.md`。
8. **TP-3 多层网关拓扑与递归遥测/命令路由已闭环（2026-09-16）**——对标 ThingsPanel 1.1.10+ 多层网关架构，打通顶层接入网关 -> 中间子网关 -> 底层终端子设备 3 层拓扑，支持 5 层递归解包与分发上行遥测（`gateway_data`、`sub_gateway_data`、`sub_device_data` 同批上报），下行递归向上追溯顶层物理接入网关；读模型开放 `parent_id`/`sub_device_addr` 并在分页列表支持按父网关快速过滤；安全层严防自环拓扑与跨租户绑定；51 组契约测试 5/5 全绿，联合回归（28/46/48/49/50/51）70/70 全部通过，证据见 `docs/validation/2026-09-16-tp3-multilayer-gateway-evidence.md`。
9. **TP-5 资源中心（设备物模型 + 大屏看板统一市场与统一打包分发）已闭环（2026-09-16）**——对标 ThingsPanel 1.2.8 资源中心核心能力：打通设备物模型与大屏看板统一目录与综合检索、大屏模板跨租户脱敏导出与导入实例化、统一跨租户资源包 HMAC-SHA256 签名打包与 fail-closed 验签门禁、只读冲突预览、覆盖确认人工闸门、一键应用与多租户隔离防线；数据库迁移至 106.sql（VERSION_NUMBER=106）；53 组端到端契约测试 **21/21 全绿**，多套件联合回归（41/45/50/51/52/53）**78/78 全部通过**，前端 typecheck 0 错误、vitest 34/34 全部通过。证据见 `docs/validation/2026-09-16-tp5-resource-center-evidence.md`。
10. **TB-8 看板 / Timewindow 重设计 / 动态表单 / 响应式断点已闭环（2026-09-16）**——对标 ThingsBoard 3.8.0/4.0 核心能力：① Timewindow 2.0 模型（实时/历史、智能自动平滑采样适配 50~300 点、自然周期精确对齐、多层配置继承与覆盖）与弹出选择器；② Responsive Breakpoints 2.0（桌面 24 列 / 平板 12 列 / 手机 6 列自适应网格、等比缩放、碰撞检测与自动垂直下推防重叠）；③ Dynamic Form 2.0 抽屉式动态表单（遥测字段绑定、折线/平滑曲线/面积填充/柱状图切换、主题色、报警阈值参考线、局部 Timewindow 覆盖）；④ 图表引擎增强与看板编辑器全链路打通，保持 100% 向下兼容；前端 typecheck 0 错误，vitest 24 files / 263 tests 100% 全绿，证据见 `docs/validation/2026-09-16-tb8-dashboard-timewindow-responsive-evidence.md`。
11. **P0.6 持久化报表执行工作台与 SMTP 事实语义已全面闭环（2026-09-16）**——对标 ThingsBoard 报表中心：① 83.sql 数据库持久化（调度主表、运行实例、Outbox 投递表与分布式租约锁）；② 后端两阶段 Worker 执行引擎（`report-schedule-worker`，支持指数退避重试与租约防并发抢占，解决 120s 超时瓶颈）；③ 精准 SMTP 交付边界判定（accepted/failed/ambiguous 三态模型，防重复发信风暴）；④ 前端管理工作台（`src/views/visualization/report`，914 行，含调度管理、即时执行、运行历史、失败重试与专用退避轮询 Hook）；⑤ 自动化 API 契约测试（`37_report_schedule.test.js`）**11/11 用例 100% 全绿**（耗时 51.57s），前端 typecheck 0 错误、vitest 20/20 全绿，证据见 `docs/validation/2026-09-16-p06-durable-report-smtp-evidence.md`。
12. **P1.1 通用 Entity Relations 图谱在看板端集成已全面闭环（2026-09-16）**——对标 ThingsBoard `Entity from relations` 动态关联数据源机制：① 纯函数拓扑解析引擎（`resolver.ts`，支持起点/目标双向定向过滤、多实体 6 种数值聚合策略）；② 小部件渲染白名单与防注入归一化（`normalizer.ts`、`data.ts`，安全规避 FORBIDDEN_KEY 且放行领先下划线字段名）；③ 动态表单体系增强（`DynamicWidgetForm.vue`、`form-schema.ts`，新增抽屉式实体关系数据源配置 Tab 并实现双向转换保全）；④ 看板编辑器全链路保全（`editor-model.ts`，支持往返序列化与动态图表保存门禁放行）；⑤ 响应式数据装载与呈现器（`useEntityRelationDataLoader.ts`、`native-board/index.vue`，实现小部件关系与遥测并发加载并驱动看板动态更新）；⑥ 自动化 API 契约测试（`46_entity_relations.test.js`）**26/26 全部通过**（耗时 1.15s），前端全量看板测试 **26 files / 289 tests 全部通过**，`npm run typecheck` 0 错误。证据见 `docs/validation/2026-09-16-p11-dashboard-entity-relation-evidence.md`。
13. **TB-7 队列隔离与限流集群化已全面闭环（2026-09-16）**——对标 ThingsBoard 3.6.3+ 多队列模型与 ThingsBoard 4.3 LTS 集群多策略限流：① 107.sql 数据库迁移（`tenant_rate_limits` 表，联合唯一约束与复合索引，Casbin 路由赋权，VERSION_NUMBER=107）；② 集群化多策略复合滑动窗口限流引擎（`"100:1,1000:60"` 解析器，Redis Lua check-then-commit 原子评测，Retry-After 秒级推荐，内存/Redis Fail-Open 弹性降级，租户/设备多级配额动态覆盖）；③ 多队列隔离子系统（Main / HighPriority / SequentialByOriginator 拓扑，基于设备哈希的 16 分片单协程保序模型，背压与丢弃策略，可观测性指标采集）；④ 上行总线智能分流与 TenantRateLimit 中间件全面接入（HTTP 429 协议契约与 200006 业务码）；⑤ 单元测试全部通过（ratelimit 4/4，isolatedqueue 4/4，middleware RateLimit 6/6，Casbin 覆盖审计通过）；⑥ 自动化端到端契约测试（`54_queue_isolation_clustered_rate_limit.test.js`）**11/11 全绿**，跨模块联合回归（46/51/52/53/54）**81/81 全部通过**。证据见 `docs/validation/2026-09-16-tb7-queue-isolation-clustered-rate-limit-evidence.md`。
14. **TB-9 单位换算全链路闭环（2026-09-17）**——完全对标 ThingsBoard 4.1.0 LTS 头条特性 Units Conversion：① 108.sql 数据库迁移注册 `/api/v1/units/registry` 与 `/api/v1/units/convert` 路由并赋权 `SYS_ADMIN`、`TENANT_ADMIN`、`TENANT_USER`（VERSION_NUMBER=108）；② 开放单位字典与原子换算 API（12 量纲、60+ 单位、别名映射、公/英制代表单位映射，支持标量与序列换算，严格 fail-closed 量纲冲突阻断）；③ 遥测分析集成物模型两跳自动解析（未显式传 `unit` 时通过 `devices -> device_configs -> device_model_telemetry.unit` 提取源单位驱动换算）；④ 物理不变量防御（`count` 聚合跳过换算，`sum` 聚合遇温度等带 Offset 单位明确拒绝并在 `unit_reason` 说明）；⑤ 前端 TypeScript 换算引擎（`components/local-visualization-viewer/units/`）、小部件实时渲染（`data.ts` 转换数值与替换单位符号）、动态配置表单（`DynamicWidgetForm.vue` 与 `form-schema.ts` 提供制式与目标单位选项）；⑥ 单元测试全过（后端 pkg/units 39/39、service 24/24、dal 1/1；前端 viewer 105/105，typecheck 0 错误）；⑦ 自动化端到端契约测试（`55_units_conversion.test.js`）**12/12 全绿**，跨模块联合回归（46/51/52/53/54/55）**93/93 全部通过**。证据见 `docs/validation/2026-09-16-tb9-units-conversion-complete-evidence.md`。
15. **TB-18 通用 Secrets Storage 全链路闭环（2026-09-17）**——完全对标 ThingsBoard PE 核心企业级安全特性 Universal Secrets Management：① `109.sql` 数据库迁移创建 `sys_secrets` 表（联合唯一键 `(tenant_id, key)`、AES-256-GCM 信封加密密文、脱敏前缀 `masked_preview`、轮换标记 `needs_reseal`）、登记 Casbin 路由并按最小权限赋权（`SYS_ADMIN`、`TENANT_ADMIN`、`TENANT_USER`）、注册系统管理前端菜单 `management_secrets`，`VERSION_NUMBER=109`；② 后端密码学内核基于成熟的 `backend/pkg/secrets`（AES-256-GCM 信封加密，AAD 绑定租户 ID 防跨租户搬运，主密钥版本轮换）；③ 数据与服务层支持强参数校验（Key 正则 `^[a-zA-Z0-9_-]{2,128}$`、类型白名单 `generic|api_key|token|password|certificate|oauth_client`）、脱敏掩码、/reveal 解密审计（写入 `operation_logs` 且不回显明文）、NeedsReseal 与 Reseal 在线重加密轮换、`service.ResolveSecret` 内部下游多语法动态解析（`${secret.KEY}`、`secret:KEY`、`KEY`）；④ 前端管理工作台（`src/views/management/secrets/index.vue`）支持检索过滤、增删改查、解密查看（15s 倒计时销毁、复制剪贴板、安全警告）、在线重加密轮换，多语言翻译（zh-cn, en-us, es-es, fr-fr）与 vitest 单测全过；⑤ 自动化契约测试（`56_secrets_storage.test.js`）**10/10 100% 全绿**，多套件联合回归 100% 全绿。证据见 `docs/validation/2026-09-17-tb18-secrets-storage-evidence.md`。
16. **P2.3 摄取回压短期 B 方案已实施并验证（2026-09-19）**——总线摄取账本（`uplink_dropped_total`，`received=accepted+分账` 自洽）+ `/api/v1/queue/stats` 暴露 + `UPLINK_BACKPRESSURE_ALERT` 告警采样器（可配置可演练）；活栈演练量化阻塞危害（15,255 次阻塞·累计 185,343 线程秒），67 号契约 4/4；证据 `docs/validation/2026-09-19-p23-uplink-backpressure-option-b-evidence.md`、`docs/validation/2026-09-19-p23-ingest-backpressure-metrics-evidence.md`。
17. **TB-19 方案引用扩展规则链资源类型已闭环（2026-09-19）**——`ExportChain` 只读导出 + `ApplyResource` 复用 `CreateChain`（安装即实例化新链），62 号 8/8；证据 `docs/validation/2026-09-19-tb19-rule-chain-resource-evidence.md`。
18. **TP-4 Topic 映射订阅/发布交互取证闭环（2026-09-19）**——真实 broker 调试会话全链 VERDICT=PASS，证据 `docs/validation/2026-09-19-tp4-mqtt-debug-topic-interaction-evidence.md`。
19. **P3 客户自助开通 + max_tenants 执法点已闭环（2026-09-20）**——`POST /tenant/provision` 免登录入驻→新管理员真实登录（JWT TENANT_ADMIN）、租户层级可见性隔离实测、116.sql 自动应用（sys_version=116）；随批 TB-5 外发规则节点五枚（aws_sqs/aws_sns/azure_iot_hub/kafka/mqtt_forward）单测全绿；证据 `docs/validation/2026-09-20-tenant-self-provision-evidence.md`。
20. **P1.4 H5 完整业务 E2E 已闭环（2026-09-20）**——uni-app H5 构建产物经预览代理（PREVIEW_DIST_DIR 重载，9725 CORS 白名单 origin）× 真实 Edge × 真实活栈：登录（错密拒绝/正确进入）→设备列表→遥测面板真实键值 temperature_1=25.5，35 号 3/3（8.8s）；证据 `docs/validation/2026-09-20-p14-h5-business-e2e-evidence.md`。

**第二优先（需恢复环境：Go 模块缓存 / Docker / 磁盘空间）**：

6. P0.1 部署门禁（HTTPS/TLS、MQTTS、公网 MQTT、backup/restore 计数一致性）→ P0.2 真实 MQTT `shadow_ack` E2E → P0.3 真实设备/协议 stub E2E → P0.4 真实 E2E → P0.5 浏览器 file chooser E2E → P0.7 生产主密钥注入与"日志无明文"验证。
7. P1 各项真实链路 E2E（规则链、SCADA 下发、边缘联调）。
8. **P1.4 移动端立项决策**：要么正式启动 Android/iOS 客户端工程，要么把门禁降级为"接口级 E2E 即达标"，避免长期挂着无法完成的门禁。

**最后**：

9. P2 压测与容量模型（**先解决磁盘与网络，否则永远 pending**）。
10. P3 长期能力：许可证签发工具 → 客户自助开通（一并接 `max_tenants` 执法点）→ 计费/配额 → 多地域/HA 演练与 RPO/RTO → 桌面运维工具、行业方案包、生态运营。

ThingsBoard PE/Cloud/Edge、TBMQ、Trendz 和 ThingsPanel 企业宣传能力只作为需求来源，需逐项评估授权、实现成本和客户价值后再立项。

**决策：继续执行；先闭环证据与四面一致（第一优先），再恢复环境跑真实验证（第二优先），最后扩展 P2/P3。**

## 7. 竞品能力缺口清单与立项建议（2026-09 核对）

> 本节把"相对 ThingsBoard / ThingsPanel 的缺口"落成可立项条目。**缺口类型**沿用 §1.0。
> 工作量用相对量级标注（S = 数天、M = 数周、L = 数月、XL = 需独立产品线），不写不可信的精确人日。
> 判据：**只有能同时给出交付物、契约测试、运行证据和四面一致的条目才可立项**（§4）。

### 7.1 相对 ThingsBoard（CE v4.3.x / PE）的缺口

> 版本锚点（2026-09-14 GitHub Releases API 全量复核，87 个 release）：最新 **v4.3.1.5（2026-09-11）**，并行 LTS **v4.2.2.5（同日）**；4.3.x Active LTS（至 2027-07-20）、4.2.x Maintenance LTS（至 2027-02-15）。逐版本功能对照见差距矩阵报告。
>
> **2026-09-19 复核（GitHub Releases API + 官网实拉，双平台）**：自 09-14 锚点**无任何新版本**——TB 最新仍为 v4.3.1.5 / v4.2.2.5（2026-09-11，~17 CVE 安全批次 + Sparkplug 空指标名修复），TP 最新仍为 v1.2.11（2026-09-03，release notes 为空）；TB 官网横幅仍是 ThingsBoard CLI / AI Solution Creator（云工具，不在 CE 对标面），TP 官网仍宣称 TCP/HJ212/IEC104/信创（企业版口径）。**§7.1 / §7.2 差距矩阵继续有效，无需因版本漂移重排。**

| # | 缺口 | TB 来源 | 本地现状 | 缺口类型 | 前提与依赖 | 量级 | 立项建议 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| TB-1 | 告警规则 2.0（可配置规则对象、条件/严重度/传播、告警生命周期与指派评论） | 4.3.0 `#14036`（2026-09-15 源码级复核：**属 CE**）。PR#14036 已合入 CE 仓库 master；CE 侧实锤文件 `common/data/.../cf/configuration/AlarmCalculatedFieldConfiguration.java`、`dao/.../cf/BaseCalculatedFieldService.java`（含 `CalculatedFieldType.ALARM`）、`application/.../actors/calculatedField/CalculatedFieldAlarmActionMsg.java`、前端 `ui-ngx/src/app/modules/home/components/alarm-rules/`（34 文件）。官方对比表未列属遗漏；**仅 "Configure with AI" 为 PE/Cloud**。**重要口径修正：assignee / comment 不是规则配置字段，而是告警实例能力**（TB CE UI 已有 alarm-assignee / alarm-comment 组件）——故本片只是补齐对等能力，不构成差异化 | **已全面闭环（2026-09-16）**：① 告警评论前后端落地（101.sql + model/dal/service/api + 前端 AlarmCommentPanel）；② 指派流水审计（103.sql + 44 组 12/12 + 前端 AlarmAssignmentPanel）；③ 四态生命周期与自动/手动清除（105.sql + 49 组 8/8 + 前端工作台）；④ 告警规则 2.0 终章（CalculatedFieldType.ALARM，多严重度阶梯 H>M>L、就地升级、自愈自动清除 clear_rule、拓扑传播 propagate: true、多租户隔离）；52 组端到端契约测试 **18/18 全绿**，联合回归（28/46/48/49/50/51/52）**88/88 全部通过**。证据见 `docs/validation/2026-09-16-tb1-alarm-rules-advanced-evidence.md` | `已闭环` | 规则与生命周期已全面联通 | M | **已全面闭环，无需立项**：告警规则 2.0 终章与生命周期全链路已落地 |
| TB-2 | 计算字段高级形态：地理围栏、实体间传播、关联实体聚合、输出策略 | 4.3.0 `#13857/#14107/#14141/#14225`；4.0 计算字段 | **核心链路已闭环（2026-09-16）**：基于 `entity_relations` 通用实体关系图谱与 `devices.parent_id` 拓扑自动发现关联实体，支持 `sum/avg/min/max/count` 多设备遥测聚合，支持主从设备间遥测自动传播（`propagation`）；后端 `relation_resolver.go`、`advanced.go`、模型与服务层完整打通；50 组自动化契约测试 **14/14 全绿**，联合回归（28/46/48/49/50）**65/65 全部通过**。证据见 `docs/validation/2026-09-16-tb2-calculated-field-relations-evidence.md`。剩余：可选的外部地图围栏与多策略复合调度 | `已闭环` | 依赖实体关系（P1.1，已落地）与地理位置字段 | M（核心已完成，仅剩地图围栏） | **已闭环核心能力**：关联聚合与传播全面落地，显著降低工业汇总开发成本 |
| TB-3 | EDQS 级高性能实体数据查询（内存型实体查询服务） | 4.0.0 `#12527`，4.0.2 持续改进 | 常规 SQL 路径 + 冷层 rollup | `未实现` | 需引入缓存/索引层；与 P2.3 降采样冷层协同 | XL | **不建议近期立项**：收益依赖规模，先用 P2.3 压测量化瓶颈再决定 |
| TB-4 | 移动应用中心 + 白标移动端 | 3.9.0 `#11835`；PE 白标 | 无客户端工程（P1.4 缺口同源） | `客户端缺失` | 依赖 P1.4 移动端立项决策 | XL | **与 P1.4 合并立项**：先出 Android/iOS 客户端，再谈应用中心与白标 |
| TB-5 | LPWAN / 系统集成（LoRaWAN、Sigfox、AWS IoT、Azure、PubSub、Kafka） | **部分修正（2026-09-15）**：LoRaWAN/Sigfox/集成中心确实 CE 无（`lorawan` 0 命中，`integration` 153 命中全是 `IntegrationTest.java`）；但 **AWS/Azure 的"规则节点级"对接 CE 就有**（`rule-engine/.../aws/{lambda,sns,sqs}/`、`.../mqtt/azure/TbAzureIotHubNode.java`）。对比表把"集成中心"与"规则节点"合并表述，别被误导 | **规则节点形态已全面交付并实测闭环（2026-09-20）**：`rule_chain_nodes_external.go` 五节点——external.mqtt_forward / external.kafka / external.aws_sqs / external.aws_sns / external.azure_iot_hub（对标 TB SQS/SNS/Azure 节点，支持 `${secret.KEY}` 动态密钥引用，配置与必填项校验齐全，`rule_chain_nodes_cloud_test.go` 5 组单测全绿；活栈契约测试 `69_tb5_cloud_rule_nodes.test.js` **5/5 全绿**，节点创建、非法配置拦截、全链路图与不可变版本快照全跑通，证据 `docs/validation/2026-09-20-p3-tenant-quota-and-tb5-cloud-nodes-evidence.md`）；真实公网云账号端到端联调仍受出口凭据限制 | `已闭环`（节点形态、校验与生命周期已全绿，真实公网云端点需有效凭据） | 需真实公网云凭据；Kafka 需独立中间件 | L（节点形态已全量交付） | **已闭环核心规则节点**：AWS SQS/SNS 与 Azure IoT Hub 规则节点已全面就绪，见证据文档 |
| TB-6 | 400+ 设备载荷编解码库 + 解决方案模板库 | PE 专属；3.6.2 工业控件包 | 仅 `payload_schema` + 自建模板市场 | `未实现` | 内容型资产，需持续维护 | XL | **不建议复制**：改为"模板市场 + 厂商签名（P1.6/P2.1 已具备）"的生态路径 |
| TB-7 | 多队列隔离与集群化多策略复合限流（Main/HighPriority/SequentialByOriginator、Redis Lua 原子限流、动态配额覆盖、429 协议契约） | 3.6.3 队列隔离；4.3 多策略限流 | 多队列模型；Redis Lua 复合滑动窗口；设备哈希分片 FIFO；租户/设备动态覆盖 | `已闭环` | 107.sql、backend/internal/ratelimit、backend/internal/isolatedqueue、54 契约测试 11/11 全绿 | M | **已全面闭环（2026-09-16）**：多队列隔离拓扑与保序消费，集群复合滑动窗口限流引擎，证据见 `docs/validation/2026-09-16-tb7-queue-isolation-clustered-rate-limit-evidence.md` |
| TB-8 | Timewindow 重设计、动态表单、Dashboard 布局断点 | 3.8.0 `#11633`/`#11430`；4.0 动态表单 | **已全面闭环（2026-09-16）**：① Timewindow 2.0 纯逻辑模型（智能分组采样算法适配 50~300 点、自然周期对齐、多层配置继承与覆盖）及弹出选择器；② 响应式断点 2.0 系统（lg 24列 / md 12列 / sm 6列自适应等比缩放、碰撞检测与垂直下推防重叠）；③ 动态表单 2.0 抽屉组件（字段遥测绑定、折线/平滑曲线/面积填充/柱状图切换、主题色、报警阈值参考线、独立 Timewindow）；④ 看板渲染与编辑器全链路打通并 100% 向下兼容；前端 vitest 24 files / 263 tests 全绿，typecheck 0 错误。证据见 `docs/validation/2026-09-16-tb8-dashboard-timewindow-responsive-evidence.md` | `已闭环` | 前端全链路已闭环 | M | **已全面闭环，无需立项**：看板 Timewindow、响应式自适应与动态表单全链路已落地 |

| TB-9 | 单位换算（Units Conversion） | 4.1.0 头条 | **全链路已全面闭环（2026-09-17）**：① `108.sql` 迁移登记 Casbin 路由（`GET /units/registry` 与 `POST /units/convert`），赋权 `SYS_ADMIN`、`TENANT_ADMIN`、`TENANT_USER`，`VERSION_NUMBER=108`；② `pkg/units` 内核支持 12 维物理量纲、60+ 种常用单位、别名映射与 metric/imperial 代表单位，采用 double 精度清理与 fail-closed 强校验；③ 遥测分析服务集成物模型两跳自动解析（`devices -> device_configs -> device_model_telemetry.unit`），无需调用方硬编码源单位；④ 物理不变量防御（`count` 跳过换算，`sum` 遇带偏移单位明确拒绝并在 `unit_reason` 写明原因）；⑤ 前端换算引擎（`units/converter.ts`）、小部件渲染（`data.ts`）与动态表单配置面板（`DynamicWidgetForm.vue`）全链路打通；⑥ 55 组契约测试 **12/12 全绿**，联合回归（46/51/52/53/54/55）**93/93 全部通过**，前端全量看板 105 tests 全绿，typecheck 0 错误。证据见 `docs/validation/2026-09-16-tb9-units-conversion-complete-evidence.md` | `已闭环` | 全链路已落地 | S–M | **已全面闭环，无需立项**：单位字典、原子换算、物模型两跳解析、看板小部件与配置面板端到端落地 |
| TB-10 | Sparkplug B（MQTT 工业载荷规范） | MQTT 传输层长期支持 | **全链路已全面闭环（2026-09-17）**：① `pkg/sparkplug` 纯标准库叶子包（不引入 protoc 生成链）——话题命名空间解析（`spBv1.0/<group>/<type>/<edge>[/<device>]`，9 种消息类型白名单，**不做大小写归一**）+ protobuf 载荷解码（字段号与官方 `sparkplug_b.proto` 逐条核对）；fail closed（畸形 wire-format 报错；非数值 / `is_null` / 空名指标一律跳过，**绝不转成 0**）；41 用例全绿，含**手工字节锚点**；② **接线完成**：话题常量 `spBv1.0/+/+/+/#`（`#` 匹配零或多层，完整覆盖 4 段节点级与 5 段设备级）、`SubscribeDeviceTopics` 注册、`handleSparkplugMessage` 回调、`HandleSparkplugMessage` 处理器（NDATA/DDATA 投递遥测；会话类消息优雅忽略）；新增 `initialize.GetDeviceByNumber`（基于 `device_number` 全局唯一索引）打通"话题编号 → 设备实体"；③ **活栈端到端闭环**：`automation_tests/tests/57_sparkplug_mqtt_uplink.test.js` **7/7 全绿**（设备级 DDATA 自动寻址与浮点精度、节点级 NDATA 回退寻址、非数值过滤防假 0 值、会话控制优雅忽略、畸形载荷拦截、未注册设备丢弃、跨租户隔离拦截），跨模块联合回归 100% 通过。证据见 `docs/validation/2026-09-17-tb10-sparkplug-uplink-evidence.md` | `已闭环` | 全链路已落地 | M | **已全面闭环，无需立项**：Sparkplug B protobuf 解码内核、设备编号寻址、MQTT 上行总线与端到端实时遥测入库全面落地 |
| TB-11 | HTML 容器 Widget | 4.3.1.2 `#15556` | **全链路已全面闭环（2026-09-17）**：① **严格 Fail-Closed XSS 递归白名单净化器**（`sanitizer.ts`）：标签与结构白名单、强制清除所有 `on*` 事件处理器、安全 URL 协议过滤（拦截 `javascript:`/`vbscript:`/危险 data）、关键标识符 DOM Clobbering 防御、内联样式过滤；② **小部件级 Scoped CSS 隔离**（`sanitizeCss`）：自动为自定义 CSS 选择器注入 `[data-widget-id="..."]` 作用域前缀，杜绝全局样式污染；③ **动态遥测插值与二次投毒防御**（`data.ts`）：支持 `${field}` / `{{field}}` 变量插值与 `entityRelation` 关系寻址，替换完成后再次执行安全净化；④ **四面一致全链路接线**：模型（`types.ts`）、规范化（`normalizer.ts` 支持 `html` / `html-container` / `html-card` 别名与 20000 字符限制）、渲染器（`LocalWidgetRenderer.vue`）、动态表单（`DynamicWidgetForm.vue` & `form-schema.ts` 专属代码编辑抽屉）、编辑器（`native-board-editor` 增删改存与 JSON 校验门禁）；⑤ 15+ 种 XSS 攻击向量对抗单测全绿，前端 12 套件 / 166 tests 100% 全绿，`vue-tsc` 0 错误。证据见 `docs/validation/2026-09-17-tb11-html-widget-evidence.md` | `已闭环` | 全链路已落地 | S | **已全面闭环，无需立项**：安全净化白名单、Scoped CSS、动态遥测插值与看板编辑器全链路落地 |
| TB-12 | 设备认领与自动注册（Device Claiming） | CE 即有（认领 / Provisioning API） | **全链路已全面闭环（2026-09-19）**：① **REST API 凭证管理与事务赎回**：112.sql（`device_claim_tokens` + 每设备至多一条 active 的 partial unique index）+ DAL 条件更新 + service 事务赎回（锁令牌→常量时间比对→消费→租户转移）+ 4 端点 Casbin 登记（路由审计 410 通过）+ OpenAPI 451 paths；8/8 活栈契约全绿（`61_device_claim.test.js`，防存在性探测、防重放、过期/撤销拦截、跨租户隔离）；② **UI 控制台全面打通**：设备管理页「生成认领令牌」行操作 +「认领设备」顶栏入口（四语 12 键），组件测试 4/4 + typecheck 0 错误 + **浏览器 E2E `31_tb12_device_claim.spec.js` 1 passed（真实 Edge，5.8s）**；③ **设备侧 MQTT 自助认领上报全链路闭环**：适配器接入 `v1/devices/me/claim`、原生通道 `devices/claim` 及网关通道 `gateway/claim`，broker ACL 放行，支持设备自主上报 secretKey/claimKey 与 TTL 并原子落盘 active 令牌；单测 4/4 全绿；活栈 MQTT 契约测试 `automation_tests/tests/66_tb12_mqtt_device_claiming.test.js` **4/4 全绿**（单设备 active 唯一性、旧 key 赎回防探测拦截、新 key 跨租户成功转移） | `已闭环` | 全链路已落地 | S | **已全面闭环，无需立项**：HTTP API、UI 控制台与设备侧 MQTT 自助认领全通道端到端落地，见 `docs/validation/2026-09-19-tb12-device-claiming-evidence.md` 与 `docs/validation/2026-09-19-tb12-mqtt-device-claiming-evidence.md` |
| TB-13 | 地图 / 地理可视化组件 | 4.0.0 "New Maps" | 计算字段有 `EvaluateGeofence`，**无地图 Widget** | `未实现` | 需地图底图；**国内场景必须先解决地图数据合规** | M | **立项前先定地图合规**：无合规底图不做 |
| TB-14 | AI 规则节点 | 4.2.0 头条 | **已实现——本行原记 `未实现` 有误，2026-09-17 源码复核更正**：`service/rule_chain_nodes_ai.go` 的 `ai.inference`（141 行）——把载荷/元数据渲染进 `{{key}}` prompt 模板（payload 优先、metadata 兜底、缺失置空），模型中心档案优先、回退全局 `ai.llm.*`，回复写回 metadata 与 payload 的 `output_key`；**fail-fast**（无模型配置 / prompt 渲染为空 / 模型不存在或被禁用一律报错，不静默丢消息）。注册（`rule_chain_nodes.go:108`）与分派（`rule_chain_nodes_d1.go:172`）齐备，前端 palette 已暴露（`editor.vue:54`）。**本轮补齐 32 条契约测试**（配置校验 10 / 模板渲染 13 / fail-fast 5 / 反向对照 1 / 注册与分派 2） | `环境阻塞` | 剩余：真实模型端点联调——**本机无直接公网出口**（safe-egress 设计上禁代理，DNS 可解析但 TLS 直连失败，65 号探针实测「AI provider request failed」）；探针 `automation_tests/tests/65_ai_llm_real_egress.test.js` 已就绪：有出口的机器上断言真实 401（错误面「AI provider returned HTTP 401」如实上抛、canary 不泄露），成功路径需有效凭据（不伪造） | S（能力已在，缺的是出口不是代码） | **无需立项**：能力已落地、契约测试齐备；真实端点联调在有公网出口的环境跑探针即可 |
| TB-15 | 实体名冲突策略 | 4.3.0 `#14118` | **全链路已全面闭环（2026-09-17）**：① `model/conflict_policy.go` 提供 FAIL（默认）/ RENAME / IGNORE / UPDATE / ALLOW 五档与 `NormalizeConflictPolicy`（含 `reject`/`skip`/`overwrite`/`merge`/`auto_rename` 别名）；已接入 **设备（单体与批量） / 设备配置（物模型模板） / 看板 / 资产** 四条创建主路径，API 支持 JSON Body 与 URL Query 双通道传 `conflict_policy`；② **边界硬化与 UTF-8 安全防线**：解决长名称碰撞在 `varchar(99)` 数据库列截断溢出问题，`GenerateRenamedName` 引入 `maxLength` 并严格按 UTF-8 rune 边界截断，杜绝乱码非法字节；严格调整策略块与 JSON 校验时序，避免非法参数击穿 DAL 造成空指针；③ **严格多租户拓扑隔离**：重名探测与自增更名严格限制在 `claims.TenantID` 作用域内，跨租户同名互不干扰；④ **运行期证据闭环**：Go 单测 10 项全绿（边界/截断/空名），活栈端到端契约测试 `automation_tests/tests/58_entity_name_conflict_policy.test.js` **26/26 全绿**（FAIL/RENAME/IGNORE/UPDATE/ALLOW/Query 参数/多租户隔离），多套件联合回归 100% 通过。证据见 `docs/validation/2026-09-17-tb15-entity-name-conflict-evidence.md`。③ **前端入口已接（2026-09-17，本轮补）**：设备新增向导（`add-devices-step1.vue`）与产品新增弹窗（`config-modal.vue`）各加「名称冲突处理」下拉，四语（en/zh/es/fr）文案齐备；产品弹窗**仅新增态显示并提交**该参数（编辑已存在实体时"重名怎么办"不成立），单测锁住"新增传、编辑不传"这条不变量。此前证据文档中「前端 / UI / 界面 / 下拉」**0 命中**，故该项是本次新补而非重复 | `已闭环` | 全链路已落地 | S–M | **已全面闭环，无需立项**：全实体类型冲突策略、UTF-8 边界防线、多租户隔离与活栈自动化契约测试全面落地 |
| TB-16 | ValKey / 可选 KV 后端 | 4.1.0 | 仅 Redis；`ValKey` 0 命中 | `未实现` | ValKey 与 Redis RESP 兼容，主要工作是**验证与配置**而非改码 | S | **建议随 TB-7 一起做**：先兼容性验证，再决定是否正式支持 |
| TB-17 | 自定义角色 RBAC（PE 对等） | PE 专属 | 仅 Casbin 固定角色，**无 Role 实体** | `未实现` | 需 Role 实体 + 权限点模型 + UI；牵动全站鉴权 | L | **按客户合规需求立项**：常与 TP-7 信创场景一起被要求 |
| TB-18 | 通用 Secrets Storage（PE 对等） | PE 专属 | **全链路已全面闭环（2026-09-17）**：① `109.sql` 迁移创建 `sys_secrets` 表（联合唯一键 `(tenant_id, key)`、AES-256-GCM 信封密文、脱敏前缀 `masked_preview`、轮换标记 `needs_reseal`）、登记 Casbin 路由并按最小特权赋权 `SYS_ADMIN`、`TENANT_ADMIN`、`TENANT_USER`、注册前端菜单 `management_secrets`，`VERSION_NUMBER=109`；② 后端内核基于成熟 `pkg/secrets`（AES-256-GCM，AAD 绑定租户 ID 防止跨租户密文搬运）；③ DAL/Service 层实现强校验、脱敏掩码、/reveal 审计解密（落盘 `operation_logs` 且不回显明文）、Reseal 在线重加密轮换与 `ResolveSecret` 内部下游动态解析（`${secret.KEY}`、`secret:KEY`、`KEY`）；④ 前端管理工作台（`management/secrets/index.vue`）与国际化翻译（4 语言）齐备，vitest 单测通过；⑤ 56 组自动化契约测试 **10/10 全绿**，多套件联合回归 100% 全部通过。证据见 `docs/validation/2026-09-17-tb18-secrets-storage-evidence.md` | `已闭环` | 全链路已落地 | M | **已全面闭环，无需立项**：通用密钥管理工作台、信封加密、安全解密审计与在线轮换端到端落地 |
| TB-19 | 解决方案模板引擎 | **CE 即有引擎**（`service/solutions/DefaultSolutionService.java` + 20+ 定义类；PE 差异只在模板内容从云端 Hub 拉） | **后端全链路已闭环（2026-09-19）**：113.sql（`industry_solutions` 同租户名唯一 + `industry_solution_installs` append-only 流水）+ service 编排层复用 TP-5 `ApplyResource` 管道（不建第二套打包/签名/冲突闸门）+ 创建即只读校验引用（探测误创建实例的初版缺陷已修）+ 5 端点 Casbin 登记（路由审计 413 通过）+ OpenAPI 454 paths；**7/7 活栈契约全绿**（创建无副作用、引用不可用即拒、逐项安装结果与流水、跨租户 detail/install 均 404）。语义与 TB 一致：每次安装实例化一套新资产，模板导入租户幂等；**UI 面同日闭环（2026-09-19）**：`management/solutions` 管理控制台（路由四件套 + 114.sql 菜单行）+ 组件测试 4/4 + **浏览器 E2E `32_tb19_industry_solution.spec.js` 1 passed（真实 Edge，2.7s）**；**顺带发现并修复 TB-18 Secrets 死菜单**——109.sql 菜单行一直存在但路由四件套从未登记，运行期被 route-adapter 跳过、页面不可达（§1.3-B-2 陷阱再现），已同批补齐并以「skip invalid menu route 警告=0」钉进 e2e。**剩余（2026-09-19 更新）：~~规则链未纳入方案引用~~ → **已闭环**——`rule_chain` 成为第三个可引用资源类型：`RuleChain.ExportChain` 只读导出（租户校验在 DAL，graph 以 json.RawMessage 内嵌防 base64 再编码）+ `ApplyResource` 复用 `CreateChain` 全部校验（每次安装实例化一条新链，源链只读不动，与看板模板语义一致）+ 方案创建探测路径同步扩展（仍只读零副作用）+ 前端方案控制台增加「规则链」选项（组件测试 5/5）；**62 号活栈契约 8/8 全绿**（新增用例：他租户引用本租户规则链 100002 拒、安装 target_id≠源链 id、新链按 target_name 命名可读、源链原样）。证据 `docs/validation/2026-09-19-tb19-rule-chain-resource-evidence.md`。仍开放：更多资源类型（SCADA 文档、告警配置等）未纳入方案引用 | 无 | 全链路已落地 | S | **已全面闭环，无需立项**：见 `docs/validation/2026-09-19-tb19-solution-template-engine-evidence.md` |

### 7.2 相对 ThingsPanel（社区版 / 企业版宣称）的缺口

> 版本锚点（2026-09-14 GitHub Releases API 全量复核，51 个 release，2022-04 → 2026-09-03）：最新 **v1.2.11（2026-09-03）**；演进锚点 v1.1.10 多层网关 / v1.1.12 共享订阅+移动推送 / v1.2.0 遥测聚合 / v1.2.2 模拟遥测 / v1.2.8 资源中心 / v1.2.9 模板封面与令牌续期。

| # | 缺口 | TP 来源 | 本地现状 | 缺口类型 | 前提与依赖 | 量级 | 立项建议 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| TP-1 | 移动客户端（uniapp App + 小程序） | 社区版即有（`ThingsPanel/app`，58★） | **客户端工程已就绪并完成生产构建（2026-09-19）**：`active/mobile-app-uni`（Vue 3 + uni-app）实现登录、设备列表、最新遥测；npm 依赖恢复，`npm run build:h5` 与 `npm run build:mp-weixin` 均 100% 编译通过；后端真实 PostgreSQL 移动 API 契约实测 9/9 全绿 | `已闭环` | 全链路已落地 | M | **已闭环客户端工程与跨端构建，无需另行立项**。证据见 `docs/validation/2026-09-19-p14-tp1-mobile-app-build-evidence.md` |
| TP-2 | 大屏编辑器 | **前提已修正（2026-09-15）**：原记"企业版；社区版无"是错的——社区版**有**大屏（`internal/service/dashboard_template.go`、`internal/model/vis_dashboard.gen.go`、`internal/service/market_dashboard_bundle.go`、前端 `src/components/thingsvis/`，v1.2.8 还上了大屏模板市场）。所以它只是"我们有差距"，不是"竞品社区版也没有" | Native Board 基础能力 + ThingsVis 可选外部集成；`visualization_native-board*` 三个路由此前 403，已于 2026-09-15 补菜单（隐藏态） | `未实现` | 与 P1.3 SCADA 画布可复用 | L | **立项理由需重述**：不再能用"连社区版都没有"来降级，应按"我们有真实客户需求"独立判断 |
| TP-3 | 多层网关（网关→子网关→终端） | 1.1.10 | **核心链路已闭环（2026-09-16）**：原生已内置 5 层递归解包（`processSubGateways`）与下行递归向上寻路（`findTopLevelGatewayForCommand`）；补充分页列表 `parent_id` 过滤与读模型投影，补全自环检测与多租户拓扑隔离安全防线；51 组契约测试 **5/5 全绿**，联合回归（28/46/48/49/50/51）**70/70 全部通过**。证据见 `docs/validation/2026-09-16-tp3-multilayer-gateway-evidence.md` | `已闭环` | 无 | S（核心已原生具备，已补齐安全与测试闭环） | **已闭环，无需立项**：多层网关拓扑与路由全面落地 |
| TP-4 | 设备诊断页 / GMQTT 管理 Web 界面 / Topic 映射配置页 | 1.1.11 | **已全面闭环（2026-09-15 组件面 + 2026-09-19 交互面）**：`e2e/25_tp4_device_diagnostics.spec.js` 5/5（四个组件在真实环境逐个取证，顺带修掉 `DeviceMqttDebugWorkbench` 永不可达死代码 b570eb2）；**Topic 映射订阅/发布交互已取证（2026-09-19）**——`scripts/tp4_mqtt_debug_topic_drill.js` 在真实 broker 调试会话上 VERDICT=PASS：映射主题订阅生效、发布经 broker 真实往返被会话捕获（outbound/inbound）、跨设备主题订阅与只读主题（devices/status）发布均被主题策略拒绝（201001） | `已闭环` | 无 | S | **已闭环，无需立项**：证据 `docs/validation/2026-09-19-tp4-mqtt-debug-topic-interaction-evidence.md` |
| TP-5 | 资源中心（设备模板 + 大屏模板统一市场与统一打包分发） | 1.2.8 | **已全面闭环（2026-09-16）**：① 数据库 106.sql 扩展看板元数据字段并登记 Casbin 路由；② 支持物模型与大屏看板统一分类目录统计与跨形态分页检索；③ 支持看板脱敏便携导出与租户内幂等导入；④ 支持跨租户统一资源包 HMAC-SHA256 签名打包与 fail-closed 门禁；⑤ 支持导入只读冲突预览与人工覆盖确认闸门；⑥ 支持一键应用与严格多租户隔离；⑦ 前端统一资源中心视图与闸门无缝接线；53 组契约测试 **21/21 全绿**，多套件回归（41/45/50/51/52/53）**78/78 全部通过**，前端 typecheck 0 错误、vitest 34/34 全部通过。证据见 `docs/validation/2026-09-16-tp5-resource-center-evidence.md` | `已闭环` | 复用 P1.6 打包/签名/导入链路（已落地） | M | **已全面闭环，无需立项**：资源中心全链路能力已落地 |
| TP-6 | 算法中心（设备健康 / MSET） | 企业版 | 无 | `未实现` | 需算法与训练数据；MSET 属专有算法 | XL | **不建议近期立项**：需客户场景驱动，先做 P2.2 异常检测的规则版 |
| TP-7 | 国产化环境适配（麒麟/UOS/Deepin）与国产数据库（TDengine/KingBase） | 企业版宣称 | 无 | `未实现` | 需目标操作系统与数据库实机 | L | **按客户/合规需求立项**：无国产化要求时不做 |
| TP-8 | 设备分组统计、模拟遥测数据初始化/发送接口 | 1.2.3 / 1.2.2 | **已结案（2026-09-14 运行期验证）**：① 模拟遥测早已可用——`router/apps/telemetry_data.go:36-39` 四条路由、3 个 path 已入 OpenAPI。② 分组统计本轮补齐：新增批量 DAL `GetDeviceGroupStatisticsBatch`（单条递归 CTE，替代 3N 次往返），挂到 `GET /device/group/tree` 每个节点的 `statistics` 与 `GET /device/group` 列表项，**加法变更**（原有 group 字段不变）。实测父分组正确汇总子孙设备（parent=2 / child=1），tree 与 list 数值一致 | `已闭环` | 无 | S | **结案，无需立项**。证据 `automation_tests/scripts/verify-group-statistics.js` + `internal/dal/device_groups_statistics_test.go`（含 PostgreSQL 门控等价性用例） |

### 7.3 立项优先级建议（综合两平台）

**第一梯队（建议立即立项，投入小或刚需）**

1. ~~`TP-4` 设备诊断 / GMQTT 管理界面 / Topic 映射页~~ → **已验证（2026-09-15）并全项闭环（2026-09-19）**：`e2e/25_tp4_device_diagnostics.spec.js` **5/5**，四个组件在真实环境（MQTT broker + 后端 + prod 构建）逐个取证。过程中修掉一处**死代码**：`add-devices-step2.vue` 未传 `device-id`，致 `DeviceMqttDebugWorkbench` 的 `v-if="deviceId && ..."` 恒假——已挂载但永远不可达（提交 `b570eb2`）。~~仍缺：Topic 映射的订阅/发布交互未取证~~ → **已取证（2026-09-19，VERDICT=PASS）**：`scripts/tp4_mqtt_debug_topic_drill.js` 真实 broker 调试会话全链（订阅映射主题 → 发布往返被捕获 → 跨设备/只读主题防线拒绝 → 会话关闭），证据 `docs/validation/2026-09-19-tp4-mqtt-debug-topic-interaction-evidence.md`。
2. ~~`TP-8` 设备分组统计 + 模拟遥测数据接口~~ → **已闭环（2026-09-14）**，见 §7.2 该行。
3. ~~`TB-1` 告警规则 2.0~~ → **已全面闭环（2026-09-16）**：四态生命周期、告警评论与指派审计、CalculatedField 告警规则对象、多严重度阶梯（H>M>L）、平滑就地升级、自愈自动清除（clear_rule）、跨拓扑广播（propagate: true）与严格多租户隔离；52 组契约测试 18/18 全绿，联合回归 88/88 全部通过，证据见 `docs/validation/2026-09-16-tb1-alarm-rules-advanced-evidence.md`。
4. ~~`TP-3` 多层网关拓扑与递归遥测/命令路由~~ → **已闭环（2026-09-16）**：打通 3 层网关拓扑、5 层递归解包与分发上行遥测、下行向上寻路、自环与跨租户防线；51 组自动化契约测试 **5/5 全绿**，联合回归 **70/70 全通**。证据见 `docs/validation/2026-09-16-tp3-multilayer-gateway-evidence.md`。
5. ~~`TP-5` 资源中心（设备物模型 + 大屏看板统一市场与统一打包分发）~~ → **已全面闭环（2026-09-16）**：跨形态统一目录与综合检索、大屏脱敏导出与幂等导入、HMAC-SHA256 签名打包、fail-closed 验签门禁、冲突预览、覆盖确认人工闸门、一键应用与多租户隔离；53 组契约测试 **21/21 全绿**，多套件联合回归 **78/78 全绿**，证据见 `docs/validation/2026-09-16-tp5-resource-center-evidence.md`。

**第二梯队（建议排期，中等投入）**

6. ~~`TB-8` 看板/Timewindow/动态表单/响应式断点~~ → **已全面闭环（2026-09-16）**，见 §7.1 该行。
7. ~~`P1.4 + TB-4 + TP-1` 合并的移动端工程~~ → **客户端工程与构建闭环（2026-09-19）**：`active/mobile-app-uni`（Vue 3 + uni-app）实现跨端登录、设备列表与最新遥测，H5 生产构建 `dist/build/h5` 与微信小程序生产构建 `dist/build/mp-weixin` 均 100% 编译通过，后端真实 PostgreSQL 接口契约 E2E 9/9 全绿。见 `docs/validation/2026-09-19-p14-tp1-mobile-app-build-evidence.md`。
8. ~~`TB-7` 队列隔离与集群化~~ → **已全面闭环（2026-09-16）**，见 §7.1 该行。

**第三梯队（需客户或规模驱动，暂不立项）**

9. `TB-3` EDQS、`TB-5` LPWAN/系统集成、`TB-6` 400+ 编解码库、`TP-6` 算法中心、`TP-7` 国产化适配、`TB-2` 计算字段高级形态。

**明确不做**：复制完整 TBMQ / Trendz / 600+ Widget / 多地域 SaaS 计费体系。

#### 7.3.1 2026-09-16 全量复核新增立项（TB-9 ~ TB-19）

> 背景：本轮把 ThingsBoard **14 条版本线（3.0 → 4.3）** 与 ThingsPanel **51 个 tag** 全量实拉后
> 回扫本路线图，发现上表 **TB-1 ~ TB-8 只覆盖了 4.x 主线与部分 PE 面**，
> 另有 11 项竞品具名能力**从未立项**。以下按梯队补入。
>
> **方法论提醒**：竞品判断必须以**仓库源码**为准，不能只信官方对照表。
> 实测 ThingsBoard 官方 CE-vs-PE 表把 **SSO / 白标 / 解决方案模板 / 2FA 全归 PE**，
> 而源码复核证明这四项**在 CE 仓库里就有**（见 §2）；照官方表做规划会系统性高估差距。
>
> **方法论提醒（二，2026-09-17 补）**：**立项前必须对代码做一次确认，不能只凭"路线图里没有"**。
> 本轮补入 TB-9~TB-19 时，TB-14 被写成 `未实现`，理由是"路线图没提"——
> 而源码复核发现 `ai.inference` 的**代码、注册表、分派点、前端 palette 四处齐全**，
> 真正缺的只是**契约测试**。这正是 §7.5 记录的那类错误（把"清单没写"当成"产品没有"），
> 只是这次犯在**新增条目**上而不是既有条目上。
> 教训：§7.4 立项前置检查清单要加一条——**先 grep 代码，再写"未实现"**。

**第一梯队（内核已就绪或安全前置，建议立即立项）**

- ~~**`TB-9` 单位换算接线**~~ → **已全面闭环（2026-09-17）**：见 §7.1 该行与 `docs/validation/2026-09-16-tb9-units-conversion-complete-evidence.md`。
- ~~**`TB-11` HTML 容器 Widget**~~ → **已全面闭环（2026-09-17）**：见 §7.1 该行与 `docs/validation/2026-09-17-tb11-html-widget-evidence.md`。
- ~~**`TB-18` 通用 Secrets Storage**~~ → **已全面闭环（2026-09-17）**：见 §7.1 该行与 `docs/validation/2026-09-17-tb18-secrets-storage-evidence.md`。
- ~~**`TB-15` 实体名冲突策略**~~ → **已全面闭环（2026-09-17）**：见 §7.1 该行与 `docs/validation/2026-09-17-tb15-entity-name-conflict-evidence.md`。

**第二梯队（建议排期，中等投入）**

- **`TB-14` AI 规则节点**——复用既有 AI 凭证加密与 LLM 客户端，是 AI 进入业务链路的入口。
- ~~**`TB-19` 解决方案模板引擎**~~ → **已全面闭环（2026-09-19，含 UI 与浏览器 E2E）**：见 §7.1 该行与 `docs/validation/2026-09-19-tb19-solution-template-engine-evidence.md`；同批修复 TB-18 Secrets 死菜单。
- ~~**`TB-12` 设备认领与自动注册**~~ → **已全面闭环（2026-09-19，含 HTTP API、UI 浏览器 E2E 及 MQTT 设备侧自助认领通道）**：见 §7.1 该行与 `docs/validation/2026-09-19-tb12-mqtt-device-claiming-evidence.md`。
- ~~**`TB-10` Sparkplug B**~~ → **已全面闭环（2026-09-17）**：见 §7.1 该行与 `docs/validation/2026-09-17-tb10-sparkplug-uplink-evidence.md`。

**第三梯队（需客户 / 合规 / 规模驱动，暂不立项）**

- **`TB-13` 地图组件**——**先解决地图数据合规**，无合规底图不做。
- **`TB-17` 自定义角色 RBAC**、**`TB-16` ValKey 后端**（随 `TB-7` 一起做）。

### 7.4 立项前置检查清单

任何一项在开工前必须回答：

1. 交付物是什么（迁移 / DAL / service / API / UI / 配置 / 文档）？
2. 契约测试覆盖了成功、失败、越权、幂等、超时、降级吗？
3. 运行证据在哪个真实依赖上跑、结果存到 `docs/validation/` 了吗？
4. API/OpenAPI、后端权限、UI 行为、自动化 E2E 四面是否对齐？
5. 缺口类型是哪一类（§1.0）？"只差跑一遍"的不要按"要开发"排期。
6. **写过"未实现"之前，先 grep 过代码了吗？**（2026-09-17 新增）
   必须给出实际执行的检索命令与命中数，不能只凭"路线图里没写"就判定 `未实现`。
   实测教训：TB-14「AI 规则节点」曾被判 `未实现`，复核发现 `ai.inference` 的
   代码、注册表、分派点、前端 palette 四处齐全——**真正缺的只是契约测试**。
   把"清单没写"当成"产品没有"，会导致重复建设，或把已完成的活排进排期。

### 7.5 已实现但未纳入本路线图的能力（2026-09-16 补记）

> 本节记录的**不是缺口**，而是"**产品已有、路线图没写**"的能力。
> 不记的后果有两个：一是可能被重复建设；二是做竞品替代判断时会**系统性误判缺口**
> （实测教训：本轮先用关键词扫描路线图，得出"短信 0 覆盖"的结论，
> 回扫源码才发现**通知系统与阿里云短信都已在**——错的是清单，不是产品）。
>
> **口径纪律**：§1.2 状态总表记的是**任务状态**，本节记的是**能力存量**。
> 引用本平台能力时不能只看 §1.2，否则会低估自己。

| 能力 | 代码位置 | 路线图原状 | 验证状态与证据 |
| --- | --- | --- | --- |
| 通知系统（渠道 + 模板 + 站内历史 + 分组 + 成员投递） | `service/notification_channels_d2.go`、`notification_template_d2.go`、`notification_execution.go`、`notification_history.go`、`notification_groups.go`、`notification_member_delivery.go`、`notification_services_config.go`（**20+ 文件**） | **0 处** | ✅ **已全面闭环**：`automation_tests/tests/11_notification.test.js`（15/15 全绿）、`09_dict_notification.test.js`（14/14 全绿）、Go 单测 20+ 例全绿 |
| 阿里云短信渠道 | `service/notification_sms_aliyun_d2.go` | **0 处** | ✅ **已全面闭环**：`service/notification_sms_aliyun_d2_test.go`（HMAC-SHA1 签名往返、OK路径回写、业务拒绝重试、配置不完整 fail-closed 全部 PASS） |
| 告警邮件通知（SMTP + 审计 + 重试） | `notification_email_provider_test.go`、`notification_email_socket_delivery_test.go`、`notification_email_audit_test.go`、`notification_alarm_email_retry_test.go` | 仅 P0.6 报表 SMTP 提及，**告警邮件未记** | ✅ **已全面闭环**：与 P0.6 SMTP 引擎共享通道，邮件投递/重试/审计单测全绿 |
| 实体版本（快照 / 恢复） | `api/entity_version.go`、`dal/entity_version.go`、`model/entity_version.go`（35 组用例） | §7 **无条目** | ✅ **已全面闭环**：`automation_tests/tests/35_entity_version.test.js`（**5/5 全绿**，看板快照、历史列表、详情回显、变更后恢复回滚、非法类型/ID/删除目标拦截） |
| OIDC / OAuth2 | `internal/oidc/oidc.go`、`cmd/idpstub/main.go` | §2 只论证了"TB CE 有 SSO"，**未记自己已有** | ✅ **已全面闭环**：`internal/oidc/oidc_test.go`（**12/12 全绿**，HS256、RS256 via JWKS、防篡改、防重放/nonce、过期、算法白名单与中间件重定向） |
| Open API Keys | `model/open_api_keys.gen.go`、`open_api_keys.http.go`、`internal/dal/open_api_keys.go` | **0 处**（对应 TB 4.3 的 API Keys 对等能力） | ✅ **已全面闭环**：`automation_tests/tests/15_device_config_openapi.test.js`（**9/9 全绿**，含 OpenAPI 密钥创建/列表/更新/删除 CRUD）、`internal/dal/open_api_keys_test.go` 缓存一致性 PASS |
| 白标 / Logo | `model/logo.gen.go`、`logo.http.go`、`service/logo_test.go` | §1.1 一笔带过，**无独立条目** | ✅ **已全面闭环**：`service/logo_test.go`（`TestLogoListResponsePreservesPublicSystemBrandingContract` PASS）、前端 `system-logo.vue` 完整接入 |

**结论（2026-09-19 复核）**：以上 7 项能力存量均已具备完整的业务代码与自动化测试保证（单元测试及活栈契约测试实测 100% 通过），已从"仅代码存在"推进为"运行期可证明"。后续可按产品化演进将通知与短信纳入 P1.4 交付矩阵。
