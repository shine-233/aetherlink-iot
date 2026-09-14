# AetherLink IoT 平台下一阶段路线图

> 版本：2026-09-13（v2 重构：单一事实源，取消补记层叠）
>
> 证据基线：`main@974c44a`（2026-09-13），工作树 clean，提交 239 次。
>
> 迁移链：`backend/sql/` 最大编号 `99.sql` = `backend/pkg/global/global.go` 的 `VERSION_NUMBER` = `99`（**以代码为准**）。
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
- 质量：迁移源码连续至 `99.sql` / `VERSION_NUMBER=99`；**但全新空库的全链 `initialize.CheckVersion` 验证只做到过 93**（`sys_version=93`、114 张表），94–99 尚无同等验证。

### 1.2 状态总表（唯一权威）

> 判定严格按 §4：`done` 仅限已运行证明。本表是各任务状态的唯一来源；下方 P0–P3 各节只补充设计理由与证据指针。

| 任务 | 主题 | 状态 | 缺口类型 | 未闭环（一句话） | 证据 |
| --- | --- | --- | --- | --- | --- |
| P0.1 | 发布同步与部署门禁 | `partial` | `环境阻塞` | 真实 HTTPS/TLS、MQTTS、公网 MQTT、backup/restore 计数一致性 | `docs/validation/P0.1-preflight-evidence.md` |
| P0.2 | 设备影子 ACK 闭环 | `partial` | `未验证` | 真实 MQTT `shadow_ack` 端到端 + 浏览器证据 | `P0.2-shadow-ack-evidence.md` |
| P0.3 | OTA 状态机 | `partial` | `未验证` | 真实设备/broker 或协议 stub E2E；灰度治理无运行期证据文档 | `P0.3-job-report-evidence.md` |
| P0.4 | 场景与 Flow 语义 | `partial` | `未验证` | 真实 E2E | `scene_execution_window_test.go` |
| P0.5 | CSV 浏览器 E2E | `partial` | `未验证` | 真实浏览器 file chooser E2E | `P0.5-*-evidence.md`（三份） |
| P0.6 | 持久化报表执行 / SMTP | `partial` | `未验证` + `未接线` | SMTP `ambiguous` 收口证据、管理员报表工作台前端、v82→v83 切换演练 | `P0.6-postgres-migration83-evidence.md` |
| P0.7 | AI 凭证静态加密 | `partial` | `未验证` | 生产主密钥注入、"日志无明文"未验证 | `P0.7-secret-encryption-evidence.md` |
| P1.1 | 通用 Entity Relations | `partial` | `未验证` | 运行期证据文档（现有全为纯单测）；看板集成 | 迁移 85、`entity_relation_test.go` |
| P1.2 | 规则链可靠性 | `partial` | `未验证` | 真实链路 E2E | `P1.2-rulechain-version-evidence.md` |
| P1.3 | Widget 与 SCADA 基础层 | `partial` | `未验证` | ~~编辑器未挂路由~~（2026-09-14 已挂）；~~widget schema 无真实字段~~（2026-09-14 已闭环 `36fd6da`）；剩余：浏览器 E2E、真实下发联调 | `scada_postgres_test.go`、`adeaf80` |
| P1.4 | 移动端控制与通知 | `partial` | `客户端缺失` + `未验证` | **Android/iOS 工程不存在**；FCM/APNs 未真机联调；真机业务 E2E | `mobile_e2e_test.go`、`push_provider_live_test.go` |
| P1.5 | 边缘运维 | `partial` | `未实现` + `未验证` | 节点证书签发、远程升级回滚、真实边缘联调与断云演练 | `P1.5-P3-completion-batch-20260912.md` |
| P1.6 | 模板市场产品化 | `partial` | `未验证` + `未接线` | 升级/回滚运行期证据（98/99.sql 未复跑）；预览/确认闸门浏览器 UI | `device_template_market_import_test.go` |
| P2.1 | 协议插件 SDK | `partial` | `未实现` | 真实外部协议适配器（CAN/BACnet/BLE/LoRaWAN）；manifest 注册 HTTP 运行期路径 | `pkg/pluginsdk`（9/9 实跑通过） |
| P2.2 | Trendz 类轻量分析 | `partial` | `未验证` | anomaly 运行期证据；受 P0.6 durable execution 约束 | `telemetry_analysis_core_test.go` |
| P2.3 | 数据保留与性能 | `partial` | `环境阻塞` | **基准压测 / 容量模型 / 冷热分层告警 pending** | `P1.5-P3-completion-batch-20260912.md` |
| P3 | 商业化与长期能力 | `partial` | `未实现` | 许可证签发工具；**其余 11 个子项零代码** | `pkg/license`（6/6 实跑通过） |

**统计：`done` 0 项 / `partial` 16 项 / `pending` 1 项（P2.3 压测子项、P3 多数子项）。**

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

### 1.3 缺口分类处置（按"要开发"与"只差跑一遍"分账）

**A. 只差跑一遍（`未验证`，恢复环境即可，无需开发）**

- P1.1 实体关系运行期证据、P0.3 灰度治理证据、P2.2 anomaly 证据、P1.5/P1.6 新端点证据、98/99.sql 在 PostgreSQL 复跑、OpenAPI 重生成。

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

**F. 环境阻塞（`环境阻塞`）**

- 本机无网络、无 Docker、Go 模块缓存被清空、系统盘约 11 GB（98%）。**P2.3 压测与 P0.1 部署验收在此之前无法开展。**

## 2. 竞品能力边界（2026-09 刷新；版本锚点 2026-09-14 经 GitHub API 全量复核）

> 版本数据来源：GitHub Releases API 全量实拉（`thingsboard/thingsboard` 87 个 release、`ThingsPanel/thingspanel-backend-community` 51 个 release，见仓库 `docs/validation/2026-09-14-gap-analysis.md` 与逐版本差距矩阵）。40 功能域量化结论：✅ 追平/领先 25 项、🟡 部分实现 10 项、❌ 未实现 5 项（2026-09-15 逐行重计）。

**ThingsBoard**（Java）：当前 Active LTS 为 **v4.3.x**（v4.3.1.4，2026-08-27）。CE 覆盖设备/资产/客户实体、遥测、MQTT/CoAP/HTTP/SNMP/LWM2M、IoT Gateway（Modbus/OPC-UA/BACnet）、Rule Engine、计算字段（4.0）、Dashboard、告警规则 2.0（4.3）、OTA、多租户、集群、AI 规则节点。PE/Cloud/Edge 额外提供高级 RBAC、白标、平台集成（AWS IoT/Azure/Kafka/LoRaWAN）、400+ 编解码库、自动报表、解决方案模板、SSO、密钥存储与 SLA。TBMQ、Trendz、Edge 是独立生态产品，不应假设 CE 自带。4.0 的破坏性变更：Kafka 强制、flex-layout 移除、Timescale 弃用。

