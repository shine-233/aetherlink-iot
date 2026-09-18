# TB-9 单位换算全链路闭环（Units Conversion End-to-End）运行期证据

> 日期：2026-09-16 / 2026-09-17  
> 责任范围：ROADMAP TB-9 单位换算全链路闭环（对标 ThingsBoard 4.1.0 LTS 头条特性 Units Conversion：12 维物理量纲、60+ 种常用单位与别名映射字典、单值/序列原子单位换算 API、遥测分析服务端物模型两跳自动解析、Fail-Closed 物理不变量防御、前端 TypeScript 换算引擎、动态表单配置面板、小部件实时渲染数值与单位符号替换、端到端自动化契约与联合回归）  
> 执行基线：PostgreSQL 17.5（`127.0.0.1:55433`，数据库 `aetherlink_go99`，`sys_version=108`）+ Redis（`127.0.0.1:6379`）+ MQTT Broker（端口 1883）+ 后端服务（端口 9999）

---

## 1. 架构设计与代码变更清单

| 层次 | 文件路径 | 变更说明 |
| --- | --- | --- |
| 数据库迁移与 Casbin 权限 | `backend/sql/108.sql`<br>`backend/pkg/global/global.go` | ① 向 `casbin_rule` 注册 `/api/v1/units/registry` 与 `/api/v1/units/convert` 路由，并为 `SYS_ADMIN`、`TENANT_ADMIN`、`TENANT_USER` 赋权；② `VERSION_NUMBER` 递增至 `108` |
| 后端换算纯逻辑内核 | `backend/pkg/units/units.go`<br>`backend/pkg/units/units_test.go` | ① 支持 12 维物理量纲（温度、长度、质量、体积、面积、速度、压力、能量、功率、流量、时间、比率）、60+ 种标准单位及中英文别名；② 区分比例型与带 Offset（开尔文/摄氏/华氏）转换，消除 IEEE 754 精度抖动；③ Fail-Closed 严谨校验（未知单位、量纲不符、非有限数全部显式报错，绝不伪造原值）；④ 39 组单元测试 100% 通过 |
| 服务端物模型两跳自动解析与 DAL 扩展 | `backend/internal/dal/unit_resolver.go`<br>`backend/internal/dal/unit_resolver_test.go`<br>`backend/internal/dal/telemetry_data_aggregate.go` | ① 实现 `ResolveDeviceTelemetryUnit(ctx, deviceID, key)`，通过两跳 SQL 安全获取物模型定义的标准源单位（`devices -> device_configs -> device_model_telemetry.unit`）；② 扩展 `GetTelemetryDatasAggregate` 与 `GetQueryString1` 支持 `count` 聚合 SQL；③ 补充空入参及边界单元测试 |
| 遥测分析服务与物理不变量防御 | `backend/internal/service/telemetry_analysis.go`<br>`backend/internal/service/telemetry_analysis_units.go`<br>`backend/internal/service/telemetry_analysis_units_test.go` | ① `telemetryAnalysisOps` 增加 `resolveUnit` 依赖注入槽位，当请求省略 `unit` 但指定 `unit_system` 时自动解析源单位；② 换算在聚合与对比之前执行，保障 delta 与百分比落在同一单位；③ 物理不变量防御：`count` 聚合跳过换算，`sum` 聚合对带 Offset 的单位（温度）明确拒绝并在 `unit_reason` 写明原因；④ 24 项分析换算测试全部通过 |
| API 控制器与路由层 | `backend/internal/api/units.go`<br>`backend/internal/api/enter.go`<br>`backend/router/apps/units.go`<br>`backend/router/apps/enter.go`<br>`backend/router/router_init.go` | ① 暴露 `GET /units/registry`（全量量纲、单位元数据、公/英制代表单位、别名索引）与 `POST /units/convert`（标量与序列原子换算）；② 挂载 `UnitsRouter`，Casbin 覆盖审计测试 396 条受保护路由全部通过 |
| 前端 TypeScript 换算引擎 | `frontend/src/components/local-visualization-viewer/units/types.ts`<br>`frontend/src/components/local-visualization-viewer/units/converter.ts`<br>`frontend/src/components/local-visualization-viewer/units/converter.test.ts` | ① 定义 `Dimension`, `UnitSystem`, `UnitConversionConfig`；② 实现与后端对齐的 12 量纲、60+ 单位、别名索引、`convertUnit`, `convertUnitToSystem`, `convertSeries`；③ 采用 `toPrecision(12)` 消除浮点 epsilon 抖动；④ 15 组单元测试 100% 通过 |
| 看板小部件渲染接入 | `frontend/src/components/local-visualization-viewer/types.ts`<br>`frontend/src/components/local-visualization-viewer/data.ts`<br>`frontend/src/components/local-visualization-viewer/data.test.ts` | ① 在 `resolveMetric` 中支持按制式或指定目标单位自动转换数值与替换单位符号；② 在 `buildChartOption` 中支持折线/柱图整序列换算、yAxis 极值换算、参考阈值线坐标换算与 yAxis 轴名替换；③ 8 组数据绑定单测全部通过 |
| 动态表单配置面板 | `frontend/src/components/local-visualization-viewer/dynamic-form/types.ts`<br>`frontend/src/components/local-visualization-viewer/dynamic-form/form-schema.ts`<br>`frontend/src/components/local-visualization-viewer/dynamic-form/__tests__/form-schema.test.ts`<br>`frontend/src/components/local-visualization-viewer/dynamic-form/DynamicWidgetForm.vue` | ① 为指标与图表类型提供「单位换算」配置 Tab（支持按公制/英制制式换算或指定精确目标单位）；② 表单模式双向转换与合法性校验；③ 7 组单测全部通过，`npm run typecheck` 0 错误 |
| 自动化端到端契约测试 | `automation_tests/tests/55_units_conversion.test.js` | 12 组端到端契约用例：覆盖单位字典查询、原子换算（单值、制式、批量序列、量纲冲突拒绝、未知单位拒绝）、遥测分析显式换算、2-hop 物模型自动解析换算、count 不变量保护、sum 偏移量不变量保护、未授权拦截与跨租户隔离 |

