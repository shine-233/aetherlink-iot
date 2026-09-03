-- 60.sql: 资产/租户层级（ROADMAP C2）
-- 目标：1) tenants 目录表（平台租户此前只是 users.tenant_id 字符串标记，此处自建目录）；
--       2) tenants.parent_tenant_id 自引用；3) 资产表 assets（租户内 parent_id 自引用树）。
-- 运行期发现（2026-09-03 隔离栈 fresh-start）：DO $$ 块与迁移执行器（gorm+pgx simple
-- protocol）整串多语句执行存在建表可见性差异（57.sql 因整文件即单个 DO 不受影响），
-- 故全部改写为顶层幂等语句（与 sql/1.sql 数百条多语句同一执行路径，已在隔离栈验证）：
--   幂等手段 = CREATE TABLE IF NOT EXISTS / ADD COLUMN IF NOT EXISTS / ON CONFLICT DO NOTHING。
-- 回滚：DROP TABLE IF EXISTS assets; DROP TABLE IF EXISTS tenants;

-- 1) tenants 目录表。
CREATE TABLE IF NOT EXISTS tenants (
    id               VARCHAR(36) PRIMARY KEY,
    name             VARCHAR(120) NOT NULL DEFAULT '',
    parent_tenant_id VARCHAR(36)  NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2) 老库已存在 tenants 但缺 parent_tenant_id（c2-tenant-tree 早期形态）时补列（PG>=9.6）。
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS parent_tenant_id VARCHAR(36) NOT NULL DEFAULT '';

-- 3) 幂等回填：已存在租户字符串入目录（父留空=根），供资产/层级读路径 JOIN。
INSERT INTO tenants (id, name, created_at, updated_at)
SELECT DISTINCT d.tenant_id,
       COALESCE(NULLIF(u.name, ''), '租户 ' || d.tenant_id),
       now(), now()
FROM (SELECT DISTINCT tenant_id FROM users WHERE tenant_id IS NOT NULL AND tenant_id <> '') d
LEFT JOIN users u ON u.id = (SELECT min(id) FROM users WHERE tenant_id = d.tenant_id)
ON CONFLICT (id) DO NOTHING;

-- 4) assets：租户内资产树（无物理外键，删除策略在应用层裁决）。
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
