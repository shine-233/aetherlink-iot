-- 补登 P1.3 SCADA / P1.4 移动端 / P0.2 影子 ACK 三批路由的 Casbin 资源与授权。
--
-- 背景：router_init.go 在 v1.Use(middleware.CasbinRBAC()) 之后（:245 取基线快照）
-- 才挂载 Scada(:286)、Mobile(:287)、DeviceShadow(:325) 三组路由，因此这三组全部落入
--「受 Casbin 保护」集合。而 auditCasbinRouteCoverage 在 casbin.route-audit-mode
-- 默认 fail-fast 下会对任何未登记路由执行 logrus.Fatalf——后端启动期直接拒绝启动
-- （见 router/casbin_audit.go:104、90.sql 与 91.sql 的两次同类修复）。
--
-- 匹配口径决定了「顺带覆盖」不成立：
--   * internal/service/casbin_route_audit.go:26 先做精确判定；
--   * 回退通道 pkg/utils/url_pattern.go:32 是 ^…$ 锚定正则，模式里的 ":name"
--     只匹配**单个**路径段（[^/]+）。
--   因此 63.sql 已登记的 'api/v1/device/shadow/:deviceId/:msgId' 匹配不到多出来的
--   '/ack' 段；'api/v1/scada/projects/:id' 也匹配不到 'projects/:id/documents'。
--   每一条路径都必须显式登记，不存在靠父路径兜底。
--
-- 登记粒度：审计用 addedKeysToPaths 去掉 METHOD 后按**路径**去重，故同一路径上的
-- 多个 HTTP 方法（如 scada/projects 的 GET+POST）只需登记一次。
--
-- 全部 INSERT 带 WHERE NOT EXISTS，可重跑；已存在的记录不会被改写。

-- ① g2 资源登记：把路径注册为 Casbin 资源。
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  -- P1.3 SCADA / Widget 基础层（router/apps/scada.go:20-40，15 个端点 / 11 条路径）
  ('api/v1/scada/projects'),
  ('api/v1/scada/projects/:id'),
  ('api/v1/scada/projects/:id/documents'),
  ('api/v1/scada/documents/:id'),
  ('api/v1/scada/documents/:id/publish'),
  ('api/v1/scada/documents/:id/rollback'),
  ('api/v1/scada/documents/:id/archive'),
  ('api/v1/scada/documents/:id/versions'),
  ('api/v1/scada/documents/:id/audits'),
  ('api/v1/scada/control/confirm'),
  ('api/v1/scada/control'),

  -- P1.4 移动端控制与通知（router/apps/mobile.go:18-31，11 个端点 / 10 条路径）
  ('api/v1/mobile/capabilities'),
  ('api/v1/mobile/push/subscribe'),
  ('api/v1/mobile/push/:id'),
  ('api/v1/mobile/commands'),
  ('api/v1/mobile/devices'),
  ('api/v1/mobile/alarms'),
  ('api/v1/mobile/alarms/:id/ack'),
  ('api/v1/mobile/devices/:id/shadow'),
  ('api/v1/mobile/devices/:id/ota'),
  ('api/v1/mobile/dashboards'),

  -- P0.2 设备影子 ACK（router/apps/device_shadow.go:21）
  ('api/v1/device/shadow/:deviceId/:msgId/ack'),

  -- P0.3 批次生命周期（router/apps/command_data.go）：暂停/恢复/回滚/进度消费。
  -- 这四个端点本批新增，用于把此前"只有服务层实现、无入口"的能力暴露出来。
  ('api/v1/command/datas/jobs/:job_id/pause'),
  ('api/v1/command/datas/jobs/:job_id/resume'),
  ('api/v1/command/datas/jobs/:job_id/rollback'),
  ('api/v1/command/datas/jobs/:job_id/progress'),

  -- P0.3 批次报告导出：与上面四端点同源，同样是在 63.sql 之后才加的端点。
  -- 63.sql 登记了同层的 jobs/:job_id/rows 与 support-bundle，report 是后补的，漏了。
  -- 只读导出，故随 ② 授予三个内置角色。
  ('api/v1/command/datas/jobs/:job_id/report')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ② p 授权（普通面）：三个内置角色均可访问。