**ThingsPanel**（Go，**与本项目同源**）：社区版仅 HTTP/MQTT/Modbus TCP·RTU + 看板（无大屏）+ 场景联动 + 固件升级 + 多租户 + APP；企业版将 21 项协议、11 项三方接入、大屏、算法中心、集群、白标、国产化系统与国产数据库列为"需单独购买"。社区仓库 `main@b646061`（2026-09-07）：572 个 `.go` 文件、33 个测试文件、20 条 SQL 迁移。对 ThingsPanel 的结论必须二分"官网宣称"与"开源仓库可验证"。

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
- 已实现：全新空库 `aetherlink_migrate_20260912` 用项目自身 `initialize.CheckVersion` 顺序跑 `sql/1.sql…93.sql`（`AETHERLINK_TIMESCALE_MODE=off`）→ `MIGRATE_OK` / `sys_version=93` / 114 张表（**注：该验证只到 93，94–99 未做同等全链验证**）。
- 未闭环：目标服务器 HTTPS/TLS、MQTTS 设备上报/下发、公网 MQTT、backup/restore 计数一致性；因执行环境无 `git` 而报 PENDING 的工作树检查。
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

**实现状态**：`partial` · 缺口类型 `未验证`。

- 已实现：迁移 `84.sql` 新增 `attempts`/`sent_at`/`ack_at`/`next_attempt_at`/`last_error`，状态词表扩展为 `pending|sent|delivered|failed|expired|canceled`（带 CHECK），并回填历史 `delivered` 行的 `ack_at`。
- 已实现：**修正了"dispatch 成功即写 delivered"的虚假成功**——dispatch 后为 `sent`，只有设备 ACK 才转 `delivered` 并写 `ack_at`。
- 已实现：退避重试 30s→60s→120s（上限 10min），超过 `ShadowMaxAttempts=3` 转 `failed`；TTL 是硬终止，到期转 `expired`。
- 已实现：端点 `POST /api/v1/device/shadow/:deviceId/:msgId/ack`（只有 pending/sent 可确认，终态行拒绝）；cron 与上线钩子先 `ExpireAndRetryShadowMessages()` 再投递。
- 已实现：MQTT 侧 ACK 上报入口 `shadow_ack`（`uplink/bus.go`）→ `ResponseUplink.processShadowAck` 解析 `{"shadow_id","result"}`；**`result` 非 0 不确认送达**，且必须在 message_id 校验之前处理。
- 已实现：UI `device-shadow.vue` 新增 sent/failed 状态与 `attempts`/`ack_at` 两列（四语 locale 同步）；API 用例 `automation_tests/tests/27_shadow_messages.test.js` 改为先断言 sent、显式 ACK 才断言 delivered，并新增"终态行不可被确认"负向用例。
- 未闭环：真实 MQTT `shadow_ack` 上报的端到端与浏览器证据（需活栈）。
- 证据：`docs/validation/P0.2-shadow-ack-evidence.md`；常驻用例 `internal/dal/device_shadow_postgres_test.go`（缺 DSN 则 Skip）。

### P0.3 OTA 状态机

**交付物**：进度消费、批次暂停/恢复/取消、失败重试、灰度、回滚和报告。

**框架代码**：

    type OTAStatus string
    const ( OTAPending OTAStatus = "pending"; OTARunning OTAStatus = "running"; OTASuccess OTAStatus = "success"; OTAFailed OTAStatus = "failed"; OTAExpired OTAStatus = "expired"; OTACanceled OTAStatus = "canceled" )
    type OTAProgressEvent struct { JobID, DeviceID string; Percent int; Status OTAStatus; Error string; At time.Time }
    func (s *OTAService) ConsumeProgress(ctx context.Context, e OTAProgressEvent) error
    func (s *OTAService) RetryFailed(ctx context.Context, jobID string, limit int) error

**门禁**：状态转移非法即拒绝；同一事件幂等；失败设备可筛选重试；回滚产生新审计事件；至少一条真实设备/broker 或协议 stub E2E。

**实现状态**：`partial` · 缺口类型 `未验证`。

- 已实现：`internal/service/fleet_command_job_state_machine.go` 集中声明合法状态转移表（`scheduled/running/paused` 及终态），非法转移返回 `CodeOpDenied` 并带双向状态，终态不可复活。
- 已实现：`PauseFleetCommandJob`/`ResumeFleetCommandJob`——新增用户语义事件 `paused`/`unpaused`，与 worker 故障恢复的 `resumed` 明确区分；暂停清 `next_dispatch_at` 使 worker 停止领取，恢复置 `next_dispatch_at=now` 并立即派发；暂停态重复调用幂等。
- 已实现：进度消费 `ConsumeFleetCommandJobProgress`（`fleet_command_job_progress_rollback.go`），**幂等令牌刻意不含上报时间**；终态/已取消批次拒绝进度上报；`87.sql` progress 列 + `UpdateFleetCommandJobDetailProgress` 回写明细行。
- 已实现：回滚 `RollbackFleetCommandJob` 只对已结束批次执行，**创建新批次而非改回原批次**，原批次历史只读，双向留 `rollback` 审计事件。
- 已实现：灰度/金丝雀 `internal/service/ota_rollout_governance_apply.go`（`ApplyRolloutGovernance` 含 `applyDispatchBatch`/`applyAbort`/`applyTimeout`/`applyComplete` 四个决策分支）+ `internal/dal/ota_rollout_governance.go` + `internal/api/ota.go`；金丝雀选取是**确定性的**（否则"限速下发"会退化成"一次性全推"）。
- 已实现：报告导出 `internal/service/fleet_command_job_report.go`（`GetFleetCommandJobReport` + `FormatFleetCommandJobReportCSV` + `sanitizeReportCell`）。
- 未闭环：真实设备/broker 或协议 stub 的 E2E；灰度/金丝雀治理无运行期证据文档。
- 证据：`docs/validation/P0.3-job-report-evidence.md`（真实 PostgreSQL 17.5，三条不变式全 PASS：进度 NULL 与 0 区分、租户隔离、缺租户/非法格式被拒）；单测 `ota_rollout_governance_apply_test.go`、`ota_rollout_governance_test.go`、`ota_rollout_governance_preview_test.go`、`fleet_command_job_report_test.go`。

### P0.4 场景与 Flow 语义

**交付物**：开始/结束时间、时区、过期、停止其他 Flow、定时器触发的统一语义。

**框架代码**：

    type ExecutionWindow struct { StartsAt, ExpiresAt *time.Time; Timezone string }
    func (e Engine) CanRun(now time.Time, w ExecutionWindow) bool
    func (e Engine) StopConflictingFlows(ctx context.Context, deviceID, flowID string) error

**门禁**：边界时间表驱动测试；重复触发幂等；停止动作可审计；服务重启后调度不丢任务。

**实现状态**：`partial` · 缺口类型 `未验证`。

