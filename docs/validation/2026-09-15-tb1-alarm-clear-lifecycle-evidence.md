# TB-1 告警规则 2.0：告警清除与四态生命周期运行期证据

> 日期：2026-09-15  
> 责任范围：ROADMAP TB-1 告警规则 2.0 第三片（对标 ThingsBoard 4.3 LTS 告警生命周期四态、显式/自动清除端点、批量清除、审计留痕、前端工作台接入与契约测试）  
> 执行基线：PostgreSQL 17.5（`127.0.0.1:55433`，数据库 `aetherlink_go99`，`sys_version=105`）+ GMQTT 回环 Broker（端口 1883）+ 后端服务（端口 9999）

---

## 1. 交付物清单

| 层次 | 文件路径 | 变更说明 |
| --- | --- | --- |
| 数据库迁移 | `backend/sql/105.sql` | 登记 `api/v1/alarm/info/history/:id/clear` 的 Casbin 权限规则（授予 SYS_ADMIN 与 TENANT_ADMIN） |
| 全局版本 | `backend/pkg/global/global.go` | `VERSION_NUMBER = 105` |
| 数据模型 | `backend/internal/model/alarm_info.http.go` | `AlarmHistoryActionResp` 增加 `cleared_by`, `cleared_at`, `lifecycle_status`；增加 `ClearAlarmReq` |
| 数据访问层 | `backend/internal/dal/alarm_history_actions.go` | 实现 `ClearAlarmHistory`、`ClearLoadedAlarmHistoryWithNote`、`computeAlarmLifecycleStatus`（四态：`ACTIVE_UNACK`, `ACTIVE_ACK`, `CLEARED_UNACK`, `CLEARED_ACK`）与 `actionAlarmHistoryClearRemark` |
| 数据访问层 | `backend/internal/dal/alarm.go` | 升级 `GetAlarmInfoHistoryByID`、`GetAlarmHistoryListByPage`、`GetAlarmHistoryListByPageForScopes`，展开 `lifecycle_status` 与 `cleared_by`/`cleared_at`/`acknowledged_by`/`acknowledged_at` |
| 服务层 | `backend/internal/service/alarm_history_actions.go` | 实现 `ClearAlarmHistory` 服务方法 |
| 服务层 | `backend/internal/service/alarm_history_batch_actions.go` | `batch-action` 批量操作原生支持 `"clear"` 动作 |
| API 控制器 | `backend/internal/api/alarm.go` | 新增 `ClearAlarmHistory` 处理函数（支持可选 JSON body note 传入） |
| 路由映射 | `backend/router/apps/alarm.go` | 挂载 `history/:id/clear`（PUT 与 POST 方法均支持） |
| 前端 API | `frontend/src/service/api/alarm.ts` | 导出 `clearAlarmHistory`，并将 `batchActionAlarmHistory` 的 action 扩展为 `'acknowledge' \| 'reset' \| 'clear'` |
| 前端逻辑 | `frontend/src/views/alarm/warning-message/components/alarm-configuration.helpers.ts` | 导出 `isCleared` 辅助判断函数 |
| 前端操作列 | `frontend/src/views/alarm/warning-message/components/alarmConfigurationColumns.tsx` | 增加 `onClear` 处理器、`actions` 列内增加带禁用态的“清除”按钮 |
| 前端工作台 | `frontend/src/views/alarm/warning-message/components/alarm-configuration.single-actions.ts` | 单条操作支持 `'clear'`，提供专用清除确认与审计原因提示文案 |
| 前端页面 | `frontend/src/views/alarm/warning-message/components/alarm-configuration.vue` | 接入 `clearAlarm` 并向列处理器传递 `onClear` |
| 契约测试 | `automation_tests/tests/49_alarm_lifecycle_clear.test.js` | 8 项自动化契约测试，覆盖四态流转、重复清除防护、POST 兼容、批量清除与跨租户越权防护 |

---

## 2. 自动化测试执行结果

### 2.1 49 组专项契约测试
```shell
$env:NO_PROXY="127.0.0.1,localhost"
npx mocha tests/49_alarm_lifecycle_clear.test.js --timeout 120000
```
输出：
```text
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

  8 passing (16s)
```

### 2.2 联合回归测试（43、44、48、49 套件）
```shell
npx mocha tests/43_alarm_comment.test.js tests/44_alarm_assignment.test.js tests/48_edge_node_ops.test.js tests/49_alarm_lifecycle_clear.test.js --timeout 120000
```
输出：
```text
  Alarm comment lifecycle [43_alarm_comment]
    √ rejects a comment whose content is empty
    √ rejects a comment whose content is only whitespace
    √ rejects a comment on an alarm history that does not exist
    √ creates a comment and lists it back
    √ trims surrounding whitespace instead of storing it
    √ lists comments in chronological order
    √ deletes a comment and it disappears from the list
    √ rejects deleting a comment that does not exist
    √ rejects reading comments from another tenant (57ms)
    √ rejects commenting on another tenant alarm history

  Alarm assignment audit trail [44_alarm_assignment]
    √ rejects an empty-string assignee (only null means unassign)
    √ rejects an assignment on an alarm history that does not exist
    √ rejects listing assignments for an alarm history that does not exist
    √ assigns the alarm to a tenant user and records the operator
    √ rejects assigning to a user that belongs to another tenant
    √ rejects assigning to a user that does not exist
    √ accepts re-assigning the same user as an idempotent audit entry
    √ lists assignments in reverse chronological order
    √ cancels the assignment by appending a null row and keeps history
    √ accepts an omitted assignee as a cancel
    √ rejects reading assignments from another tenant
    √ rejects assigning another tenant alarm history

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

  43 passing (32s)
```

### 2.3 前端 Vitest 与 Vue-TSC
- `npx vitest run src/views/alarm`: 13 个测试文件，**145/145 全部通过（145 passed）**。
- `pnpm run typecheck`: **通过，0 错误**。

---

## 3. 结论

`TB-1` 告警规则 2.0 的全部核心链路（告警评论、告警处理人指派审计、四态生命周期 `ACTIVE_UNACK / ACTIVE_ACK / CLEARED_UNACK / CLEARED_ACK`、自动与显式清除端点、批量清除、全生命周期审计留存与前端操作按钮）已全面落地并通过回归验证。
至此，对标 ThingsBoard 4.3 LTS 告警引擎的核心能力已全面追平！
