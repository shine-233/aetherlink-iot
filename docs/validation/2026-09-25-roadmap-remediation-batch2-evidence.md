# 2026-09-25 路线图缺口批次二实施证据（TB-04 / TB-17 / TB-18 / TB-27 / TB-10 / TB-25 / TB-15 / TB-41 / TB-21）

> 对应 `ROADMAP.md` §9"2026-09-25 批次二实施记录"；缺口编号沿用缺口体系（§1.5）。
> 分支 `feature/roadmap-complete-tb-tp-parity`。
>
> **口径声明（重要）**：本证据文档由路线图与证据更新会话按批次实施结果整理——实施工作由各并行批次实施会话完成，本文逐项内容（交付说明 / 改动文件 / 测试 / residual）**转录自实施会话回报**；更新会话仅对关键落点做了**存在性抽查与迁移链核验**（见 §6 实跑记录），**未复跑任何测试**。凡本文出现的"通过/全绿"结论，其执行主体均为实施会话或编排方收尾门禁，更新会话不为其背书。

## 0. 批次总览与状态口径

- 本批次实施结果（实施会话回报）共 **9 项，全部 delivered**：TB-04、TB-17、TB-18、TB-27、TB-10、TB-25、TB-15、TB-41、TB-21。（修订注记：初稿此处误写"10 项任务编号"，与 §5"9 项"口径矛盾，复核指出后已更正——实施结果实列 9 项 delivered；任务书另列本批**明确不做**的 13 项（含 TB-46 用户组/GPE 等，见 §4），本批未动其代码面。）
- **无 blocked 项、无已回退项**（见 §5）。
- **状态口径**：按 ROADMAP §2 四面一致定义，第④面（自动化测试运行）缺——契约测试 78~86 已就位、按批次约定**未在活栈运行**（本机无 docker）——故 ROADMAP §4.1 九个已交付行与 §3 新增九行状态一律记 **partial†**，待活栈回归后升 done。
- **迁移链**：本批次新增 `123.sql`~`129.sql`（TB-04/17/18/27/10/25/41 各一个迁移；TB-15 为 `57.sql` 就地修正、TB-21 无新迁移）。更新会话实测：`backend/sql/` 下 .sql 文件 129 个（最小 1、最大 129，无缺号），`backend/pkg/global/global.go:21` `VERSION_NUMBER = 129`，二者一致。
- **收尾门禁（任务书下达口径）**：`openapi=0`、`goTest=0`、`vue-tsc=0`、`vitest=0`（见 §2）。

## 1. 逐项交付记录

### 1.1 TB-04 部件库 widget_bundles —— delivered

**交付说明**（实施会话回报）：`123.sql` 建表+Casbin+菜单种子；后端 api/service/dal/router 四层 CRUD 与内置四部件种子导出/幂等落库；资源中心打包/验签/导入/一键应用接入 widget_bundle 类型；前端管理页+API 封装+路由四件套+四语言。

**迁移**：`backend/sql/123.sql`

**改动文件**：
- 后端：`backend/sql/123.sql`、`backend/pkg/global/global.go`、`backend/internal/model/widget_bundle.go`、`backend/internal/dal/widget_bundle.go`、`backend/internal/service/widget_bundle.go`、`backend/internal/service/widget_bundle_test.go`、`backend/internal/api/widget_bundle.go`、`backend/router/apps/widget_bundle.go`、`backend/internal/api/enter.go`、`backend/internal/service/enter.go`、`backend/router/apps/enter.go`、`backend/router/router_init.go`、`backend/internal/model/device_template_market.go`、`backend/internal/service/device_template_market_integrity.go`、`backend/internal/service/resource_center.go`、`backend/internal/dal/resource_center.go`、`backend/router/apps/casbin_route_registration_contract_test.go`
- 前端：`frontend/src/service/api/widget-bundle.ts`、`frontend/src/service/api/index.ts`、`frontend/src/service/api/__tests__/index.exports.test.ts`、`frontend/src/views/visualization/widget-bundles/index.vue`、`frontend/src/router/elegant/visualizationRoutes.ts`、`frontend/src/router/elegant/imports.ts`、`frontend/src/router/elegant/transform.ts`、`frontend/src/typings/elegant-router.d.ts`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/route.json`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/custom.json`

**测试**：
- Go 单测（实施会话回报 8 例通过）：`backend/internal/service/widget_bundle_test.go` 的 TestValidateWidgetsJSON、TestExportBuiltinWidgetBundle、TestSeedBuiltinWidgetBundleIdempotency、TestWidgetBundleCRUDAndTenantIsolation、TestImportWidgetBundleWithTenantIdempotent、TestCheckMarketBundleDependenciesWidgets、TestAppendWidgetBundlePreview、TestMarketBundleCanonicalCoversWidgets。
- 契约测试：`automation_tests/tests/78_widget_bundles.test.js`（17 例，**待活栈运行**）。

**门禁**：go build + 定向 go test + vue-tsc 全过（第 1 轮，实施会话自报）。

**residual 与未做项**（实施会话回报）：
1. 明确不做项：看板/SCADA 画布运行时改为从 bundle 加载部件未接线（DefaultWidgetRegistry 未改动，scada parity/registry 存量测试通过，内置部件行为不回归）；内置部件经 bundle 加载的运行时闭环列为后续项。
2. OpenAPI 重生成未执行（编排方统一执行），新端点 @Router 注释已就位。
3. 契约测试按约定未在活栈运行（本机无活栈）；其资源中心验签导入用例假定活栈已配置 market.bundle_signing_keys（与 53_resource_center_market.test.js 同口径）。
4. 全量 go test / 全量 vitest 由编排方统一执行；注：按 ask 指定命令 `node node_modules/vitest/vitest.mjs run -u src/service/api/__tests__/index.exports.test.ts` 更新 barrel 快照时，该命令实际执行了全部 441 个前端测试文件（3970 用例全部通过），属该命令自身行为而非主动扩跑。
5. 表结构在 ask 列出的核心列外额外加了 version/type_key 两列（资源中心版本冲突预览与行业目录聚合必需，已在 123.sql 头注释说明）。

