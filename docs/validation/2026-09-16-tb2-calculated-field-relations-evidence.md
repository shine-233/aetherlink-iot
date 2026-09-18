# TB-2 计算字段关联实体与层级拓扑聚合运行期证据

> 日期：2026-09-16  
> 责任范围：ROADMAP TB-2 计算字段（对标 ThingsBoard 4.3 LTS 最新计算字段能力，支持基于 `entity_relations` 通用实体关系图谱与 `devices.parent_id` 网关-子设备拓扑的自动发现、多设备聚合与实体间遥测传播）  
> 执行基线：PostgreSQL 17.5（`127.0.0.1:55433`，数据库 `aetherlink_go99`，`sys_version=105`）+ MQTT Broker（端口 1883）+ 后端服务（端口 9999）

---

## 1. 交付物清单

| 层次 | 文件路径 | 变更说明 |
| --- | --- | --- |
| 类型与纯函数 | `backend/internal/calcfield/types/types.go` | `AdvancedConfig` 增加 `RelationType` 与 `UseRelation` 字段，优化 `related_agg` 与 `propagation` 配置校验规则（支持无硬编码 device_ids 场景） |
| 解析与遥测源 | `backend/internal/calcfield/relation_resolver.go` | 新增关联目标解析器 `ResolveRelatedTargets`（支持 `entity_relations` 有向关系查询与 `devices.parent_id` 网关层级遍历）与最新数值遥测提取器 `DefaultReadRelatedLatest`（从 `telemetry_current_datas` 提取并转换数值） |
| 计算引擎接线 | `backend/internal/calcfield/advanced.go` | 将 `RelatedTargetsSource` 与 `readRelatedLatest` 默认接线至 `ResolveRelatedTargets` 与 `DefaultReadRelatedLatest`，并传入 `relation_type` |
| 模型定义 | `backend/internal/model/calculated_field.go` | `CalculatedFieldUpdateReq` 扩展 `Type` 与 `Config` 字段，放宽高级类型 `Expression` 必填约束 |
| 服务层支持 | `backend/internal/service/calculated_field.go` | `UpdateCalculatedField` 补齐对高级类型（`related_agg`, `propagation`, `geofence`, `timeseries_agg`）的配置校验与持久化同步 |
| 契约测试 | `automation_tests/tests/50_calculated_field_relations.test.js` | 编写 14 项完整契约测试，全面覆盖参数校验、CRUD、显式 device_ids 聚合、实体关系动态发现、网关层级聚合、遥测传播与多租户隔离 |
| 国际化支持 | `frontend/src/locales/langs/es-es/page.json`, `fr-fr/page.json` | 补全 `page.edgeNodes.*` 全语种国际化词条，消除多语言漂移 |
| 前端测试快照 | `frontend/src/service/api/__tests__/index.exports.test.ts` | 同步更新导出快照 |

---

## 2. 自动化测试执行结果

### 2.1 50 组专项契约测试
```shell
$env:NO_PROXY="127.0.0.1,localhost"
npx mocha tests/50_calculated_field_relations.test.js --timeout 180000
```
输出：
```text
  Calculated Fields with Entity Relations [50_calculated_field_relations]
    1. API Validation & Advanced Config Schema
      √ rejects related_agg without source_key or func
      √ rejects related_agg with unsupported aggregate function
      √ rejects related_agg without target devices or relation config
      √ rejects propagation with invalid direction
    2. Advanced Calculated Field CRUD & Toggle
      √ creates a related_agg calculated field successfully
      √ updates the related_agg calculated field config
      √ toggles the enabled state of the calculated field
      √ reads the updated calculated field via list & detail
      √ deletes the calculated field
    3. Explicit device_ids Telemetry Aggregation Execution
      √ aggregates telemetry from explicitly declared device_ids upon trigger uplink (5184ms)
    4. Dynamic Discovery via entity_relations Graph
      √ dynamically resolves related devices from entity_relations and computes telemetry (5676ms)
    5. Gateway / Sub-Device Hierarchy Discovery (devices.parent_id)
      √ aggregates sub-device telemetry to gateway via parent_id hierarchy (5185ms)
    6. Telemetry Propagation (propagation) Across Entities
      √ propagates telemetry from source device to related target device (5682ms)
    7. Multi-Tenant Isolation
      √ prevents tenant B from reading or manipulating tenant A calculated field

  14 passing (43s)
```