- 已实现：`internal/service/scene_execution_window.go`——`ExecutionWindow{StartsAt, ExpiresAt, Timezone}` 与 `FlowEngine.CanRun`，区间语义为**左闭右开 `[starts_at, expires_at)`**；**时区非法一律 fail closed**（`ErrInvalidExecutionTimezone`），不静默按 UTC 兜底；`FlowTriggerKey` 按 `(flow, device, 秒级时刻)` 提供重复触发幂等；`StopConflictingFlows` 经可注入的 `FlowRunRegistry`/`FlowAuditSink` 停止同设备其他运行中 Flow，**任一侧缺失即拒绝执行**，每次停止留审计事件。
- 已实现：定时器持久化——`internal/dal/scene_automation_window.go`（`GetSceneAutomationWindows` 批量读取执行窗口）+ 迁移 91 相关表，定时器触发已落库。
- 未闭环：真实 E2E（需活栈）。
- 证据：`scene_execution_window_test.go` 7 例（含 11 行边界表驱动：前/恰在起点/窗口内/前 1ns/恰在终点/过期后/无上界/无下界/完全无界）。

### P0.5 CSV 浏览器 E2E

**交付物**：上传、校验错误展示、批量建档、一次性凭证下载、脱敏导出和清理。

**门禁**：真实浏览器选择文件；坏行逐行反馈；下载文件可解析；凭证只出现一次；跨租户产品不可选。

**实现状态**：`partial` · 缺口类型 `未验证`。

- 已实现：导入链路 `buildFilePreRegisterRows` / `readPreRegisterImportCSV`——表头严格校验为 `device_number,name`，坏行带 `csv_row` 反馈，跨租户产品校验 `validatePreRegisterProductTenant`。
- 已实现：**一次性凭证下载**——迁移 `95.sql` `device_pre_register_credential_grants`（签发/消费/过期/撤销四态，**部分唯一索引保证一批次同时只有一个 pending 许可**）；端点 `POST …/preRegister/credentials/grants`（签发）与 `GET …/grants/:id/download`（消费即失效）；**一次性的落点是数据库条件更新**（`WHERE status='pending'` → consumed，`RowsAffected=0` 即拒绝），过期先于消费判定，有 `consumed_by`/`consumed_at` 审计。
- 已实现：脱敏导出走 Excel（`utils.MaskVoucher`、按租户过滤、分批 5000、上限 20 万行，`device_preregister_export.go`）；清理执行面 `device_preregister_cleanup.go`，分流逻辑 `classifyPreRegisterCleanup`（已激活设备永不删除、跨租户 fail closed、空批次幂等）；迁移 `90.sql` 补登 `preRegister/cleanup` 的 Casbin。
- 未闭环：真实浏览器 file chooser E2E（需活栈）。
- 边界（如实）：本项**不消除** `devices.voucher` 里的明文（broker 的 MQTT 基础认证要读，去明文需等 `voucher_hash` 模式全线切换）；它限制的是**明文下发的次数**。当前单条凭证的最大明文暴露是"创建响应 1 次 + 一次性下载 1 次"。
- 证据：`docs/validation/P0.5-cleanup-execution-evidence.md`（9 例全过）、`P0.5-credential-once-download-evidence.md`（7 例含**并发 8 个下载只有 1 个成功**，含负向对照）、`P0.5-export-cleanup-evidence.md`；`device_pre_register_csv_test.go` 3 例。

### P0.6 持久化报表执行与 SMTP 事实语义

**交付物**：`83.sql`、显式 IANA 时区与 `next_run_at`、乐观 revision、不可变 `report_schedule_runs`、一对一 `report_schedule_deliveries` outbox、数据库时间驱动的 slot materialization、`SKIP LOCKED` claim、UUID fencing token、lease 续租/恢复/最终尝试收口、手动与子重试幂等、固定报表窗口、租户级 run history/detail、精确 HTTP 202/Location，以及管理员报表工作台。

**事实边界**：SMTP 仅提供 at-least-once 尝试。`accepted` 只表示 SMTP 服务器接受消息，不表示收件人最终送达；可能已接受但客户端未收到确定响应的结果必须终止为 `ambiguous`，只能由管理员显式创建带重复投递风险提示的不可变子 run，禁止静默自动重发。调度停机期间只合并为一个有用 occurrence，并记录 bounded misfire evidence，不生成无界补跑积压。

**门禁**：同一 scheduled slot 在并发副本中至多落一个 run；过期/错误 token 不能续租或结算；生成失败不得创建"成功"报表；generation success 与 delivery outbox 插入同一 fenced transaction；过期 delivery lease 进入 `ambiguous`；手动运行不改变 recurring cadence；重复 Idempotency-Key 同形状重放原结果、异形状冲突；跨租户 ID 表现为 not found；软删除保留历史且存在 active work 时拒绝；前端独立呈现 generation/delivery 状态和 SMTP 风险。

**部署约束**：这是版本 82→83 的协调切换。先停止并 drain 全部 v82 backend，再应用迁移 83 并启动 v83 lifecycle worker；禁止 v82 cron scanner 与 v83 durable worker 重叠，失败时只允许 roll-forward。

**实现状态**：`partial` · 缺口类型 `未验证` + `未接线`。

- 已实现：`backend/sql/83.sql`；`report_schedule_runs` / `report_schedule_deliveries` 表；`internal/dal` 的 `TestReportMigration83Postgres` 系列**实测通过**（materialization 隔离 poison slot、并发 materializer 只留一个 slot、并发 claim 只有一个 owner、过期 fence 不能续租/结算、过期 owner 被 reap 前丢失所有结算）。
- 未闭环：SMTP `ambiguous` 收口的运行期证据、手动运行与子重试幂等、租户级 run history/detail、精确 HTTP 202/Location、管理员报表工作台前端，以及协调切换（停 v82 → 应用 83 → 启 v83 worker）的演练记录。
- 证据：`docs/validation/P0.6-postgres-migration83-evidence.md`、`P0.6-P0.7-evidence.md`。

### P0.7 AI 凭证静态加密

**交付物**：AI provider API key 的 envelope encryption、密钥版本、轮换与不可逆 API 掩码；现有公共 HTTPS safe-egress、DNS 重验、IP pinning、禁代理/禁重定向、origin-bound Authorization 和请求/响应上限保持不变。

**门禁**：数据库与日志不出现明文密钥；缺失/错误主密钥 fail closed；旧密文可在轮换窗口读取并可重加密；创建/更新/读取/调用、跨租户拒绝和网络错误脱敏都有定向证据。

**实现状态**：`partial` · 缺口类型 `未验证`。

- 已实现：`backend/pkg/secrets` AES-256-GCM 信封加密，密文格式 `aenv1.<keyID>.<base64(nonce||ciphertext)>`；主密钥取自 `secrets.master_keys.<keyID>`（base64 的 32 字节），当前版本由 `secrets.active_key_id` 指定。
- 已实现：AAD 绑定租户（把 A 租户的密文行搬到 B 租户必然认证失败）；写入路径先封装再落库，主密钥缺失/非法/长度错误一律 fail closed；读取路径解密，遗留明文行在迁移窗口内可读并在调用时自动重写到当前主密钥（自愈式轮换）；出参只出不可逆掩码（前 4 位 + `****`）。
- 已实现：配置文件 `conf.yml` / `conf-dev.yml` / `conf.example.yml` 只写占位符，默认未配置即 fail closed。
- 未闭环：生产环境主密钥注入与"日志无明文"未验证。
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

