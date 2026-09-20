-- 106.sql — TP-5 资源中心（设备物模型 + 大屏看板统一市场与统一打包分发）
--
-- 背景：
--   对标 ThingsPanel 1.2.8 资源中心核心能力：
--   1. 在 boards 表中扩展 type_key（行业分类）、author（作者）、version（版本号）、preview_url（预览图）与 download_count（下载量）；
--   2. 登记资源中心与大屏模板导出/导入的 Casbin 权限规则。

-- 1. 扩充 boards 字段
ALTER TABLE boards ADD COLUMN IF NOT EXISTS type_key varchar(64) DEFAULT '';
ALTER TABLE boards ADD COLUMN IF NOT EXISTS author varchar(99) DEFAULT '';
ALTER TABLE boards ADD COLUMN IF NOT EXISTS version varchar(36) DEFAULT '1.0.0';
ALTER TABLE boards ADD COLUMN IF NOT EXISTS preview_url varchar(255) DEFAULT '';
ALTER TABLE boards ADD COLUMN IF NOT EXISTS download_count bigint DEFAULT 0;

-- 2. 登记 Casbin 路由 (g2)
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/resource/center/catalog'),
  ('api/v1/resource/center/list'),
  ('api/v1/resource/center/detail/:type/:id'),
  ('api/v1/resource/center/bundle'),
  ('api/v1/resource/center/bundle/import'),
  ('api/v1/resource/center/apply'),
  ('api/v1/board/export/:id'),
  ('api/v1/board/import')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 3. 授权角色 (p)
-- SYS_ADMIN 与 TENANT_ADMIN 拥有完整管理、打包与导入权限
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/resource/center/catalog'),
  ('SYS_ADMIN',    'api/v1/resource/center/list'),
  ('SYS_ADMIN',    'api/v1/resource/center/detail/:type/:id'),
  ('SYS_ADMIN',    'api/v1/resource/center/bundle'),
  ('SYS_ADMIN',    'api/v1/resource/center/bundle/import'),
  ('SYS_ADMIN',    'api/v1/resource/center/apply'),
  ('SYS_ADMIN',    'api/v1/board/export/:id'),
  ('SYS_ADMIN',    'api/v1/board/import'),

  ('TENANT_ADMIN', 'api/v1/resource/center/catalog'),
  ('TENANT_ADMIN', 'api/v1/resource/center/list'),
  ('TENANT_ADMIN', 'api/v1/resource/center/detail/:type/:id'),
  ('TENANT_ADMIN', 'api/v1/resource/center/bundle'),
  ('TENANT_ADMIN', 'api/v1/resource/center/bundle/import'),
  ('TENANT_ADMIN', 'api/v1/resource/center/apply'),
  ('TENANT_ADMIN', 'api/v1/board/export/:id'),
  ('TENANT_ADMIN', 'api/v1/board/import'),

  ('TENANT_USER',  'api/v1/resource/center/catalog'),
  ('TENANT_USER',  'api/v1/resource/center/list'),
  ('TENANT_USER',  'api/v1/resource/center/detail/:type/:id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
