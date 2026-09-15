-- ROADMAP TB-1（告警规则 2.0）第二片：告警指派 + 指派历史审计。
--
-- 为什么另立一张表，而不是给现有告警表加列：
--   1. alarm_info 上确实有 processor（处理人），但 **alarm_info 是已废弃的 device-less
--      旧表**（见 service/alarm_notification.go 里 createAlarmInfoRecord 的注释：
--      不得从 AlarmExecute 调用）。现代告警记录走 alarm_history（带 stream 身份与
--      设备关联），而 alarm_history 没有 processor 列。给废弃表加字段等于给死代码续命。
--   2. 更根本的是：指派要回答的是"谁在什么时候把这条告警指派给了谁、又在哪一次被取消"，
--      这是**流水问题不是字段问题**。任何单列（无论挂哪张表）都只能保存当前值，
--      每次改派都会覆盖上一次，审计信息随之丢失。
--   3. 本片只做"指派 + 审计"；"可配置规则条件"与"严重度传播"要改规则求值链路，
--      属后续独立立项，不在本迁移范围内。
--
-- 语义约定（服务层与消费方必须遵守，改动前先读）：
--   - 当前处理人 = 该告警**最新一条**记录的 assignee_user_id；为 NULL 即当前无人指派。
--   - 流水 append-only：不 UPDATE、不 DELETE。取消指派同样新写一行
--     assignee_user_id IS NULL，而不是删掉上一行 —— 历史永不改写，
--     "曾经指派过谁"必须能查到。
--   - assignee_user_id 允许 NULL 但**禁止空串**：空串会让人分不清"取消指派"与
--     "指派给了一个空 ID"，也让 `IS NULL` 判定失效，故加
--     CHECK (assignee_user_id IS NULL OR assignee_user_id <> '')。
--   - operator_user_id 非空：每条流水都必须能追责到操作人，允许匿名指派等于放弃审计。
--   - 重复指派同一个人不报错（幂等）：每次操作都留一行，语义是"再次确认"而非覆盖。
--
-- 与 101.sql（告警评论）同构：varchar(36) 主键、tenant_id 非空 + CHECK、
-- created_at 默认 now()，全部带 IF NOT EXISTS / NOT EXISTS 守卫，可重复执行。

CREATE TABLE IF NOT EXISTS public.alarm_assignment (
    id               varchar(36) PRIMARY KEY,
    tenant_id        varchar(36) NOT NULL,
    alarm_history_id varchar(36) NOT NULL,
    -- NULL = 取消指派；空串被 CHECK 拒绝（见上方语义约定）。
    assignee_user_id varchar(36),
    -- 谁做的这次指派/取消，永远非空。
    operator_user_id varchar(36) NOT NULL,
    remark           text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT alarm_assignment_tenant_check   CHECK (tenant_id <> ''),
    CONSTRAINT alarm_assignment_alarm_check    CHECK (alarm_history_id <> ''),
    CONSTRAINT alarm_assignment_assignee_check CHECK (assignee_user_id IS NULL OR assignee_user_id <> ''),
    CONSTRAINT alarm_assignment_operator_check CHECK (operator_user_id <> '')
);

-- 列表查询固定按 (租户, 告警, 时间倒序) —— 倒序是因为消费方第一个要的是
-- "当前处理人"，即最新一行，倒序能让它落在结果集首行。
CREATE INDEX IF NOT EXISTS alarm_assignment_alarm_idx
    ON public.alarm_assignment (tenant_id, alarm_history_id, created_at DESC);

-- Casbin：与 101.sql 同构，先补 g2 路由分组，再补 p 角色授权。
-- GET（列表）与 POST（指派）共用一条路径，只登记一次。
-- 只授 SYS_ADMIN / TENANT_ADMIN：告警历史的读权限对 TENANT_USER 是 fail-closed
-- （见 service/alarm.go 的 ensureAlarmHistoryReadAccess 与 owner 收窄逻辑），
-- 指派沿用同一策略，避免出现"casbin 放行但服务层必拒"的死授权。
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/alarm/info/history/:id/assignment')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/alarm/info/history/:id/assignment'),
  ('TENANT_ADMIN', 'api/v1/alarm/info/history/:id/assignment')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