**实现状态**：`partial` · 缺口类型 `未验证`。

- 已实现：迁移 `85.sql` `entity_relations`——唯一约束含 `tenant_id`，**CHECK 直接拒绝自环**，按起点/终点各建索引支持正查与反查。
- 已实现：`internal/model/entity_relation.go`——**关系是有向的**（反向必须显式写入，提供 `IsReverseOf` 提示而非自动生成）；实体类型走受控白名单（device/asset/customer/gateway）；关系类型长度与元数据大小有上限，超限拒绝而非静默截断。
- 已实现：DAL `internal/dal/entity_relation.go` 提供 `CreateEntityRelation`/`FindEntityRelationInTenant`/`GetEntityRelationInTenant`/`DeleteEntityRelationInTenant`/`ListEntityRelations`，均强制带 `tenant_id`（跨租户表现为未命中而非"存在但无权限"）；`internal/service/entity_relation.go` 已接线 HTTP（`router/apps/entity_relation.go`）。
- 已实现：权限集成——`api/v1/entity-relations` 系列 3 条路径在迁移 `91.sql` 完成 Casbin 登记。
- 已实现：前端 UI `src/views/device/entity-relation/`（纯模型 `entity-relation-model.ts` 与后端校验规则对齐；`index.vue` 三区块；`service/api/entity-relation.ts` **刻意不提供 tenant_id 参数**）；路由 `device_entity-relation` 已注册（`elegant-router.d.ts` 与 `transform.ts` 两处手工同步——`pnpm gen-route` 在本机报 `ERR_PACKAGE_PATH_NOT_EXPORTED`）；i18n 四语各 15 键。
- 未闭环：运行期证据文档（现有用例全为纯单测，无 PostgreSQL 常驻用例）；看板集成。
- 证据：`entity_relation_test.go` 6 例；`entity-relation-model.test.ts` 37 例、`__tests__/index.test.ts` 2 例；迁移 85 已在常驻验证库 `aetherlink_verify` 执行通过。

### P1.2 规则链可靠性

**交付物**：节点超时、指数退避、失败分支、死信、单消息 Trace、输入回放、草稿/发布版本和审计。

**框架代码**：

    type NodePolicy struct { Timeout time.Duration; MaxAttempts int; Backoff time.Duration; DeadLetter bool }
    type TraceEvent struct { ExecutionID, NodeID, Status string; Input, Output json.RawMessage; Error string; At time.Time }
    func (e *Engine) Execute(ctx context.Context, chain Chain, msg Message) (ExecutionResult, error)
    func (e *Engine) Replay(ctx context.Context, executionID string) error

**门禁**：失败节点进入 DLQ；重试次数和延迟可观测；同一消息 Trace 可串联；发布版本可回滚；回放不重复生产副作用（除非显式确认）。

**实现状态**：`partial` · 缺口类型 `未验证`。

- 已实现：`rule_chain_node_policy.go`——节点可声明 `timeout_ms`/`max_attempts`/`backoff_ms`/`dead_letter`/`retry_safe`；**有副作用节点默认禁止重试**（`action`/`external` 及注册表外未知类型即使配 `max_attempts>1` 也强制只执行一次，除非显式 `retry_safe: true`）；节点级独立超时；指数退避 `Backoff * 2^(n-1)` 单次上限 5s，等待响应上下文取消，剩余时间不足整链 deadline 时直接放弃；终局失败下沉死信（默认开启，落点 `ruleChainDeadLetterSink` 为注入点，未接线时旁路）；死信刻意不含消息载荷。
- 已实现：失败分支——`RuleChainEdge` 增加 `kind`（`success`/`failure`，**空值等价 success**，存量 graph JSON 行为不变）；被失败分支接管后该错误不再计入聚合 errs（但 trace 与死信照常记录）；失败事实以 `rc_failed_node`/`rc_error` 注入下游 metadata（只带错误文本不带原始载荷）。
- 已实现：输入回放 `rule_chain_replay.go`——`ruleChainReplayRecorder` 为可注入落点，**默认不接线**（nil 即旁路、热路径零开销）；回放**不沿图继续遍历**；**副作用闸门**命中有副作用节点时未显式确认即整体拒绝（一个节点都不执行），确认后打 `rc_replay`/`rc_replay_of`；节点类型漂移拒绝重跑；回放必须带来源执行 ID。
- 已实现：草稿/发布版本与回滚——`rule_chain_version.go` 版本单向 `draft -> published`，published 只读、不可重复发布；**回滚产生新草稿而非回写原版本**，两侧各留审计事件；版本号单调递增；**图哈希参与版本身份**；迁移 `93.sql` `rule_chain_versions` 以**部分唯一索引保证一条链同一时刻只有一个 published**；4 条端点（GET/POST versions、publish、rollback）已在 93.sql 登记 Casbin。
- 未闭环：真实链路 E2E（需活栈）。回放持久化未接数据库（迁移号位需另排）。
- 证据：`docs/validation/P1.2-rulechain-version-evidence.md`（真实 PostgreSQL 7 项全过，含"第二条 published 被部分唯一索引拒绝"，并做 `DROP INDEX` 负向对照）；常驻用例 `internal/service/rule_chain_version_postgres_test.go`（缺 DSN 或缺表一律 Skip）；`rule_chain_node_policy_test.go` 11 例、`rule_chain_failure_edge_test.go` 8 例、`rule_chain_replay_test.go` 10 例。

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

**实现状态**：`partial` · 缺口类型 `未接线` + `未验证`。