--
-- 这里沿用 63.sql 对 'api/v1/command/datas/pub' 的处理——命令下发类接口同样授予
-- TENANT_USER。属于**刻意沿用既有策略**而非新立规则：收紧它会立刻改变移动端与
-- SCADA 的既有可达性，应当作为独立的产品决策来做，而不是藏在一次补登记里。
-- 唯一例外是 scada/control 与 scada/control/confirm，见 ③。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, t.path, 'allow'
FROM (VALUES ('SYS_ADMIN'), ('TENANT_ADMIN'), ('TENANT_USER')) AS r(role)
CROSS JOIN (VALUES
  ('api/v1/scada/projects'),
  ('api/v1/scada/projects/:id'),
  ('api/v1/scada/projects/:id/documents'),
  ('api/v1/scada/documents/:id'),
  ('api/v1/scada/documents/:id/publish'),
  ('api/v1/scada/documents/:id/rollback'),
  ('api/v1/scada/documents/:id/archive'),
  ('api/v1/scada/documents/:id/versions'),
  ('api/v1/scada/documents/:id/audits'),

  ('api/v1/mobile/capabilities'),
  ('api/v1/mobile/push/subscribe'),
  ('api/v1/mobile/push/:id'),
  ('api/v1/mobile/commands'),
  ('api/v1/mobile/devices'),
  ('api/v1/mobile/alarms'),
  ('api/v1/mobile/alarms/:id/ack'),
  ('api/v1/mobile/devices/:id/shadow'),
  ('api/v1/mobile/devices/:id/ota'),
  ('api/v1/mobile/dashboards'),

  ('api/v1/device/shadow/:deviceId/:msgId/ack'),

  ('api/v1/command/datas/jobs/:job_id/report')
) AS t(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
   WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = t.path AND c.v2 = 'allow'
);

-- ③ p 授权（控制面）：SCADA 实时控制 + P0.3 批次生命周期，只授予管理员。
--
-- 与 93.sql 对规则链 publish/rollback 的处理同因：这些端点会改变线上正在执行的东西，
-- 且不是"编辑草稿"这类可回滚的动作。
--   * scada/control 把命令真正下发到设备；
--   * jobs/:job_id/pause|resume|rollback 改变批次的执行走向；
--   * jobs/:job_id/progress 写入进度——放给 TENANT_USER 等于允许任意普通用户
--     伪造推进速度，而这正是 P0.3 去重令牌刻意要防的那件事，故一并收紧。
--
-- internal/service/scada_control.go 内部还有五道闸（存在性 → 归档终态 → 已注册 →
-- 权限 → 二次确认）与审计先于执行，这里的角色限制是又一层，不是唯一一层。
--
-- 已知取舍：若将来协议网关需要以非管理员身份上报进度，正确做法是给它一个专用角色，
-- 而不是把 TENANT_USER 放宽——后者会让所有普通用户一并获得伪造进度的能力。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/scada/control/confirm'),
  ('TENANT_ADMIN', 'api/v1/scada/control/confirm'),
  ('SYS_ADMIN',    'api/v1/scada/control'),
  ('TENANT_ADMIN', 'api/v1/scada/control'),
  ('SYS_ADMIN',    'api/v1/command/datas/jobs/:job_id/pause'),
  ('TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/pause'),
  ('SYS_ADMIN',    'api/v1/command/datas/jobs/:job_id/resume'),
  ('TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/resume'),
  ('SYS_ADMIN',    'api/v1/command/datas/jobs/:job_id/rollback'),
  ('TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/rollback'),
  ('SYS_ADMIN',    'api/v1/command/datas/jobs/:job_id/progress'),
  ('TENANT_ADMIN', 'api/v1/command/datas/jobs/:job_id/progress')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
   WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
