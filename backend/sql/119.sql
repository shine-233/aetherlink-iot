-- 119.sql — TB-13 地理空间追踪与看板地图部件（Geospatial Map Tracking & Dashboard Map Widget）
--
-- 背景：
--   1. 对标 ThingsBoard 4.0 "New Maps" 核心能力（多设备位置标绘、实时状态呈现与轨迹回放）；
--   2. 支持租户快速拉取全部设备的最新 GPS 经纬度位置（GET /api/v1/devices/locations/latest 及 GET /api/v1/device/locations/latest）；
--   3. 支持按时间范围拉取指定设备的历史行车/运动轨迹点（GET /api/v1/device/:id/location/history 及 GET /api/v1/devices/:id/location/history）；
--   4. Casbin 路由登记与赋权（SYS_ADMIN, TENANT_ADMIN, TENANT_USER）。

-- ---- 1. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/devices/locations/latest'),
  ('api/v1/device/locations/latest'),
  ('api/v1/devices/:device_id/location/history'),
  ('api/v1/devices/:id/location/history'),
  ('api/v1/device/:id/location/history')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 2. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/devices/locations/latest'),
  ('SYS_ADMIN',    'api/v1/device/locations/latest'),
  ('TENANT_ADMIN', 'api/v1/devices/locations/latest'),
  ('TENANT_ADMIN', 'api/v1/device/locations/latest'),
  ('TENANT_USER',  'api/v1/devices/locations/latest'),
  ('TENANT_USER',  'api/v1/device/locations/latest'),
  ('SYS_ADMIN',    'api/v1/devices/:device_id/location/history'),
  ('SYS_ADMIN',    'api/v1/devices/:id/location/history'),
  ('SYS_ADMIN',    'api/v1/device/:id/location/history'),
  ('TENANT_ADMIN', 'api/v1/devices/:device_id/location/history'),
  ('TENANT_ADMIN', 'api/v1/devices/:id/location/history'),
  ('TENANT_ADMIN', 'api/v1/device/:id/location/history'),
  ('TENANT_USER',  'api/v1/devices/:device_id/location/history'),
  ('TENANT_USER',  'api/v1/devices/:id/location/history'),
  ('TENANT_USER',  'api/v1/device/:id/location/history')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