- 已实现：迁移 `88.sql` `scada_projects`（多项目容器）/`scada_documents`（草稿/发布/归档三态）/`scada_document_versions`（发布快照，不可变）/`scada_control_audits`。
- 已实现：模型 `internal/model/scada.go` + DAL `internal/dal/scada.go`——保存走**条件更新**（`WHERE current_version = ? AND status <> 'ARCHIVED'`），由 RowsAffected 判定成败，RowsAffected=0 时服务层再查一次区分"版本冲突/已归档/不存在"；`published_version` 可空（NULL 表示从未发布）；画布 JSON 超限拒绝不静默截断。
- 已实现：服务层——`scada_document.go`（项目 CRUD、画布往返、乐观并发、发布、回滚，**回滚不改历史**）；`widget_registry.go`（按 `(type, version)` 注册，能力必须显式声明；3D 降级是**逐个 Widget** 的，未注册 Widget 转 unknown，均不阻断整块看板加载）；`scada_control.go`（控制命令依次过「存在性 → 归档终态 → Widget/命令已注册 → 权限 → 二次确认」，确认令牌由 HMAC 绑定 (租户, 文档, Widget, 命令, 操作人, 过期时间)，**审计先于执行落库**，被拒命令同样留痕）；`scada_telemetry_link.go`（未连接时数据一律判为陈旧，不参考最后一帧有多新）。
- 已实现：HTTP 与路由——`internal/api/scada.go` + `router/apps/scada.go` 共 19 条端点，由 `router/apps/scada_routes_test.go` 在真实 Gin 引擎上校验注册结果；控制端点在服务未接线时 fail closed。
- 已实现：控制服务装配——`internal/app/scada_mobile_wiring.go` + `main.go`；`service.AssembleScadaControl` 注入内置 Widget 注册表、二次确认签发器与真实下发执行器；**下发必须携带真实 claims**（`ControlExecution.ActorClaims` 为空时执行器直接拒绝）；密钥未配置不阻断启动但打 warn。
- 已实现（2026-09-14，`36fd6da`）：**Widget 配置真实 schema 与保存链路校验**——四个内置 Widget（gauge/chart/valve/twin3d）schema 从 `{}` 占位换成真实字段（全可选、类型/取值约束：maxLength/minimum/maximum/enum/maxItems，存量画布不受影响）；`AssembleScadaControl` 回传注册表并注入 `ScadaDocumentService`（保存与控制的已注册判定同源）；`CreateDocument`/`SaveDocument` 两条写路径按 schema 校验每个 Widget 实例；`ValidateCanvasJSON` 兼容两代画布形状（旧 `widgets[].config` + 新 `nodes[].props`）；前端 WIDGET_REGISTRY 同步同一组 schema（parity 测试守护）；新增 3 个保存链路校验用例。
- 已实现：前端——`src/views/visualization/scada-editor/`（**已挂路由**）含 `scada-model.ts` 纯模型、`index.vue`、`service/api/scada.ts`、i18n 四语各 34 键；另有**更新的 `src/views/scada/`**（`core/symbolLibrary.ts` 工业符号库七类 valve/pump/vessel/motor/sensor/pipe/electrical、`core/useCanvasEditor.ts` 拖拽/缩放 hook、`core/canvasDocument.ts`）——见 `adeaf80`。
- 未闭环（2026-09-14 刷新）：~~新 `views/scada/` 编辑器未挂路由~~ 已挂（见 §1.3-B-2 白屏条目附带的五处路由注册）；~~widget schema 无真实字段、后端不校验画布内单个 Widget 配置~~ 已闭环（见上 `36fd6da`）；剩余：widget 注册表仍是前后端各一份常量（有一致性测试兜底，未改为前端从后端拉取）；无浏览器/运行时 E2E；**未做真实下发联调**。
- 证据：`docs/validation/P1.5-P3-completion-batch-20260912.md`；`scada_postgres_test.go` 4 例真实 PostgreSQL（复合唯一约束、jsonb 往返、乐观并发、审计 pending→success、状态 CHECK）；`scada-model.test.ts` 24 例；`TestBuiltinWidgetRegistryMatchesFrontend`（已用「改前端版本号 → 用例失败」做过负向对照）。

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

**实现状态**：`partial` · 缺口类型 `未实现` + `未验证`。