### 1.2 TB-17 租户配额执法（API 维度）—— delivered

**交付说明**（实施会话回报）：租户 API 日配额执法：`124.sql` 建 api_usage_daily 计量表+casbin/菜单种子；新 `internal/quota` 包在 TenantRateLimit 既有路径上 Redis INCR 同日累加+定期落库（fail-open），按 billing 套餐 max_api_calls_per_day 执法返回 429+Retry-After（与既有 429 同构）；新增 GET /billing/api-quota 配额查询端点与前端配额页（菜单+四语言）。

**迁移**：`backend/sql/124.sql`

**改动文件**：
- 后端：`backend/sql/124.sql`、`backend/pkg/global/global.go`、`backend/internal/model/api_usage.go`、`backend/internal/dal/api_usage.go`、`backend/internal/quota/decision.go`、`backend/internal/quota/meter.go`、`backend/internal/quota/limit.go`、`backend/internal/quota/service.go`、`backend/internal/quota/api_daily_incr.lua`、`backend/internal/middleware/tenant_rate_limit.go`、`backend/internal/api/billing.go`、`backend/internal/service/billing.go`、`backend/router/apps/billing.go`
- 前端：`frontend/src/views/billing/api-quota/index.vue`、`frontend/src/router/elegant/billingRoutes.ts`、`frontend/src/router/elegant/routes.ts`、`frontend/src/router/elegant/imports.ts`、`frontend/src/router/elegant/transform.ts`、`frontend/src/typings/elegant-router.d.ts`、`frontend/src/service/api/billing.ts`、`frontend/src/service/api/index.ts`、`frontend/src/service/api/__tests__/index.exports.test.ts`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/route.json`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/custom.json`

**测试**：
- Go 单测 **22 例**（修订注记：实施会话原回报口径"20 例"，与其 testsAdded 明细 22 个函数名不符；复核会话以 `grep -c "^func Test"` 实测 decision_test.go 8 + meter_test.go 9 + tenant_daily_quota_test.go 5 = **22**，函数名与明细逐一吻合，**以代码实测 22 为准**）：`backend/internal/quota/decision_test.go`（TestDecideDailyQuotaUnlimitedWhenLimitNotConfigured、TestDecideDailyQuotaAllowsBelowThreshold、TestDecideDailyQuotaWarnsAtEightyPercent、TestDecideDailyQuotaAllowsLastCallAndMarksExceeded、TestDecideDailyQuotaBlocksOverLimitWithRetryAfter、TestDecideDailyQuotaRemainingNeverNegative、TestSecondsToNextUTCMidnight、TestUsageDateUsesUTC）；`backend/internal/quota/meter_test.go`（TestDailyMeterMemoryIncrAccumulatesPerDay、TestDailyMeterResetsOnDayRollover、TestDailyMeterSeedsFromPersistedSnapshotOnColdStart、TestDailyMeterRedisIncrSharesCountAndSetsTTL、TestDailyMeterRedisSeedsMissingKeyFromSnapshot、TestDailyMeterRedisFailureFallsBackToMemoryAndFailsOpen、TestDailyMeterSeedErrorPropagatesAsFailOpenSignal、TestDailyMeterFlushOncePersistsCanonicalCounts、TestBillingLimitSourceReadsPlanThroughDalCacheAndStaleFallback）；`backend/internal/middleware/tenant_daily_quota_test.go`（TestDailyQuotaBlocksOverLimitWith429Contract、TestDailyQuotaAllowsUnderLimitAndUnlimited、TestDailyQuotaFailsOpenOnMeteringError、TestDailyQuotaSkipsWhenNoTenantContext、TestDailyQuotaStillEnforcedWhenPerMinuteLimitDisabled）。
- 契约测试：`automation_tests/tests/79_billing_api_quota.test.js`（**待活栈运行**）。

**门禁**：go build + 定向 go test + vue-tsc 全过（第 1 轮，实施会话自报）。

**residual 与未做项**（实施会话回报）：
1. transport 维度（broker/网关路径接线）与 CheckTenantTransport/CheckDeviceTransport 的生产调用点未做——按本项范围明确排除。
2. 契约测试 79 未实际执行（本机无活栈，按约定只写不跑；已通过 `node --check` 语法校验）。
3. 计量语义：被日配额拒绝的请求同样计入当日用量（计费口径"尝试调用即消耗"，已在 internal/quota/decision.go 注释固化）。
4. 日窗口按 UTC 日期切分（usage_date），不按租户时区；Retry-After=距次日 UTC 零点秒数。
5. api_usage_daily 是定期快照（默认 30s，GREATEST 幂等只增不减），执法真值在 Redis；进程/Redis 重启后从 DB 快照续起，最坏丢失一个落库周期内的计数。
6. 套餐限额 <=0 视为未配置执法阈值（fail-open 不封禁），与 117.sql GetTenantUsage 的套餐容错口径一致。
7. 本地实际执行的验证：`go build ./...` 全仓编译通过；`go test ./internal/quota/... ./internal/middleware/`（含 -count=1 复跑）全绿；静态守卫 TestTenantScopeQueryAudit（suspects=0）、TestRequestPathContextBackgroundBudget（api 4/4、middleware 1/1，未新增 context.Background()）、TestMigrationChainIsConsistent（124==VERSION_NUMBER）通过；dal/service/router 包定向回归通过。前端：npm run typecheck（vue-tsc）无错误；eslint 新文件无告警；按 ask 指定命令 `node node_modules/vitest/vitest.mjs run -u src/service/api/__tests__/index.exports.test.ts` 执行（该 runner 忽略路径过滤实际跑了全量前端套件：441 文件/3970 测试全绿，快照 +1，含 getApiQuota/ApiQuotaReport）。未运行全量 go test（按约定）。

### 1.3 TB-18 设备 Profile 档案级默认规则链 —— delivered

