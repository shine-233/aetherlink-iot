# TB-1 告警规则 2.0 终章（Alarm Rules 2.0 via Calculated Fields）运行期证据

> 日期：2026-09-16  
> 责任范围：ROADMAP TB-1 告警规则 2.0 终章（完全对标 ThingsBoard 4.3 LTS PR#14036 `CalculatedFieldType.ALARM` / `AlarmCalculatedFieldConfiguration.java`：遥测驱动计算告警字段、多严重度阶梯告警规则、严重度平滑升级、自愈自动清除、实体与拓扑告警传播、多租户防线）  
> 执行基线：PostgreSQL 17.5（`127.0.0.1:55433`，数据库 `aetherlink_go99`，`sys_version=105`）+ MQTT Broker（端口 1883）+ 后端服务（端口 9999）

---

## 1. 架构设计与代码变更清单

| 层次 | 文件路径 | 变更说明 |
| --- | --- | --- |
| 类型定义与配置校验 | `backend/internal/calcfield/types/types.go` | ① 扩展常量 `TypeAlarm = "alarm"`；② 增加 `AlarmSeverityRule`、`AlarmClearRule` 结构，并在 `AdvancedConfig` 扩展 `alarm_name`、`rules`、`clear_rule`、`propagate` 字段；③ 在 `ParseAdvancedConfig` 中实现完整的 `govaluate` 表达式语法校验、严重度等级约束（`H`, `M`, `L`）和自愈规则校验 |
| 数据访问层（DAL） | `backend/internal/dal/alarm_rules_cf.go` | ① `GetActiveAlarmHistoryForDeviceAndName`：基于 `alarm_device_list::text` 与 `alarm_status IN ('H', 'M', 'L')` 准确检索特定设备与告警规则的活动告警；② `EscalateAlarmHistorySeverity`：就地升级活动告警的严重度与描述内容，消除重复报警噪声；③ `AutoClearAlarmHistoryRecord`：与生命周期系统联动，自动清除活动告警并置为 `CLEARED_UNACK`（状态 'N'，记录 `auto-cleared by rule`） |
| 计算字段告警引擎 | `backend/internal/calcfield/alarm_eval.go` | 实现 `evaluateAlarmRule`：① 规则按严重度（H > M > L）降序求值；② 首次触发时落库 `alarm_history`（`ACTIVE_UNACK`）；③ 遥测恶化时触发平滑升级；④ 遥测恢复正常且命中 `clear_rule` 时自动生命周期清除；⑤ 支持 `propagate: true` 跨层级/关联实体广播告警 |
| 计算字段总线路由 | `backend/internal/calcfield/advanced.go` | 接入 `FieldTypeAlarm` 分支，由计算字段统一上行调度驱动，将告警状态同步写入派生遥测并驱动告警流转 |
| 单元测试套件 | `backend/internal/calcfield/advanced_test.go` | 新增 5 组 `TypeAlarm` 配置校验单元测试（合法规则、缺 rules、非法 severity、非法表达式、合法传播），全部一次性通过 |
| 自动化端到端契约测试 | `automation_tests/tests/52_alarm_rules_advanced.test.js` | 编写 18 组完整端到端契约测试，覆盖参数模式校验、告警字段 CRUD、遥测触发生成活动告警、严重度就地升级、自愈自动清除、拓扑传播与租户隔离防线 |

---

## 2. 自动化测试执行结果

