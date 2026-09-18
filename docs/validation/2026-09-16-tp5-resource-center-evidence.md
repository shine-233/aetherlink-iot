# TP-5 资源中心（设备物模型 + 大屏看板统一市场与统一打包分发）运行期证据

> 日期：2026-09-16  
> 责任范围：ROADMAP TP-5 资源中心（完全对标 ThingsPanel 1.2.8 资源中心核心能力：跨形态物模型与大屏看板统一目录与检索、大屏模板跨租户脱敏导出与导入实例化、跨租户统一资源包 HMAC-SHA256 签名打包分发、验签 fail-closed 门禁、冲突预览、覆盖确认人工闸门、一键应用与多租户拓扑隔离防线）  
> 执行基线：PostgreSQL 17.5（`127.0.0.1:55433`，数据库 `aetherlink_go99`，`sys_version=106`）+ MQTT Broker（端口 1883）+ 后端服务（端口 9999）

---

## 1. 架构设计与代码变更清单

| 层次 | 文件路径 | 变更说明 |
| --- | --- | --- |
| 数据库迁移与 Casbin 权限 | `backend/sql/106.sql`<br>`backend/pkg/global/global.go` | ① 在 `boards` 表补充扩展字段：`type_key`（行业分类）、`author`（作者）、`version`（版本号）、`preview_url`（预览图）、`download_count`（下载量）；② 向 `casbin_rule` 注册 8 个资源中心与看板模板路由权限（`/api/v1/resource/center/*`、`/api/v1/board/export/:id`、`/api/v1/board/import`）；③ `VERSION_NUMBER` 升级至 `106` |
| 模型定义层 | `backend/internal/model/boards.gen.go`<br>`backend/internal/model/boards.http.go`<br>`backend/internal/model/device_template_market.go` | ① 扩展 `Board` 实体与 `CreateBoardReq`/`UpdateBoardReq` 结构体，支持看板行业分类与版本元数据；② 定义 `BoardTemplateExport`；③ 扩展 `MarketBundle` 支持 `Boards` 切片，扩展 `MarketBundleImportPreview` 细分列表；④ 定义 `ResourceCenterItem`、`ResourceCenterCatalogEntry`、`ResourceCenterListReq`、`ResourceCenterApplyReq/Rsp` |
| 查询生成模型 | `backend/internal/query/boards.gen.go` | 补全 `boards` 表的 `TypeKey`、`Author`、`Version`、`PreviewURL`、`DownloadCount` 字段映射与 `fillFieldMap` 注册，确保 GORM GEN 操作准确存取元数据 |
| 数据访问层（DAL） | `backend/internal/dal/resource_center.go`<br>`backend/internal/dal/device_template_market.go` | ① 实现 `ListResourceCenterCatalog`（跨表统计物模型与大屏看板数量与下载量）；② 实现 `ListResourceCenterItems`（跨形态综合分页检索与排序）；③ 实现 `ListBoardTemplateVersionsInTenant`、`ListBoardIDsByTypeKey`（按名称最新版本去重导出）、`IncrementBoardDownloadCounts`；④ `ListTemplateIDsByTypeKey` 实现同名模板最新版本去重，杜绝出包内包含历史重名模板 |
| 服务层（Service） | `backend/internal/service/board_export_import.go`<br>`backend/internal/service/resource_center.go`<br>`backend/internal/service/device_template_market_integrity.go`<br>`backend/internal/service/device_template_market_import.go`<br>`backend/internal/service/board.go` | ① `ExportBoard` 实现看板配置脱敏（抹除 tenant_id 与私有上下文），`ImportBoard` 实现安全校验与幂等创建/更新；② `ResourceCenterCatalog` 与 `ResourceCenterList` 驱动目录与检索；③ `ExportResourceBundle` 统一打包导出并签名；④ `ImportResourceBundle` 执行数字验签、依赖检查、只读预览与人工覆盖门禁；⑤ `ApplyResource` 实现一键应用与命名克隆；⑥ `buildCreateBoardPayload` 与 `buildUpdateBoardPayload` 完整映射元数据 |
| 控制器与路由层 | `backend/internal/api/resource_center.go`<br>`backend/internal/api/board.go`<br>`backend/router/apps/resource_center.go`<br>`backend/router/apps/board.go`<br>`backend/router/apps/enter.go`<br>`backend/router/router_init.go` | ① 暴露 5 个资源中心 API（`/catalog`, `/list`, `/bundle`, `/bundle/import`, `/apply`）；② 暴露看板导出/导入接口；③ 完成路由装配与 Casbin 权限挂载 |
| 前端展现层 | `frontend/src/service/api/resource-center.ts`<br>`frontend/src/views/market/browse/bundle-import-model.ts`<br>`frontend/src/views/market/browse/index.vue`<br>`frontend/src/locales/langs/zh-cn/page.json`<br>`frontend/src/locales/langs/en-us/page.json` | ① 封装 API 客户端（包含 Catalog、List、Bundle Export/Import、Apply）；② 扩展打包导入业务纯逻辑层；③ 升级模板市场浏览页为一体化资源中心（支持全部/物模型/大屏形态切换、行业分类、卡片列表、一键应用、统一资源包导入）；④ 国际化双语支持 |
| 端到端契约测试 | `automation_tests/tests/53_resource_center_market.test.js` | 编写 21 组完整契约测试，覆盖跨形态检索、看板脱敏导出导入、综合包导出验签、篡改阻断、只读预览、覆盖门禁、一键应用及多租户隔离 |