**交付说明**（实施会话回报）：`125.sql` 加 UUID 外键列；绑定/解绑 API 透传与租户校验；GetEffectiveRuleChainsForDevice 档案链优先+租户链兜底（稳定去重，OnTelemetry/OnDeviceOnline 已接线）；解析端点 GET /rule-chains/device-effective/:deviceId；前端 config-detail 设置页规则链下拉与四语文案。

**迁移**：`backend/sql/125.sql`

**改动文件**：
- 后端：`backend/sql/125.sql`、`backend/pkg/global/global.go`、`backend/internal/model/device_configs.gen.go`、`backend/internal/model/device_configs.http.go`、`backend/internal/model/rule_chain_effective.go`、`backend/internal/dal/device_config.go`、`backend/internal/dal/rule_chains.go`、`backend/internal/service/device_config.go`、`backend/internal/service/rule_chain.go`、`backend/internal/service/rule_chain_effective.go`、`backend/internal/service/rule_chain_effective_test.go`、`backend/internal/api/rule_chain.go`、`backend/router/apps/rule_chain.go`
- 前端：`frontend/src/service/api/rule_chain.ts`、`frontend/src/service/api/__tests__/index.exports.test.ts`、`frontend/src/views/device/config-detail/modules/setting-info.vue`、`frontend/src/views/device/config-detail/modules/__tests__/setting-info.test.ts`、`frontend/src/typings/device/device.d.ts`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/generate.json`
- 契约测试：`automation_tests/tests/80_device_profile_rule_chain.test.js`

**测试**：
- Go 单测：`backend/internal/service/rule_chain_effective_test.go` 的 TestResolveEffectiveRuleChainGraphsProfileFirstTenantFallbackDedup、TestGetEffectiveRuleChainsForDeviceResolution、TestResolveEffectiveRuleChainsForDeviceAccessAndContract、TestUpdateDeviceConfigDefaultRuleChainBindUnbindFlow、TestDeleteChainGuardAgainstProfileBinding。
- 前端 vitest：`setting-info.test.ts` 4 用例（loads only enabled rule chains and echoes current binding on mount / keeps a placeholder option when the bound chain is missing / onSaveRuleChain submits the selected chain id / onSaveRuleChain submits empty string to unbind）。
- 契约测试：`automation_tests/tests/80_device_profile_rule_chain.test.js`（7 用例：绑定回显/不存在链拒绝/跨租户链拒绝/解析契约 profile 优先 tenant 兜底/解绑回落租户链/删除守卫/租户隔离；**待活栈运行**）。

**门禁**：go build + 定向 go test + vue-tsc 全过（第 1 轮，实施会话自报）。

**residual 与未做项**（实施会话回报）：
1. 按要求未做：默认队列绑定的档案维度化（平台当前为全局三队列，无按档案维度）与告警规则收敛为 Profile 字段（现有场景联动间接实现保留）——见 125.sql 头注释。
2. 契约测试已写好并通过 `node --check` 语法校验，但本机无活栈、按约定未运行，需编排方在活栈执行。
3. 快照命令 `node node_modules/vitest/vitest.mjs run -u src/service/api/__tests__/index.exports.test.ts` 在本机实际跑了全量前端 vitest（441 文件/3970 用例全过、快照仅新增本批与其他在途批次的既有导出）；首轮暴露 setting-info.test.ts 因新增的挂载期 ruleChainList 请求产生 5 个未处理 rejection，已在测试中 mock @/service/api/rule_chain 并补 4 个用例后复跑归零（3 文件 14 用例全绿）。
4. 执行面为保守接线：档案链解析失败/跨租户/停用时回落租户级链（fail-open 到租户级执行，不中断上行）；删除被档案引用的规则链被 service 守卫+FK RESTRICT 双层拒绝，需先解绑。

### 1.4 TB-27 告警 SLA 计时与超时升级 —— delivered

**交付说明**（实施会话回报）：`126.sql` 三列迁移（alarm_config.sla_hours / alarm_history.sla_due_at + sla_breached）；触发时按 sla_hours 起算 due_at；每 5 分钟 cron 扫描超时活动告警标记 breach 并升一档（L→M→H，N 不动，remark 追加 sla_escalation 审计）；列表/详情 API 经 ah.*/ac.* 透出 sla 字段；前端配置页 SLA 小时设置与历史超时标记（四语言 locale）。

**迁移**：`backend/sql/126.sql`

**改动文件**：
- 后端：`backend/sql/126.sql`、`backend/pkg/global/global.go`、`backend/internal/model/alarm_config.gen.go`、`backend/internal/model/alarm_history.gen.go`、`backend/internal/model/alarm_config.http.go`、`backend/internal/dal/alarm.go`、`backend/internal/dal/alarm_sla.go`、`backend/internal/service/alarm.go`、`backend/internal/service/alarm_notification.go`、`backend/internal/service/alarm_sla.go`、`backend/initialize/croninit/cron.go`
- 前端：`frontend/src/views/alarm/warning-message/components/pop-up.vue`、`frontend/src/views/alarm/warning-message/components/new-information.vue`、`frontend/src/views/alarm/warning-message/components/alarmConfigurationColumns.tsx`、`frontend/src/views/alarm/warning-message/components/__tests__/pop-up.test.ts`、`frontend/src/views/alarm/warning-message/components/__tests__/new-information.test.ts`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/custom.json`
- 契约测试：`automation_tests/tests/81_alarm_sla.test.js`

**测试**：
- Go 单测：`backend/internal/service/alarm_sla_test.go`（TestAlarmSlaEscalationTarget / TestAlarmSlaEscalationDue / TestAlarmSlaDueAt / TestValidateAndNormalizeAlarmSlaHours，实施会话回报 `go test ./internal/service/ -run TestAlarmSla -count=1` 全部 PASS）。
- 前端 vitest：`pop-up.test.ts`（add/edit 提交载荷断言更新为携带 sla_hours）；`new-information.test.ts`（7 列断言 + sla_hours 列渲染用例）。
- 契约测试：`automation_tests/tests/81_alarm_sla.test.js`（8 用例：SLA 配置回显/负数拒绝/场景触发写 due_at≈+1h/详情回读/未启用为空/sla_hours=0 关闭回显 null/租户 B 隔离×2；**无活栈未运行**，实施会话回报 `node --check` 语法通过）。

