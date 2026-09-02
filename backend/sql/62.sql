-- 62.sql: OAuth2/OIDC 单点登录（ROADMAP C7 剩余）——租户级 IdP 配置存储。
-- 设计：一租户可配多个外部 IdP；start 走 /sso/{id}/start，回调 /sso/{id}/callback；
--       client_secret 落库为密文（由服务端派生密钥加密）或明文旧配置兼容（升级由应用层负责）。
-- 回滚：DROP TABLE IF EXISTS tenant_oidc_providers;

CREATE TABLE IF NOT EXISTS tenant_oidc_providers (
    id                 VARCHAR(36) PRIMARY KEY,
    tenant_id          VARCHAR(36) NOT NULL DEFAULT '',
    name               VARCHAR(120) NOT NULL,
    issuer             VARCHAR(255) NOT NULL,
    client_id          VARCHAR(255) NOT NULL,
    client_secret      TEXT        NOT NULL DEFAULT '',
    discovery_url      VARCHAR(255) NOT NULL DEFAULT '',
    scopes             VARCHAR(255) NOT NULL DEFAULT 'openid profile email',
    frontend_redirect  VARCHAR(255) NOT NULL DEFAULT '',
    enabled            BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_oidc_providers_tenant ON tenant_oidc_providers(tenant_id);