---

## 2. 自动化测试执行结果

### 2.1 53 组资源中心端到端契约测试
```shell
$env:NO_PROXY="127.0.0.1,localhost"
npx mocha automation_tests/tests/53_resource_center_market.test.js
```
输出：
```text
  TP-5 Resource Center & Unified Market [53_resource_center_market]
    1. Resource Center Catalog & Unified List
      √ retrieves unified catalog with template and board counts
      √ queries unified list with all resources
      √ filters list by resource_type = board_template
      √ filters list by resource_type = device_template
      √ filters list by keyword query
    2. Board Template Export & Direct Import
      √ exports board as portable BoardTemplateExport without tenant context
      √ imports board template with a new name successfully
      √ rejects board template import with invalid JSON config
      √ rejects board template import with invalid vis_type
    3. Unified Resource Bundle Export & Digital Signature
      √ exports a signed unified resource bundle containing both templates and boards
      √ exports resource bundle for specific resource_type = board_template
    4. Cryptographic Integrity & Fail-Closed Gate
      √ rejects unsigned resource bundle
      √ rejects tampered resource bundle (modified board payload)
      √ rejects resource bundle with duplicate board names in dependencies check
    5. Dry-Run Conflict Preview & Overwrite Confirmation Gate
      √ returns preview without applying changes when preview=true
      √ requires confirm_overwrite=true when bundle contains overwrite items
      √ safely applies bundle when confirm_overwrite=true or all items are idempotent/new
    6. One-Click Apply & Multi-Tenant Isolation
      √ applies board template into tenant with custom name
      √ applies device template into tenant with custom name
      √ prevents Tenant B from exporting Tenant A private board
      √ prevents Tenant B from applying Tenant A private board directly

  21 passing (393ms)
```

### 2.2 多套件联合回归验证（41、45、50、51、52、53 套件）
```shell
npx mocha automation_tests/tests/41_market_bundle_import.test.js
npx mocha automation_tests/tests/45_template_upgrade_rollback.test.js
npx mocha automation_tests/tests/50_calculated_field_relations.test.js
npx mocha automation_tests/tests/51_multilayer_gateway.test.js
npx mocha automation_tests/tests/52_alarm_rules_advanced.test.js
npx mocha automation_tests/tests/53_resource_center_market.test.js
```
结果汇总：
- `41_market_bundle_import.test.js`：5 passing (124ms)
- `45_template_upgrade_rollback.test.js`：15 passing (220ms)
- `50_calculated_field_relations.test.js`：14 passing (43s)
- `51_multilayer_gateway.test.js`：5 passing (2s)
- `52_alarm_rules_advanced.test.js`：18 passing (22s)
- `53_resource_center_market.test.js`：21 passing (393ms)
- **多套件回归通过率：78 / 78 (100%)**

### 2.3 前端静态类型检查与单元测试
```shell
npm run typecheck
npx vitest run src/views/market/browse
```
输出：
- `vue-tsc --noEmit --skipLibCheck`：0 errors
- `vitest run src/views/market/browse`：34 passed (34 tests in 2 test files)

---

## 3. 对标结论
AetherLink 物联网平台在 TP-5 资源中心与统一打包分发能力上已完全对标并超越 ThingsPanel 1.2.8 / 1.2.11：
1. **统一目录与跨形态检索**：打通了设备物模型与大屏看板的孤岛，支持在统一资源中心下按行业、形态混合与精准筛选；
2. **大屏看板便携导出导入**：提供原生的脱敏跨租户描述符导出与安全导入，支持 native 与 thingsvis 双引擎看板；
3. **数字签名与全链路防篡改门禁**：通过 HMAC-SHA256 签名机制与 fail-closed 门禁，确保在公网市场与私有云之间分发的资源包真实完整；
4. **只读预览与人工覆盖闸门**：导入前提供详尽的同版本幂等、新建项、覆盖项分析，强制人工确认覆盖；
5. **多租户严格防护**：全面继承租户边界检查，坚决阻断跨租户直接访问或非法克隆。