**门禁**：go build + 定向 go test + vue-tsc 全过（第 1 轮，实施会话自报）。

**residual 与未做项**（实施会话回报）：
1. 【按指示明确不做】details 结构化 JSONB 列迁移未实施：sla_escalation 审计沿用 alarm_history.remark（43.sql 已加宽为 text）存 JSON 的兼容模式（dal/alarm.go expandMapRemarkFields 已把 sla_escalation 加入展开键，详情接口可读）。
2. 真实时钟下的 cron 超时升级行为（sla_breached 标记+严重度升档+WS 事件广播）未在活栈契约验证——契约测试不等待 5 分钟 cron，升级判定逻辑由 alarm_sla_test.go 纯函数单测锚定（L→M→H/H 到顶/N 不动/幂等守卫）。
3. 迁移前已存在的历史告警不回填 sla_due_at，SLA 仅对新触发告警从触发时刻起算。
4. calcfield 告警（alarm_eval.go 直写 history）与 RDI 直连事件两条旁路不写 alarm_config 关联、无 sla_hours 可依，不参与 SLA 计时（v1 仅覆盖 AlarmExecute/saveAlarmHistoryRecord 路径，与任务书范围一致）。
5. 本机无活栈：81 契约测试与全量 go test/vitest 按约定交由编排方执行；已运行的门禁为 `go build ./...`、`go vet`（3 包）、gofmt、`go test ./pkg/global/ ./internal/dal/ ./internal/api/ -count=1`（含 migration_chain、tenant_scope_audit、context_background_budget 三个静态守卫）、前端 locale-completeness（4 通过）、warning-message 组件套件（83 通过）、API barrel 快照（1 通过，未加新导出无需 -u）、vue-tsc typecheck（干净）。

### 1.5 TB-10 实体级审计日志 —— delivered

**交付说明**（实施会话回报）：operation_logs 增实体级审计四列（action/entity_type/entity_id/status_code，`127.sql` 幂等迁移+partial index）；middleware 落库经纯函数解析器（自脱敏路径解析实体、HTTP 方法映射动作、writer 状态码）并配 4 个 Go 单测；列表/导出 API 增 action/entity_type/entity_id 筛选与新列；前端 system-log 页加同款筛选与四列（四语言键齐）。

**迁移**：`backend/sql/127.sql`

**改动文件**：
- 后端：`backend/sql/127.sql`、`backend/pkg/global/global.go`、`backend/internal/model/operation_logs.gen.go`、`backend/internal/model/operation_logs.http.go`、`backend/internal/middleware/operation_entity.go`、`backend/internal/middleware/operations_log.go`、`backend/internal/dal/operation_logs.go`、`backend/internal/service/audit_export.go`、`backend/internal/api/operation_log.go`
- 前端：`frontend/src/typings/api.d.ts`、`frontend/src/views/system-management-user/system-log/index.vue`、`frontend/src/views/system-management-user/system-log/__tests__/index.test.ts`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/custom.json`
- 契约测试：`automation_tests/tests/82_operation_entity_audit.test.js`

**测试**：
- Go 单测：`backend/internal/middleware/operation_entity_test.go`（TestOperationActionForMethod / TestOperationEntityForPath / TestIsUUIDShape / TestOperationEntityFromRedactedShareTokenPath）。
- 前端 vitest：`system-log/__tests__/index.test.ts`（同步新筛选参数与重置断言）。
- 契约测试：`automation_tests/tests/82_operation_entity_audit.test.js`（覆盖 create/update/delete 全生命周期+租户隔离+导出新列；**待活栈运行**）。

**门禁**：go build + 定向 go test + vue-tsc 全过（第 1 轮，实施会话自报）。

**residual 与未做项**（实施会话回报）：
1. 明确不做（按 ask）：TB 式独立 audit_logs 实体与告警/权限的领域事件审计流水未实施。
2. 存量兼容口径：127.sql 之前的行四新列为 NULL，不回填（回填需重放请求语义，超出 v1）。
3. 非中间件审计写入口（如 internal/service/secret.go:271 的 SECRET_REVEAL 合成审计行）不经中间件、新列保持 NULL；解析器留在 middleware 包内未做跨包导出，避免 scope 蔓延。
4. 解析器门闩：entity_id 仅认 UUID 形态第二段（本平台实体主键均为 varchar(36) UUID），非 UUID 主键与动词形第二段（如 /device/update/voucher）不产生 entity_id；现路由清单内未发现非 UUID 实体主键。
5. action 按 ask 口径由 HTTP 方法映射（POST→create/PUT·PATCH→update/DELETE→delete/GET·HEAD→read/其他→other），不区分业务语义意图。
6. 契约测试 82 号按约定未在本机运行（无活栈），已过 `node --check` 语法检查与依赖解析；运行时验证由编排方执行。
7. status_code 只做记录+展示列，未加为列表筛选维度（ask 筛选口径为 action/entity_type/entity_id）。
8. OpenAPI 文档未重新生成（编排方统一执行）。本机已跑检查：`go build ./...` 通过；gofmt 干净；定向 `go test ./internal/middleware/`（含新旧用例）全过；TestMigrationChainIsConsistent、TestTenantScopeQueryAudit、TestRequestPathContextBackgroundBudget（api=4/middleware=1 未增）全过；定向 vitest：system-log 页 7 用例与 locale-completeness 4 用例全过。未运行全量 go test / 全量 vitest。

### 1.6 TB-25 实体版本控制差异对比 —— delivered

**交付说明**（实施会话回报）：后端纯函数 DiffEntityVersionSnapshots（递归 JSON 语义 diff，新增/删除/修改点号路径列表）+ 8 例单测全绿；新端点 GET /api/v1/entity_versions/:id/diff/:target_id（api+service+router+`128.sql` Casbin 三角色登记+VERSION_NUMBER=128 时点值）；前端 entity-version 页新增对比入口（目标版本下拉 + 左右两栏快照 JSON + 变更列表，零新依赖）与四语言 locale（每语言 +15 键 parity 校验通过）。

**迁移**：`backend/sql/128.sql`

**改动文件**：
- 后端：`backend/internal/model/entity_version_diff.go`、`backend/internal/service/entity_version_diff.go`、`backend/internal/service/entity_version_diff_test.go`、`backend/internal/api/entity_version.go`、`backend/router/apps/entity_version.go`、`backend/pkg/global/global.go`、`backend/sql/128.sql`
- 自动化测试基座：`automation_tests/lib/endpoint-coverage/catalog.js`、`automation_tests/lib/coverage-contract/business-capabilities.js`、`automation_tests/tests/83_entity_version_diff.test.js`
- 前端：`frontend/src/service/api/entity_version.ts`、`frontend/src/service/api/__tests__/index.exports.test.ts`、`frontend/src/views/management/entity-version/index.vue`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/custom.json`

