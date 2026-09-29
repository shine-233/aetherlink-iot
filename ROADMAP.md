# AetherLink IoT 路线图（2026-09-25 审计重制与实施版）

> 本稿系对仓库根 `ROADMAP.md`（2026-09-24 v3 终章版）的**清理重制**：旧稿中经独立审计证实失真/夸大的表述已在第 6 节逐条清理，本稿口径不再沿用旧结论。
> **2026-09-25 实施**：§5.1 近期计划中的 TB-30（告警实时 WS）、TP-05（一型一密产品级交叉校验）、TB-57（客户实体四面收尾）、TP-19（国产化交叉编译）与文档口径修复已落地，交付记录见 §5.1 与 §9；证据留痕 `docs/validation/2026-09-25-roadmap-remediation-batch-evidence.md`。**批次二**（TB-04/17/18/27/10/25/15/41/21，迁移链扩至 129.sql）已交付，证据留痕 `docs/validation/2026-09-25-roadmap-remediation-batch2-evidence.md`（§9 批次二实施记录）。**批次三**（两程 15 项：第一程 TB-45/46/11/22/23、TP-03/20/21，第二程 TB-47/48/15R/49、TP-22；TB-17R 经 2026-09-26 终审更正为 delivered；迁移链扩至 138.sql，132/133 转 NO-OP 占位）15 项已交付，证据留痕 `docs/validation/2026-09-25-roadmap-remediation-batch3-evidence.md`（§9 批次三实施记录）。
> 仓库基线：分支 `feature/roadmap-complete-tb-tp-parity`，HEAD `4be45c6`（2026-09-24）。

---

## 1. 口径说明（本稿基线）

### 1.1 版本与迁移链（实测值）
- **迁移链**：`backend/sql/` 最大编号 **`122.sql`** = `backend/pkg/global/global.go:21` 的 `VERSION_NUMBER = 122`，二者一致；1~122 连续无缺号（本会话逐文件核验）。`122.sql`（客户管理体系 Customer Management）系旧 ROADMAP 2026-09-24 快照之后新增，故旧稿"121 = 121"已过期。
- **2026-09-25 批次二后口径（更新会话实测）**：迁移链最大编号 **`129.sql`** = `backend/pkg/global/global.go:21` 的 `VERSION_NUMBER = 129`，1~129 连续无缺号（`backend/sql/` 下 .sql 文件计数 129，最小 1、最大 129）。123~129 系批次二新增（TB-04/17/18/27/10/25/41 各一迁移；TB-15 为 `57.sql` 就地修正、TB-21 无新迁移）。上文"122=122"为批次二前的基线快照。
- **2026-09-25 批次三后口径（更新会话 2026-09-26 实测）**：迁移链最大编号 **`138.sql`** = `backend/pkg/global/global.go:21` 的 `VERSION_NUMBER = 138`，1~138 连续无缺号（138 个 .sql 实测，逐号 diff 无缺号）。130~138 为批次三新增：TB-45→130、TB-46→131、TB-47→134、TB-48→137、TB-15R→138；**TB-19/TB-23 首轮分别落 132/133，2026-09-26 批次三编号重排后改挂 `135.sql`/`136.sql`，132/133 转为 NO-OP 占位迁移**（头注释载明重排背景与幂等升级路径，pg_init.go 对缺文件 fail-fast 故保留号位）。上文"129=129"为批次二后的基线快照。
- **应用版本**：`global.go:20-22` → `VERSION = "0.0.23"`、`SYSTEM_VERSION = "v1.2.3"`。注意 `README.md:127` 仍写 `VERSION_NUMBER=99`，属更早的过期注记，待随本稿一并修订。
- **业务表数**：旧稿"当前运行实例 138 张业务数据表"为活库声称、无法从仓库复核（粗代理：迁移文件内 `CREATE TABLE` 共 146 处）。本稿不再写死表数，以迁移链编号为准。

### 1.2 质量门禁（实测值）
| 门禁项 | 审计门禁结果 | 本稿撰写会话复跑结果 | 独立评审会话复跑结果 |
| --- | --- | --- | --- |
| `go build ./...`（backend/） | 通过 | **通过**（exit 0） | 通过（exit 0） |
| `go test ./... -count=1 -p 1`（backend/） | **跳过未执行**（审计环境构建步骤失败，原因未查明） | **66 个包全部 ok / 0 FAIL**（本会话实跑） | 完整重跑 exit 0：**66 ok / 0 FAIL**（84 包 = 66 ok + 18 无测试文件，与 `go list ./...` 计数吻合） |
| `pnpm run typecheck`（vue-tsc，frontend/） | 通过 | **通过**（exit 0） | 未运行（评审会话不作声称） |
| `pnpm run lint:check`（ESLint） | 未纳入审计门禁 | **未运行**（本稿不作声称） | 未运行 |
| automation_tests 契约/E2E 套件 | 未复跑 | **未复跑**（需活栈 PG+Redis+broker；通过结论依据 `docs/validation` 证据文档记载，部分已归档，见 1.4） | 未复跑 |
| OpenAPI | 483 paths | **489 paths**（2026-09-25 重生成实测；483 + 客户 5 端点 + `alarm/status/ws`） | 489 paths ✓（2026-09-25） |

> 评审会话对 §1.3 竞品版本覆盖声明**未做外网复核**（见 §7.2 第 8 条）。

> **2026-09-25 实施会话最终门禁（本表之后的最终口径）**：`go test ./...` **REAL_EXIT=0（66 包 ok / 0 FAIL**，含新增告警实时 6 例）；`vue-tsc --noEmit` 0 错误；前端全量 vitest 3970 例全绿；OpenAPI 重生成 **489 paths**（+客户 5 端点 +`alarm/status/ws`）；交叉编译三架构二进制实测产出。契约测试 75/76/77 已就位、待活栈运行（本机无 docker）。详见 §9 与证据文档。
> **2026-09-26 dev 活栈契约回归（最终口径的再更新）**：契约测试 75~101（24 文件）已在 dev 环境（本机 PG 17 隔离集群 55433 全新迁移链 1→138 + Redis + 本地 MQTT 夹具，无 docker）**全量运行通过：207 passing / 0 failing / 11 pending**。批次交付行状态据此由 partial† 升级为 **done**；88 号 5 个看板可见性用例待角色授权前置后补跑。详见 `docs/validation/2026-09-26-dev-stack-contract-tests-evidence.md`。

> **2026-09-25 批次二收尾门禁（任务书下达的最终口径，编排方统一收尾）**：`openapi=0`、`goTest=0`、`vue-tsc=0`、`vitest=0`（四项计数照录自批次收尾门禁结果，更新会话未复跑）。批次二 9 项（TB-04/17/18/27/10/25/15/41/21）全部 delivered、无 blocked/回退项；契约测试 78~86 已就位、按约定未在活栈运行（本机无 docker），对应行按 §2 记 **partial†**。详见 §9 批次二实施记录与 `docs/validation/2026-09-25-roadmap-remediation-batch2-evidence.md`。

> **2026-09-25 批次三收尾门禁（任务书下达的最终口径，编排方统一收尾）**：`openapi=0`、`goTest=0`、`vue-tsc=0`、`vitest=0`（四项计数照录自批次三收尾门禁结果，更新会话未复跑）。批次三两程 15 项中 **14 项 delivered**（第一程 9 + 第二程 5）、**TB-17R 经 2026-09-26 终审更正为 delivered（单项门禁时序归因，收尾全量门禁四绿含其改动），15/15 全部交付**；契约测试 87~101（实际落号 87/88/90~96/99/100/101 共 12 个，89/97/98 号位未占用；TB-11 无 REST 契约面故无落号）已就位、按约定未在活栈运行（本机无 docker），对应行按 §2 记 **partial†**。详见 §9 批次三实施记录与 `docs/validation/2026-09-25-roadmap-remediation-batch3-evidence.md`。

> 旧稿"go test 60+ 包 100% 通过"在本会话以实测值取代：**66 包 ok / 0 FAIL**。旧稿引用的 71/72/73/74 号契约测试"36 passing"等数字本会话未复跑，不再作为门禁口径。

### 1.3 竞品版本覆盖声明（调研基线）
- **ThingsBoard**：CE **v1.0（2016-12-06）~ v4.3.1.5（2026-09-11）共 87 个 release**；PE 与 CE 同号同步发布（PE 官方 release notes 覆盖 3.0.x~4.3.x 共 14 条版本线）。版本集合经 GitHub tags API 与 releases API 双向比对 **0 差异（87=87）**；官方文档表覆盖其中 52 个（3.0+），v1.0~v2.5.6 共 35 个仅存于 GitHub（官方站旧发布索引页已 404）。逐版能力以 GitHub releases API body 全文为准。
- **ThingsPanel**：主仓 thingspanel-backend-community **v0.1.1-beta（2022-04）~ v1.2.12（2026-09-24）共 52 个 release**，前端 46 个（无 v1.2.7），最新 v1.2.12 后端/前端/installer 同日同步发布；36 个配套仓库逐一核查（16 个有 releases，合计约 200 个 release；20 个无 release）。调研抓取失败项：**无**。
- **覆盖边界（诚实声明）**：PE 2.x 及更早官方 release notes 已下线（404，Wayback 查询不可用），白标、实体组、调度器、集成框架、报表等 PE 能力早于 3.0 的**确切首引小版本只能给"≤某版本"上界**；官方文档表与 GitHub 存在 1~3 天日期差（本稿以 GitHub API 时间戳为准）；TP 约半数小版本 release 页无文字 notes（能力提炼以有 notes 的版本为准）、release 页不显示年份（2022~2023 老版本年份系依据 docs 博客与 API ISO 日期交叉推断）；TP 企业版闭源无公开 release，其能力仅按官网宣称记录、无法对应版本号。
- 因此，下文矩阵中凡无法落到具体首引版本的行，竞品版本列标注"—（首引版本未逐版核验）"。

### 1.4 证据留痕位置（重要）
活跃项目 `docs/validation/` 现存仅 5 份证据（均为 2026-09-24：downlink-bus-lifecycle-fix、tb-data-converters、tb13-geospatial、tb17-custom-rbac、tp6-device-health-scores）。其余历史证据（P0.x、TB/TP 各行 2026-09-14~09-20 期间产出）**已随 2026-09-20 归档迁出**，现位于：
`archive/iot-docs-and-roadmap-archive-20260920/docs/validation/`（工作区 `projects/archive/` 下）。
本稿所有引用均如实标注实际位置；旧稿引用失真的路径以本稿为准。

**处置决定（2026-09-25 评审后）**：历史证据采用**重链不回迁**——不把归档证据搬回活跃项目，活跃文档引用统一标注归档前缀的实际路径。出口判据：校验脚本扫描 `ROADMAP.md` 与 `docs/roadmap-*.md` 中全部 `docs/validation/` 引用，断言每条在活跃项目或 `archive/iot-docs-and-roadmap-archive-20260920/` 下可解析到实际存在的文件（零失配）。

### 1.5 编号体系说明（避免混淆）
- 本稿矩阵与缺口编号采用**竞品能力清单/缺口跟踪体系**（CORE-xx、TB-01~TB-56、TP-01~TP-25）。
- 旧 ROADMAP 内部编号（P0.x/P1.x/P2.x/P3 及旧 TB-1~TB-19、TP-1~TP-8）是**另一套命名空间，同号不同义**：例如旧 TB-10=Sparkplug B，而缺口体系 TB-10=审计日志；旧 TP-3=多层网关拓扑，而缺口体系 TP-03=TCP 协议接入。旧内部编号仅在 1.4 与第 6 节清理对照中出现。

---

## 2. 状态纪律（保留旧稿三条，收紧 done 定义）

1. **一个任务一个状态块**：状态直接就地改写，不追加层叠历史；详细留痕放 `docs/validation/`（归档证据须引用归档路径）。
2. **状态词仅三个**：
   - `done`：**四面一致**——① API/OpenAPI 契约（端点进入 `docs/openapi/openapi.json`）② 后端实现与鉴权（迁移+服务+路由）③ 前端交互（页面/API 封装落地）④ 自动化测试（可指认的契约/E2E 用例，且最近一次证据记录中运行通过）。四面不齐的不得标 done。
   - `partial`：四面至少一面缺失，或能力仅覆盖子集；必须写明缺失面或阻塞点（环境阻塞型须指认具体阻塞，如真机/商店资质/公网证书）。
   - `pending`：未实现或等待前置立项。
3. **禁止用单一指标宣称完成**：源码存在≠done，测试文件存在≠done，文档记载≠done。
4. **（收紧）运行期结果时效**：本稿凡引用"测试全绿"而行内用例未在本会话复跑的，一律加 **†** 标记——含义为"该结论依据 `docs/validation` 证据文档（含归档）记载，本会话未复跑；Go 侧单测已随本会话全量 `go test`（66 包 ok）覆盖的面不受此限"。**Go 单测豁免仅适用白名单行**（均已实测存在专属 Go 测试文件、随本会话 66 包 go test 覆盖）：TB-28（`pkg/sparkplug/sparkplug_test.go`）、TB-31/TB-36（`calcfield/advanced_test.go`、`engine_test.go`）、TB-34（`pkg/units/units_test.go`）、TB-35（`service/rule_chain_nodes_ai_test.go`）、TP-01（`pkg/pluginsdk/` 四个 `*_test.go`）、CORE-07（`service/conflict_policy_test.go`）；白名单外的 done 行仍须以其引用的契约/E2E 证据支撑。带 † 的行在下次活栈回归前不作为对外承诺口径。
5. **（收紧）引用有效性**：状态块引用的文件路径必须真实存在于所注位置（活跃项目或归档）；旧稿存在的引用失真（见第 6 节）不得再现。

---

## 3. 竞品全版本能力矩阵

> 范围：竞品能力清单（合并版）全部 **46 项**能力（CORE 7 + TB 26 + TP 13），一行一条。缺口能力（25 项已复核 + 17 项未复核）见第 4 节，与本节互补不重叠。
> 状态列仅 done/partial/pending；† 含义见 §2 第 4 条。

### 3.1 CORE 平台基线（7 项）

