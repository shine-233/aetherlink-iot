-- 58.sql: 实体版本控制（ROADMAP C7，对标 ThingsBoard 3.5+ Version Control）
-- 背景：设备/看板/规则链等核心实体缺少 Git 式备份恢复能力，误改或误删后无法回滚。
-- 设计：快照以 JSONB 存放整行数据；entity_type 经白名单映射为固定表名，
--       用户输入永不参与 SQL 拼接，避免注入或越表访问。
-- 边界：本表只存快照与元数据，不承担实体主数据的读写一致性；
--       恢复动作由 service 层在同一租户作用域内按快照 map 回写。
-- 回滚：DROP TABLE entity_versions;

CREATE TABLE IF NOT EXISTS entity_versions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      VARCHAR(36) NOT NULL,
    entity_type    VARCHAR(32) NOT NULL,
    entity_id      VARCHAR(36) NOT NULL,
    version_number INT         NOT NULL,
    snapshot       JSONB       NOT NULL,
    remark         VARCHAR(500),
    created_by     UUID,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 同一实体内版本号递增唯一，保证顺序可预期且幂等写入不会重号。
CREATE UNIQUE INDEX IF NOT EXISTS idx_entity_versions_entity_version
    ON entity_versions (tenant_id, entity_type, entity_id, version_number);

-- 列表页按实体查历史版本：最新在前。
CREATE INDEX IF NOT EXISTS idx_entity_versions_entity_created
    ON entity_versions (tenant_id, entity_type, entity_id, created_at DESC);