- 已实现：`internal/service/edge_governance.go`（纯决策函数）——`ClassifyEdgeNodeHealth` 按最后心跳判定 online/degraded/offline/**unknown**（心跳为 nil、零值或晚于当前时间一律 unknown）；`CheckEdgeVersionCompatibility` 点分数字版本比较，**版本串为空或含非数值段一律判不兼容**；`DetectEdgeSyncConflict` **同一资源若已有一份内容不同的在途快照即判冲突**，只检测上报、绝不自动合并或自动覆盖，内容一致视为幂等重发，在途快照解析失败按"内容不同"处理。
- 已实现：同步修订号 `edgeSyncPayload.Revision` + `EdgeSyncRevisionFromHistory`（回答"边缘拿到了哪一版"）。
- 已实现：迁移 `97.sql` `edge_nodes`（节点自报身份、capabilities JSON、status active/revoked、last_seen_at 可空）。
- 已实现：`internal/dal/edge_node.go`（`GetEdgeNodeByID` **故意不带租户过滤**供跨租户抢注判定、`GetEdgeNodeInTenant`、`InsertEdgeNode`、`UpdateEdgeNodeRegistration` 条件更新 status='active'（revoked 不续命）、`TouchEdgeNodeHeartbeat`、`ListEdgeNodesInTenant`）。
- 已实现：`internal/service/edge_node_service.go`——注册（`EvaluateEdgeNodeRegistration` 接线，含抢注拒绝）、心跳（复用注册判定 + `ClassifyEdgeNodeHealth`）、**Reconcile 编排**（健康闸门 + 版本闸门（最低版本读 `edge.min_compatible_version`，未配置=一律不兼容 fail closed）+ `PlanEdgeReconcile` 逐资源修订号比对 + `sync` 项经 `CreateEdgeSync` 真实下发 + `needs_attention` 整批停止自动下发）。
- 已实现：路由 `POST/GET /edge/nodes`、`POST /edge/nodes/:node_id/heartbeat`、`POST /edge/nodes/:node_id/reconcile`（97.sql 登记 Casbin，SYS_ADMIN/TENANT_ADMIN），Casbin 契约测试挂入 EdgeSync 组。
- 未闭环：**节点证书签发**（复用 D5 X.509 为接线点）、**远程升级回滚**、真实边缘节点联调与断云演练（需活栈）。
- 证据：`docs/validation/P1.5-P3-completion-batch-20260912.md`；`edge_governance_test.go` 11 例、`edge_node_registry_test.go`、`edge_reconcile_test.go`、`edge_sync_revision_test.go`（决策层）；94–97.sql 已在真实 PostgreSQL 17（`aetherlink_v97_test`）实跑通过，含一板一项目唯一索引与节点状态 CHECK 的负向对照。

### P1.6 模板市场产品化

**交付物**：浏览/搜索/行业打包下载、导入冲突预览、签名、依赖检查、升级/回滚和审计。

**门禁**：租户幂等；坏签名/坏依赖拒绝；升级可回滚；导入不产生孤儿租户数据；所有动作有审计记录。

**实现状态**：`partial` · 缺口类型 `未验证` + `未接线`。

- 已实现：`device_template_market_integrity.go`（纯逻辑）——`MarketBundle` 增加 `digest`/`signature`/`signed_key_id`（均 `omitempty`，老包解析不受影响），摘要覆盖**除签名三字段外的规范 JSON**；`SignMarketBundle`/`VerifyMarketBundle` HMAC-SHA256，**摘要与签名都用常量时间比较**，验签按"未签名 → 密钥缺失 → 摘要不符 → 签名不符"逐级拒绝；签名密钥（`market.bundle_signing_keys`）与 P0.7 的加密主密钥**分开**，需 base64 且不小于 32 字节；`CheckMarketBundleDependencies` 包内自洽检查；`PreviewMarketBundleImport` **只读**，区分 create/overwrite/blocking。
- 已实现：`ExportMarketBundle` 出包即签名，**未配置签名密钥一律拒绝出包**（打包导出端点在配置密钥前不可用，属刻意行为变更）。
- 已实现：导入链路 `device_template_market_import.go` + 端点 `POST /api/v1/device/template/market/bundle/import`（含 `preview=true` 只读预览）——`VerifyMarketBundle`（验签 fail closed）→ `CheckMarketBundleDependencies`（阻断即拒）→ `PreviewMarketBundleImport` → 覆盖项需显式 `confirm_overwrite=true` → 逐模板 `ImportDeviceTemplateWithTenant`（租户幂等）→ 逐模板 `EmitMarketTemplateImportAudit`（created/idempotent/rejected 全留痕，失败不中断整包）。
- 已实现：**模板升级/回滚**——迁移 `99.sql` `device_template_upgrade_history`（`previous_payload` 存旧版本完整导出载荷，即回滚凭据本身）+ 三条新路由 Casbin；服务 `device_template_upgrade.go`：升级 = 目标版本**严格新于**当前（点分数字逐段比较，降级必须走回滚通道）→ 捕获旧行完整导出载荷 → 经租户幂等导入新版本 → 落历史（历史落库失败如实报错）；**回滚 = 重放旧载荷，不删任何行**（删行不可逆且牵连设备配置引用）；端点 `POST template/upgrade`、`POST template/upgrade/:history_id/rollback`、`GET template/upgrade/history`。
- 未闭环：升级/回滚的运行期证据（**98/99.sql 未在 PostgreSQL 实例复跑**，因 `al_pg_verify` 出现 0xC0000142 后端崩溃）；预览/确认闸门的浏览器端 UI 未接入。
- 证据：`device_template_market_integrity_test.go` 11 例、`device_template_market_import_test.go`、`edge_node_upgrade_service_test.go`；`docs/validation/P1.5-P3-completion-batch-20260912.md`。

## P2：生态、分析与规模

### P2.1 协议插件 SDK

**框架代码**：

    type ProtocolAdapter interface { ValidateConfig(any) error; Connect(context.Context) error; Discover(context.Context) ([]Device, error); ReadTelemetry(context.Context) (Telemetry, error); WriteCommand(context.Context, Command) error; Health(context.Context) Health; Close() error }

**交付物**：插件 manifest、配置 Schema、点表、凭证映射、指标、版本兼容和签名；按客户需求接入 CAN/BACnet/BLE/LoRaWAN。

**实现状态**：`partial` · 缺口类型 `未实现`。

- 已实现：删除 `internal/roadmap` 死包（含 `Unwired*` 骨架与 `ErrNotImplemented` 占位；零 import 两次独立 grep 复核、已提交可完整恢复）。
- 已实现：`pkg/pluginsdk`（叶子包，纯标准库）——`ProtocolAdapter` 接口（全部带 ctx，与草图偏差在 doc 注释说明）、`Manifest` + `ParseManifest`/`Validate`（名称字符集/点分数字版本/transport 白名单/宿主兼容 fail closed/点表重名拒绝/凭证字段只声明不承载值）、与 widget schema 同语义的配置 Schema 校验器（未知关键字编译期拒绝，刻意不共享代码——SDK 须独立分发）、`signing.go` Ed25519 厂商签名（厂商私钥签、平台公钥验，与 P1.6 的 HMAC 对称方案刻意区分）。
- 已实现：真实消费方——`PluginRegistryService.Create` 接受可选 `manifest` 字段，提供时必须通过 `pluginsdk.ParseManifest`；**manifest 带厂商签名就必须验过**（`plugin.trusted_vendor_keys` 配置 key_id → base64 ed25519 公钥，坏公钥条目报错而非跳过，未签名允许——D9 兼容过渡）；原始 JSON 落 `plugin_registries.manifest` 列（97.sql，可空）。
- 未闭环：真实外部协议适配器（CAN/BACnet/BLE/LoRaWAN 按客户需求接入）；manifest 注册的 HTTP 运行期路径（服务层依赖无法离线编译，测试以源码交付）。
- 证据：`GOTOOLCHAIN=local go test ./pkg/pluginsdk/ -count=1` → **9/9 全过**（宿主兼容表、manifest 合法/8 类破坏输入、schema 贯通含未知关键字拒绝、integer 拒非整数、接口编译期锚点、签名往返/篡改/未知厂商/空受信表/非法 manifest 拒签）；go build / go vet 同过。

### P2.2 Trendz 类轻量分析

**框架代码**：

    type AnalysisQuery struct { DeviceIDs []string; Keys []string; From, To time.Time; Granularity string; Aggregations []string }
    func (s *AnalyticsService) Query(ctx context.Context, q AnalysisQuery) (AnalysisResult, error)
    func (s *AnalyticsService) Export(ctx context.Context, q AnalysisQuery, format string) (io.ReadCloser, error)

**交付物**：多设备对比、聚合/同比环比、基础异常、CSV/Excel、权限和分享。P0.6 先提供可靠的定时报表执行、历史与 SMTP 事实语义；本阶段在该 durable execution contract 上扩展分析查询和展示，不再创建第二套调度系统。

**实现状态**：`partial` · 缺口类型 `未验证`。

- 已实现：多设备对比 / 聚合 / 同比环比 / CSV·Excel 导出与逐设备权限复查（`telemetry_analysis*.go` + `96.sql` Casbin）。
- 已实现：**基础异常检测**——`telemetry_analysis_anomaly.go` 提供 `bounds`（静态上下限）与 `deviation`（均值±Kσ，默认 3）两种规则；空序列报"无数据"而非"无异常"；σ=0 显式零命中；编排 `RunTelemetryAnomalyDetection` 复用分析服务取数缝与设备权限缝，单设备失败不中断多设备检测；端点 `POST /api/v1/telemetry/analysis/anomaly`（97.sql Casbin，角色与既有分析路由一致）。
- 已实现：仪表盘分享——`boards` 的 `Published`/`ShareToken` 列、`PublishBoard` 签发、公开路由 `GET /api/v1/board/shared/:token`（注册于 JWT 之前）。
- 未闭环：anomaly 的运行期证据（需环境）；P0.6 durable execution 仍 partial——在其上扩展的定时报表类分析仍受同一约束。
- 证据：`telemetry_analysis_core_test.go`、`telemetry_analysis_export_test.go`；`docs/validation/P1.5-P3-completion-batch-20260912.md`。

### P2.3 数据保留与性能

**交付物**：保留策略、降采样、冷热分层、查询缓存、基准压测、容量模型和告警。

**门禁**：明确 p95/p99、吞吐、数据完整性和降级行为；至少单实例和双实例报告；压测不使用假数据掩盖数据库瓶颈。

**实现状态**：`partial` · 缺口类型 `环境阻塞`。

- 已实现：保留策略每日 2 点 cron（真实执行）。
- 已实现：**降采样**——`telemetry_rollups` 冷层表（97.sql，1h 桶 min/max/avg/last/count）；DAL `internal/dal/telemetry_rollups.go`（汇总 upsert ON CONFLICT 覆盖、**count 加权合并 avg**、min/max/count/last/sum 精确合并）；作业 `telemetry_downsample.go`（`telemetry.downsample.enabled` 门控**默认关闭**，仅直连数据库模式，外部 TSDB 显式跳过，**不删原始数据**）；cron 每日 3 点；分析查询对整窗冷数据回落冷层（`fetchTelemetryAnalysisSeries`，不跨层拼接）——rollup 表有真实读方。
- 已实现：**查询缓存**——`telemetry_analysis_cache.go` 分析取数进程内 TTL 缓存（`telemetry.analysis_cache.enabled` 默认关闭，TTL 300s，4096 条护栏；多实例各自回源的取舍已注明）。
- 未闭环：**基准压测 / 容量模型 / 冷热分层告警——本机系统盘约 11 GB 且无网络，压测前提不成立，保持 pending**；rollup SQL 与冷读路径需真实 PostgreSQL 验证（rollup 数学已在 `aetherlink_v97_test` 真 PG 验证：2 行同桶 → min=5/max=7/avg=6/last=7/count=2，upsert 幂等）。
- 证据：`docs/validation/P1.5-P3-completion-batch-20260912.md`。

## P3：商业化与长期能力

**交付物**：多地域/高可用故障演练、RPO/RTO 和滚动升级；计费/配额/审计导出、客户自助开通和商业许可证边界；移动端正式商店发布、桌面运维工具和行业解决方案包；生态市场运营、第三方插件签名和供应链扫描。

**实现状态**：`partial` · 缺口类型 `未实现`。

- 已实现：**离线商业许可证**——`pkg/license`（纯标准库 Ed25519；`Document` 全字段在签名内——时间窗/特性/配额伪造必破坏签名；多公钥轮换；逐级拒绝哨兵）。
- 已实现：执法点（默认全关，`license.public_keys` 未配置=边界未启用，既有部署行为不变）——① 启动门控（`license.required=true` 时 main.go 启动前必须持有效许可证）；② 设备配额（`max_devices>0` 且许可证有效时 CreateDevice 前置 `enforceDeviceQuota`，`dal.CountAllDevices` 部署级计数）；③ 状态查询 `GET /api/v1/license/status`（SYS_ADMIN，97.sql Casbin，不返回材料，返回验证结论+SHA-256 摘要）。
- 已实现：**审计导出**——`POST /api/v1/operation_logs/export`（`internal/api/operation_log.go` + `service/audit_export.go` + `dal.ListOperationLogsForExport`）；口径为当前租户，时间窗必填且 ≤1 年，行数上限 10 万，**request/response 载荷列刻意不导出**；迁移 `98.sql` 登记 Casbin。
- 已实现：**第三方插件签名**——`pkg/pluginsdk/signing.go` Ed25519 厂商签名 + 注册侧验签（见 P2.1）。
- 未闭环：**许可证签发工具**（验证侧先行，签发侧另列）；`max_tenants` 无执法点（全仓无独立"创建租户"服务/端点，留待租户自助开通时一并接线）；**其余 11 个子项零代码**——多地域/HA 演练、RPO/RTO、滚动升级、计费/配额（除设备配额）、客户自助开通、移动端商店发布、桌面运维工具、行业解决方案包、生态市场运营、插件供应链扫描。
- 证据：`GOTOOLCHAIN=local go test ./pkg/license/ -count=1` → **6/6 全过**（有效往返、篡改拒绝、过期/未生效、未受信密钥、坏公钥表、特性门）；`docs/validation/P1.5-P3-completion-batch-20260912.md`。

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

1. 重生成 OpenAPI（`cd backend && go run ./cmd/openapigen -out docs/openapi/openapi.json`，swag 注解已备）——补齐"四面一致"的 API 面。
2. SCADA 新 `views/scada/` 编辑器挂路由（符号库与拖拽已就绪，仅差路由注册）。
3. 补 anomaly / 打包导入 / 报表工作台 的前端 UI。
4. 补 edge / license / anomaly / bundle-import / operation_logs-export 的自动化 E2E 用例。
5. 补运行期证据文档：P1.1 实体关系（PostgreSQL 常驻用例）、P0.3 灰度治理、P2.2 anomaly、P1.5/P1.6 新端点、98/99.sql 在 PostgreSQL 复跑。

**第二优先（需恢复环境：Go 模块缓存 / Docker / 磁盘空间）**：

6. P0.1 部署门禁（HTTPS/TLS、MQTTS、公网 MQTT、backup/restore 计数一致性）→ P0.2 真实 MQTT `shadow_ack` E2E → P0.3 真实设备/协议 stub E2E → P0.4 真实 E2E → P0.5 浏览器 file chooser E2E → P0.6 报表工作台 + v82→v83 切换演练 → P0.7 生产主密钥注入与"日志无明文"验证。
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

| # | 缺口 | TB 来源 | 本地现状 | 缺口类型 | 前提与依赖 | 量级 | 立项建议 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| TB-1 | 告警规则 2.0（可配置规则对象、条件/严重度/传播、告警生命周期与指派评论） | 4.3.0 `#14036`；3.5 告警指派与评论 | **已开工（2026-09-14，第一片已闭环）**。核对修正：① 告警历史/确认/备注已有；② **"指派"其实部分存在**——`alarm_info.processor` 就是当前处理人字段（原写"无指派"不准确）；③ `alarm_info` 是**已废弃**的 device-less 旧表（见 `service/alarm_notification.go` 的 `createAlarmInfoRecord` 注释），现代告警走 `alarm_history`。**本轮补齐：告警评论**（`alarm_comment` 表 + POST/GET/DELETE 三条路由 + Casbin）。**仍缺**：可配置规则条件、严重度传播、指派历史审计 | `部分实现` | 剩余部分需改规则求值链路；与现有 `rule_chain` 告警节点划清边界 | L（剩余 L，已完成 S 片） | **继续立项（高）**：评论片已落地，剩余"条件/传播/指派审计"按原计划排 |
| TB-2 | 计算字段高级形态：地理围栏、实体间传播、关联实体聚合、输出策略 | 4.3.0 `#13857/#14107/#14141/#14225`；4.0 计算字段 | `calculated_field` 路由存在，为较基础形态 | `未实现` | 依赖实体关系（P1.1，已落地）与地理位置字段 | L | **建议立项（中）**：地理围栏需地图 provider（当前属可选外部能力），可先做传播与聚合 |
| TB-3 | EDQS 级高性能实体数据查询（内存型实体查询服务） | 4.0.0 `#12527`，4.0.2 持续改进 | 常规 SQL 路径 + 冷层 rollup | `未实现` | 需引入缓存/索引层；与 P2.3 降采样冷层协同 | XL | **不建议近期立项**：收益依赖规模，先用 P2.3 压测量化瓶颈再决定 |
| TB-4 | 移动应用中心 + 白标移动端 | 3.9.0 `#11835`；PE 白标 | 无客户端工程（P1.4 缺口同源） | `客户端缺失` | 依赖 P1.4 移动端立项决策 | XL | **与 P1.4 合并立项**：先出 Android/iOS 客户端，再谈应用中心与白标 |
| TB-5 | LPWAN / 系统集成（LoRaWAN、Sigfox、AWS IoT、Azure、PubSub、Kafka） | PE 专属（CE 无） | 无对应集成 | `未实现` | 需真实云账号与网络出口；Kafka 需独立中间件 | L（每项 M–L） | **按客户需求单项立项**：无客户时不做，避免建无消费方抽象 |
| TB-6 | 400+ 设备载荷编解码库 + 解决方案模板库 | PE 专属；3.6.2 工业控件包 | 仅 `payload_schema` + 自建模板市场 | `未实现` | 内容型资产，需持续维护 | XL | **不建议复制**：改为"模板市场 + 厂商签名（P1.6/P2.1 已具备）"的生态路径 |
| TB-7 | HAProxy 级速率/连接限制、多队列隔离、Cassandra/Timescale 可选后端 | 3.6.3 队列隔离；4.0 弃 Timescale | 单库 + 进程内缓存；限流为进程内计数 | `未实现` | 多实例部署前提；共享存储计数 | L | **建议立项（中）**：集群化必做项，建议与 P3 多地域/HA 一起排 |
| TB-8 | Timewindow 重设计、动态表单、Dashboard 布局断点 | 3.8.0 `#11633`/`#11430`；4.0 动态表单 | 看板能力较基础 | `未实现` | 前端改造为主 | M–L | **建议立项（中）**：纯前端收益，不依赖后端环境，可优先排 |

### 7.2 相对 ThingsPanel（社区版 / 企业版宣称）的缺口

> 版本锚点（2026-09-14 GitHub Releases API 全量复核，51 个 release，2022-04 → 2026-09-03）：最新 **v1.2.11（2026-09-03）**；演进锚点 v1.1.10 多层网关 / v1.1.12 共享订阅+移动推送 / v1.2.0 遥测聚合 / v1.2.2 模拟遥测 / v1.2.8 资源中心 / v1.2.9 模板封面与令牌续期。

| # | 缺口 | TP 来源 | 本地现状 | 缺口类型 | 前提与依赖 | 量级 | 立项建议 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| TP-1 | 移动客户端（uniapp App + 小程序） | 社区版即有（`ThingsPanel/app`，58★） | 无客户端工程 | `客户端缺失` | 同 P1.4 / TB-4 | XL | **与 P1.4 合并立项**，三处缺口一次解决 |
| TP-2 | 大屏编辑器 | 企业版；社区版无 | Native Board 基础能力 + ThingsVis 可选外部集成 | `未实现` | 与 P1.3 SCADA 画布可复用 | L | **建议立项（中）**：优先合并 P1.3 两条 SCADA 编辑器，再谈大屏 |
| TP-3 | 多层网关（网关→子网关→终端） | 1.1.10 | 未见对应实现 | `未实现` | 需网关拓扑模型与多级上下行路由 | L | **建议立项（高）**：工业场景常见，且与既有 MQTT 网关能力衔接 |
| TP-4 | 设备诊断页 / GMQTT 管理 Web 界面 / Topic 映射配置页 | 1.1.11 | `topic_mapping`、`device_debug` 代码存在，无独立管理界面证据 | `未接线` | 后端能力已具备，缺前端页面 | S–M | **建议立项（高）**：投入小、补齐"四面一致"的 UI 面，性价比最高 |
| TP-5 | 资源中心（设备模板 + 大屏模板统一市场） | 1.2.8 | 模板市场已建，无资源中心形态 | `未实现` | 复用 P1.6 打包/签名/导入链路 | M | **建议立项（中）**：把 P1.6 已有能力产品化，边际成本低 |
| TP-6 | 算法中心（设备健康 / MSET） | 企业版 | 无 | `未实现` | 需算法与训练数据；MSET 属专有算法 | XL | **不建议近期立项**：需客户场景驱动，先做 P2.2 异常检测的规则版 |
| TP-7 | 国产化环境适配（麒麟/UOS/Deepin）与国产数据库（TDengine/KingBase） | 企业版宣称 | 无 | `未实现` | 需目标操作系统与数据库实机 | L | **按客户/合规需求立项**：无国产化要求时不做 |
| TP-8 | 设备分组统计、模拟遥测数据初始化/发送接口 | 1.2.3 / 1.2.2 | **已结案（2026-09-14 运行期验证）**：① 模拟遥测早已可用——`router/apps/telemetry_data.go:36-39` 四条路由、3 个 path 已入 OpenAPI。② 分组统计本轮补齐：新增批量 DAL `GetDeviceGroupStatisticsBatch`（单条递归 CTE，替代 3N 次往返），挂到 `GET /device/group/tree` 每个节点的 `statistics` 与 `GET /device/group` 列表项，**加法变更**（原有 group 字段不变）。实测父分组正确汇总子孙设备（parent=2 / child=1），tree 与 list 数值一致 | `已闭环` | 无 | S | **结案，无需立项**。证据 `automation_tests/scripts/verify-group-statistics.js` + `internal/dal/device_groups_statistics_test.go`（含 PostgreSQL 门控等价性用例） |

### 7.3 立项优先级建议（综合两平台）

**第一梯队（建议立即立项，投入小或刚需）**

1. ~~`TP-4` 设备诊断 / GMQTT 管理界面 / Topic 映射页~~ → 已接线（四个组件均有挂载点），只差浏览器证据。
2. ~~`TP-8` 设备分组统计 + 模拟遥测数据接口~~ → **已闭环（2026-09-14）**，见 §7.2 该行。
3. `TB-1` 告警规则 2.0——工业刚需，可复用既有告警链路。**（现为第一梯队唯一未开工项）**

**第二梯队（建议排期，中等投入）**

4. `P1.4 + TB-4 + TP-1` 合并的移动端工程（客户端 + 应用中心 + 白标）。
5. `TP-3` 多层网关。
6. `TP-5` 资源中心（复用 P1.6）。
7. `TB-8` 看板/Timewindow/动态表单（纯前端，不依赖环境）。
8. `TB-7` 队列隔离与集群化（与 P3 多地域/HA 合并）。

**第三梯队（需客户或规模驱动，暂不立项）**

9. `TB-3` EDQS、`TB-5` LPWAN/系统集成、`TB-6` 400+ 编解码库、`TP-6` 算法中心、`TP-7` 国产化适配、`TB-2` 计算字段高级形态。

**明确不做**：复制完整 TBMQ / Trendz / 600+ Widget / 多地域 SaaS 计费体系。

### 7.4 立项前置检查清单

任何一项在开工前必须回答：

1. 交付物是什么（迁移 / DAL / service / API / UI / 配置 / 文档）？
2. 契约测试覆盖了成功、失败、越权、幂等、超时、降级吗？
3. 运行证据在哪个真实依赖上跑、结果存到 `docs/validation/` 了吗？
4. API/OpenAPI、后端权限、UI 行为、自动化 E2E 四面是否对齐？
5. 缺口类型是哪一类（§1.0）？"只差跑一遍"的不要按"要开发"排期。