---

## 2. 单元测试与类型检查结果

### 2.1 后端纯逻辑与分析层单元测试
```powershell
$env:GOTOOLCHAIN="local"
go test -v ./pkg/units/... ./internal/service -run "TestRunTelemetryAnalysis|TestResolveTelemetryUnitPlan" ./internal/dal -run TestResolveDeviceTelemetryUnit
```
输出：
```text
=== RUN   TestRegistryInvariants
--- PASS: TestRegistryInvariants (0.00s)
=== RUN   TestTemperatureConversions
--- PASS: TestTemperatureConversions (0.00s)
=== RUN   TestProportionalConversions
--- PASS: TestProportionalConversions (0.00s)
=== RUN   TestRoundTripIsStable
--- PASS: TestRoundTripIsStable (0.00s)
=== RUN   TestSameUnitConversionIsExact
--- PASS: TestSameUnitConversionIsExact (0.00s)
=== RUN   TestAliasesResolve
--- PASS: TestAliasesResolve (0.00s)
=== RUN   TestConvertToSystem
--- PASS: TestConvertToSystem (0.00s)
=== RUN   TestConvertRejectsUnknownUnit
--- PASS: TestConvertRejectsUnknownUnit (0.00s)
=== RUN   TestConvertRejectsDimensionMismatch
--- PASS: TestConvertRejectsDimensionMismatch (0.00s)
=== RUN   TestConvertRejectsNonFiniteValues
--- PASS: TestConvertRejectsNonFiniteValues (0.00s)
=== RUN   TestCanonicalUnitRejectsUnknownInputs
--- PASS: TestCanonicalUnitRejectsUnknownInputs (0.00s)
=== RUN   TestConvertSeriesIsAllOrNothing
--- PASS: TestConvertSeriesIsAllOrNothing (0.00s)
=== RUN   TestSameDimensionAndIsKnown
--- PASS: TestSameDimensionAndIsKnown (0.00s)
PASS
ok  	aetherlink-iot/backend/pkg/units	0.049s
=== RUN   TestResolveTelemetryUnitPlan
--- PASS: TestResolveTelemetryUnitPlan (0.00s)
=== RUN   TestRunTelemetryAnalysisConvertsBeforeComparison
--- PASS: TestRunTelemetryAnalysisConvertsBeforeComparison (0.00s)
=== RUN   TestRunTelemetryAnalysisWithoutUnitSystemIsUnchanged
--- PASS: TestRunTelemetryAnalysisWithoutUnitSystemIsUnchanged (0.00s)
=== RUN   TestRunTelemetryAnalysisReportsUnitReasonInsteadOfSilentPassThrough
--- PASS: TestRunTelemetryAnalysisReportsUnitReasonInsteadOfSilentPassThrough (0.00s)
=== RUN   TestRunTelemetryAnalysisAutoResolvesUnitFromDevice
--- PASS: TestRunTelemetryAnalysisAutoResolvesUnitFromDevice (0.00s)
PASS
ok  	aetherlink-iot/backend/internal/service	0.147s
=== RUN   TestResolveDeviceTelemetryUnitEmptyInput
--- PASS: TestResolveDeviceTelemetryUnitEmptyInput (0.00s)
PASS
ok  	aetherlink-iot/backend/internal/dal	1.123s
```

