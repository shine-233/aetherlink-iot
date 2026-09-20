-- ROADMAP TB-1（告警规则 2.0）第一片：告警评论。
--
-- 现状核对（2026-09-14）：
--   alarm_config 已有 name/description/alarm_level/notification_group_id/enabled/
--   trigger_duration；alarm_info 已有 processor（处理人）与 processing_result，
--   但 **alarm_info 是已废弃的 device-less 旧表**（见 service/alarm_notification.go
--   里 createAlarmInfoRecord 的注释：不得从 AlarmExecute 调用），
--   现代告警记录走 alarm_history（带 stream 身份与设备关联）。
--   因此评论挂 alarm_history，不挂 alarm_info —— 挂后者等于给废弃表加功能。
--   真正完全缺失的是**评论**；"指派"其实部分存在（alarm_info.processor），
--   而"可配置规则条件 / 严重度传播"需要改规则求值链路，属后续独立立项。
--
-- 约定与 99.sql 一致：varchar(36) 主键、tenant_id 非空 + CHECK、created_at 默认 now()，
-- 全部带 IF NOT EXISTS / NOT EXISTS 守卫，可重复执行。

CREATE TABLE IF NOT EXISTS public.alarm_comment (
    id               varchar(36) PRIMARY KEY,
    tenant_id        varchar(36) NOT NULL,
    alarm_history_id varchar(36) NOT NULL,
    content          text        NOT NULL,
    author_user_id   varchar(36) NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT alarm_comment_tenant_check  CHECK (tenant_id <> ''),
    CONSTRAINT alarm_comment_alarm_check   CHECK (alarm_history_id <> ''),
    CONSTRAINT alarm_comment_author_check  CHECK (author_user_id <> ''),
    CONSTRAINT alarm_comment_content_check CHECK (content <> '')
);

-- 列表查询固定按 (租户, 告警, 时间倒序)，索引与之一致。
CREATE INDEX IF NOT EXISTS alarm_comment_alarm_idx
    ON public.alarm_comment (tenant_id, alarm_history_id, created_at DESC);

-- Casbin：与 99.sql 同构，先补 g2 路由分组，再补 p 角色授权。
-- 只授 SYS_ADMIN / TENANT_ADMIN：告警历史的读权限对 TENANT_USER 是 fail-closed
-- （见 service/alarm.go 的 ensureAlarmHistoryReadAccess 与 owner 收窄逻辑），
-- 评论沿用同一策略，避免出现"casbin 放行但服务层必拒"的死授权。
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/alarm/info/history/:id/comment'),
  ('api/v1/alarm/info/history/:id/comment/:comment_id')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/alarm/info/history/:id/comment'),
  ('TENANT_ADMIN', 'api/v1/alarm/info/history/:id/comment'),
  ('SYS_ADMIN',    'api/v1/alarm/info/history/:id/comment/:comment_id'),
  ('TENANT_ADMIN', 'api/v1/alarm/info/history/:id/comment/:comment_id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