**测试**：
- Go 单测：TestDiffEntityVersionSnapshotsAddedPaths、TestDiffEntityVersionSnapshotsRemovedPaths、TestDiffEntityVersionSnapshotsNestedModified、TestDiffEntityVersionSnapshotsArrayChanges、TestDiffEntityVersionSnapshotsIdenticalAndRootScalar、TestDiffEntityVersionSnapshotsRejectsInvalidJSON、TestEntityVersionDiffServiceScopesAndDiff、TestEntityVersionDiffServiceFailsClosed（8 例）。
- 契约测试：`automation_tests/tests/83_entity_version_diff.test.js`（4 例：正差/零差/404 负向/跨租户 fail-closed；**待活栈运行**）。

**门禁**：go build + 定向 go test + vue-tsc 全过（第 1 轮，实施会话自报）。

**residual 与未做项**（实施会话回报）：
1. Git 仓库后端适配（branch/commit 语义）未做——本项明确不做，快照模式仍是唯一后端，diff 基于快照 JSONB 先行。
2. 契约测试 83 未在活栈执行（本机无活栈，按指示只写不跑，仅 `node --check` 语法验证）。
3. OpenAPI 文档由编排方统一重生成，新端点尚未写入 docs/openapi/openapi.json（更新会话注：收尾门禁 openapi=0 为编排方回报）。
4. 端点路径命名偏差：任务书写 GET /api/v1/entity_version/:id/diff/:target_id（单数），实际按 router/apps/entity_version.go 既有复数资源命名约定（组内 4 条既有路由均为 entity_versions，文件注释明确"保持复数资源命名"）实现为 /api/v1/entity_versions/:id/diff/:target_id，casbin 128.sql、契约测试、endpoint catalog 三处登记一致使用复数路径。
5. 前端 barrel 快照 -u 运行时除新增 entityVersionDiff 外，还一并纳入了并行批次已存在但快照滞后的 8 个导出（widget bundle/quota/rule-chain-effective），无删除项。

### 1.7 TB-15 时序数据保留策略（TimescaleDB retention + 冷层清理）—— delivered

**交付说明**（实施会话回报）：TimescaleDB 路径按 data_policy 设备数据 retention_days 幂等装配原生 add_retention_policy（毫秒 drop_after + set_integer_now_func，57.sql 修正"压缩≠保留"口径），并为 telemetry_rollups 冷层增加与热层同边界的分批清理，纯函数/SQL 构造单测与 data_policy 读写契约测试齐备。

**迁移**：无新迁移（`backend/sql/57.sql` 就地修正口径）。

**改动文件**：
- 后端：`backend/initialize/timescale_retention.go`、`backend/initialize/timescale_retention_test.go`、`backend/initialize/pg_init.go`、`backend/sql/57.sql`、`backend/internal/dal/telemetry_rollups.go`、`backend/internal/dal/telemetry_rollups_retention_test.go`、`backend/internal/service/datapolicy.go`、`backend/internal/service/telemetry_downsample.go`、`backend/internal/model/telemetry_rollup.go`
- 契约测试：`automation_tests/tests/84_data_policy.test.js`

**测试**：
- Go 单测：`backend/initialize/timescale_retention_test.go`（TestRetentionDropAfterMs、TestSelectDeviceDataRetentionDays、TestIntegerNowFuncDDL、TestBuildSetIntegerNowFuncSQL、TestBuildAddRetentionPolicySQL、TestBuildRemoveRetentionPolicySQL、TestBuildRetentionJobGuardSQL）；`backend/internal/dal/telemetry_rollups_retention_test.go`（TestDeleteTelemetryRollupsBatchSQLShape）。
- 契约测试：`automation_tests/tests/84_data_policy.test.js`（7 用例：读契约/写后读回/天数与 enabled 与 remark 负例/双租户越权拒绝；**待活栈运行**）。

**门禁**：go build + 定向 go test + vue-tsc 全过（第 1 轮，实施会话自报）。

**residual 与未做项**（实施会话回报）：
1. retention 在 TimescaleDB 的实际生效（drop_chunks 真删、set_integer_now_func 注册成功、timescaledb_information.jobs 守卫命中）需活 TimescaleDB 库验证，本机无活栈未运行，仅覆盖纯函数与 SQL 构造单测；装配失败按设计 warn 不阻断启动、下次启动重试。
2. drop_after 采用整数时间列按毫秒表达的官方文档口径（TimescaleDB api.md：integer-based time column 的 drop_after 为 integer 且要求 integer_now_func，2026-09-25 经 WebFetch 核对），与 57.sql 既注的压缩 chunk 数口径并存；旧版扩展无 set_integer_now_func replace_if_exists 参数时的 "already" 兜底路径未在旧版本实测。
3. alarm_info hypertable 未挂 retention——data_policy 无告警数据类型行，需先扩 data_policy 类型（另立批次）。
4. 档案/租户粒度 TTL（data_policy 行级扩展）按本项约定明确不做，未实施。
5. 契约测试 84 未运行（本机无活栈），仅 `node --check` 语法校验通过；未跑全量 go test / vitest / OpenAPI 生成（编排方统一执行）。

