-- 118.sql — TB-17 自定义角色与细粒度权限控制体系（Custom RBAC & Granular Permissions）
--
-- 背景：
--   1. 对标 ThingsBoard PE Advanced RBAC 与企业级多租户权限细化要求；
--   2. 支持租户管理员在固定系统角色之外创建自定义业务角色（如设备运维员、数据分析师、只读访客、安全审计员等）；
--   3. 提供系统权限点字典表 sys_permissions 与角色-权限绑定表 sys_role_permissions；
--   4. 动态生成并同步 Casbin p 规则（p, role_id, pattern, allow），打通自定义角色与 URL 级强制鉴权；
--   5. Casbin 路由登记与赋权：
--      - /api/v1/permissions
--      - /api/v1/roles/:id/permissions
--      - /api/v1/roles/:id/users

-- ---- 1. 权限点字典表 ----
CREATE TABLE IF NOT EXISTS public.sys_permissions (
    code varchar(64) PRIMARY KEY,
    name varchar(100) NOT NULL,
    module varchar(50) NOT NULL,
    description text,
    api_patterns jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sys_permissions_module ON public.sys_permissions(module);

-- ---- 2. 角色-权限关联表 ----
CREATE TABLE IF NOT EXISTS public.sys_role_permissions (
    id varchar(36) PRIMARY KEY,
    role_id varchar(36) NOT NULL,
    permission_code varchar(64) NOT NULL,
    tenant_id varchar(36) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_role_permission UNIQUE (role_id, permission_code)
);

CREATE INDEX IF NOT EXISTS idx_sys_role_permissions_role_id ON public.sys_role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_sys_role_permissions_tenant_id ON public.sys_role_permissions(tenant_id);

-- ---- 3. 内置标准权限点种子数据 ----
INSERT INTO public.sys_permissions (code, name, module, description, api_patterns)
VALUES
  ('device:read', '查看设备与模板', 'device', '允许浏览设备列表、设备详情、产品物模型配置与分组树',
   '["api/v1/device", "api/v1/device/:id", "api/v1/device/list", "api/v1/device/group", "api/v1/device/group/tree", "api/v1/device/config", "api/v1/device/config/:id"]'::jsonb),
  ('device:write', '管理设备与模板', 'device', '允许创建、更新、删除设备与产品配置，导入预注册',
   '["api/v1/device", "api/v1/device/batch", "api/v1/device/preRegister", "api/v1/device/config", "api/v1/product"]'::jsonb),
  ('device:control', '下发设备命令与影子控制', 'device', '允许执行设备下行控制、RPC 指令下发与设备影子 ACK',
   '["api/v1/device/command", "api/v1/device/command/batch", "api/v1/device/shadow/:deviceId/:msgId/ack", "api/v1/device/shadow/messages"]'::jsonb),
  ('telemetry:read', '读取遥测与数据分析', 'telemetry', '允许读取实时遥测、历史聚合数据、同比环比与分析导出',
   '["api/v1/telemetry/datas/current", "api/v1/telemetry/datas/statistic", "api/v1/telemetry/analysis/query", "api/v1/telemetry/analysis/export", "api/v1/telemetry/analysis/anomaly"]'::jsonb),
  ('alarm:read', '查看告警历史与配置', 'alarm', '允许读取告警记录、告警配置与统计数据',
   '["api/v1/alarm/history", "api/v1/alarm/history/:id", "api/v1/alarm/config", "api/v1/alarm_config"]'::jsonb),
  ('alarm:operate', '告警操作与处置', 'alarm', '允许确认告警、清除告警、指派责任人与添加评论',
   '["api/v1/alarm/history/:id/ack", "api/v1/alarm/history/:id/clear", "api/v1/alarm/history/:id/assign", "api/v1/alarm/history/:id/comments"]'::jsonb),
  ('rule:read', '查看规则链与版本', 'rule_chain', '允许浏览规则链图谱、版本快照、死信队列与 Trace 追踪',
   '["api/v1/rule/chains", "api/v1/rule/chain/:id", "api/v1/rule/chain/versions", "api/v1/rule/chain/dead_letters", "api/v1/rule/chain/traces"]'::jsonb),
  ('rule:write', '编排与发布规则链', 'rule_chain', '允许编辑规则链、发布新版本、回滚与重放',
   '["api/v1/rule/chain", "api/v1/rule/chain/publish", "api/v1/rule/chain/rollback", "api/v1/rule/chain/replay"]'::jsonb),
  ('report:read', '查看报表与历史', 'report', '允许浏览报表调度计划、运行历史与交付记录',
   '["api/v1/report_schedules", "api/v1/report_schedules/:id/runs"]'::jsonb),
  ('report:write', '管理与触发报表', 'report', '允许创建、更新、删除报表任务及手动即时执行',
   '["api/v1/report_schedules", "api/v1/report_schedules/:id/run"]'::jsonb),
  ('scada:read', '查看 SCADA 画布与符号', 'scada', '允许浏览组态工程、画布文档与工业符号库',
   '["api/v1/scada/projects", "api/v1/scada/documents", "api/v1/scada/documents/:id"]'::jsonb),
  ('scada:control', 'SCADA 工业控制下发', 'scada', '允许通过 HMAC 二次确认下发工业组态控制命令',
   '["api/v1/scada/controls/dispatch", "api/v1/scada/controls/confirm"]'::jsonb),
  ('ota:read', '查看固件包与升级任务', 'ota', '允许查看 OTA 固件包列表与升级批次明细',
   '["api/v1/ota/packages", "api/v1/ota/tasks", "api/v1/ota/tasks/:id/report"]'::jsonb),
  ('ota:write', '创建与管理 OTA 升级', 'ota', '允许上传固件包、创建升级任务、暂停与回滚批次',
   '["api/v1/ota/packages", "api/v1/ota/tasks", "api/v1/ota/tasks/:id/pause", "api/v1/ota/tasks/:id/resume", "api/v1/ota/tasks/:id/rollback"]'::jsonb)
ON CONFLICT (code) DO NOTHING;

-- ---- 4. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/permissions'),
  ('api/v1/role/:id/permissions'),
  ('api/v1/role/:id/users'),
  ('api/v1/roles/:id/permissions'),
  ('api/v1/roles/:id/users')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 5. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/permissions'),
  ('TENANT_ADMIN', 'api/v1/permissions'),
  ('SYS_ADMIN',    'api/v1/role/:id/permissions'),
  ('TENANT_ADMIN', 'api/v1/role/:id/permissions'),
  ('SYS_ADMIN',    'api/v1/role/:id/users'),
  ('TENANT_ADMIN', 'api/v1/role/:id/users'),
  ('SYS_ADMIN',    'api/v1/roles/:id/permissions'),
  ('TENANT_ADMIN', 'api/v1/roles/:id/permissions'),
  ('SYS_ADMIN',    'api/v1/roles/:id/users'),
  ('TENANT_ADMIN', 'api/v1/roles/:id/users')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
