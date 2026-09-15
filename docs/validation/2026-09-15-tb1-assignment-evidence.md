# 2026-09-15 TB-1 第二片：告警指派与指派历史审计（证据）

对应 ROADMAP §7.1 `TB-1`（告警规则 2.0）剩余三片中的「指派历史审计」。
三片分别是：可配置规则条件 / 严重度传播（升级与清除）/ 指派历史审计；
本片是唯一**不依赖规则求值链路改造**、可独立闭环的一片，故优先做。

## 1. 设计前提（为什么要有新表）

`processor`（处理人）只存在于 `alarm_info`，而 `alarm_info` 是**已废弃**的 device-less 旧表
（见 `service/alarm_notification.go` 的 `createAlarmInfoRecord` 注释）。现代告警走 `alarm_history`，
其模型 `internal/model/alarm_history.gen.go` **没有 processor 列**。

因此指派不能复用旧字段，新增 `103.sql`：`alarm_assignment`。

**语义**：append-only 审计流水，不删不改。`assignee_user_id` 为 **NULL 表示"取消指派"**；
当前处理人 = 最新一条记录的 `assignee_user_id`（为 NULL 即当前无人指派）。
取消指派同样留一条 NULL 记录，历史永不改写。

约束（4 条 CHECK，均经 sqlite 实测真的会拦截）：

- `tenant_id <> ''`
- `alarm_history_id <> ''`
- `operator_user_id <> ''`
- `assignee_user_id IS NULL OR assignee_user_id <> ''` —— **禁止用空串冒充"取消指派"**

## 2. 交付物

| 层 | 文件 |
| --- | --- |
| 迁移 | `backend/sql/103.sql`（建表 + 索引 + Casbin g2/p 共 3 行） |
| 版本 | `backend/pkg/global/global.go` `VERSION_NUMBER` 102 → **103** |
| 模型 | `backend/internal/model/alarm_assignment.go` |
| DAL | `backend/internal/dal/alarm_assignment.go`（`CreateAlarmAssignment` / `ListAlarmAssignments`） |
| 服务 | `backend/internal/service/alarm_assignment.go` |
| API | `backend/internal/api/alarm_assignment.go` |
| 路由 | `backend/router/apps/alarm.go` → `POST/GET /alarm/info/history/:id/assignment` |
| 前端 | `AlarmAssignmentPanel.vue` + `alarm.ts` 两个 wrapper + 4 语言包各 12 键 + 挂进详情弹窗 |
| 用例 | `internal/dal/alarm_assignment_test.go`、`automation_tests/tests/44_alarm_assignment.test.js`、前端 `AlarmAssignmentPanel.test.ts` 10 条 |

安全口径（与评论片一致）：租户一律取**告警自身的 `tenant_id`**，不取 `claims.TenantID`
（SYS_ADMIN 的 claims 为空，取它会撞 CHECK）；读写前必过 `ensureAlarmHistoryReadAccess`；
被指派人必须存在**且属于该告警所在租户**（新增最小查询 `dal.GetUserTenantIDByID`）。

## 3. 运行证据

### 3.1 构建与 DAL 用例

```
cd backend && GOTOOLCHAIN=local go build -p 1 ./...                       → rc=0
GOTOOLCHAIN=local go test ./internal/dal/ -run AlarmAssignment -count=1 -p 1 -v
→ 3/3 PASS，日志中 sqlite 真实触发：
  CHECK constraint failed: alarm_assignment_assignee_check
  CHECK constraint failed: alarm_assignment_tenant_check
  CHECK constraint failed: alarm_assignment_operator_check
```

### 3.2 迁移应用到活库

本机无 psql CLI、`automation_tests` 亦无 pg 客户端，故用一次性 Go 小程序
（复用 `initialize.ViperInit + LoadDbConfig + ExecuteSQLFile`，跑完已移出仓库，未入库）：

```
before: sys_version.version_number = 102, program VERSION_NUMBER = 103
after : sys_version.version_number = 103, alarm_assignment_exists = true, casbin_rule_rows = 3
```

### 3.3 重启后端并自证新二进制生效

Go 不重新编译就没有新路由。重启前后对照：

```
旧二进制：/comment -> 401（路由存在）   /assignment -> 404（路由不存在）
新二进制：/comment -> 401               /assignment -> 401（路由已存在，被 JWT 拦）
health = 200
```

`/assignment` 从 404 变为 401，同时证明 Casbin fail-fast 审计没有把新路由拦在启动外。

### 3.4 回归既有门禁（43 组）

```
set -a && . ./.env.local && set +a
npx mocha tests/43_alarm_comment.test.js --timeout 90000  → 10 passing (13s)
```

### 3.5 新增用例（44 组）

```
npx mocha tests/44_alarm_assignment.test.js --timeout 90000
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
  12 passing (13s)
```

### 3.6 租户作用域审计（新增查询未踩绊线）

```
go test ./internal/dal/ -run TestTenantScopeQueryAudit -count=1 -p 1 -v
→ tenant-scope suspects=0 (frozen baseline)   PASS   ok 0.217s
```

### 3.7 前端

```
cd frontend && npx vitest run src/views/alarm/warning-message/components/__tests__/
→ Test Files 7 passed (7) / Tests 79 passed (79)   （含 AlarmAssignmentPanel 10 条）
cd frontend && npx vue-tsc --noEmit → rc=0
npx vitest run src/styles/__tests__/design-token-contract.test.ts → 2/2（新组件无 <style> 块，hex 基线不动）
npx vitest run src/locales/__tests__/ → 6/6
```

挂载点：`alarm-configuration.vue:738`，**未加 `v-if`**（`n-modal` 默认懒渲染，
`showDialog` 只在 `getInfo(row)` 写好 `infoData` 后为 true，不存在恒假死挂载）。

## 4. 本片仍未验证（不得宣称 done）

- **浏览器内真实点击路径未取证**：未跑 Playwright（本机被沙箱拦截），只有单测与 API 层证据。
- **全量 `go test ./...` 未跑**：本机链接期 OOM（见 AGENTS.md），只跑了 build 全包 + 分包测试。
- **全新空库全链 `CheckVersion` 只做到 93**：103.sql 是在 102 的活库上**正向应用**的，未从空库跑到 103。
- 44 组会在库里留下少量 append-only 指派流水（表无 DELETE 通道，设计如此）。

## 5. 提交

`957c839`（103.sql + 版本）、`a48cc68`（四层 + 路由）、`052cd45`（DAL 用例）、
`dcc7211`（44 组 E2E）、`b125bf4`（前端面板）。均未 push。