### 1.8 TB-41 文件存储与媒体库管理 —— delivered

**交付说明**（实施会话回报）：`129.sql` 建 media_files 表（UNIQUE(tenant_id,file_path)+Casbin 登记+sys_ui_elements 顶级媒体库菜单）；UpFile 落盘成功即落登记（登记失败回滚文件）；/api/v1/media/files 列表/详情/删除三端点（租户隔离 fail-closed，删除前实时扫描看板 config/SCADA 文档/OTA 升级包引用，referenced_count>0 以 202004 拒绝并返回引用方）；前端 /media/library 网格预览/上传/删除页与四语言。

**迁移**：`backend/sql/129.sql`

**改动文件**：
- 后端：`backend/sql/129.sql`、`backend/pkg/global/global.go`、`backend/pkg/errcode/code.go`、`backend/configs/messages.yaml`、`backend/pkg/common/upload_paths.go`、`backend/internal/model/media_file.go`、`backend/internal/dal/media_file.go`、`backend/internal/dal/media_file_test.go`、`backend/internal/service/media_library.go`、`backend/internal/service/media_library_test.go`、`backend/internal/service/enter.go`、`backend/internal/api/media_library.go`、`backend/internal/api/upload.go`、`backend/internal/api/enter.go`、`backend/router/apps/media_library.go`、`backend/router/apps/enter.go`、`backend/router/apps/casbin_route_registration_contract_test.go`、`backend/router/router_init.go`
- 前端：`frontend/src/service/api/media.ts`、`frontend/src/service/api/index.ts`、`frontend/src/service/api/__tests__/index.exports.test.ts`、`frontend/src/views/media/library/index.vue`、`frontend/src/router/elegant/mediaRoutes.ts`、`frontend/src/router/elegant/routes.ts`、`frontend/src/router/elegant/imports.ts`、`frontend/src/router/elegant/transform.ts`、`frontend/src/typings/elegant-router.d.ts`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/route.json`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/custom.json`
- 自动化测试基座：`automation_tests/lib/api_client.js`、`automation_tests/tests/85_media_library.test.js`

**测试**：
- Go 单测：`backend/internal/service/media_library_test.go`（TestRegisterMediaUploadSkipsWithoutTenant/IsIdempotentByPath/RejectsEmptyPath、TestListMediaFilesIsTenantScoped/SearchAndMimeFilter、TestGetMediaFileDetailRefreshesReferencedCount、TestDeleteMediaFileRejectsWhenReferenced/RemovesFileAndRowWhenUnreferenced/TenantIsolation、TestResolveMediaDiskRelativePath[+RejectsNestedTraversal]）；`backend/internal/dal/media_file_test.go`（TestListMediaFilesForScopePaginationAndFilters、TestCreateMediaFileConflictIsIdempotent、TestCountMediaReferencesForPathAcrossSurfaces）。
- 契约测试：`automation_tests/tests/85_media_library.test.js`（全生命周期+跨租户隔离；**待活栈运行**）。

**门禁**：go build + 定向 go test + vue-tsc 全过（第 1 轮，实施会话自报）。

**residual 与未做项**（实施会话回报）：
1. 按任务边界明确不做：看板/SCADA 部件内嵌图片选择器（画布部件侧选择媒体库资产的 UI 与保存链路引用计数写入维护）与邮件附件外发——未实施。
2. 本机无活栈，契约测试 85 与全部 automation_tests 未实际运行；OpenAPI 重生成与全量 go test/vitest 由编排方统一执行（barrel 快照命令 vitest run -u index.exports.test.ts 实际触发了全量 441 文件/3975 用例并通过，但那是在页面/locale 追加之前；追加后仅复跑 typecheck、route-coverage、store route、locale-completeness/loader 四组定向测试，均绿）。
3. v1 引用统计口径为读时 LIKE 包含扫描（boards.config/preview_url、scada_documents.json_data、ota_upgrade_packages.package_url 四列，均带租户过滤）：列表页展示最近一次扫描值，详情/删除时实时刷新并回写；引用关系精确到部件级需后续 media_file_refs 明细表。
4. 存量 ./files 历史文件不回填登记，media_files 自本版本上传起生效；OTA upgradePackage 上传同样落登记（其对外路径与磁盘路径映射特例已在 service 层处理）。
5. errcode 202004 已入 messages.yaml（zh/en），es/fr 语言包未含该码文案（存量配置本就只有 zh_CN/en_US 两语，遵循现状）。

### 1.9 TB-21 边缘本地规则执行器（scoped v1）—— delivered

**交付说明**（实施会话回报）：新包 `backend/internal/edgerules`（快照信封/子集图解析、阈值求值、告警事件、修订号四态重连去重与升级重装载、re-trigger 窗口去重，零 DB/broker 依赖）；`cmd/edgemqttbroker` 可选接入开关（-edgerules 默认关闭）并完成活栈无关的端到端冒烟验证；快照下发 API 既有契约守护测试 86 号。

**迁移**：无新迁移。

**改动文件**：
- 后端：`backend/internal/edgerules/graph.go`、`backend/internal/edgerules/snapshot.go`、`backend/internal/edgerules/eval.go`、`backend/internal/edgerules/executor.go`、`backend/cmd/edgemqttbroker/main.go`、`backend/cmd/edgemqttbroker/edgerules.go`、`backend/cmd/edgemqttbroker/README.md`、`mqtt-broker/README.md`
- 契约测试：`automation_tests/tests/86_edge_sync_snapshot_contract.test.js`

**测试**：
- Go 单测：`backend/internal/edgerules/graph_test.go`、`snapshot_test.go`、`eval_test.go`、`executor_test.go`（实施会话回报合计 32 例全绿）。
- 契约测试：`automation_tests/tests/86_edge_sync_snapshot_contract.test.js`（快照下发 API 既有契约守护；**待活栈运行**）。

**门禁**：go build + 定向 go test + vue-tsc 全过（第 1 轮，实施会话自报）。