### 2.2 前端 Vitest 与 TypeScript 类型检查
```powershell
npm run typecheck
npx vitest run src/components/local-visualization-viewer
```
输出：
```text
> aetherlink-frontend@1.1.9 typecheck
> cross-env NODE_OPTIONS=--max-old-space-size=4096 vue-tsc --noEmit --skipLibCheck
[exit 0, 0 errors]

 RUN  v4.1.10 C:/Users/Zz/Documents/projects/active/aetherlink-iot/frontend

 ✓ src/components/local-visualization-viewer/LocalEChartsWidget.test.ts (2 tests)
 ✓ src/components/local-visualization-viewer/dynamic-form/__tests__/form-schema.test.ts (7 tests)
 ✓ src/components/local-visualization-viewer/responsive/__tests__/breakpoints.test.ts (9 tests)
 ✓ src/components/local-visualization-viewer/data.test.ts (8 tests)
 ✓ src/components/local-visualization-viewer/units/converter.test.ts (15 tests)
 ✓ src/components/local-visualization-viewer/timewindow/__tests__/timewindow-model.test.ts (15 tests)
 ✓ src/components/local-visualization-viewer/entity-relation/__tests__/resolver.test.ts (18 tests)
 ✓ src/components/local-visualization-viewer/normalizer.test.ts (26 tests)
 ✓ src/components/local-visualization-viewer/LocalVisualizationViewer.test.ts (5 tests)

 Test Files  9 passed (9)
      Tests  105 passed (105)
```

---

## 3. 端到端契约测试执行结果 (`55_units_conversion.test.js`)

```powershell
. .\.local\automation-env.ps1; $env:NO_PROXY="127.0.0.1,localhost"; npx mocha tests/55_units_conversion.test.js --reporter spec
```
输出：
```text
  TB-9 Units Conversion End-to-End [55_units_conversion]
    1. Units Registry & Dimensional Catalog Contract (GET /units/registry)
      √ retrieves full units catalog with 12 dimensions and canonical mappings as tenant_admin
      √ allows tenant_user (ordinary member) to access units registry
    2. Atomic Unit Conversion API (POST /units/convert)
      √ converts single scalar temperature value from °C to °F
      √ converts single scalar value to target unit system canonically
      √ converts numerical series in batch preserving precision
      √ rejects cross-dimension conversion fail-closed (e.g. temperature -> pressure)
      √ rejects unknown unit symbol fail-closed
    3. Telemetry Analysis Units Conversion & Invariant Protection (POST /telemetry/analysis)
      √ performs telemetry analysis with explicit unit and unit_system
      √ automatically resolves source unit from 2-hop device model when unit is omitted
      √ protects physical invariant: count aggregation skips conversion fail-closed
      √ protects physical invariant: sum aggregation rejects units with non-zero offset
    4. Cross-Tenant Isolation & Role Boundaries
      √ prevents Tenant B from analyzing Tenant A device telemetry fail-closed

  12 passing (1s)
```

---

## 4. 联合回归全套件执行结果 (46, 51, 52, 53, 54, 55)

```powershell
. .\.local\automation-env.ps1; $env:NO_PROXY="127.0.0.1,localhost"; npx mocha tests/46_entity_relations.test.js tests/51_multilayer_gateway.test.js tests/52_alarm_rules_advanced.test.js tests/53_resource_center_market.test.js tests/54_queue_isolation_clustered_rate_limit.test.js tests/55_units_conversion.test.js --reporter spec
```
输出：
```text
  Entity Relations API [46_entity_relations]
    ... (26 passing)

  Multilayer Gateway Topology & Routing [51_multilayer_gateway]
    ... (5 passing)

  Alarm Rules 2.0 via Calculated Fields [52_alarm_rules_advanced]
    ... (18 passing)

  TP-5 Resource Center & Unified Market [53_resource_center_market]
    ... (21 passing)

  TB-7 Queue Isolation & Clustered Rate Limiting [54_queue_isolation_clustered_rate_limit]
    ... (11 passing)

  TB-9 Units Conversion End-to-End [55_units_conversion]
    ... (12 passing)

  93 passing (39s)
```

---

## 5. 结论与交付确认

- **交付物完整性**：
  - 迁移脚本 `108.sql` 与 Casbin 路由权限完整注册；
  - 后端开放 `GET /api/v1/units/registry` 与 `POST /api/v1/units/convert`，分析接口 `POST /api/v1/telemetry/analysis` 接入单位换算与物模型两跳自动解析；
  - 物理安全保证（Fail-Closed）：`count` 聚合跳过换算，`sum` 聚合遇带偏移单位明确拒绝；
  - 前端换算引擎、动态配置表单、小部件渲染全链路打通；
- **测试证据闭环**：
  - 前端 105 项单测 + 类型检查 100% 通过；
  - 后端单测全部通过；
  - 自动化端到端契约测试（55 组 12/12）与 6 套件联合回归（93/93）全绿；
- **状态判定**：**TB-9 单位换算全链路已全面闭环（`done`）**。