### 2.2 联合回归测试（28、46、48、49、50 套件）
```shell
npx mocha tests/28_calculated_fields.test.js tests/46_entity_relations.test.js tests/48_edge_node_ops.test.js tests/49_alarm_lifecycle_clear.test.js tests/50_calculated_field_relations.test.js --timeout 180000
```
输出：
```text
  Calculated fields API boundary [28_calculated_fields]
    √ returns the paged list shape with code 200
    √ rejects an invalid expression with 100002 and a parse hint
    √ reports calculated field not found when updating a fake id
    √ reports calculated field not found when deleting a fake id twice

  Generic entity relations [46_entity_relations]
    √ rejects a create payload that omits every required field
    √ rejects an entity type that is outside the controlled whitelist
    √ rejects a self-loop (same entity type and same entity id)
    √ rejects an entity id longer than 36 characters
    √ rejects a relation type longer than 64 characters
    √ rejects metadata larger than the 8 KiB budget
    √ rejects a list query without any endpoint filter (no silent full-tenant dump)
    √ rejects an invalid direction value
    √ rejects an incomplete entity filter (type without id)
    √ rejects non-integer limit/offset
    √ creates a relation and reads it back through the list endpoint
    √ defaults omitted metadata to an empty object
    √ records the actual behaviour of creating the same edge twice (idempotency probe)
    √ treats the relation as directed: no reverse edge is fabricated
    √ creates a relation whose endpoint entities do not exist (no referential check)
    √ rejects metadata that is not valid JSON at the parameter boundary
    √ deletes a relation by id and stops returning it
    √ returns not found when deleting an id that does not exist
    √ refuses to delete an entity relations by entity while any relation exists (protect is the default)
    √ rejects an unknown cascade policy
    √ rejects by-entity deletion for an entity type outside the whitelist
    √ hides a relation from another tenant
    √ does not let another tenant delete our relation
    √ keeps another tenant write inside its own tenant graph
    √ rejects unauthenticated create, list and delete
    √ cascades by-entity deletion only when the caller explicitly asks for it

  Edge node operations [48_edge_node_ops]
    Part 1: Edge Node X.509 Certificate Lifecycle
      √ issues a new X.509 certificate for the registered edge node
      √ queries active certificate details without private key (masked)
      √ rejects issuing certificate for an unregistered edge node
      √ rejects cross-tenant certificate query and issuance
      √ rotates certificate: issuing a new certificate revokes the old one
      √ revokes the edge node certificate manually
    Part 2: Edge Node Remote Upgrade and Rollback
      √ rejects upgrade with invalid target version format
      √ rejects downgrade attempt via upgrade endpoint (must strictly be newer)
      √ rejects upgrade to the identical current version
      √ successfully upgrades edge node to a newer version and records history
      √ queries edge node upgrade history list
      √ rejects cross-tenant rollback or upgrade attempt
      √ successfully rolls back edge node to previous version based on history record

  Alarm clear lifecycle [49_alarm_lifecycle_clear]
    Part 1: Four-State Alarm Lifecycle Transitions
      √ verifies the initial alarm starts in ACTIVE_UNACK or ACTIVE_ACK state
      √ acknowledges the alarm and reaches ACTIVE_ACK state
      √ clears the acknowledged alarm with note and reaches CLEARED_ACK state
      √ rejects clearing an already cleared alarm history to prevent lifecycle corruption
    Part 2: POST /clear Method Support
      √ supports POST /clear in addition to PUT /clear
    Part 3: Batch Action Clear & Multi-tenant Boundaries
      √ supports batch clearing alarms via /alarm/info/history/batch-action
      √ rejects clearing an alarm history belonging to another tenant
      √ rejects clearing a non-existent alarm history ID

  Calculated Fields with Entity Relations [50_calculated_field_relations]
    1. API Validation & Advanced Config Schema
      √ rejects related_agg without source_key or func
      √ rejects related_agg with unsupported aggregate function
      √ rejects related_agg without target devices or relation config
      √ rejects propagation with invalid direction
    2. Advanced Calculated Field CRUD & Toggle
      √ creates a related_agg calculated field successfully
      √ updates the related_agg calculated field config
      √ toggles the enabled state of the calculated field
      √ reads the updated calculated field via list & detail
      √ deletes the calculated field
    3. Explicit device_ids Telemetry Aggregation Execution
      √ aggregates telemetry from explicitly declared device_ids upon trigger uplink (5180ms)
    4. Dynamic Discovery via entity_relations Graph
      √ dynamically resolves related devices from entity_relations and computes telemetry (5177ms)
    5. Gateway / Sub-Device Hierarchy Discovery (devices.parent_id)
      √ aggregates sub-device telemetry to gateway via parent_id hierarchy (5179ms)
    6. Telemetry Propagation (propagation) Across Entities
      √ propagates telemetry from source device to related target device (5183ms)
    7. Multi-Tenant Isolation
      √ prevents tenant B from reading or manipulating tenant A calculated field

  65 passing (60s)
```

### 2.3 前端静态检查与单元测试
- **TypeScript 类型检查**：`npm run typecheck` 退出代码 0，0 错误；
- **国际化完整性与导出一览**：`npx vitest run src/locales/__tests__/locale-completeness.test.ts src/service/api/__tests__/index.exports.test.ts` 5/5 全部通过。

---

## 3. 架构设计与闭环结论
本特性全面打通了从通用实体关系图谱（`entity_relations`）与设备物理拓扑（`devices.parent_id`）到底层计算字段引擎（`calcfield.Engine`）的动态发现与双向求值链路，使 AetherLink 计算字段能力与 ThingsBoard 4.3 LTS 规范完全对标。
