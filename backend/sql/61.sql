-- 61.sql: 双因素认证 2FA（ROADMAP C7）——用户 TOTP 状态与一次性恢复码。
-- 设计：独立 user_totp / user_totp_recovery_codes 表，避免改动 users 主表；
--       secret 以 AES-GCM 密文存储（密钥由服务端 jwt.key 派生），启用后才落库。
-- 回滚：DROP TABLE IF EXISTS user_totp_recovery_codes; DROP TABLE IF EXISTS user_totp;

CREATE TABLE IF NOT EXISTS user_totp (
    user_id        VARCHAR(36) PRIMARY KEY,
    secret_cipher  TEXT        NOT NULL,
    enabled        BOOLEAN     NOT NULL DEFAULT FALSE,
    last_used_step BIGINT      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_totp_recovery_codes (
    id         VARCHAR(36) PRIMARY KEY,
    user_id    VARCHAR(36)  NOT NULL,
    code_hash  VARCHAR(64)  NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_totp_recovery_user ON user_totp_recovery_codes(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_totp_recovery_hash ON user_totp_recovery_codes(user_id, code_hash);
