-- 104.sql — P1.5 边缘运维：边缘节点证书签发（复用 D5 X.509 接线）与远程升级/回滚。
--
-- 背景与设计：
--   1) 边缘节点证书（edge_node_certificates）：
--      边缘节点（EdgeNode）在云边协同与 mTLS 接入场景中需要客户端证书身份。
--      复用平台 CA 签发 ECDSA P-256 证书，CommonName 取 node_id，Organization 取 tenant_id。
--      私钥仅在签发时返回一次，平台只存证书与元数据。支持轮换与吊销。
--   2) 边缘节点升级与回滚（edge_node_upgrade_history）：
--      边缘节点运行期具备版本号，升级必须满足严格大于当前版本，降级必须通过回滚通道。
--      历史流水 append-only，回滚产生新的回滚历史记录，状态标记为 rolled_back，双向审计。
--   3) Casbin 登记：
--      新增证书与升级/回滚相关路由，遵循 fail-fast 路由覆盖契约，授予 SYS_ADMIN 与 TENANT_ADMIN。

-- ---- 1. 边缘节点证书表 ----
CREATE TABLE IF NOT EXISTS public.edge_node_certificates (
    id             varchar(36)  PRIMARY KEY,
    tenant_id      varchar(36)  NOT NULL,
    node_id        varchar(64)  NOT NULL,
    serial_number  varchar(64)  NOT NULL,
    fingerprint    varchar(64)  NOT NULL,
    common_name    varchar(128) NOT NULL,
    certificate    text         NOT NULL,
    not_before     timestamptz  NOT NULL,
    not_after      timestamptz  NOT NULL,
    status         varchar(16)  NOT NULL DEFAULT 'active',
    issued_at      timestamptz  NOT NULL,
    revoked_at     timestamptz  NULL,
    revoke_reason  varchar(255) NULL,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    updated_at     timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT edge_node_certificates_status_check
        CHECK (status IN ('active', 'revoked', 'expired')),
    CONSTRAINT edge_node_certificates_tenant_check
        CHECK (tenant_id <> ''),
    CONSTRAINT edge_node_certificates_node_check
        CHECK (node_id <> '')
);

CREATE INDEX IF NOT EXISTS edge_node_certificates_tenant_node_idx
    ON public.edge_node_certificates (tenant_id, node_id, status);

-- ---- 2. 边缘节点升级历史表 ----
CREATE TABLE IF NOT EXISTS public.edge_node_upgrade_history (
    id             varchar(36)  PRIMARY KEY,
    tenant_id      varchar(36)  NOT NULL,
    node_id        varchar(64)  NOT NULL,
    from_version   varchar(32)  NOT NULL,
    target_version varchar(32)  NOT NULL,
    package_url    varchar(512) NULL,
    checksum       varchar(128) NULL,
    status         varchar(16)  NOT NULL DEFAULT 'pending',
    operator_id    varchar(36)  NOT NULL,
    description    text         NULL,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    updated_at     timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT edge_node_upgrade_status_check
        CHECK (status IN ('pending', 'dispatched', 'success', 'failed', 'rolled_back')),
    CONSTRAINT edge_node_upgrade_tenant_check
        CHECK (tenant_id <> ''),
    CONSTRAINT edge_node_upgrade_node_check
        CHECK (node_id <> '')
);

CREATE INDEX IF NOT EXISTS edge_node_upgrade_tenant_node_idx
    ON public.edge_node_upgrade_history (tenant_id, node_id, created_at DESC);

-- ---- 3. Casbin 资源与权限登记 ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/edge/nodes/:node_id/certificate'),
  ('api/v1/edge/nodes/:node_id/upgrade'),
  ('api/v1/edge/nodes/:node_id/rollback'),
  ('api/v1/edge/nodes/:node_id/upgrade/history')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/edge/nodes/:node_id/certificate'),
  ('TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/certificate'),
  ('SYS_ADMIN',    'api/v1/edge/nodes/:node_id/upgrade'),
  ('TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/upgrade'),
  ('SYS_ADMIN',    'api/v1/edge/nodes/:node_id/rollback'),
  ('TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/rollback'),
  ('SYS_ADMIN',    'api/v1/edge/nodes/:node_id/upgrade/history'),
  ('TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/upgrade/history')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