### 2.1 52 组告警规则 2.0 专项契约测试
```shell
$env:NO_PROXY="127.0.0.1,localhost"
npx mocha tests/52_alarm_rules_advanced.test.js --reporter spec
```
输出：
```text
  Alarm Rules 2.0 via Calculated Fields [52_alarm_rules_advanced]
    1. Schema & Configuration Validation
      √ rejects alarm type without rules array
      √ rejects alarm rule with invalid severity
      √ rejects alarm rule with empty expression
      √ rejects alarm rule with malformed expression syntax
      √ rejects alarm with malformed clear_rule expression syntax
    2. Alarm Field CRUD & Lifecycle Toggle
      √ creates an alarm calculated field successfully
      √ updates alarm calculated field configuration
      √ toggles the enabled status of the alarm field
      √ reads the updated alarm field via list and detail endpoints
      √ deletes the alarm calculated field
    3. Telemetry Trigger, Severity Escalation & Auto-Clear Lifecycle
      √ triggers Medium severity alarm when telemetry reaches 65 (5719ms)
      √ escalates alarm severity in-place to High (M -> H) without duplicate noise when temp reaches 85 (5179ms)
      √ self-heals and auto-clears the alarm when telemetry recovers below clear threshold (temp = 25) (5178ms)
    4. Topology Alarm Propagation (Broadcast to Parent Gateway)
      √ propagates the alarm to the parent gateway when the subdevice triggers an alarm (5683ms)
    5. Multi-Tenant Isolation
      √ prevents Tenant B from retrieving Tenant A alarm calculated field
      √ prevents Tenant B from modifying Tenant A alarm calculated field
      √ prevents Tenant B from toggling Tenant A alarm calculated field
      √ prevents Tenant B from deleting Tenant A alarm calculated field

  18 passing (22s)
```

### 2.2 联合回归测试（28、46、48、49、50、51、52 套件）
```shell
npx mocha tests/28_calculated_fields.test.js tests/46_entity_relations.test.js tests/48_edge_node_ops.test.js tests/49_alarm_lifecycle_clear.test.js tests/50_calculated_field_relations.test.js tests/51_multilayer_gateway.test.js tests/52_alarm_rules_advanced.test.js --reporter spec
```
输出：
```text
  Calculated fields API boundary [28_calculated_fields] (4 passing)
  Generic entity relations [46_entity_relations] (26 passing)
  Edge node operations [48_edge_node_ops] (13 passing)
  Alarm clear lifecycle [49_alarm_lifecycle_clear] (8 passing)
  Calculated Fields with Entity Relations [50_calculated_field_relations] (14 passing)
  Multilayer Gateway Topology & Routing [51_multilayer_gateway] (5 passing)
  Alarm Rules 2.0 via Calculated Fields [52_alarm_rules_advanced] (18 passing)

  88 passing (1m)
```

### 2.3 后端构建与单元测试验证
```shell
$env:GOTOOLCHAIN="local"; go test -v ./internal/calcfield/...
$env:GOTOOLCHAIN="local"; go build -p 1 ./...
```
输出：
- `go test -v ./internal/calcfield/...`：全部通过（`PASS ok aetherlink-iot/backend/internal/calcfield 1.085s`）
- `go build -p 1 ./...`：退出码 0，零警告零编译错误。

### 2.4 前端类型与单元测试验证
```shell
npm run typecheck
npm test
```
输出：
- `npm run typecheck`：`vue-tsc --noEmit --skipLibCheck` 0 错误；
- `npm test`：`431 passed (431)` test files，`3850 passed (3850)` tests 全部全绿。

---

## 3. 结论

TB-1 告警规则 2.0 终章（对标 ThingsBoard 4.3 LTS `CalculatedFieldType.ALARM`）已全面闭环：
1. **参数与模式校验**：严格对齐 OpenAPI 与 ThingsBoard 字段约束，非法表达式/非法严重度/空规则均能在 API 参数层被精确阻断（100002）；
2. **多严重度阶梯判定与平滑升级**：支持高/中/低多阶梯规则，遥测恶化时就地平滑升级活动告警，不制造重复告警记录；
3. **自愈与自动生命周期清除**：遥测恢复并命中 `clear_rule` 时自动流转至 `CLEARED_UNACK`（状态 'N'，操作人 'system'）；
4. **拓扑告警广播与溯源**：支持通过 `propagate: true` 将子设备告警广播至父网关或关联实体，且保留完整的来源追溯审计标记；
5. **租户边界绝对隔离**：跨租户不可见、不可改、不可删、不可触发、不可升级。
