-- 121.sql — TP-6 / TB PE 算法中心：设备综合健康度评估与健康评分引擎 (Device Health Score Engine)
--
-- 背景：
--   1. 对标 ThingsBoard PE 异常状态诊断与 ThingsPanel 企业版算法中心 (Health Score / MSET)；
--   2. 多维健康量化评分 (0.00 ~ 100.00)：结合活跃告警严重度加权、在线/离线连续时长衰减与物模型异常惩罚；
--   3. 四级健康等级划分：HEALTHY (健康 >=85)、SUB_HEALTHY (亚健康 70~84)、WARNING (注意 50~69)、CRITICAL (严重危险 <50)；
--   4. 提供租户设备健康度看板统计 (GET /api/v1/devices/health/summary) 与单设备健康诊断 (GET /api/v1/devices/:device_id/health)；
--   5. Casbin 路由登记与赋权 (SYS_ADMIN, TENANT_ADMIN)。

-- ---- 1. 设备健康评分表 (device_health_scores) ----
CREATE TABLE IF NOT EXISTS device_health_scores (
    id VARCHAR(36) PRIMARY KEY,
    device_id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    score NUMERIC(5,2) NOT NULL DEFAULT 100.00,
    health_status VARCHAR(32) NOT NULL DEFAULT 'HEALTHY',
    alarm_penalty NUMERIC(5,2) NOT NULL DEFAULT 0.00,
    offline_penalty NUMERIC(5,2) NOT NULL DEFAULT 0.00,
    anomaly_penalty NUMERIC(5,2) NOT NULL DEFAULT 0.00,
    details TEXT NOT NULL DEFAULT '{}',
    evaluated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_device_health_scores_tenant_device UNIQUE (tenant_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_device_health_scores_tenant_status ON device_health_scores(tenant_id, health_status);
CREATE INDEX IF NOT EXISTS idx_device_health_scores_device ON device_health_scores(device_id);

-- ---- 2. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/devices/health/summary'),
  ('api/v1/devices/health/evaluate'),
  ('api/v1/devices/:device_id/health'),
  ('api/v1/devices/:device_id/health/evaluate'),
  ('api/v1/device/health/summary'),
  ('api/v1/device/health/evaluate'),
  ('api/v1/device/:id/health'),
  ('api/v1/device/:id/health/evaluate')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 3. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/devices/health/summary'),
  ('SYS_ADMIN',    'api/v1/devices/health/evaluate'),
  ('SYS_ADMIN',    'api/v1/devices/:device_id/health'),
  ('SYS_ADMIN',    'api/v1/devices/:device_id/health/evaluate'),
  ('SYS_ADMIN',    'api/v1/device/health/summary'),
  ('SYS_ADMIN',    'api/v1/device/health/evaluate'),
  ('SYS_ADMIN',    'api/v1/device/:id/health'),
  ('SYS_ADMIN',    'api/v1/device/:id/health/evaluate'),
  ('TENANT_ADMIN', 'api/v1/devices/health/summary'),
  ('TENANT_ADMIN', 'api/v1/devices/health/evaluate'),
  ('TENANT_ADMIN', 'api/v1/devices/:device_id/health'),
  ('TENANT_ADMIN', 'api/v1/devices/:device_id/health/evaluate'),
  ('TENANT_ADMIN', 'api/v1/device/health/summary'),
  ('TENANT_ADMIN', 'api/v1/device/health/evaluate'),
  ('TENANT_ADMIN', 'api/v1/device/:id/health'),
  ('TENANT_ADMIN', 'api/v1/device/:id/health/evaluate')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