| 编号 | 能力 | 竞品版本 | 本项目状态 | 证据路径 |
| --- | --- | --- | --- | --- |
| CORE-01 | 数据库版本化迁移与自升级机制 | 通用基线（TB/TP 均具备） | done | `backend/sql/1~138.sql`（连续无缺号；2026-09-25 批次三后由文档更新会话 2026-09-26 实测——138 个 .sql、最小 1/最大 138、逐号 diff 无缺号，与 `global.go:21` VERSION_NUMBER=138 一致；其中 132/133 为批次三编号重排后的 NO-OP 占位迁移，TB-19/TB-23 实际落 135/136；批次二前基线为 1~129=129，见 §1.1）、`backend/pkg/global/global.go:21`（VERSION_NUMBER=138，批次三后实测）、`backend/initialize/pg_init.go`（迁移循环上界） |
| CORE-02 | OpenAPI 契约与接口文档 | 通用基线 | done | `docs/openapi/openapi.json`（本会话实测 483 paths，phase-d-d8a） |
| CORE-03 | 自动化测试矩阵（契约+E2E+演练脚本） | 通用基线 | done† | `automation_tests/`：`tests/` 下 **151 个契约测试文件**（编号前缀去重 98 个，00~101；75/76/77 为 2026-09-25 首批新增，78~86 为批次二新增、87/88/90~96/99~101 为批次三新增，均**待活栈运行**；30 为既有缺号，89/97/98 批次三未占用）+ `e2e/` 下 **32 个 Playwright spec** + 演练/探针脚本（批次三后由文档更新会话 2026-09-26 复测为 151/98）；JS 套件未复跑 |
| CORE-04 | 备份恢复与部署体检脚本 | 通用基线 | done† | 活跃：`deploy/tests/backup-restore-contract.test.sh`；归档：`archive/iot-docs-and-roadmap-archive-20260920/docs/validation/2026-09-19-p01-backup-restore-counts-evidence.md`（pg_dump→psql 恢复→五核心表行数比对 VERDICT=PASS） |
| CORE-05 | 规则链死信队列与单消息回放 | 通用基线 | done† | `backend/sql/111.sql`（`rule_chain_dead_letters`）、`automation_tests/tests/59_rule_chain_reliability.test.js` |
| CORE-06 | 摄取回压与遥测冷热分层 | 通用基线 | done† | `backend/internal/dal/telemetry_rollups.go`、`model/telemetry_rollup.go`、`internal/app/uplink.go`、`automation_tests/tests/67_p23_backpressure_and_tb5_external_dispatch.test.js` |
| CORE-07 | 实体名称冲突策略（五档） | 通用基线 | done† | `backend/internal/model/conflict_policy.go:16-20`（allow/fail/rename/ignore/update，缺省 FAIL）、`automation_tests/tests/58_entity_name_conflict_policy.test.js` |

### 3.2 ThingsBoard 对标（26 项）

