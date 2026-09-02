-- 60.sql: 资产/租户层级（ROADMAP C2 第一步）
-- 目标：1) tenants 增加 parent_tenant_id（租户自引用，父=平台/根租户）；
--       2) 资产表 assets：租户内资产树（parent_id 自引用），供"资产管理层级"。
-- 边界：
--   - 列/表均 IF NOT EXISTS 幂等，PG 与迁移事务兼容（沿用 57.sql 的 DO 风格）；
--   - 不设物理外键（租户/资产删除策略在应用层裁决，避免迁移与既有删除流程耦合）；
--   - 成环/悬空父由应用层 hierarchy 包在写入时拒绝（见 internal/hierarchy）。
-- 回滚：DROP TABLE IF EXISTS assets; ALTER TABLE tenants DROP COLUMN IF EXISTS parent_tenant_id;

DO $$
BEGIN
    -- tenants.parent_tenant_id：自引用根为空（空串=根租户）。
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema='public' AND table_name='tenants' AND column_name='parent_tenant_id'
    ) THEN
        ALTER TABLE tenants ADD COLUMN parent_tenant_id VARCHAR(36) NOT NULL DEFAULT '';
        COMMENT ON COLUMN tenants.parent_tenant_id IS '父租户ID（空=根租户），层级数据隔离见 internal/hierarchy';
    END IF;

    -- assets：租户内资产树。
    CREATE TABLE IF NOT EXISTS assets (
        id           VARCHAR(36) PRIMARY KEY,
        tenant_id    VARCHAR(36) NOT NULL,
        parent_id    VARCHAR(36) NOT NULL DEFAULT '',
        name         VARCHAR(120) NOT NULL,
        asset_type   VARCHAR(64)  NOT NULL DEFAULT 'device',
        meta         JSONB,
        created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
        updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
    );
    CREATE INDEX IF NOT EXISTS idx_assets_tenant_parent ON assets(tenant_id, parent_id);
    COMMENT ON TABLE assets IS '租户资产树节点（设备/区域/产线等），parent_id 自引用';
END $$;