**residual 与未做项**（实施会话回报）：
1. 断云期间本地动作与云端告警的去重收敛演练未做（需活栈+边缘环境，TB-21 后续批次；本地事件仅落 EventSink/JSONL，不回传云端）。
2. 完整规则链节点矩阵未覆盖——v1 仅支持 trigger.telemetry / filter.threshold / action.alarm 子集，其余云端类型（含 trigger.device_online）装载通过但求值时 fail-closed 阻断路径并在 LoadResult.UnsupportedTypes 暴露。
3. ROADMAP 64 号断云演练扩展本地执行场景未做（需活栈+边缘环境）。
4. 契约测试 86 按约定未在活栈执行（本机无活栈），仅完成 `node --check` 语法校验、run_tests.js 自动发现确认与端点覆盖目录核对（4 个 edge/sync 端点均已在 endpoint-coverage catalog 中）。
5. 阈值 op 白名单与 NaN/Inf 拒收比云端更严（装载期强制 vs 云端运行期报错），已在代码注释与 README 标注为刻意差异。

## 2. 收尾门禁（2026-09-25）

| 门禁项 | 结果 | 执行主体与口径 |
| --- | --- | --- |
| openapi | **0**（openapi=0） | 任务书下达的收尾门禁结果，编排方统一收尾执行；更新会话照录、未复跑。各实施会话按约定未自行重生成 OpenAPI，新端点 @Router 注释已就位（TB-04/25/41 等回报） |
| goTest | **0**（goTest=0） | 同上（编排方统一收尾；各实施会话仅跑定向 go test，见各小节"门禁/residual"行） |
| vue-tsc | **0**（vue-tsc=0） | 同上；各实施会话第一轮自报 vue-tsc 全过 |
| vitest | **0**（vitest=0） | 同上；barrel 快照命令在全量前端套件上的运行记录见 1.1/1.2/1.3（实施会话回报 441 文件全绿） |

- 各实施会话自报的第一轮定向门禁一致：`go build ./...` + 定向 `go test` + vue-tsc 全过（第 1 轮）。
- 实施会话各自运行的定向检查明细（静态守卫、locale-completeness、组件套件等）见 §1 各小节 residual/门禁行，本文不重复、不汇总升格。

## 3. 未运行面与局限（诚实边界）

1. **契约测试 78~86 未运行**：本机无 docker，活栈（PG+Redis+broker+backend）不可用。九个用例已就位，实施会话回报均通过 `node --check` 语法校验；按批次约定只写不跑，运行由编排方在活栈执行。用例与活栈依赖同既有口径（如 78 号资源中心验签导入假定已配置 market.bundle_signing_keys，与 53 号同口径）。
2. **OpenAPI 重生成不在实施会话范围**（编排方统一执行；收尾门禁 openapi=0 为编排方回报）。TB-25 的 diff 端点采用复数路径 /api/v1/entity_versions/:id/diff/:target_id（与既有 router 命名一致，任务书原文为单数，三处登记一致，见 1.6 residual④）。
3. **全量 `go test ./...` 与全量 vitest 未在各实施会话运行**（按约定交编排方统一执行；收尾门禁 goTest=0/vitest=0 为编排方回报）。barrel 快照指定命令在多个会话中被该 runner 自身展开为全量前端套件运行（441 文件，回报全绿），属命令行为而非主动扩跑。
4. **更新会话（本文档撰写者）未复跑任何测试**：go test / vitest / vue-tsc / 契约测试均未在本会话执行；本会话实跑仅限 §6 所列存在性与迁移链核验。
5. 本文逐项 residual 中标注"明确不做/按指示不做"的子项为任务书范围决定，非遗漏；其中跨批次的"明确不做"汇总见 §4。

## 4. 本轮明确不做项（L 级及其他，留后续批次）

以下 13 项本批次**未动任何代码面**，维持 ROADMAP §4.1 在册缺口状态；按 §5.2 批次依赖排序留待后续批次立项：

| 编号 | 能力 | 本轮处置 | 后续安排与原因 |
| --- | --- | --- | --- |
| TB-45 | 集成连接器与数据转换器框架（统一 Integration 实体） | 不做 | 本批次范围外；§5.2 批 1 首项——Integration 实体先行，避免 TB-19 转换器返工 |
| TB-46 | 实体组与高级 RBAC（用户组/GPE/组级共享） | 不做 | 本批次范围外；§5.2 批 2（v1 范围已在 §4.1 验收点写死） |
| TB-11 | 部署架构与水平扩展（K8s/Helm/多副本验证） | 不做 | 本批次范围外；§5.2 批 3——需部署环境与双副本演练条件 |
| TP-03 | TCP 协议接入（pluginsdk 插件形态） | 不做 | 本批次范围外；§5.2 批 1 |
| TB-19 | 自定义主题与通用 Protobuf 载荷（proto 上传+dynamicpb） | 不做 | 本批次范围外；§5.2 批 1——按 TB-45 规划接入上行管线 |
| TB-22 | LwM2M DTLS(PSK)/SNMPv3/observe 服务端集成 | 不做 | 本批次范围外；§5.2 批 3 |
| TB-23 | 移动应用中心（bundle/版本/发布管理） | 不做 | 本批次范围外；§5.2 批 4——跨仓依赖 `active/mobile-app-uni` |
| TB-47 | 白标全套（自定义翻译/Advanced CSS/菜单治理） | 不做 | 本批次范围外；§5.2 批 2 |
| TB-48 | 统一事件调度器（Scheduler 实体+日历 UI） | 不做 | 本批次范围外；§5.2 批 2 |
| TB-49 | 报表 PDF/HTML 渲染与 SMTP 附件投递 | 不做 | 本批次范围外；§5.2 批 2 |
| TP-20 | 国产数据库适配（TDengine/KingBase 驱动+方言层） | 不做 | 本批次范围外；§5.2 批 4 |
| TP-21 | 设备健康算法中心（MSET/预测性维护） | 不做 | 本批次范围外；§5.2 批 4 |
| TP-22 | 可视化大屏（DataRoom 类/多屏轮播/投屏） | 不做 | 本批次范围外；§5.2 批 4 |

