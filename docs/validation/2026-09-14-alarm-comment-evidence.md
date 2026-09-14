# 2026-09-14 告警评论（ROADMAP TB-1 第一片）证据

> 对应 ROADMAP §7.2 TB-1。执行环境：PostgreSQL 55433 + Redis + `local_mqtt_broker.js` + 后端 9999。

## 0. 开工前的核对修正（三处）

动手前先核对了"告警规则 2.0"的缺口描述，发现原表述有三处不准：

| 原表述 | 实际 |
| --- | --- |
| 无独立"告警规则"实体 | `alarm_config` 已存在（name / description / alarm_level / notification_group_id / enabled / trigger_duration）。它确实是**初级形态**（无条件表达式、无传播），但不是"没有" |
| 无"指派" | `alarm_info.processor`（处理人 id）就是当前处理人字段，`processing_result`（DOP/UND/IGN）是处理状态。"指派"**部分存在**，缺的是指派**历史/审计** |
| 评论挂哪张表 | `alarm_info` 是**已废弃**的 device-less 旧表——`service/alarm_notification.go` 的 `createAlarmInfoRecord` 注释明写"不得从 AlarmExecute 调用"。现代告警记录走 `alarm_history`（带 stream 身份与设备关联） |

**结论**：本片只做**评论**（完全缺失、可独立验证），并挂到 `alarm_history` 而非废弃表。
"可配置规则条件 / 严重度传播"需要改规则求值链路，留作后续独立立项。

## 1. 交付物

### 1.1 迁移 `backend/sql/101.sql`

```sql
CREATE TABLE IF NOT EXISTS public.alarm_comment (
    id               varchar(36) PRIMARY KEY,
    tenant_id        varchar(36) NOT NULL,
    alarm_history_id varchar(36) NOT NULL,
    content          text        NOT NULL,
    author_user_id   varchar(36) NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ... CHECK (...)   -- tenant / alarm / author / content 四个非空检查
);
CREATE INDEX ... ON public.alarm_comment (tenant_id, alarm_history_id, created_at DESC);
-- Casbin：2 条 g2 路由分组 + 4 条 p 角色授权（SYS_ADMIN / TENANT_ADMIN）
```

- `VERSION_NUMBER` 100 → **101**。
- 幂等复跑：`INSERT 0 0`。
- **只授 SYS_ADMIN / TENANT_ADMIN**：告警历史的读权限对 TENANT_USER 是 fail-closed
  （`alarm_history` 的 owner 收窄见 `service/alarm.go`），评论沿用同一策略，
  避免出现"casbin 放行但服务层必拒"的死授权。

### 1.2 路由

| 方法 | 路径 |
| --- | --- |
| POST | `/api/v1/alarm/info/history/:id/comment` |
| GET | `/api/v1/alarm/info/history/:id/comment` |
| DELETE | `/api/v1/alarm/info/history/:id/comment/:comment_id` |

路径形状是有约束的：Gin 的路由树不允许同一段既有 `:id` 又有静态串，
`history/comment/:id` 会与 `history/:id` 冲突并 panic，所以删除也用 `:comment_id` 参数。

### 1.3 分层

- `model/alarm_comment.go`：`AlarmComment` + 三个请求结构（URI 字段带 `uri` tag）。
- `dal/alarm_comment.go`：增 / 按 (租户,告警) 列 / 按 (租户,ID) 取 / 按 (租户,ID) 删，
  外加一个**只给 SYS_ADMIN 路径**用的 `GetAlarmCommentByIDUnscoped`。全部查询带 `tenant_id`。
- `service/alarm_comment.go`：
  - 读写前统一过 `ensureAlarmHistoryReadAccess`（校验告警存在 + 租户边界 + owner 作用域）。
  - **租户取告警自身**而非 `claims.TenantID`——SYS_ADMIN 的 TenantID 为空，取 claims 会写出
    `tenant_id=''` 直接撞 CHECK 约束。
  - 删除限作者本人或 TENANT_ADMIN/SYS_ADMIN；且**先校验告警可见、再校验评论归属**，
    防止拿 A 告警的权限去删 B 告警的评论。
- `api/alarm_comment.go`：只做绑定与转发，权限判定不复制到 handler。

## 2. 开发中真实踩到的坑

**`ShouldBindUri` 之后立刻校验整个结构体会误拒合法请求。**
第一版 handler 在 URI 绑定后直接 `ValidateStructLang(&req)`，此时正文字段
`Content` 还没绑定，于是所有创建请求都被判成 `Field 'Content' is required`（100002）。
修法：把 URI 绑定与结构体校验拆开，正文绑定之后再统一校验。
这个错误的表现很像"客户端没传 body"，实际是服务端校验顺序错了。

## 3. 运行期证据

### 3.1 API 契约测试 `automation_tests/tests/43_alarm_comment.test.js`（10 例，全绿）

夹具走 `seedData.ensureSceneAlarmHistory`：建场景 → 激活 → 产生一条真实 `alarm_history`。
该夹具在环境不满足时返回 `blocked`，本用例把它当**硬失败**而非跳过——
没有真实告警历史就一条链路都验证不了，跳过会让"0 覆盖"看起来像通过。

```
  Alarm comment lifecycle [43_alarm_comment]
    √ rejects a comment whose content is empty
    √ rejects a comment whose content is only whitespace
    √ rejects a comment on an alarm history that does not exist
    √ creates a comment and lists it back
    √ trims surrounding whitespace instead of storing it
    √ lists comments in chronological order
    √ deletes a comment and it disappears from the list
    √ rejects deleting a comment that does not exist
    √ rejects reading comments from another tenant
    √ rejects commenting on another tenant alarm history
  10 passing
```

跨租户用 `tenant_admin_b` 而不是 TENANT_USER：后者对告警历史本身就是 fail-closed，
测出来的拒绝分不清是"跨租户被拦"还是"角色本身没权限"。

### 3.2 Go 单测 `internal/dal/alarm_comment_test.go`（3 例，全绿）

- 租户隔离 + 同租户不同告警的过滤 + 时间正序
- 跨租户按 ID 取**未命中**（而非"存在但无权"）、跨租户删除是 no-op
- `Unscoped` 查询只供管理员路径
- 无评论时返回空切片而非 nil

### 3.3 回归

| 项 | 结果 |
| --- | --- |
| `go build -p 1 ./...` | exit 0 |
| `go test -p 1 ./...` | exit 0，**61 包全 ok** |
| API E2E `tests/38–43` | **48/48 通过** |
| OpenAPI | 413 → **415 paths**，两个评论端点已收录 |

## 4. 仍未完成（TB-1 剩余部分）

- **可配置规则条件**：`alarm_config` 目前只有级别/通知组/持续时长，没有条件表达式。
- **严重度传播**：无跨实体传播与关联聚合。
- **指派历史审计**：`processor` 只有当前值，没有"谁在何时指派给谁"的流水。
- 前端尚未接入评论 UI（本片只做后端能力）。

## 5. 环境提示

- 本机 `alarm_info` / `alarm_history` / `alarm_config` 三表初始都是空的，
  任何"读告警"的用例都必须先跑 `ensureSceneAlarmHistory` 造数据，不能假设有存量。
- 重启后端前先确认 `local_mqtt_broker.js` 在 1883 上——否则后端启动即 FATAL。
