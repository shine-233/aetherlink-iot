-- 80.sql: Phase D5 接入安全 X.509——设备证书与平台 CA 表 + casbin 受保护路由登记（2026-09-07）。
-- 功能：设备证书生命周期（签发/轮换/吊销/校验）的存储结构：
--   1) platform_cas：平台设备 CA 单行（id 固定 "default"，首次签发自举生成，私钥仅本地栈存库，生产应迁移 KMS/HSM）；
--   2) device_certificates：设备证书记录（仅存证书 PEM，不存设备私钥；状态机 active/revoked/expired）。
-- casbin：登记 6 条受保护路由（列表/详情/签发/校验/吊销/轮换）。
--   授权口径：证书属租户级安全资产，仅 SYS_ADMIN + TENANT_ADMIN 可管理。
-- 幂等性：CREATE TABLE/INDEX IF NOT EXISTS；casbin 行 INSERT 前置 NOT EXISTS 守卫。

CREATE TABLE IF NOT EXISTS platform_cas (
    id          VARCHAR(36) PRIMARY KEY,
    certificate TEXT NOT NULL,
    private_key TEXT NOT NULL,
    not_before  TIMESTAMP NOT NULL,
    not_after   TIMESTAMP NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT now(),
    updated_at  TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS device_certificates (
    id            VARCHAR(36) PRIMARY KEY,
    tenant_id     VARCHAR(36) NOT NULL,
    device_id     VARCHAR(36) NOT NULL,
    serial_number VARCHAR(64) NOT NULL,
    fingerprint   VARCHAR(64) NOT NULL,
    common_name   VARCHAR(255) NOT NULL,
    certificate   TEXT NOT NULL,
    not_before    TIMESTAMP NOT NULL,
    not_after     TIMESTAMP NOT NULL,
    status        VARCHAR(16) NOT NULL DEFAULT 'active',
    issued_at     TIMESTAMP NOT NULL DEFAULT now(),
    revoked_at    TIMESTAMP,
    revoke_reason VARCHAR(255),
    created_at    TIMESTAMP NOT NULL DEFAULT now(),
    updated_at    TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_device_certs_tenant ON device_certificates (tenant_id);
CREATE INDEX IF NOT EXISTS idx_device_certs_device ON device_certificates (device_id);
CREATE INDEX IF NOT EXISTS idx_device_certs_serial ON device_certificates (serial_number);
CREATE INDEX IF NOT EXISTS idx_device_certs_status ON device_certificates (status);

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/device-certificates'),
  ('api/v1/device-certificates/issue'),
  ('api/v1/device-certificates/verify'),
  ('api/v1/device-certificates/:id'),
  ('api/v1/device-certificates/:id/revoke'),
  ('api/v1/device-certificates/:id/renew')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/device-certificates'),
  ('TENANT_ADMIN', 'api/v1/device-certificates'),
  ('SYS_ADMIN',    'api/v1/device-certificates/issue'),
  ('TENANT_ADMIN', 'api/v1/device-certificates/issue'),
  ('SYS_ADMIN',    'api/v1/device-certificates/verify'),
  ('TENANT_ADMIN', 'api/v1/device-certificates/verify'),
  ('SYS_ADMIN',    'api/v1/device-certificates/:id'),
  ('TENANT_ADMIN', 'api/v1/device-certificates/:id'),
  ('SYS_ADMIN',    'api/v1/device-certificates/:id/revoke'),
  ('TENANT_ADMIN', 'api/v1/device-certificates/:id/revoke'),
  ('SYS_ADMIN',    'api/v1/device-certificates/:id/renew'),
  ('TENANT_ADMIN', 'api/v1/device-certificates/:id/renew')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
