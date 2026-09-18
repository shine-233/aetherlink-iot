# TP-3 多层网关拓扑与递归遥测/命令路由运行期证据

> 日期：2026-09-16  
> 责任范围：ROADMAP TP-3 多层网关能力（对标 ThingsPanel 1.1.10+ / 1.2.11 多层网关架构：顶层接入网关 -> 中间子网关 -> 底层终端子设备；递归上行遥测解包与分发；下行命令顶层寻路；租户拓扑安全防线）  
> 执行基线：PostgreSQL 17.5（`127.0.0.1:55433`，数据库 `aetherlink_go99`，`sys_version=105`）+ MQTT Broker（端口 1883）+ 后端服务（端口 9999）

---

## 1. 交付物清单与代码审查

| 层次 | 文件路径 | 变更说明 |
| --- | --- | --- |
| 上行递归路由 | `backend/internal/uplink/telemetry.go` | 原生内置递归子网关遥测解包分发逻辑（`processSubGateways`），解包嵌套深度可达 5 层，支持 `gateway_data`、`sub_gateway_data` 和 `sub_device_data` 同批上报 |
| 下行递归寻路 | `backend/internal/service/command_gateway_payload.go` | 原生内置多层网关回溯寻路逻辑（`findTopLevelGatewayForCommand`），由末端子设备自动向上递归追溯到具备真实网络物理连接的顶层网关 |
| HTTP 读模型扩展 | `backend/internal/model/devices.http.go` | ① `GetDeviceListByPageReq` 增加 `parent_id` 过滤参数；② `GetDeviceListByPageRsp` 开放 `parent_id` 与 `sub_device_addr` 字段至 JSON 序列化，支持前端拓扑图谱展示 |
| 列表查询计划优化 | `backend/internal/dal/devices_list_read_model_plan.go` | `applyConfigFieldFilters` 接入 `plan.req.ParentID` 条件过滤，支持按父网关快速检索直接子设备/子网关 |
| 拓扑安全与租户防线 | `backend/internal/service/device_update.go`, `device_create.go` | 增加 `parent_id` 安全校验：① 禁止将设备父节点设为自身；② 强制父网关与子设备归属同一租户；③ 递归深度回溯检测，阻断网关层级循环引用（Cycle Detection） |
| 自动化契约测试 | `automation_tests/tests/51_multilayer_gateway.test.js` | 编写 5 组端到端契约测试，覆盖 3 层网关拓扑构建、3 层嵌套载荷递归解包分发、稀疏载荷容错、级联绑定断言与多租户拓扑隔离防御 |

---

## 2. 自动化测试执行结果

### 2.1 51 组多层网关专项契约测试
```shell
$env:NO_PROXY="127.0.0.1,localhost"
npx mocha tests/51_multilayer_gateway.test.js --reporter spec
```
输出：
```text
  Multilayer Gateway Topology & Routing [51_multilayer_gateway]
    1. Multilayer Gateway Topology Construction
      √ builds a 3-tier gateway hierarchy: Top Gateway -> Sub Gateway -> Leaf Sub-device (55ms)
    2. Recursive Telemetry Uplink (TP-3 Full Depth Fan-out)
      √ processes nested GatewayPublish payload and fans out telemetry across 3 tiers (1256ms)
      √ correctly processes partial/sparse payload containing only leaf-device data without gateway_data (628ms)
    3. Downlink Hierarchy & Routing Proof
      √ recursively traces leaf-device back to top-level gateway and verifies configuration binding
    4. Multi-Tenant Topology Isolation
      √ prevents Tenant B from adopting or binding under Tenant A gateway

  5 passing (2s)
```

### 2.2 联合回归测试（28、46、48、49、50、51 套件）
```shell
npx mocha tests/28_calculated_fields.test.js tests/46_entity_relations.test.js tests/48_edge_node_ops.test.js tests/49_alarm_lifecycle_clear.test.js tests/50_calculated_field_relations.test.js tests/51_multilayer_gateway.test.js --reporter spec
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
      √ aggregates telemetry from explicitly declared device_ids upon trigger uplink (5183ms)
    4. Dynamic Discovery via entity_relations Graph
      √ dynamically resolves related devices from entity_relations and computes telemetry (5704ms)
    5. Gateway / Sub-Device Hierarchy Discovery (devices.parent_id)
      √ aggregates sub-device telemetry to gateway via parent_id hierarchy (5702ms)
    6. Telemetry Propagation (propagation) Across Entities
      √ propagates telemetry from source device to related target device (5172ms)
    7. Multi-Tenant Isolation
      √ prevents tenant B from reading or manipulating tenant A calculated field

  Multilayer Gateway Topology & Routing [51_multilayer_gateway]
    1. Multilayer Gateway Topology Construction
      √ builds a 3-tier gateway hierarchy: Top Gateway -> Sub Gateway -> Leaf Sub-device (54ms)
    2. Recursive Telemetry Uplink (TP-3 Full Depth Fan-out)
      √ processes nested GatewayPublish payload and fans out telemetry across 3 tiers (650ms)
      √ correctly processes partial/sparse payload containing only leaf-device data without gateway_data (1232ms)
    3. Downlink Hierarchy & Routing Proof
      √ recursively traces leaf-device back to top-level gateway and verifies configuration binding
    4. Multi-Tenant Topology Isolation
      √ prevents Tenant B from adopting or binding under Tenant A gateway

  70 passing (1m)
```

### 2.3 前端与后端编译全绿
- `npm run typecheck`: 0 错误（`vue-tsc` 0 错误）
- `npm test`: 431 test files passed (431), 3850 tests passed (3850), 0 failures.
- 后端编译：`$env:GOTOOLCHAIN="local"; go build -p 1 ./...` 0 错误，通过。