另：各项 residual 内的"明确不做"子项（如 TB-04 画布运行时 bundle 加载闭环、TB-17 transport 维度接线、TB-18 默认队列档案维度化与告警规则 Profile 化、TB-27 details 结构化 JSONB、TB-25 Git 仓库后端、TB-15 档案/租户粒度 TTL 与 alarm_info retention、TB-41 部件图片选择器与邮件附件外发、TB-21 断云去重收敛演练）为单项范围内的范围决定，已在 §1 对应小节逐条记录。

## 5. blocked / 回退记录

本批次实施结果中**无 blocked 项**：批次任务书下达的 9 项（TB-04/17/18/27/10/25/15/41/21）全部 delivered，**无"已回退"项**。若后续批次出现 blocked 项，按约定应在此登记为"已回退"并注明原因与回退范围。

## 6. 文档更新会话核验记录（本会话实跑）

本会话（路线图与证据更新员）为撰写本文与更新 ROADMAP.md 实际执行的核验，均在仓库根 `C:/Users/Zz/Documents/projects/active/aetherlink-iot/` 下：

1. **迁移链与版本号**：`grep -n "VERSION_NUMBER" backend/pkg/global/global.go` → `:21 VERSION_NUMBER = 129`；`ls backend/sql/*.sql | wc -l` = **129**，编号最小 1、最大 129、无缺号（非 .sql 条目仅 `README.md`），即 1~129 连续，与 VERSION_NUMBER=129 一致；123~129 七个批次二迁移文件逐个存在。
2. **契约测试就位**：`ls automation_tests/tests/ | grep -E "^(78|79|80|81|82|83|84|85|86)_"` → 9 个文件全部在位（78_widget_bundles / 79_billing_api_quota / 80_device_profile_rule_chain / 81_alarm_sla / 82_operation_entity_audit / 83_entity_version_diff / 84_data_policy / 85_media_library / 86_edge_sync_snapshot_contract）。**仅确认文件存在，未运行**。
3. **关键落点存在性抽查**：共 57 个文件逐一 `[ -f ]` 检查全部 OK（backend/sql/123~129.sql；internal/quota/ 五件套+decision/meter/limit；internal/edgerules/ executor+eval；service/widget_bundle、alarm_sla、entity_version_diff、media_library、rule_chain_effective 及各自 _test.go；middleware/operation_entity、tenant_daily_quota_test；dal/alarm_sla、api_usage、media_file_test、telemetry_rollups_retention_test；model/rule_chain_effective、entity_version_diff；api/billing、media_library、entity_version、rule_chain、upload；pkg/common/upload_paths；initialize/timescale_retention(+_test)；cmd/edgemqttbroker/edgerules(+README)；frontend service/api/media、entity_version、rule_chain、billing；frontend views：widget-bundles、billing/api-quota、media/library、management/entity-version、device/config-detail setting-info(+test)、alarm pop-up(+test)；automation_tests/lib/api_client、endpoint-coverage/catalog、coverage-contract/business-capabilities）。**仅存在性核验，未读实现、未运行**。
4. **未执行**：任何 go test / go build / vitest / vue-tsc / 契约测试 / OpenAPI 生成；竞品版本调研（故 ROADMAP §3 新增行的竞品版本列一律标"—（首引版本未逐版核验）"）。

## 7. 修订记录（2026-09-25，依据外部复核意见）

外部复核抽查 TB-17/TB-27/TB-21 三项与两项横切声明，结论"主体失实不存在、有 1 处数字错误 + 2 处批次二后未同步的过期数字"；本节为对应修订，修订会话对每处数字均重新实跑命令确认：

1. **TB-17 Go 单测计数 20 → 22**：复核与修订会话各自实测 `grep -c "^func Test"`：decision_test.go **8** + meter_test.go **9** + tenant_daily_quota_test.go **5** = **22**，函数名与 §1.2 testsAdded 明细逐一吻合；实施会话原回报"20 例"系口径失实，本文 §1.2 及 ROADMAP §3 TB-17 行、§9 TB-17 行已改为 22（并注记差异）。
2. **§0 交付项数口径统一**：删除初稿"10 项任务编号下 9 项 delivered"中的"10 项"误写（实施结果实列 9 项），与 §5 一致；并注明任务书另列本批明确不做的 13 项（含 TB-46，见 §4）。复核建议的"第 10 项 = TB-46"表述未采纳为事实声明——该解释无法从实施结果材料中证实，故以"9 项 delivered + 13 项明确不做见 §4"的口径替代。
3. **ROADMAP §3.1 两处批次二后过期数字同步**（修订会话实测）：
   - CORE-01：`ls backend/sql/*.sql | wc -l` = **129**（最小 1、最大 129），`grep VERSION_NUMBER global.go` → `:21 VERSION_NUMBER = 129`；证据列由"1~122.sql / =122"更新为"1~129.sql / =129"并注明批次二前基线（指向 §1.1）。
   - CORE-03：`ls automation_tests/tests/*.test.js | wc -l` = **139**（批次二前 130），数字前缀去重 **86** 个（最大 86，批次二前 77）；证据列更新为"139 个 / 前缀 00~86"，并标注 78~86 为批次二新增、待活栈运行。
4. **ROADMAP §4.1 九个 ✅ 注记**统一补"范围排除项见 §9 residual"（复核意见 4 指出 TB-17/18/27 的行内验收点含明确未做子项；因 TB-04/10/15/21/25/41 各行验收点同样存在范围排除项，故九行统一补注，防止据 ✅ 误读为整行验收点达成）。
5. 复核确认无需改动的部分：TB-27（cron 每 5 分钟、三列迁移、升档映射、sla_hours 前端）、TB-21（32 例计数、-edgerules 默认关、零 DB/broker 依赖、四态去重）、partial† 状态标注（9/9）、迁移链与契约测试存在性横切声明——本文相应表述维持原状。