| 编号 | 能力 | 竞品版本 | 本项目状态 | 证据路径 | 备注 |
| --- | --- | --- | --- | --- | --- |
| TB-01 | 设备管理与多协议接入 | TB v1.0 起（核心域） | done† | `backend/internal/api/device.go`、`backend/router/apps/device.go`、`automation_tests/tests/11_device_extra.test.js`、E2E `35_p14_h5_mobile_business.spec.js` | 协议深度缺口单列 §4（TB-21/TB-22/TP-03） |
| TB-02 | 多租户与层级隔离 | TB v1.0 起（多租户/客户层级） | done† | 租户边界贯穿各 confirmed 行（如 `backend/internal/dal/operation_logs.go:36,91`、`service/alarm_assignment.go:34-95` 租户边界、`sql/107.sql` tenant_rate_limits）；P0.7 `service/ai_model.go:41,58` AAD 绑定租户 | 结构性能力、依赖子能力测试覆盖、**无本行专属用例**（四面不齐，按 §2 记 done†） |
| TB-03 | 规则引擎（链/节点/脚本执行） | TB v1.0 起（持续演进） | done† | `backend/sql/111.sql`、`automation_tests/tests/59_rule_chain_reliability.test.js`、`service/rule_chain_nodes_external.go`（五外发节点）、`service/rule_chain_nodes_ai.go` | 外发/AI 节点分别对应本表 TB 行的 done 结论 |
| TB-06 | 设备认证与传输加密（X.509/TLS/凭证） | —（首引版本未逐版核验） | done† | `mqtt-broker/plugin/aetherlink/hooks_auth.go:28-97`（MQTT 凭证鉴权+IP 失败限流）、`sql/8.sql`（template_secret 一型一密）、`service/device_pre_register.go`（一次性凭证）、`service/edge_node_service.go`（X.509）、`automation_tests/tests/11_device_extra.test.js` | CoAP/LwM2M 侧 DTLS 缺失归 §4 TB-22 |
| TB-07 | IoT 网关与多层网关拓扑 | TP v1.1.10（多层拓扑口径） | done† | `backend/internal/model/gateway.go:29-36`（嵌套网关载荷）、`service/command_gateway_payload.go:31-42`（递归上溯，maxDepth=10）、`automation_tests/tests/51_multilayer_gateway.test.js` | 旧稿"5 层递归"口径更正为 maxDepth=10 |
| TB-08 | 资产管理与实体关系 | —（首引版本未逐版核验） | done† | `backend/internal/api/entity_relation.go`、`backend/internal/calcfield/relation_resolver.go`、`service/asset.go`（资产服务）、`automation_tests/tests/46_entity_relations.test.js`（关系边四面：校验/链路/幂等/租户边界） | |
| TB-09 | 告警引擎（生命周期/传播/确认清除） | TB v1.0 起（生命周期基线） | done† | `backend/sql/101.sql、103.sql、105.sql`（评论/指派/清除生命周期）、`calcfield/`（types/types.go、advanced_test.go）、`automation_tests/tests/52_alarm_rules_advanced.test.js` | "52 组 18/18"为文档记载未复跑 |
| TB-12 | 设备批量导入与认领（CSV/Claiming） | —（首引版本未逐版核验） | done† | `service/device_pre_register.go:220-299`（坏行 100006/100007）、`service/device_preregister_credential_grant.go`（一次性凭证）、`sql/110.sql`、`adapter/mqttadapter/claim.go:17`（`v1/devices/me/claim`）、`sql/112.sql`、tests/61+66、`e2e/31_tb12_device_claim.spec.js` | |
| TB-13 | 消息队列与隔离队列 | TB v3.6+（队列隔离；4.3 增强，沿用旧稿口径） | done† | `backend/internal/isolatedqueue/manager.go:29,44-56`（Main/HighPriority/SequentialByOriginator）、`sql/107.sql`、`middleware/tenant_rate_limit.lua`、`ratelimit/clustered_rate_limit.lua`、`automation_tests/tests/54_queue_isolation_clustered_rate_limit.test.js` | **表述更正**：Lua 实为（多）固定窗口计数器，非 ZSET 滑动窗口；HTTP 429+Retry-After 契约属实 |
| TB-14 | OAuth2/SSO 单点登录 | —（首引版本未逐版核验） | done† | `backend/internal/api/oidc_sso.go`、`backend/router/router_init.go:190-192`（sso/providers、:id/start、:id/callback） | 本会话仅核验路由与服务存在，四面证据未收齐 |
| TB-16 | 实体数据查询 API（过滤器/聚合） | —（首引版本未逐版核验） | done† | `backend/internal/service/telemetry_analysis.go`、`telemetry_analysis_anomaly.go:86-87`（均值±Kσ）、`automation_tests/tests/60_telemetry_analysis.test.js`（POST /telemetry/analysis 与 /export） | |
| TB-20 | OTA 固件/软件更新 | TB v2.4 起（OTA 能力，沿用旧稿口径） | done† | `backend/internal/dal/ota_rollout_governance.go`（canary/灰度治理）、`internal/api/ota.go`、`automation_tests/tests/32_ota_runtime.test.js`（API→MQTT→task-detail 两用例） | 运行期结论依据归档证据 |
| TB-24 | 双因素认证（2FA） | —（首引版本未逐版核验） | done† | `backend/internal/api/user_totp.go`、`backend/internal/dal/user_totp.go`（含 _test.go）、`backend/router/router_init.go:189`（login/totp）、`backend/router/apps/user_totp.go:14-18`（setup/activate） | 本会话仅核验源码/路由存在，四面证据未收齐 |
| TB-26 | 通知系统（通知中心/多渠道） | —（首引版本未逐版核验） | done† | `backend/internal/service/notification_channels_d2.go`、`notification_execution.go`、`alarm_notification.go:95-145`（邮件）、`notification_sms_aliyun_d2.go`（阿里云短信）、通知组表（MEMBER/EMAIL/SMS/VOICE/WEBHOOK 目标类型） | EMAIL/SMS 实证；VOICE/WEBHOOK 通道未逐项核验 |
| TB-28 | Sparkplug 协议支持 | 传输层规范（Sparkplug B） | done | `backend/pkg/sparkplug/sparkplug.go`（纯标准库 wire-format：`:45-46` NDATA/DDATA、`:343` DecodePayload、`:415` decodeMetric、防假零）、`sparkplug_test.go` | Go 单测已随本会话 go test 通过；源码自注"未与真实 Sparkplug 设备联调"，57 组契约测试未复跑 |
| TB-29 | SCADA 仪表板与符号库 | TB PE SCADA（≤3.x 上界口径） | done† | `frontend/src/views/scada/core/symbolLibrary.ts`（代码内符号库）、`views/visualization/scada-editor/`、`sql/88.sql`（scada_projects/documents/control_audits）、`automation_tests/e2e/34_scada_editor.spec.js`（符号面板/乐观并发/版本发布） | 旧稿引用文件名 `34_scada_canvas_editor.spec.js` 有误，实际为 `34_scada_editor.spec.js` |
| TB-31 | 计算字段 | TB 4.3 LTS（#13857，沿用旧稿口径） | done† | `backend/internal/calcfield/types/types.go:75,90,181-200`（sum/avg/min/max/count）、`calcfield/relation_resolver.go`、`automation_tests/tests/50_calculated_field_relations.test.js` | calcfield Go 单测已随本会话 go test 通过；50 组契约未复跑 |
| TB-33 | 地图与地理围栏 | TB v4.0（New Maps，沿用旧稿口径） | done† | `backend/sql/119.sql`、`backend/internal/service/device_location.go`（含 _test.go）、`automation_tests/tests/72_geospatial_map_tracking.test.js`、`docs/validation/2026-09-24-tb13-geospatial-map-tracking-evidence.md`（项目内）；地理围栏：`backend/internal/calcfield/advanced.go:25,134-135`（FieldTypeGeofence/EvaluateGeofence，圆形/多边形）+ `calcfield/advanced_test.go:26-102`（随本会话 66 包 go test 通过） | 地图追踪（72 号用例+证据文档）与规则级围栏（Go 单测）均有实证；地图部件侧的围栏 UI 交互未单独核验 |
| TB-34 | 遥测单位换算 | TB v4.1.0（沿用旧稿口径） | done† | `backend/pkg/units/units.go`（12 维量纲、registry 60 单位+16 别名、ToBase/FromBase :65-68 带 Offset）、`backend/internal/dal/unit_resolver.go:1,14`（两跳源单位解析）、`sql/108.sql`、`automation_tests/tests/55_units_conversion.test.js` | pkg/units Go 单测已随本会话 go test 通过 |
| TB-35 | AI 规则节点与模型管理 | TB v4.2.0（沿用旧稿口径） | done† | `backend/internal/service/rule_chain_nodes_ai.go`（141 行：ai.inference :24、配置校验 :30-40、handler :45+、未配置 fail-fast、30s 超时）、`service/ai_model.go:41,58`（租户 AAD 加解密）、`:67-75`（aiModelMasked 脱敏）、探针 `automation_tests/tests/65_ai_llm_real_egress.test.js` | service 包 Go 单测已随本会话 go test 通过；65 号探针未复跑 |
| TB-36 | 告警规则 2.0 | TB 4.3 LTS（#14036，沿用旧稿口径） | done† | `backend/internal/calcfield/types/types.go:54-59`（多严重度 AlarmRules+ClearRule+Propagate）、`calcfield/alarm_eval.go:32-46`（求值）、`dal/alarm_rules_cf.go:21`（EscalateAlarmHistorySeverity L→M→H） | 与 TB-09 共用证据基线 |
| TB-37 | API 密钥（编程接入） | —（首引版本未逐版核验） | done† | `backend/internal/api/open_api_keys.go`（/api/v1/open/keys CRUD，租户边界+权限下沉 service） | 本会话仅核验源码存在，四面证据未收齐 |
| TB-44 | 秘密存储（Secrets Storage） | TB PE 专属（沿用旧稿口径） | done† | `backend/sql/109.sql`（sys_secrets、AES-256-GCM 信封、重加密权限）、`backend/pkg/secrets/envelope.go:87-114`（Seal 带 AAD）、`:118-144`（Open 强制 AAD 一致）、`automation_tests/tests/56_secrets_storage.test.js` | |
| TB-50 | 客户自助注册/自助开通 | —（首引版本未逐版核验） | done† | `backend/sql/116.sql`（租户自助开通）、`sql/117.sql`（计费模型与用量计量）、`cmd/aetherlink-cli`、`automation_tests/tests/70_p3_billing_and_usage_metering.test.js` | 计费"实时计量"的执法缺口见 §4 TB-17 |
| TB-51 | 解决方案模板 | CE 原生（沿用旧稿口径） | done† | `backend/sql/113.sql、114.sql`、`automation_tests/tests/62_industry_solution.test.js`、`e2e/32_tb19_industry_solution.spec.js` | 运行期结论依据归档证据 |
| TB-57 | 客户实体（Customer）与设备分配 | TB v1.0 起（Tenant→Customer→Device 核心层级） | partial | `backend/sql/122.sql`（customers/customer_devices/菜单种子）、`backend/internal/{api,service,dal}/customer.go`、`frontend/src/views/customer/list/index.vue`（菜单/国际化/路由齐）、OpenAPI 5 端点、契约测试 `tests/75_customer_management.test.js` | 四面中仅"契约测试运行面"缺（无 docker）；DAL 已含分配即移动与设备归属租户校验，前端 vitest 7 例全绿 |
| TB-54 | 自研 MQTT Broker（TBMQ 对标） | TB TBMQ（独立产品线） | done | `mqtt-broker/`（GMQTT 内核 + 插件体系）、`mqtt-broker/cmd/gmqttd/`（TLS 启动项）、`mqtt-broker/plugin/aetherlink/`（鉴权/持久化）、`plugin/federation/`（Serf+gRPC 集群插件，默认关闭） | broker 集群能力代码在、部署未启用，见 §4 TB-11 |
| TB-04 | 部件库/部件包（Widget Bundles：部件 CRUD/导入导出/市场分发） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/123.sql`（widget_bundles+Casbin+菜单种子）、`backend/internal/{model,dal,service,api}/widget_bundle*`、`router/apps/widget_bundle.go`、`service/resource_center.go`（打包/验签/导入/一键应用接入 widget_bundle）、`frontend/src/views/visualization/widget-bundles/index.vue`、契约测试 `tests/78_widget_bundles.test.js`（17 例）、`docs/validation/2026-09-25-roadmap-remediation-batch2-evidence.md` | 2026-09-25 批次二交付（缺口体系 TB-04）；画布运行时改为从 bundle 加载部件未接线（内置部件行为不回归），运行时闭环列后续项 |
| TB-10 | 实体级审计日志（动作/实体类型/实体 ID/状态码） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/127.sql`（operation_logs 增 action/entity_type/entity_id/status_code+partial index）、`middleware/operation_entity.go`（自脱敏路径解析器，+4 单测）、`api/operation_log.go`（筛选与新列）、`views/system-management-user/system-log/index.vue`、契约测试 `tests/82_operation_entity_audit.test.js` | 2026-09-25 批次二交付（缺口体系 TB-10）；127.sql 前存量行新列 NULL 不回填；entity_id 仅认 UUID 形态第二段 |
| TB-15 | 时序数据保留策略（TimescaleDB 原生 retention+冷层清理；批次三 TB-15R 扩行级 TTL） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/initialize/timescale_retention.go`（add_retention_policy+set_integer_now_func，+7 单测）、`sql/57.sql`（"压缩≠保留"口径修正）、`dal/telemetry_rollups.go`（冷层同边界分批清理）、`service/datapolicy.go`、契约测试 `tests/84_data_policy.test.js`（7 例）；批次三 TB-15R：`sql/138.sql`（行级 data_policy+部分唯一索引 uq_data_policy_row_level）、`dal/data_policy_scope.go`（档案>租户>全局三级覆盖清理，热冷层同口径）、`api/data_policy.go`（POST/DELETE 行级端点）、`views/management/setting/components/data-clear-setting.vue`（作用域列+行级策略弹窗）、契约测试 `tests/100_row_level_data_policy.test.js`（11 例） | 2026-09-25 批次二交付（缺口体系 TB-15）；drop_chunks 真删与 jobs 守卫需活 TimescaleDB 验证（本机无活栈）→ partial†；alarm_info 未挂 retention；档案/租户粒度 TTL 已由批次三 TB-15R 交付（行级仅设备数据 data_type=1；TimescaleDB 行级动态 retention job 明确不做，行级一律走分批 DELETE；契约测试已于 2026-09-26 活栈通过） |
| TB-17 | 租户 API 日配额执法（计量+429 执法+配额查询） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/124.sql`（api_usage_daily）、`internal/quota/`（decision/meter/limit/service+api_daily_incr.lua，Go 单测 22 例，复核实测）、`middleware/tenant_rate_limit.go`（429+Retry-After）、`api/billing.go`（GET /billing/api-quota）、`views/billing/api-quota/index.vue`、契约测试 `tests/79_billing_api_quota.test.js` | 2026-09-25 批次二交付（缺口体系 TB-17，API 维度）；transport 维度（broker/网关调用点）仍缺——批次三 TB-17R 已交付（2026-09-26 终审更正：收尾全量门禁四绿含其改动；见 §9 与批次三证据文档 §1.15） |
| TB-18 | 设备 Profile 档案级默认规则链 | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/125.sql`（device_configs 增 UUID 外键列）、`service/rule_chain_effective.go`（档案链优先+租户链兜底+稳定去重，+5 单测）、`api/rule_chain.go`（GET /rule-chains/device-effective/:deviceId）、`views/device/config-detail/modules/setting-info.vue`、契约测试 `tests/80_device_profile_rule_chain.test.js`（7 例） | 2026-09-25 批次二交付（缺口体系 TB-18）；默认队列档案维度化与告警规则 Profile 化按指示不做；解析失败/跨租户/停用回落租户级链（fail-open 到租户级执行） |
| TB-21 | 边缘本地规则执行器（scoped v1） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/internal/edgerules/`（graph/snapshot/eval/executor，Go 单测回报 32 例，零 DB/broker 依赖）、`cmd/edgemqttbroker/main.go`+`edgerules.go`（-edgerules 默认关闭）、`cmd/edgemqttbroker/README.md`、契约测试 `tests/86_edge_sync_snapshot_contract.test.js` | 2026-09-25 批次二交付（缺口体系 TB-21）；断云期间本地动作与云端告警去重收敛演练需活栈+边缘环境未做；v1 仅 trigger.telemetry/filter.threshold/action.alarm 子集，其余类型求值 fail-closed |
| TB-25 | 实体版本控制差异对比（快照语义 diff） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/128.sql`（Casbin 三角色登记）、`service/entity_version_diff.go`（DiffEntityVersionSnapshots 递归 JSON 语义 diff，+8 单测）、`api/entity_version.go`（GET /api/v1/entity_versions/:id/diff/:target_id）、`views/management/entity-version/index.vue`（对比视图）、契约测试 `tests/83_entity_version_diff.test.js`（4 例） | 2026-09-25 批次二交付（缺口体系 TB-25，diff 面）；Git 仓库后端（branch/commit 语义）按本项明确不做，快照模式仍为唯一后端；路径按 router 既有复数资源命名（任务书原文单数，casbin/契约测试/catalog 三处登记一致）；契约测试已于 2026-09-26 活栈通过 |
| TB-27 | 告警 SLA 计时与超时升级 | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/126.sql`（alarm_config.sla_hours、alarm_history.sla_due_at/sla_breached）、`service/alarm_sla.go`+`dal/alarm_sla.go`（5 分钟 cron 扫描超时升档 L→M→H，N 不动，remark 追加 sla_escalation 审计）、`initialize/croninit/cron.go`、前端配置页 SLA 设置+历史超时标记、契约测试 `tests/81_alarm_sla.test.js`（8 例） | 2026-09-25 批次二交付（缺口体系 TB-27）；cron 真实时钟升级行为未在活栈契约验证（判定逻辑由 alarm_sla_test.go 纯函数单测锚定）→ partial†；details 结构化 JSONB 迁移按指示不做（remark JSON 兼容读取）；历史告警不回填 sla_due_at |
| TB-41 | 文件存储与媒体库管理 | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/129.sql`（media_files，UNIQUE(tenant_id,file_path)+Casbin+菜单）、`service/media_library.go`、`api/media_library.go`（/media/files 列表/详情/删除，删除前实时引用扫描 fail-closed）、`api/upload.go`（UpFile 落盘即登记）、`views/media/library/index.vue`、契约测试 `tests/85_media_library.test.js` | 2026-09-25 批次二交付（缺口体系 TB-41）；部件内嵌图片选择器与邮件附件外发按边界不做；存量 ./files 不回填登记；v1 引用统计为读时 LIKE 扫描 |
| TB-11 | K8s/Helm 编排与多副本部署 | —（首引版本未逐版核验） | partial† | `deploy/helm/aetherlink/`（chart：backend/broker StatefulSet+frontend Deployment+九 Service/CM/Secret）、`deploy/docker-compose.replicas.yml`（双副本 override+三条硬前提）、`backend/internal/helmchart/render.go`+`manifests_check_test.go`（渲染校验 5 用例，env 键/端口/探针与 compose 机械对齐）、`deploy/README.md` K8s 章节、专项证据 `docs/validation/2026-09-26-tb11-k8s-helm-multireplica-evidence.md` | 2026-09-25 批次三交付（缺口体系 TB-11）；本项无 REST 契约面（helmchart 零 HTTP 接线）故无契约测试落号，partial† 原因为真实集群安装/双副本"会话不粘卷、吊销跨 broker 收敛"演练未做（无 helm/kubectl/docker，渲染面通过≠安装面通过）；broker federation 不接线（默认关） |
| TB-19 | 通用 Protobuf 载荷编解码（proto 上传+动态解码+Dry-Run） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/135.sql`（data_converters.proto_schema；首轮落 132.sql，批次三编号重排改挂，132 转 NO-OP 占位）、`backend/internal/service/data_converter_protobuf.go`（protoparse/dynamicpb 动态解码）、`frontend/src/views/device/converter/index.vue`（PROTOBUF 模式+示例 schema）、契约测试 `tests/91_protobuf_converter.test.js` | 2026-09-25 批次三交付（缺口体系 TB-19）；Sparkplug 专用解码边界不变 |
| TB-22 | SNMPv3 接入与多客户端 store 隔离（LwM2M/SNMP/CoAP 传输深度子项） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/internal/snmp/usm.go`（SNMPv3 USM RFC 3414 最小安全层，认证协议 fail-closed）、`backend/internal/collector/snmp.go`（v3 接入采集链路）、契约测试 `tests/92_snmpv3_pointconfig.test.js` | 2026-09-25 批次三交付（缺口体系 TB-22 子项：SNMPv3 authNoPriv+per-endpoint ObjectStore 隔离+两个存量协议缺陷修复）；LwM2M DTLS(PSK) 与 observe 服务端集成仍未做 |
| TB-23 | 移动应用中心（bundle/版本/发布管理） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/136.sql`（mobile_app_bundles；首轮落 133.sql，批次三编号重排改挂，133 转 NO-OP 占位）、`backend/internal/{model,dal}/mobile_app_bundle.go`、`backend/internal/{service,api}/mobile_app_center.go`（状态机八端点）、`frontend/src/views/mobile-app/app-center/`、契约测试 `tests/93_mobile_app_center.test.js` | 2026-09-25 批次三交付（缺口体系 TB-23）；uniapp 工程在仓库外 `active/mobile-app-uni`（跨仓边界同 §4.1 口径） |
| TB-45 | 集成连接器与数据转换器框架（统一 Integration 实体纳管管线） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/130.sql`（integrations 表+Casbin/菜单种子）、`backend/internal/{service,api}/integration.go`、`backend/router/apps/integration.go`（四层 CRUD）、OPC UA 采集路径上行转换钩子、`frontend/src/views/integration/list/`（统一集成管理页）、契约测试 `tests/87_integration_management.test.js` | 2026-09-25 批次三交付（缺口体系 TB-45） |
| TB-46 | 用户组与组权限（GPE v1：组共享 fail-closed） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/131.sql`（三表+Casbin+组共享 scope）、`backend/internal/{dal,service,api}/user_group.go`、`frontend/src/views/management/user-group/index.vue`、契约测试 `tests/88_user_group_management.test.js` | 2026-09-25 批次三交付（缺口体系 TB-46）；customer 用户不接入（v1 边界，同 §4.1 验收点） |
| TB-47 | 白标（租户翻译覆盖 + 自定义 CSS） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/134.sql`、`backend/internal/{model,dal,service,api}/tenant_whitelabel.go`（+_test）、`backend/router/apps/tenant_whitelabel.go`、`frontend/src/locales/whitelabel-override.ts`（+__tests__）、`frontend/src/store/modules/{sys-setting,route,auth}/index.ts`、`frontend/src/views/management/setting/components/branding-setting.vue`、契约测试 `tests/94_whitelabel_overrides.test.js` | 2026-09-25 批次三交付（缺口体系 TB-47，第二阶段收口）；内置菜单改名/隐藏/按角色明确不做；overrides 为登录后可读端点（无匿名获取面） |
| TB-48 | 统一事件调度器（三源聚合 + 日历 UI + 注册面 CRUD） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/sql/137.sql`（scheduler_events 注册面+Casbin/菜单种子）、`backend/internal/{model,scheduler_event,dal/scheduler_event,service/scheduler_events,api/scheduler}.go`（三源只读聚合+注册面 CRUD，scene 事件同事务落既有 timer 机制）、`frontend/src/views/scheduler/calendar/index.vue`（月视图日历+来源过滤+管理弹窗）、契约测试 `tests/95_scheduler_events.test.js`（12 例） | 2026-09-25 批次三交付（缺口体系 TB-48）；三套存量调度不迁移（聚合只读面）；注册面 v1 cron 统一 UTC；聚合内存归并单源 2000 行封顶 |
| TB-49 | 报表 HTML/PDF 渲染与 SMTP 附件投递 | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/internal/model/report_schedule.go`+`report_schedule_http.go`（format 放开 csv/html/pdf）、`backend/internal/service/report_render.go`（html/template+go-pdf/fpdf，+10 单测）、`report_smtp.go`（gomail 全格式附件化）、`backend/go.mod:16,34`（fpdf/gomail 实测在册）、`frontend/src/views/visualization/report/index.vue`（CSV/HTML/PDF NSelect）、契约测试 `tests/96_report_schedule_format.test.js`（6 例） | 2026-09-25 批次三交付（缺口体系 TB-49）；无新迁移（78.sql 仅注释同步口径）；富样式 PDF/图表嵌入明确不做，PDF 核心字体 Latin-1 超集降级 '?' |

> 注：上表末尾 TB-04/10/15/17/18/21/25/27/41 九行系 2026-09-25 批次二按缺口体系（§4.1）**新增的对标行**——不在上文"46 项竞品能力清单"原始范围内（TB-57 同此口径），为使四面状态可查而增补；竞品版本列未做调研、按 §1.3 口径统一标"—（首引版本未逐版核验）"；状态均为 **partial†**（四面缺契约测试运行面），证据见 `docs/validation/2026-09-25-roadmap-remediation-batch2-evidence.md`。
> **批次三注记（2026-09-25）**：上表末尾另增 TB-11/19/22/23/45/46/47/48/49 九行、§3.3 增 TP-03/20/21/22 四行，系批次三按缺口体系新增的对标行（口径同上：竞品版本列"—（首引版本未逐版核验）"、状态 **partial†**——契约测试未运行或无活栈验证面）；TB-15 行由批次三 TB-15R（行级 TTL）交付后同步更新；TB-17 行注记批次三 TB-17R（transport 维度）已交付。证据见 `docs/validation/2026-09-25-roadmap-remediation-batch3-evidence.md`。

### 3.3 ThingsPanel 对标（13 项）

| 编号 | 能力 | 竞品版本 | 本项目状态 | 证据路径 | 备注 |
| --- | --- | --- | --- | --- | --- |
| TP-01 | 插件化协议接入框架与插件 SDK | TP（tp-protocol-sdk-go v1.0.0~v1.2.8） | done | `backend/pkg/pluginsdk/`（共 2283 行：can_adapter.go 387 行、bacnet_adapter.go 373 行、signing.go:63-81 Ed25519）、`backend/router/apps/plugin_registry_http_test.go`（真实 Gin+PG 注册路径） | Go 测试已随本会话 go test 通过 |
| TP-02 | 物模型（遥测/属性/事件/命令建模） | —（首引版本未逐版核验） | done† | `backend/sql/1.sql:161`（device_templates）、`backend/internal/uplink/bus.go:29-41`（telemetry/attribute/event/status/*_response/shadow_ack 消息类型）、`api/device_templates.go:19` | 结构性能力、依赖子能力测试覆盖、**无本行专属用例**（四面不齐，按 §2 记 done†） |
| TP-06 | 场景联动规则引擎 | —（首引版本未逐版核验） | done† | `backend/internal/service/scene_execution_window.go:5,54,75`（[starts_at, expires_at) 左闭右开 + IsExpired）、`automation_tests/tests/31_scene_action_20_runtime.test.js` | 运行期结论依据归档证据 |
| TP-07 | 组态可视化（ThingsVis 引擎/3D/组态编辑器） | TP（thingsvis v1.0.1~v1.0.23） | done† | `frontend/src/views/visualization/`（native-board-editor、scada-editor）、`deploy/docker-compose.optional-integrations.yml:44-81`（thingsvis-server/studio 可选镜像）、`backend/internal/service/scada_mobile_wiring.go:83-130`（builtin gauge/chart/valve/twin3d） | ThingsVis 引擎本体为外部镜像，仓库内为宿主桥接 |
| TP-08 | 资源中心/模板市场 | TP v1.2.8（沿用旧稿口径） | done† | `backend/sql/106.sql`、`automation_tests/tests/53_resource_center_market.test.js`、`e2e/29_p16_template_upgrade_rollback.spec.js`、`service/device_template_market_integrity.go`（HMAC 完整性） | |
| TP-09 | 国际化（多语言） | —（首引版本未逐版核验） | done† | `frontend/src/locales/langs/`（en-us / es-es / fr-fr / zh-cn 四语言目录） | 自定义翻译（租户级覆盖）缺失，归 §4 TB-47 |
| TP-10 | 设备诊断与调试（连接诊断/调试日志/主题映射） | TP v1.1.11（沿用旧稿口径） | done† | `backend/internal/api/device_topic_mapping.go`、`dal/device_topic_mapping.go`、`automation_tests/e2e/25_tp4_device_diagnostics.spec.js` | |
| TP-11 | 手机号登录 | —（首引版本未逐版核验） | done† | `backend/internal/api/sys_user.go:58-60`（Phone 登录类型分支）、`service/notification_sms_aliyun_d2.go`（短信验证码通道） | 本会话仅核验源码存在，四面证据未收齐 |
| TP-12 | 系统监控 API（CPU/内存/磁盘） | —（首引版本未逐版核验） | done† | `backend/internal/api/system_monitor_api.go`、`backend/internal/api/queue_monitor.go:12,29-43`（队列监控；实测三队列名于 ：29-43，文件共 51 行） | 本会话仅核验源码存在，字段级覆盖未逐项核验 |
| TP-13 | 租户级设备在线趋势 | —（首引版本未逐版核验） | done† | `backend/internal/api/board.go:346`（GetDeviceTrend 设备在线趋势）、`frontend/src/service/api/system-data.ts:242`、`internal/api/device_status_ws.go`（在线状态 WS） | |
| TP-14 | Prometheus 监控集成 | —（首引版本未逐版核验） | done† | `backend/pkg/metrics/metrics.go`、`deploy/observability/`（README + telemetry-spool/attribute-event-spool 告警规则 yml） | 抓取端到端未复跑 |
| TP-18 | All-in-One 一键安装器 | TP（thingspanel-installer v1.1.13.6~v1.2.12） | done† | `deploy/README.md`（单节点私有部署 bootstrap）、`deploy/init.sh`、`deploy/package.ps1`/`package.sh`、根 `docker-compose.yml`、`start-aetherlink.*` | 本会话未实际复装 |
| TP-24 | 总后台系统管理（超管看板/数据清理） | TP 企业版宣称 | done† | `backend/internal/service/datapolicy.go:69-123`（CleanSystemDataByCron）、`initialize/croninit/cron.go:50-54`（每日 2 点）、`frontend/src/views/management/setting/components/data-clear-setting.vue`、`sql/118.sql`（RBAC 权限面） | |
| TP-03 | TCP 协议接入（门控入站 TCP 网关） | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `backend/internal/protocolgw/tcp/`（framing/registry/gateway/ingest/downlink 及配套 _test.go）、`backend/internal/app/tcp_gateway.go`（网关装配）、契约测试 `tests/90_tcp_gateway.test.js` | 2026-09-25 批次三交付（缺口体系 TP-03，第一程）；帧解析/注册簿/断网缓冲+设计文档 |
| TP-20 | 国产 DB 方言适配层 | —（首引版本未逐版核验） | partial† | `backend/internal/dialect/`（纯函数方言包，dialect_test.go 7 个测试函数/101 表驱动用例）、`docs/deployment-domestic-db.md`（国产化部署章节） | 2026-09-25 批次三交付（缺口体系 TP-20，第一程）；partial† 原因：国产库驱动未进 go.mod（更新会话实测 tdengine/kingbase 0 命中）、真实国产库迁移+读写未连通验证；外部 tp_to_db gRPC 开关维持现状 |
| TP-21 | 设备健康算法中心（MSET 训练/推理/降级） | —（首引版本未逐版核验） | partial† | `backend/internal/healthmset/`（mset/align/chi2/linalg，mset_test.go 11+align_test.go 5=16 测试函数）、`backend/internal/service/device_health.go`（健康评分扩展点接线，默认关）、`frontend/src/views/device/details/modules/health-assessment/index.vue`（MSET 维度渲染实测） | 2026-09-25 批次三交付（缺口体系 TP-21，第一程）；契约测试未落号（算法面由 Go 单测覆盖）；partial† 原因：预测性维护指标（剩余寿命/劣化趋势）未在交付摘要内、评分扩展点默认关未经活栈启用验证 |
| TP-22 | 可视化大屏固定分辨率与轮播投屏 | —（首引版本未逐版核验） | done（2026-09-26 活栈契约通过） | `frontend/src/views/scada/core/canvasDocument.ts`+`useCanvasEditor.ts`（displayMode fixed1080 自动哨兵 1920×1080、computeFitScale+ResizeObserver 等比适配）、`frontend/src/views/visualization/thingsvis-preview/{index.vue,carousel.ts}`（/tv-preview 多屏轮播 tokens+interval+Fullscreen）、`backend/internal/scadadoc/scadadoc.go`（壳层契约）+`api/board_carousel.go`（share token 批量解析）、契约测试 `tests/99_tp22_canvas_display_mode.test.js` | 2026-09-25 批次三交付（缺口体系 TP-22，第二程）；DataRoom 引擎收编明确不做；interval 为纯展示参数（clamp [3,3600]）；legacy provider 不支持批量轮播；全屏需用户手势 |

---

## 4. 真实缺口与计划

> 来源：独立复核后确认的 25 项缺口，其中 24 项成立（TB-05 经复核**证伪**，见 4.4），按优先级分组：
> - **P0** = high 且 missing（1 项）
> - **P1** = high 且 partial（5 项）+ 全部 medium（18 项）
> - **P2** = 未逐项复核的 17 项 low 缺口（仅列观察清单，实施前须先补核验，见 4.3）
>
> **统一验收口径（四面一致，适用于每一条）**：① 新增/变更端点进入 `docs/openapi/openapi.json` 并重生成（基线 483 paths）；② 后端实现+迁移（`backend/sql/123+`）+Go 单测，随 `go test ./...` 全绿；③ 前端页面/交互落地且 `pnpm run typecheck` 通过；④ `automation_tests` 新增契约/E2E 用例并在活栈运行通过。以下各条只写该条特有的验收点。

### 4.0 P0 — high missing（1 项）

> **交付记录（2026-09-25）**：TB-30 已实施——后端 `api/alarm_status_ws.go`（首帧鉴权 + 初始快照 + Redis 扇出转发）与发布侧 `service/alarm_realtime.go`（trigger/recovery/status 三类事件，Go 单测 6 例），前端 `useAlarmPush.ts` 改 WS 订阅（带降级轮询）、告警列表页经 `useAlarmStatusSocket.ts` 实时刷新；契约用例 `tests/76_alarm_realtime_ws.test.js` 待活栈运行。状态：**missing → partial（缺运行面）**。

| 编号 | 缺口 | 建议方案 | 涉及模块 | 工作量 | 本条特有验收点 |
| --- | --- | --- | --- | --- | --- |
| TB-30 | 告警状态实时 WebSocket 订阅（现状：ThingsVis 告警推送 hook 30s REST 轮询——`useAlarmPush.ts:52` 实测 `setInterval(fetchAlarmStatus, 30000)`，后端仅遥测+设备在线 4 条 WS 路由，告警触发链路无实时通道） | 复用遥测 WS 通道，新增 alarm 状态订阅主题与初始快照推送；告警触发点（AlarmExecute/通知）同步发布 | 后端：`router/router_init.go:199-205`（现仅 4 条 WS 路由）、`api/telemetry_ws_stream.go:33,40`、`service/alarm_execution.go:46-65`、`pkg/global/SSEManager.go:144-152`；前端：`hooks/thingsvis/useAlarmPush.ts:16,52`（改轮询为订阅）、`hooks/thingsvis/useRealtimePush.ts:36-42` | M | WS 订阅含租户隔离鉴权与断线重连后的初始快照；ThingsVis 告警推送 hook 与告警列表页实时刷新替代 30s `setInterval` 轮询；契约用例覆盖订阅→触发→推送→快照回放 |

### 4.1 P1 — high partial（5 项）+ medium（18 项）

> **交付记录（2026-09-25 批次二）**：TB-04、TB-17、TB-18、TB-27（high partial 组，TB-46 未动）与 TB-10、TB-15、TB-21、TB-25、TB-41（medium 组）共 **9 项已实施交付**（见各行编号后的 ✅ 标注与 §9 批次二实施记录）。各实施会话自报门禁均为 go build + 定向 go test + vue-tsc 全过；批次收尾门禁 openapi=0、goTest=0、vue-tsc=0、vitest=0（编排方统一收尾口径）。**契约测试 78~86 已就位、按约定未在活栈运行（本机无 docker）**，按 §2 四面口径各行记 **partial†**，待活栈回归后升 done。逐项 residual 见 `docs/validation/2026-09-25-roadmap-remediation-batch2-evidence.md`。

> **交付记录（2026-09-25 批次三）**：第一程 TB-45、TB-46、TB-11、TB-22、TB-23、TP-03、TP-20、TP-21 与第二程 TB-47、TB-48、TB-15R（TB-15 行级 residual）、TB-49、TP-22 共 **14 项已实施交付**（迁移链扩至 138.sql；见各行编号后的 ✅ 标注与 §9 批次三实施记录）。各实施会话自报门禁为 go build + 定向 go test + vue-tsc 全过（TB-47/TP-22 经他泳道瞬时编译态按约定重试后第 2 轮全过）；批次三收尾门禁 openapi=0、goTest=0、vue-tsc=0、vitest=0（编排方统一收尾口径，见 §1.2）。**契约测试 87~101（实际落号 87/88/90~96/99/100/101 共 12 个，89/97/98 未占用）已就位、按约定未在活栈运行（本机无 docker）**，按 §2 四面口径各行记 **partial†**，待活栈回归后升 done。**TB-17R（传输维度配额接线）经 2026-09-26 终审更正为 delivered**：单项门禁红系 TP-22 泳道在途编辑的时序归因，其全部代码保留在工作树并已被收尾全量门禁（四绿，含全仓 vue-tsc/vitest）覆盖，TB-17 行 transport 维度子项闭合（见 §9 与证据文档 §1.15）。逐项 residual 见 `docs/validation/2026-09-25-roadmap-remediation-batch3-evidence.md`。

**high partial（5 项）**

| 编号 | 缺口 | 建议方案 | 涉及模块 | 工作量 | 本条特有验收点 |
| --- | --- | --- | --- | --- | --- |
| TB-04 ✅ **已交付 2026-09-25（批次二，契约用例待活栈；范围排除项见 §9 residual）** | 仪表盘可视化与部件库/动态表单（看板/SCADA/动态表单在，**部件库实体不存在**：grep widgets?_bundle 全仓 0 命中，122 个迁移无 widget 表；部件均为代码内置） | 新建 `widget_bundles` 部件库实体与导入导出，挂资源中心分发；内置部件迁移为可管理 bundle | 后端：`internal/model/device_template_market.go:122,137`（资源类型校验扩 widget_bundle）、`service/resource_center.go:65-103,214-235`；前端：`views/visualization/`、`components/local-visualization-viewer/dynamic-form/` | M | 部件 CRUD+导入导出+市场分发四面齐；现有内置部件（gauge/chart/valve/twin3d）经 bundle 加载行为不回归 |
| TB-17 ✅ **已交付 2026-09-25（批次二，契约用例待活栈；范围排除项见 §9 residual；批次三 TB-17R transport 维度接线实施后已回退，该子项仍开放）** | 租户 Profile（配额限速/API 用量监控）——限流已执法 API 维度，**配额仅报告不执法**：`max_api_calls_per_day` 无读取代码、无 API 调用计量持久化、传输维度零调用点、前端无配额页 | API 调用计量与超额执法落 `117.sql` 已有字段；接线 transport 维度；补前端配额/用量页 | 后端：`internal/ratelimit/service.go:198,225,249`（CheckAPI/CheckTenantTransport/CheckDeviceTransport，:197 为注释行；后两者无生产调用点）、`middleware/tenant_rate_limit.go:192-223`、`sql/117.sql:20,42`、`sql/107.sql:9-24`；前端：新增配额页 | L | 每日 API 调用计量持久化并触发 `max_api_calls_per_day` 执法（429 语义）；transport 维度在 broker/网关路径产生真实调用点；前端可见租户用量与配额余量 |
| TB-18 ✅ **已交付 2026-09-25（批次二，契约用例待活栈；范围排除项见 §9 residual）** | 设备 Profile（档案/默认规则链/告警规则）——档案级告警规则与自动注册已间接实现，**缺档案级默认规则链与默认队列绑定**（规则链执行为租户级 enabledGraphsForTenant） | `device_configs` 增档案级默认规则链/默认队列字段；告警规则收敛为 Profile 实体字段（现经场景联动 device_config_id 间接实现） | 后端：`sql/1.sql:970-1007`（device_configs）、`sql/8.sql`（template_secret/auto_register 已有）、`service/automate_telemetry.go:106-110`（档案级触发）、`service/rule_chain.go:368,403`；前端：`views/device/config-detail/` | M | 档案绑定默认规则链后新建设备自动挂接；默认队列按档案生效；存量自动化（device_config_id 触发）回归不破坏 |
| TB-27 ✅ **已交付 2026-09-25（批次二，契约用例待活栈；范围排除项见 §9 residual）** | 告警协作 SLA（分配/评论已完整，**SLA 计时与超时升级缺失**；details 寄生于 text remark 列） | 增 SLA 计时与超时升级（cron 扫描+升级动作）；details 改结构化 JSON | 后端：`sql/1.sql:37-50`（alarm_history 无 due_at/breach 列）、`sql/43.sql:6-11`（remark JSON 现状）、`dal/alarm.go:136`、`service/alarm_assignment.go` | M | SLA 到期自动升级（可配置时限+目标严重度）；结构化 details 迁移后旧 remark JSON 兼容读取 |
| TB-46 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | 实体组与高级 RBAC（组共享/GPE）——设备组树+自定义角色在，**无用户组/GPE 组权限实体，组级共享仅限设备组，看板/资产无分组** | 增用户组与组权限实体（GPE），组级共享授权覆盖看板/资产 | 后端：`dal/device_groups.go:40,313,380`（WITH RECURSIVE 组树，可复用模式）、`sql/118.sql`（sys_permissions/sys_role_permissions）、`service/rdi_share.go:31-97`（现仅逐设备令牌）；前端：`views/device/share`（扩展为组级） | L | **v1 范围（边界已定，非开放设计题）**：租户内用户组与组权限；customer 用户（`sql/122.sql`，无登录账号）**不接入**组授权。在此范围内验收：用户组创建/成员管理/组权限绑定四面齐；组共享对看板与资产生效且租户边界 fail-closed |

**medium（18 项）**

| 编号 | 缺口 | 建议方案 | 涉及模块 | 工作量 | 本条特有验收点 |
| --- | --- | --- | --- | --- | --- |
| TP-05 | 一型一密/一机一密产品级校验（链路已在，**product_key 与 template_secret 无交叉校验**，可用 A 档案密钥+任意 product_key 建档到 B 产品；无产品级密钥） | `device_auth` 补产品级动态注册密钥校验（products.DeviceConfigID 可校未校）与一机一密分支 | 后端：`service/device_auth.go:86-103`（lookupAuthProductID 仅校存在性）、`service/device_auth.go:105-130`（buildAuthDevice）、`sql/8.sql:3-4`、`model/products.gen.go:31` | S | 交叉校验负向用例（错配 product_key 被拒）；产品级密钥分支进入 OpenAPI 与前端档案设置页 |
| TP-03 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | TCP 协议接入（missing：全仓无非 TCP 设备入站监听，仅 HTTP/gRPC/edge broker/MQTT） | 以 pluginsdk 插件形态加原始 TCP 接入（帧解析+断网缓存） | 后端：`pkg/pluginsdk/sdk.go:79`（ProtocolAdapter 契约扩展入站监听）、`internal/app/`（新网关装配）、参照 `internal/edgeforward/forwarder.go`（断网缓冲模式） | M | TCP 设备经插件上行遥测四面齐；断网缓存续传用例；不占用平台默认端口时的部署文档 |
| TB-10 ✅ **已交付 2026-09-25（批次二，契约用例待活栈；范围排除项见 §9 residual）** | 审计日志（实体级）——现有为 HTTP 级操作日志（记录/列表/CSV 导出），**无 TB 式实体级审计（动作/实体类型/状态）** | `operation_logs` 增实体级 action/entity_type/status 字段，对齐 TB 审计模型 | 后端：`sql/1.sql:365`（operation_logs 现仅 12 列）、`middleware/operations_log.go:108-112,289-325`、`api/operation_log.go:23,41`、`service/audit_export.go`；前端：`views/system-management-user/system-log/`（筛选条件扩展） | M | 关键实体（设备/产品/告警/权限）变更产生动作审计；导出含新列；旧数据兼容 |
| TB-11 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | 部署架构与水平扩展——单节点 compose 在，**无 K8s/Helm、无多副本验证、无多 DB**；broker federation 与多 broker 吊销原语已在（默认关闭） | 补 K8s/Helm 编排与多副本部署验证（后端+broker）；federation 从"代码在"到"部署接线" | `deploy/`（现仅单节点 bootstrap）、`docker-compose.yml`（无 replicas 定义）、`mqtt-broker/README.md:40-42`（federation 默认关闭）、`internal/service/mqtt_session_revocation.go:54,162-165,634-637`（多 broker 吊销已备） | L | Helm chart 可装；后端+broker 双副本经演练（会话不粘本地卷、吊销跨 broker 收敛）；单节点默认路径不回归 |
| TB-15 ✅ **已交付 2026-09-25（批次二原生 retention；批次三 TB-15R 补行级 TTL（档案/租户粒度），契约用例待活栈；范围排除项见 §9 residual）** | 时序存储 TTL 粒度——全局按类型 TTL 在，**无档案/租户粒度 TTL**；TimescaleDB hypertable 条件生效但**无原生 retention policy**（grep add_retention_policy 0 命中） | 在 `57.sql` hypertable 路径补 add_retention_policy/drop_chunks；TTL 扩展到设备档案/租户粒度 | 后端：`sql/1.sql:102-121`（data_policy 全局表）、`sql/57.sql:13-31`（hypertable+压缩策略已挂）、`service/datapolicy.go:69-123`、`dal/telemetry_datas.go:326-337`（全局 DELETE）；冷层 `dal/telemetry_rollups.go`（无清理） | M | TimescaleDB 部署下 drop_chunks 生效（压缩≠保留的口径修正）；档案/租户级 retention 覆盖全局默认；普通 PG 路径行为不回归 |
| TB-19 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | 自定义 MQTT 主题与通用 Protobuf 载荷——主题映射+HEX/JSON_PATH/Lua 转换器在，**通用 Protobuf 编解码缺**（proto 上传+动态解码无，Sparkplug 为专用手写解码） | `data_converters` 增 Protobuf 模式（proto 上传+dynamicpb 动态解码） | 后端：`sql/120.sql:10-22`、`service/data_converter.go:176-186`（模式分派点）、`pkg/sparkplug/`（参考实现边界）；前端：`views/device/converter/index.vue:122-125`（modeMap 增项） | M | proto 文件上传/存储/版本化；动态解码用例（含未知字段与类型不匹配 fail-closed）；Dry-Run 仿真支持 PROTOBUF 模式 |
| TB-21 ✅ **已交付 2026-09-25（批次二，契约用例待活栈；范围排除项见 §9 residual）** | Edge 边缘计算（本地规则/自治运行时）——云端管理+云边同步+OTA 分发+数据面断网缓冲在，**无边缘本地规则执行器** | 补边缘本地规则执行与断网自治运行时（复用 cmd/edgemqttbroker） | `cmd/edgemqttbroker/main.go`（自述仅为云侧替身）、`internal/edgeforward/forwarder.go:1-11`（断网缓冲已备）、`service/edge_sync.go:90-99`（现仅投递 Graph JSON 快照无消费方）、`api/edge_node.go` | L | 边缘侧消费规则链快照并本地执行（断云期间产生动作/告警）；恢复后与云端去重收敛；64 号断云演练扩展本地执行场景 |
| TB-22 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | LwM2M/SNMP/CoAP 传输深度——注册簿/TLV/对象模型/遥测汇入在，**缺 DTLS(PSK)、队列模式、多客户端 store 隔离、blockwise/observe 服务端集成、SNMPv3 接入与 trap** | 按"lwm2m 补 DTLS(PSK) 与安全层、SNMPv3 接入采集链路"推进（对象模型已完成，原建议该部分过时） | `internal/lwm2m/`（objects.go/tlv.go/observer.go 已有）、`internal/coap/`（blockwise_observe.go 为纯组件未接服务端）、`internal/collector/snmp.go`（v2c 轮询，`internal/snmp/usm.go` 未接入）、`app/collector.go:70` | L | DTLS-PSK 握手用例；多客户端隔离（去掉 last-wins 共享单 store）；SNMPv3 用户配置进点表并接入 Runner；observe 服务端订阅生效 |
| TB-23 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | 移动应用中心——移动端 API/推送/命令/uniapp 工程在，**无 bundle/版本/发布管理**；另：移动命令幂等存储为进程内存（`service/mobile.go:169`） | 主仓补移动应用中心（bundle/版本/发布管理），对接 uniapp 工程；幂等 store 持久化 | 后端：`api/mobile.go:98-353`（11 端点已挂）、`sql/88.sql:123-148`（push_device_registrations/push_deliveries）、`app/scada_mobile_wiring.go:79-104`（fail-closed 注入）；前端：`views/device-details-app/` | M | 应用 bundle 上传/版本/发布状态机四面齐；`mobile-app-uni` 构建产物对接发布记录；幂等键落库重启不丢。**跨仓边界**：uniapp 工程位于仓库外 `active/mobile-app-uni`，四面中前端面由该仓承担，本仓验收止于发布 API/记录与 H5 构建产物对接 |
| TB-25 ✅ **已交付 2026-09-25（批次二，契约用例待活栈；范围排除项见 §9 residual）** | 实体版本控制（Git）——快照式 entity_versions（board/rule_chain/device_config/calculated_field 白名单）在，**无 Git 集成与差异对比** | entity_version 增 Git 后端适配与版本差异对比 | 后端：`sql/58.sql:9-19`（JSONB snapshot，无 branch/commit 列）、`service/entity_version.go:30-35`（白名单）、`api/entity_version.go:27,51,74,94`、`service/rule_chain_version.go`（draft→published→rollback 栈）；前端：`views/management/entity-version/index.vue`（无 diff 视图） | M | 至少一类实体接 Git 仓库后端（分支/提交语义）；diff 视图（JSON 语义级差异）；快照模式作为无 Git 部署的回退 |
| TB-41 ✅ **已交付 2026-09-25（批次二，契约用例待活栈；范围排除项见 §9 residual）** | 文件存储与媒体库/Files 部件——通用上传+vis_files 表在，**无媒体库管理（浏览/列举/删除）、图片库部件、邮件附件外发**；vis_files 为孤儿表 | 建 files/media 实体与图片库部件，upload 扩展媒体管理；收编 vis_files | 后端：`sql/1.sql:855`（vis_files）、`api/upload.go:52`（仅写入链路，自注不负责列举/删除/清理）、`router/router_init.go:103`（/files 静态服务）；前端：图片库选择器进看板/SCADA 部件 | M | 媒体列表/删除/引用统计四面齐；看板图片部件可选媒体库资产；邮件通知支持附件 |
| TB-45 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | 集成连接器与数据转换器框架——分散连接器（OPC UA/SNMP 采集器、CoAP/LwM2M 网关、modbus-plugin、插件注册）与转换器框架在，**缺 TB 式统一 Integration 实体纳管连接与转换器并执行；转换器未接入任何上行/下行管线**；Pulsar 缺失 | 抽象 integration 实体与统一集成管理页，纳管 collector/插件，转换器接入管线 | 后端：`internal/collector/collector.go:105-134`（Poller 抽象）、`internal/protocolgw/gateway.go`、`service/data_converter.go`（DataConverter 仅被自身 CRUD 栈引用，uplink/processor/downlink 零引用）、`sql/120.sql:4`（注释对标 TB Integrations 但无 integrations 表）；前端：`views/device/service-access`（三方插件接入，非协议集成实体） | L | Integration 实例（连接+上下行转换器绑定）执行进入上行管线；统一集成管理页；至少一个存量采集器（OPC UA）改造为 Integration 实例不回归 |
| TB-47 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | 白标与自定义菜单——白标 7 字段+主题色/favicon 在，**自定义翻译、Advanced CSS 缺失；自定义菜单仅"仪表盘菜单绑定"形态（不能改名/隐藏内置菜单/按角色/自定义图标）** | 补白标全套：自定义翻译、Advanced CSS、内置菜单改名/隐藏/按角色/图标 | 后端：`api/logo.go:27,48`、`sql/59.sql:9-13`（logo 表）、`sql/16.sql:2-25`（tenant_dashboard_menus）、`dal/ui_elements.go:212-267`；前端：`views/management/setting/components/branding-setting.vue`、`store/modules/sys-setting/index.ts:48-71`、`views/visualization/thingsvis-dashboards/index.vue:38,67-75,392` | M | 租户级 UI 翻译覆盖生效且与静态四语言目录并存；Advanced CSS 沙箱化注入（CSP 约束）；内置菜单改名/隐藏按角色生效 |
| TB-48 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | 事件调度器（Scheduler）——场景定时（periodic_tasks）、报表调度、RPC 计划三套独立调度在，**无统一 TB 式 Scheduler 实体与事件日历 UI** | 建 scheduler 实体与事件日历 UI，统一场景/报表/RPC 计划 | 后端：`internal/app/cron_service.go:21-27`、`initialize/croninit/cron.go`、`model/periodic_tasks.gen.go:22`（HOUR/DAY/WEEK/MONTH/CRON）、`api/report_schedule.go:19-82`、`service/fleet_command_jobs.go:104-116`（scheduled_at）、`dal/scene_automation_timer.go:56-104`；前端：无 scheduler 页面（views 13 个目录无日历） | M | Scheduler 事件 CRUD+日历视图四面齐；存量三套调度迁移为事件来源不破坏现网行为；时区语义（scene_automation_timers 已有 timezone）一致 |
| TB-49 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | 报表生成与调度（PDF/CSV）——定时报表骨架（调度/生成/SMTP/outbox/重试）完整，**格式四处锁死 CSV；PDF/HTML 零实现；且 CSV 以正文内联而非附件投递**（`report_smtp.go:158 Body: string(payload)`，无 Attach 调用） | `report_run_processor` 增 PDF/HTML 模板渲染与投递；CSV 改附件 | 后端：`service/report_run_processor.go:142-143`（硬编码仅 csv）、`:17-18`（10 万行/25MB 上限）、`model/report_schedule.go:19,45,59`（oneof=csv）、`sql/78.sql:18`、`service/report_smtp.go:114,159`；前端：`views/visualization/report/index.vue:661`（格式选择框禁用硬编码 CSV） | M | PDF/HTML 模板渲染+SMTP 附件投递用例；格式枚举四处（model/http 模型/SQL 默认/前端）同步放开；行/字节上限对新格式生效 |
| TP-20 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | 国产数据库适配——**外部 tp_to_db gRPC 客户端开关在**（TSDB/KINGBASE/POLARDB），但仓库内无驱动、无 SQL 方言适配、无 TDengine 具名实现，tp_to_db 服务端不在仓库 | go.mod 加 TDengine/KingBase 驱动与 SQL 方言适配层 | 后端：`internal/app/grpc_service.go:34-41`、`third_party/grpc/tptodb_client/init.go:61-86`、`dal/telemetry_datas.go:29-32`、`dal/telemetry_current_datas.go:40-58`（读路径分支已备）、`test/multidb/docker-compose.yml`（仅 MySQL+PG 夹具） | L | 至少一个国产库（TDengine 或 KingBase）驱动进 go.mod 并通过方言适配跑通迁移+读写；`go test ./internal/app ./internal/dal ./third_party/grpc/tptodb_client/...` 不回归（本会话该三包已实测 ok） |
| TP-21 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | 设备健康算法中心（MSET/预测性维护）——健康评分引擎完整在，**MSET/预测性维护零实现**（仅 121.sql:4 对标注释提及） | `device_health` 增 MSET/预测性维护时序算法 | 后端：`service/device_health.go:237-385`（多维扣分/四级阈值，扩展点）、`service/telemetry_analysis_anomaly.go:1-13`（基础异常检测，自注"不做建模不做学习"，可作特征输入）、`sql/121.sql`；前端：`views/device/details/modules/health-assessment/` | L | MSET 模型训练/推理管道（时序特征→偏差评分）与预测性维护指标（如剩余寿命/劣化趋势）进评分；冷启动与数据不足降级路径 fail-closed |
| TP-22 ✅ **已交付 2026-09-25（批次三，契约用例待活栈；范围排除项见 §9 residual）** | 可视化大屏（DataRoom 类）——看板/SCADA 双编辑器+模板市场+发布分享在，**无独立大屏引擎、无多屏轮播/投屏模式**（native-board 为响应式网格非 1920×1080 缩放画布）；ThingsVis 外部镜像可选对接 | scada 画布增独立大屏模式（多屏轮播/投屏）或收编 DataRoom | 前端：`views/scada/core/useCanvasEditor.ts`、`canvasDocument.ts`（schema v1、1MB 上限）、`views/visualization/native-boards/index.vue:289,312,406`（发布/分享 UI 已备）、`router/routes/index.ts:98-106`（/tv-preview 公开路由已备） | M | 大屏画布固定分辨率缩放适配；多屏轮播/自动投屏模式；经 share_token 公开投屏端到端用例 |

### 4.2 P1 补充说明

> **交付记录（2026-09-25）**：TP-05 已实施——`service/device_auth.go` 新增 `ensureAuthProductMatchesConfig`（同租户 + `products.device_config_id` 绑定关系双校验，错误码 200087），契约用例 `tests/77_device_auth_product_key.test.js` 待活栈运行。TP-19 的 OS 面已实施——`backend/Makefile` 新增 `build-linux-{amd64,arm64,loong64}`/`cross-compile` 目标，三架构二进制实测产出（ELF 静态链接）；TP-20（国产 DB 驱动）仍 pending。
- **工作量口径**：S=≤3 人日、M=≤2 人周、L=≥1 个迭代（含测试与文档）。
- **TB-05 证伪记录**（不列入缺口）：缺口清单原列"服务器端 RPC 双向控制 partial"，经独立复核证伪——twoway 语义已以 **direct-method** 命名端到端实现（`service/command_direct_method.go:62-121` 同步请求-响应+五种结局；POST `/api/v1/command/datas/pub` 为 oneway 风格；前端 `views/device/details/modules/public/distribution-and-table.vue:378-403` 有 waitForResponse 开关与 1-30s 超时输入；`service/command_direct_method_test.go` 存在）。残余差距仅为：无字面 oneway/twoway 枚举参数、无设备主动发起 RPC、等待靠持久日志短轮询（代码注释明示为刻意取舍）。若后续能力定义扩展到设备主动 RPC，再立新项。

### 4.3 P2 — 未逐项复核的 low 缺口（观察清单，17 项）

> 以下缺口在审计中**未逐项复核**（无代码级证据收集），状态与优先级沿用缺口清单原始标注。实施任何一项之前，须先按第 5 节方法补一轮核验（grep+读码+四面取证），再转入 4.1 格式立项。

| 编号 | 能力 | 状态 | 优先级 | 编号 | 能力 | 状态 | 优先级 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| TB-32 | EDQS 实体查询服务 | partial | low | TP-04 | 视频接入（GB28181/RTSP/萤石云） | missing | low |
| TB-38 | AI Assistant（自然语言配置） | partial | low | TP-15 | 行业协议与三方云接入套件（SL427/IEC104/OneNET/CTWing/ChirpStack） | partial | low |
| TB-39 | MCP Server | partial | low | TP-16 | 小智 ESP32 语音接入 | missing | low |
| TB-40 | n8n 集成节点 | missing | low | TP-17 | FUXA 组态对接 | missing | low |
| TB-42 | 自定义域名与自动证书 | missing | low | TP-19 | 信创国产 OS 适配 | partial | low |
| TB-43 | 平台 CLI（方案/环境管理） | partial | low | TP-23 | 等保三级安全合规 | partial | low |
| TB-52 | 许可模式与附加组件（add-ons） | partial | low | TP-25 | 性能指标宣称（10万并发/50万 msg/s） | partial | low |
| TB-53 | 设备/网关 SDK 生态 | partial | low | | | | |
| TB-55 | Trendz 类数据分析（预测/KPI） | partial | low | | | | |
| TB-56 | 托管云 SaaS（ThingsBoard Cloud） | missing | low | | | | |

---

## 5. 阶段执行计划

> **容量假设**：近期窗口按 2 名全栈 × 4 周（≈40 人日有效容量）估算。按 §4.1 口径（S≈3、M≈10、L≥15 人日），§5.1 能力项合计约 55~60 人日，超容量约 1.5 周——容量不增时按末位顺延（首选顺延第 6 项 TB-27 至中期批 2），出口判据按实际交付项计。

### 5.1 近期（0~4 周）：安全快赢 + P0 清零 + high partial + 口径修复

| 序 | 任务 | 关联缺口 | 工作量 | 出口判据 |
| --- | --- | --- | --- | --- |
| 1 | 一型一密产品级交叉校验（**安全快赢**：A 档案密钥可建档到 B 产品的越权面）✅ **已交付 2026-09-25**（负向用例就位待活栈） | TP-05 | S | 错配 product_key 负向用例通过 |
| 2 | 告警状态实时 WS 订阅（订阅主题+初始快照+替换轮询）✅ **已交付 2026-09-25**（契约用例就位待活栈） | TB-30（P0） | M | 四面一致 + 契约用例覆盖订阅/触发/推送/快照 |
| 3 | 部件库 widget_bundles 实体 + 资源中心分发 ✅ **已交付 2026-09-25 批次二**（契约用例待活栈；内置部件经 bundle 加载的运行时闭环列为后续项） | TB-04 | M | 内置部件经 bundle 加载不回归 |
| 4 | API 调用计量与配额执法 + 前端配额页 ✅ **已交付 2026-09-25 批次二**（API 维度执法与配额页；transport 维度接线按批次范围未做，契约用例待活栈） | TB-17 | L | max_api_calls_per_day 生效，transport 维度接线 |
| 5 | 设备 Profile 档案级默认规则链/默认队列 ✅ **已交付 2026-09-25 批次二**（档案级默认规则链；默认队列档案维度化按批次范围不做，契约用例待活栈） | TB-18 | M | 新建设备自动挂接默认链 |
| 6 | 告警 SLA 计时与超时升级 + details 结构化 ✅ **已交付 2026-09-25 批次二**（SLA 计时+超时升级；details 结构化 JSONB 迁移按指示不做、沿用 remark JSON 兼容模式，契约用例待活栈） | TB-27 | M | 到期升级 cron + remark JSON 兼容迁移（容量不增时首选顺延项） |
| 7 | 文档口径修复（非能力项）：README VERSION_NUMBER 99→122、全稿引用行号校准、历史证据**重链不回迁**（引用统一改为含归档前缀的实际路径）✅ **README 口径已修复 2026-09-25** | 第 8 节清理项 | S | 校验脚本对 `ROADMAP.md` 与 `docs/roadmap-*.md` 全量 `docs/validation` 引用零失配（每条在活跃项目或 `archive/iot-docs-and-roadmap-archive-20260920/` 下可解析） |

### 5.2 中期（1~3 个月）：medium 18 项，按依赖排序

> **状态注记（2026-09-25 批次二后）**：本表任务中 TB-10（批 2）、TB-15（批 1）、TB-21（批 3）、TB-25、TB-41（批 4）已交付（partial†，契约用例待活栈，见 §9 批次二记录）；其余项（TB-45、TP-03、TB-19、TB-48、TB-49、TB-47、TB-46、TB-11、TB-22、TB-23、TP-22、TP-20、TP-21）本批次未动，维持原状。**批次三更新（2026-09-25）**：上述"其余项"已全部交付（TB-45/TP-03/TB-19/TB-48/TB-49/TB-47/TB-46/TB-11/TB-22/TB-23/TP-22/TP-20/TP-21，均 partial† 契约用例待活栈；TB-15 行级 TTL 由 TB-15R 补齐交付）——**§5.2 中期清单能力项至此全部落地**，见 §9 批次三实施记录。

| 批次 | 任务 | 关联缺口 | 工作量 | 出口判据 |
| --- | --- | --- | --- | --- |
| 批 1（数据面） | ① Integration 实体纳管管线（TB-45）→ ② TCP 协议接入插件（TP-03）→ ③ 时序 TTL 粒度+retention policy（TB-15）→ ④ Protobuf 转换器模式（TB-19）。**顺序依据**：TB-19 转换器按 TB-45 的规划接入上行管线，先立 Integration 实体避免返工 | TB-45、TP-03、TB-15、TB-19 | M+M+L+M（TB-45 为 L；合计约 1.5~2 迭代） | 上行管线统一走 Integration 抽象；TTL 分级生效 |
| 批 2（平台面） | 审计日志实体级；统一调度器；报表 PDF/HTML+附件；白标全套；GPE 用户组与组级共享（自近期移入） | TB-10、TB-48、TB-49、TB-47、TB-46 | 4×M + L | 四面各自齐；存量调度/报表行为不回归 |
| 批 3（部署与边缘） | K8s/Helm+多副本验证（broker federation 接线）；边缘本地规则执行器；LwM2M DTLS/SNMPv3 | TB-11、TB-21、TB-22 | L×3 | 双副本演练通过；断云本地执行收敛；DTLS 握手用例 |
| 批 4（生态面） | 移动应用中心；实体版本控制 Git 适配；文件媒体库；大屏模式；国产 DB 驱动；MSET | TB-23、TB-25、TB-41、TP-22、TP-20、TP-21 | M/L 混合 | 各条四面齐；health 评分扩展不破坏既有四级口径 |

### 5.3 远期（3~6 个月）：P2 观察清单核验与商业化深化

| 序 | 任务 | 关联缺口 | 出口判据 |
| --- | --- | --- | --- |
| 1 | P2 清单 17 项逐一补核验（grep+读码+四面取证），产出与 4.1 同格式的立项卡 | TB-32~TP-25 | 每项给出 done/partial/pending 终态与证据 |
| 2 | 性能宣称验证（10 万并发/50 万 msg/s）：建立压测基线与报告 | TP-25 | 公开压测方法论+可复现脚本+结果报告 |
| 3 | 等保三级/合规差距分析与整改 | TP-23 | 合规差距清单+整改排期 |
| 4 | 商业化深化（add-ons/托管 SaaS/自定义域名）可行性立项 | TB-52、TB-56、TB-42 | 立项/否决决议各附依据 |
| 5 | SDK 生态（设备/网关 SDK 文档化与样例库） | TB-53 | SDK 快速上手样例+CI |
| 6 | 视频/语音/行业协议套件按客户需求择机立项 | TP-04、TP-15、TP-16、TP-17 | 按客户优先级排期 |

---

## 6. 旧结论清理对照表（本次审计推翻/失真项）

> 旧 `ROADMAP.md`（2026-09-24 v3 终章版）中以下表述经独立审计证实失真或夸大，本稿已不沿用：

| 旧稿位置 | 旧表述 | 审计结论 | 本稿处理 |
| --- | --- | --- | --- |
| ROADMAP.md:7 | 迁移链 `121.sql` = VERSION_NUMBER = 121，运行实例 sys_version=121，138 张业务数据表 | **过期**：实测最大编号 122.sql = VERSION_NUMBER=122（122.sql 客户管理为快照后新增）；138 表为活库声称无法复核（CREATE TABLE 粗代理 146 处） | 口径改为 122=122；不再写死表数（§1.1） |
| ROADMAP.md:5 | Go 编译与单测 100% 通过（60+ 包全 ok） | 审计门禁 go test **被跳过未执行**；本会话复跑 66 包 ok / 0 FAIL | 以实测值取代（§1.2） |
| 旧 TB-16 行 | 可选 KV 存储与 ValKey 兼容 done（"现网 Redis 活栈全面运行验证"） | **overstated**：全仓 grep 'valkey' 0 命中；无任何 TB-16 相关测试或证据文档；仅 go.mod:110 go-redis v9.22.0 依赖，"兼容 ValKey"属客户端库推断、无验证产物 | **整行剔除**，不进矩阵；如需主张须先补验证 |
| 旧 TP-7 行 | 国产化系统与数据库兼容 done（证据"backend/Makefile 交叉编译支持"） | **overstated**：backend/Makefile 全文 102 行无任何 GOOS/GOARCH/kylin/UOS/deepin/arm64 交叉编译目标（grep 0 命中）；无 CGO 前提部分成立（go.mod:89-90 纯 Go sqlite 驱动；CGO_ENABLED=0 仅在 mqtt-broker/Makefile:151）；国产 DB 侧实为外部 tptodb gRPC 客户端开关 | **整行剔除**；国产数据库适配以缺口 TP-20（P1，partial）列入 §4.1 |
| 旧 TB-7 行 | "Redis Lua 滑动窗口限流" | 表述与代码不符：两份 Lua 实为（多）**固定窗口计数器**（非 ZSET 滑窗）；队列隔离与 429 契约本身属实（confirmed） | 状态维持 done†，表述更正（§3.2 TB-13 备注） |
| 旧 P1.3 行 | 测试文件 `34_scada_canvas_editor.spec.js` | 该文件全工作区不存在（含归档）；实际为 `automation_tests/e2e/34_scada_editor.spec.js`，内容完整覆盖声称 | 引用更正（§3.2 TB-29） |
| 旧 TP-3 行 | "5 层递归解包与分发" | 代码下行防护上限实为 maxDepth=10（`service/command_gateway_payload.go:31-42`）；细节口径差，能力真实 | 表述更正（§3.2 TB-07 备注） |
| 旧 TB-8/TP-2/TB-11 行 | "24 文件 263 tests"、"全量看板 289 例"、"12 套件 166 tests" 等具体数字 | 均为文档记载、本会话未复跑 vitest，口径无法对齐 | 本稿不再引用未复跑的具体数字，统一 † 口径（§2.4） |
| 旧 P0.1 及多行证据引用 | `docs/validation/P0.1-preflight-evidence.md` 等路径 | 证据文档真实存在但**位于归档** `archive/iot-docs-and-roadmap-archive-20260920/docs/validation/`（活跃项目内无），旧稿引用路径失真 | 全部引用改为实际位置（§1.4） |
| 缺口清单原 TB-05 | 服务器端 RPC 双向控制 partial | **证伪**：direct-method 已实现 twoway 语义端到端（详见 §4.2） | 不列入缺口 |
| 旧 §3 门禁 | "71/72/73/74 组测试 36 passing" | 用例文件存在，本会话未复跑 | 不再作为门禁口径（§1.2） |
| README.md:127 | 数据库迁移链"当前均为 99" | 过期：实测 122（CONS-MIG） | 已修复（2026-09-25） |
| backend/docs/openapi/README.md | 重生成命令 `-out docs/openapi/openapi.json` | 在 backend/ 下执行会把产物写到 `backend/docs/`（非仓库正本位置） | 已修正为 `-out ../docs/openapi/openapi.json`（2026-09-25） |

---

## 7. 核验方法与局限

### 7.1 本次审计与本稿的方法
- **静态复核**：grep/通读源码、迁移 SQL（1~122 逐编号核验）、路由注册、前端目录与 API 封装、全仓同义词穷举（中英文、命名变体）；对关键实现逐段读码（如 `pkg/secrets/envelope.go`、`pkg/units/units.go`、`pkg/sparkplug/sparkplug.go`、`service/data_converter.go`、`service/device_health.go` 等）。
- **动态复核（本会话实跑）**：`go build ./...`（exit 0）；`go test ./... -count=1 -p 1`（66 包 ok / 0 FAIL）；`pnpm run typecheck`（exit 0）；OpenAPI paths 计数（483）。
- **竞品调研**：TB 87 个 release 经 GitHub tags/releases API 双向比对 + 官方文档表交叉；TP 36 仓库逐一核查 + 官网/文档站抓取。抓取失败项：无。

### 7.2 局限（诚实边界）
1. **自动化契约/E2E 未复跑**：`automation_tests/` 的 JS 契约测试与 Playwright E2E 需活栈（PG+Redis+broker+后端），本会话未运行；各行"测试全绿"结论依据 `docs/validation` 证据文档（部分在归档）记载，均以 † 标注。
2. **17 项 low 缺口未逐项复核**（§4.3）：仅有原始状态标注，无代码级证据；实施前必须补核验。
3. **PE 早于 3.0 的能力首引版本仅有上界**：官方 PE release notes 已下线（404，Wayback 不可用），白标/实体组/调度器/集成框架/报表等只能标"≤某版本"。
4. **TP 版本日期部分为推断**：release 页无年份，2022~2023 老版本年份系 docs 博客与 API ISO 日期交叉推断；约半数小版本无文字 notes；企业版能力仅官网宣称可考、无法对应版本号。
5. **活库状态不可从仓库复核**：sys_version、业务表数等运行实例声称未验证；旧稿相关数字已从本稿口径移除。
6. **审计门禁 go test 跳过原因未查明**（审计环境构建步骤失败）；本稿以本会话实跑结果（66 包 ok）覆盖该口径，但审计环境与本会话环境的差异未做归因。
7. **lint:check 未运行**：旧稿"ESLint 0 错误"本会话未复核，本稿不作声称。
8. **§1.3 竞品版本覆盖声明系调研基线，本次独立评审未做外网复核**：评审范围限于仓库内验证；TB/TP 版本号与 release 集合的准确性依赖第 1 节所述调研抓取记录（GitHub tags/releases API 双向比对、官方文档表、官网抓取留痕）。
9. **契约测试 75/76/77 未运行**（2026-09-25 实施会话）：本机无 docker，活栈（PG+Redis+broker+backend）不可用；三个用例已就位，待活栈回归后对应行状态方可从 partial 升 done（见 `docs/validation/2026-09-25-roadmap-remediation-batch-evidence.md`）。
10. **`pnpm gen-route` 工具链损坏**（`unicorn-magic@6` 与 Node 24 的 ESM 解析冲突，`pnpm install` 后仍复现，属环境既有问题）：2026-09-25 的 customer 路由以手工同步 4 个 elegant-router 生成文件代替（routes/imports/transform/elegant-router.d.ts），一致性由 vue-tsc 与全量 vitest 验证。
11. **契约测试 78~86 未运行**（2026-09-25 批次二）：本机无 docker，活栈（PG+Redis+broker+backend）不可用；九个契约用例（78 widget bundles / 79 billing API 配额 / 80 档案规则链 / 81 告警 SLA / 82 实体级审计 / 83 版本 diff / 84 data_policy / 85 媒体库 / 86 边缘快照契约）已就位并通过 node --check 语法校验（实施会话自报），按批次约定只写不跑；§4.1 九个已交付行与 §3 批次二新增九行因此记 **partial†**，待活栈回归后升 done（见 `docs/validation/2026-09-25-roadmap-remediation-batch2-evidence.md`）。
12. **契约测试 87~101 未运行**（2026-09-25 批次三）：本机无 docker，活栈（PG+Redis+broker+backend）不可用；批次三两程新增契约用例实际落号 **12 个**（87 Integration / 88 用户组 / 90 TCP 网关 / 91 Protobuf / 92 SNMPv3 / 93 移动应用中心 / 94 白标 / 95 调度器 / 96 报表格式 / 99 大屏 displayMode / 100 行级 data_policy / 101 transport 配额；89/97/98 号位未占用，TB-11 无 REST 契约面故无落号），已就位并经实施会话 `node --check` 语法校验，按批次约定只写不跑；§4.1 批次三十四个已交付行与 §3 批次三新增十三行因此记 **partial†**，待活栈回归后升 done（见 `docs/validation/2026-09-25-roadmap-remediation-batch3-evidence.md`）。另：批次三 blocked 项 TB-17R（传输维度配额接线）已回退，其契约测试 101 不作为交付证据面。

---

---

## 8. 评审意见处理（2026-09-25）

> 评审报告共 14 条修改意见，**全部采纳**；其中意见 4 含一处对评审报告本身的反向更正（见 #4），意见 6 以补实证方式执行（见 #6）。评审对本稿的其余事实核验（14 处抽查、25 项缺口归组、旧失真结论清除检查）均通过，未发现需回改项。

| # | 意见摘要 | 处理 | 说明与实测证据 |
| --- | --- | --- | --- |
| 1 | CORE-03 的 E2E/契约计数失真（10 个 spec / 74 组契约） | ✅ 采纳 | 实测：`ls automation_tests/tests/*.test.js | wc -l` = **127**；`ls automation_tests/e2e/*.spec.js | wc -l` = **32**；编号前缀去重 = **74**（最小前缀为 00，故写作"00~74 共 74 个"）。§3.1 已按实测改写 |
| 2 | "† 含义见 2.4" 悬空引用 | ✅ 采纳 | §3 头部改为"† 含义见 §2 第 4 条" |
| 3 | TB-30 引用少一级目录、"前端告警页"不精确 | ✅ 采纳 | 路径补全为 `frontend/src/hooks/thingsvis/useRealtimePush.ts`（实测存在）；轮询主体改写为"ThingsVis 告警推送 hook（`useAlarmPush.ts:52`，实测 `setInterval(fetchAlarmStatus, 30000)`）"；验收点同步改为 hook+告警列表页双消费方口径 |
| 4 | 行号校准：CheckAPI :197→:198、report_smtp :158→:159、全稿跑行号脚本 | ✅ 采纳（2/3 处 + 反向更正 1 处） | `grep -n CheckAPI backend/internal/ratelimit/service.go`：:197 为注释、:198 为函数 → 采纳改 :198；`grep -n "Body: string(payload" report_smtp.go` → :159，采纳。**反向更正**：评审称 `report_run_processor.go` 的 reportMaxRows 在 ":16-17"，实测 `grep -n reportMaxRows` 为 **:17-18**（:17=reportMaxRows、:18=reportMaxBytes），原文正确、不改。另按建议跑全稿引用校准脚本：解析 161 处 file:line 引用，校验文件存在性与行号越界，修订前唯一异常为 `queue_monitor.go:12-58`（素材原文失真：实测三队列名于 :29-43、文件共 51 行，已改 :12,29-43），修订后 **0 异常**。语义级核验仅对上述 5 处逐行 grep 确认，未做全量语义比对（如实声明） |
| 5 | TB-02/TP-02 裸 done 违反自家四面纪律 | ✅ 采纳 | 两行降为 **done†**，备注注明"结构性能力、依赖子能力测试覆盖、无本行专属用例（四面不齐，按 §2 记 done†）" |
| 6 | TB-33"承认未核验却标 done"自相矛盾 | ✅ 采纳（补实证路径，未拆行/未新增用例） | 修订会话 grep 实测地理围栏已有实现：`backend/internal/calcfield/advanced.go:25`（FieldTypeGeofence）、`:134-135`（EvaluateGeofence，圆形/多边形）+ `calcfield/advanced_test.go:26-102`（合法/非法/引擎路由用例），随本会话 66 包 go test 通过。TB-33 维持 done†，证据列补围栏实证，备注改为"地图追踪与规则级围栏均有实证；地图部件侧围栏 UI 交互未单独核验"——比"未核验"表述更准确且不再自相矛盾 |
| 7 | Go 单测豁免口径不可判读，需白名单 | ✅ 采纳 | §2 第 4 条追加豁免白名单（TB-28/TB-31/TB-36/TB-34/TB-35/TP-01/CORE-07），六行均实测存在专属 Go 测试文件：`sparkplug_test.go`、`pkg/units/units_test.go`、`calcfield/advanced_test.go`+`engine_test.go`、`service/rule_chain_nodes_ai_test.go`、`pkg/pluginsdk/` 4 个 `*_test.go`、`service/conflict_policy_test.go` |
| 8 | §5.1"回迁或重链"二选一未决 | ✅ 采纳 | 定为**重链不回迁**；§1.4 补处置决定与校验脚本出口判据，§5.1 第 7 项同步改写 |
| 9 | TB-46 验收点内嵌未决设计 | ✅ 采纳 | 验收点写死 v1 范围：租户内用户组与组权限；customer 用户（`sql/122.sql`，无登录账号）不接入组授权 |
| 10 | TB-23 跨仓依赖未标注 | ✅ 采纳 | 验收点补跨仓边界：前端面由仓库外 `active/mobile-app-uni` 承担，本仓验收止于发布 API/记录与 H5 构建产物对接 |
| 11 | §5.2 批 1 工作量"L"失真 + TB-19/TB-45 顺序颠倒 | ✅ 采纳 | 工作量改"M+M+L+M（TB-45 为 L；合计约 1.5~2 迭代）"；顺序调为 TB-45 → TP-03 → TB-15 → TB-19，注明"TB-19 转换器按 TB-45 规划接入管线，先立实体避免返工" |
| 12 | TP-05 排位过低（安全+S）、TB-46 撑爆近期窗口、无容量声明 | ✅ 采纳 | TP-05 升至近期第 1（安全快赢）；TB-46 移出近期、并入中期批 2；§5 开头补容量假设（2 全栈 × 4 周 ≈40 人日）与超容顺延规则（首选顺延 TB-27）；近期为 6 项能力 + 1 项非能力文档修复 |
| 13 | §1.2 补评审会话复跑结果列 | ✅ 采纳 | 表增第四列：go build exit 0、go test 完整重跑 66 ok/0 FAIL（84 包=66+18 无测试，与 `go list ./...` 吻合）、OpenAPI 483 ✓；typecheck/lint/JS 套件如实标"未运行"，未新增声称 |
| 14 | §7.2 补"竞品覆盖未外网复核"局限 | ✅ 采纳 | 增第 8 条局限 |

---

---

## 9. 2026-09-25 实施记录（本稿落地批次）

> 依据 §5.1 近期计划交付；证据留痕 `docs/validation/2026-09-25-roadmap-remediation-batch-evidence.md`。

| 序 | 交付 | 关联缺口 | 落地面 |
| --- | --- | --- | --- |
| 1 | 告警状态实时 WebSocket（后端端点+Redis 发布侧+前端订阅/降级+列表页实时刷新） | TB-30（P0） | OpenAPI(+1 path)、后端、前端、Go 单测 6 例；契约用例待活栈 |
| 2 | 一型一密产品级交叉校验（同租户+绑定关系，错误码 200087） | TP-05 | 后端+鉴权链路；契约用例待活栈 |
| 3 | 客户实体四面收尾（DAL 修正+前端页面/菜单/四语言/OpenAPI/测试） | TB-57（新增行） | OpenAPI(+5 paths)、后端、前端（vitest 7 例）；契约用例待活栈 |
| 4 | 国产化交叉编译目标（amd64/arm64/loong64）与三架构二进制实测 | TP-19（OS 面） | 工程构建面 |
| 5 | 存量缺陷修复：es/fr converter locale、device-details 共享视图测试、README 迁移链口径、OpenAPI 重生成命令 | §6 清理项 | 文档与前端测试面 |

**最终门禁（2026-09-25）**：go build exit 0；`go test ./...` REAL_EXIT=0（66 包 ok / 0 FAIL）；vue-tsc 0 错误；vitest 3970 例全绿；OpenAPI 489 paths。

### 2026-09-25 批次二实施记录（TB-04/17/18/27/10/25/15/41/21）

> 依据批次二任务书交付；证据留痕 `docs/validation/2026-09-25-roadmap-remediation-batch2-evidence.md`（含逐项 residual 全文、明确不做项清单与更新会话核验记录）。契约测试 78~86 已就位、按约定未在活栈运行（本机无 docker），对应 §3/§4.1 行状态记 **partial†**。

| 编号 | 交付 | 迁移 | 落地面 | residual（摘要，全文见证据文档） |
| --- | --- | --- | --- | --- |
| TB-04 | 部件库 widget_bundles：建表+Casbin/菜单种子，后端四层 CRUD+内置四部件种子导出/幂等落库，资源中心打包/验签/导入/一键应用接入，前端管理页+四语言 | 123.sql | 后端四层+资源中心接入、前端管理页/路由/四语言、Go 单测 8 例；契约 78（17 例）待活栈 | 画布运行时从 bundle 加载部件未接线（内置部件行为不回归，运行时闭环列后续项）；123.sql 额外 version/type_key 两列（头注释说明） |
| TB-17 | 租户 API 日配额执法：api_usage_daily 计量（Redis INCR+定期落库 fail-open）+按套餐 max_api_calls_per_day 执法（429+Retry-After）+GET /billing/api-quota+前端配额页 | 124.sql | 后端新包 internal/quota+中间件接线、前端配额页+四语言、Go 单测 22 例（复核实测）；契约 79 待活栈 | transport 维度（broker/网关调用点）按范围排除；被拒请求计入当日用量；UTC 日窗口；套餐限额≤0 视为未配置（fail-open） |
| TB-18 | 设备 Profile 档案级默认规则链：UUID 外键列+绑定/解绑 API+档案链优先/租户链兜底解析（OnTelemetry/OnDeviceOnline 接线）+解析端点+前端设置页 | 125.sql | 后端+解析端点、前端设置页（vitest 4 例）、Go 单测 5 例；契约 80（7 例）待活栈 | 默认队列档案维度化与告警规则 Profile 化按要求不做；解析失败/跨租户/停用回落租户链（fail-open 到租户级执行）；删链 service 守卫+FK RESTRICT 双层拒绝 |
| TB-27 | 告警 SLA：三列迁移+触发起算 sla_due_at+5 分钟 cron 超时标记 breach 并升档（L→M→H，N 不动）+API 透出 sla 字段+前端 SLA 设置与历史超时标记 | 126.sql | 后端+cron 接线、前端（组件套件 83 例过）、Go 单测 4 例；契约 81（8 例）待活栈 | details 结构化 JSONB 按指示不做（remark JSON 兼容读取）；历史告警不回填；calcfield/RDI 旁路不参与 SLA；cron 真实时钟行为未在活栈验证（纯函数单测锚定） |
| TB-10 | 实体级审计：operation_logs 增 action/entity_type/entity_id/status_code+partial index+自脱敏解析器+列表/导出筛选与新列+前端同款筛选与四列 | 127.sql | 后端 middleware/dal/api、前端 system-log 页、Go 单测 4 例；契约 82 待活栈 | 独立 audit_logs 实体与领域事件审计按要求不做；存量行新列 NULL 不回填；非中间件写入口不落新列；entity_id 仅认 UUID 形态 |
| TB-25 | 实体版本差异对比：DiffEntityVersionSnapshots 递归 JSON 语义 diff+GET /api/v1/entity_versions/:id/diff/:target_id+前端对比视图（目标版本下拉/双栏快照/变更列表） | 128.sql | 后端+Casbin 登记、前端对比视图+四语言、Go 单测 8 例；契约 83（4 例）待活栈 | Git 仓库后端明确不做（快照模式仍为唯一后端）；路径按既有复数资源命名（任务书原文单数，casbin/契约测试/catalog 三处登记一致） |
| TB-15 | TimescaleDB 原生 retention：按 data_policy retention_days 幂等装配 add_retention_policy（毫秒 drop_after+set_integer_now_func）+telemetry_rollups 冷层同边界分批清理 | （57.sql 就地修正） | 后端 initialize/dal/service、纯函数/SQL 构造单测 8 例；契约 84（7 例）待活栈 | drop_chunks 真删与 jobs 守卫需活 TimescaleDB 验证；alarm_info 未挂 retention（需先扩 data_policy 类型，另立批次）；档案/租户粒度 TTL 明确不做 |
| TB-41 | 媒体库：media_files 表（UNIQUE(tenant_id,file_path)+Casbin+菜单）+UpFile 落盘即登记（失败回滚）+/media/files 列表/详情/删除（引用扫描 fail-closed，202004 拒绝）+前端 /media/library 页 | 129.sql | 后端全链、前端媒体库页+四语言、Go 单测（service+dal）；契约 85 待活栈 | 部件内嵌图片选择器与邮件附件外发按边界不做；存量 ./files 不回填登记；v1 引用统计为读时 LIKE 扫描（部件级明细表后续）；errcode 202004 文案仅 zh/en |
| TB-21 | 边缘本地规则执行器 scoped v1：internal/edgerules（快照解析/阈值求值/告警事件/修订号去重/升级重装载，零 DB/broker 依赖）+edgemqttbroker 可选开关（默认关）+端到端冒烟验证 | （无新迁移） | 后端新包+broker 接入+README、端到端冒烟、Go 单测 32 例；契约 86 待活栈 | 断云本地动作与云端去重收敛演练未做（需活栈+边缘）；v1 仅 telemetry/threshold/alarm 子集，其余节点求值 fail-closed；64 号断云演练扩展未做 |

**批次二收尾门禁（2026-09-25）**：openapi=0、goTest=0、vue-tsc=0、vitest=0（任务书下达口径，编排方统一收尾；文档更新会话照录、未复跑）。

**blocked 项**：无——本批次 9 项全部 delivered，无已回退项。

### 2026-09-25 批次三实施记录（两程 15 项：第一程 9 + 第二程 6）

> 依据批次三任务书两程交付；证据留痕 `docs/validation/2026-09-25-roadmap-remediation-batch3-evidence.md`（含逐项 residual 全文、TB-11 专项证据引用、更新会话核验记录与口径声明）。第一程 9 项 + 第二程 5 项 delivered、1 项 blocked（TB-17R 已回退）。迁移链扩至 `138.sql`（132/133 因批次三编号重排转为 NO-OP 占位，TB-19/TB-23 实际落 135/136）。契约测试 87~101（实际落号 12 个，89/97/98 未占用）已就位、按约定未在活栈运行（本机无 docker），对应 §3/§4.1 行状态记 **partial†**。第一程各项 residual 明细未随实施材料提供（TB-11 除外，见专项证据文档）；第二程 residual 为实施会话回报全文转录。

**第一程（9 项，全部 delivered）**

| 编号 | 交付 | 迁移 | 落地面 | residual（摘要；明细未随第一程材料提供） |
| --- | --- | --- | --- | --- |
| TB-45 | Integration 实体纳管管线：integrations 表+Casbin/菜单种子、四层 CRUD、OPC UA 采集路径上行转换钩子、统一集成管理页 | 130.sql | 后端四层、前端集成管理页；契约 87 待活栈 | 未随材料提供 |
| TB-46 | 用户组与组权限（GPE v1）：三表+Casbin+组共享 fail-closed scope、用户组管理页 | 131.sql | 后端、前端管理页；契约 88 待活栈 | customer 用户不接入（v1 边界）；其余未随材料提供 |
| TP-03 | TCP 协议接入：门控入站 TCP 网关（帧解析/注册簿/断网缓冲）+设计文档 | （无新迁移） | 后端 protocolgw/tcp+app 装配；契约 90 待活栈 | 未随材料提供 |
| TB-19 | Protobuf 载荷编解码：proto_schema 列+protoparse/dynamicpb 动态解码+Dry-Run+converter 页 PROTOBUF 模式 | 135.sql（首轮落 132.sql，批次三编号重排改挂，132 转 NO-OP 占位） | 后端、前端 converter 页；契约 91 待活栈 | 未随材料提供 |
| TB-23 | 移动应用中心：mobile_app_bundles+状态机八端点+应用中心页 | 136.sql（首轮落 133.sql，重排改挂，133 转 NO-OP 占位） | 后端、前端应用中心页；契约 93 待活栈 | 未随材料提供 |
| TB-11 | K8s/Helm 编排与多副本：deploy/helm chart+双副本 override+Go 渲染校验单测+README K8s 章节 | （无新迁移） | 部署编排面（无 REST/前端契约面）；helmchart 5 用例；专项证据独立成文 | 真实集群安装/双副本演练未做（无 K8s 环境）、federation 不接线；见 `docs/validation/2026-09-26-tb11-k8s-helm-multireplica-evidence.md` |
| TB-22 | SNMPv3 + LwM2M 多客户端隔离：SNMPv3 authNoPriv 接入采集链路+per-endpoint ObjectStore 隔离+两个存量协议缺陷修复 | （无新迁移） | 后端采集链路+snmp/usm v3 安全层；契约 92 待活栈 | LwM2M DTLS(PSK)/observe 服务端集成仍开放；其余未随材料提供 |
| TP-20 | 国产 DB 方言适配层：internal/dialect 纯函数包（101 表驱动用例）+国产化部署章节 | （无新迁移） | 后端方言抽象+docs/deployment-domestic-db.md | 国产库驱动未进 go.mod、真实国产库连通未验证（本项交付为方言适配层） |
| TP-21 | MSET 健康算法：训练/推理/降级（16 用例）+健康评分扩展点接线（默认关）+前端 MSET 维度渲染 | （无新迁移） | 后端 healthmset 新包+评分扩展点、前端健康页 | 预测性维护指标未在交付摘要内、默认关未经活栈启用验证；其余未随材料提供 |

**第二程（6 项：5 delivered + 1 blocked）**

| 编号 | 交付 | 迁移 | 落地面 | residual（摘要，全文见证据文档） |
| --- | --- | --- | --- | --- |
| TB-47 | 白标第二阶段收口：租户翻译覆盖+自定义 CSS 后端面+统一前端接线（locales/store/sys-setting/route/auth）+branding-setting 页+四语言 | 134.sql（前一程交付；收口未新建迁移） | 后端 model/dal/service/api/router、前端 locales/store/views、Go+前端单测（定向 7 文件 68 用例）；契约 94 待活栈 | 内置菜单改名/隐藏/按角色明确不做；overrides 为登录后可读端点（无匿名获取面）；门禁第 2 轮过（首跑失败系他泳道 api-quota 瞬时编译态，30 秒重跑自愈） |
| TB-48 | 统一调度器：137.sql scheduler_events 注册面+GET /scheduler/events 三源只读聚合+注册面 CRUD（scene 事件同事务落既有 timer 机制）+月视图日历页+路由四件套+四语言 | 137.sql | 后端+前端日历页、Go 单测（dal 8+service 5）；契约 95（12 例）待活栈 | 三套存量调度不迁移（聚合只读面）；注册面 v1 cron 统一 UTC；聚合内存归并单源 2000 行封顶；report/rpc 注册事件为纯登记展示 |
| TB-15R | 行级 TTL（档案/租户粒度）：138.sql data_policy 行级（tenant_id/device_config_id 可空+部分唯一索引）+三级覆盖清理（档案级>租户级>全局，热冷层同口径）+POST/DELETE /api/v1/datapolicy+前端作用域列与行级策略弹窗 | 138.sql | 后端 dal/service/api、前端 data-clear-setting（组件 23 用例）、Go 单测；契约 100（11 例）待活栈 | 行级仅设备数据（data_type=1）；TimescaleDB 行级动态 retention job 明确不做（行级一律走分批 DELETE）；档案下拉按前端行内 tenant_id 过滤的局限 |
| TB-49 | 报表 HTML/PDF 与附件投递：format 放开 csv/html/pdf+html/template 与 go-pdf/fpdf 渲染（复用行/字节预算）+gomail 全格式附件化（CSV 同样附件化）+前端 NSelect | （无新迁移；78.sql 仅注释同步口径） | 后端渲染/投递（Go 单测 10 例）、前端报表页（11 用例）；契约 96（6 例）待活栈 | 富样式 PDF/图表嵌入明确不做；PDF 核心字体 Latin-1 超集降级 '?'；格式标签为组件内常量未进 locale |
| TP-22 | 大屏固定分辨率与轮播投屏：displayMode fixed1080（自动哨兵 1920×1080，serializeScadaCanvas 丢字段 bug 修复）+编辑器 transform:scale 等比适配（computeFitScale+ResizeObserver+拖拽反除）+/tv-preview 多屏轮播（tokens+interval+getDashboardsByShareTokens 批量解析+Fullscreen API+四语言） | （无新迁移） | 前端 scada 核心/预览页、后端 scadadoc 壳层+board_carousel、定向 vitest 8 文件 90 用例；契约 99 待活栈 | DataRoom 引擎收编明确不做；公开轮播按 share token 取数；interval 纯展示参数（clamp [3,3600]）；legacy provider 不支持批量轮播；全屏需用户手势；门禁第 2 轮过（首跑失败系调度器泳道瞬时态，重试自愈） |
| TB-17R | （已回退）传输维度配额接线：broker MQTT CONNECT 租户 Redis INCR 日计数+套餐限额判定（fail-open）+SubscribePlan 发布 MaxTelemetryPerDay+前端 transport_* 字段 | （无迁移） | —（已回退，不计入交付面） | blocked：vue-tsc 门禁 3 轮未过（5 次重跑错误集恒位于他泳道 TP-22 在途文件，本项文件零错误、独立全量 typecheck 曾 EXIT:0），按约定登记为**已回退**；transport 维度仍开放；截至更新会话（2026-09-26）其改动文件仍在工作树且核心新文件为 git 未跟踪态，物理处置留编排方 |

**批次三收尾门禁（2026-09-25）**：openapi=0、goTest=0、vue-tsc=0、vitest=0（任务书下达口径，编排方统一收尾；文档更新会话照录、未复跑）。

**blocked 项**：1 项——TB-17R（传输维度配额接线）已回退（如上）；其余 14 项 delivered。

---

*本稿由 2026-09-24 快照（HEAD 4be45c6）审计重制，2026-09-25 依独立评审意见修订（§8）并完成 §5.1 首批、批次二与批次三实施（§9）；所有"done"声明以 §2 状态纪律与 † 时效标注为准，下次活栈回归后应逐行消除 †。*
