-- 64.sql: 白标定制（ROADMAP C5）——logo 品牌表补充 tenant_id 租户隔离列
ALTER TABLE logo
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(36) NOT NULL DEFAULT '';
COMMENT ON COLUMN logo.tenant_id IS '租户ID（C5 白标：每租户一套品牌，空串为全局默认）';
