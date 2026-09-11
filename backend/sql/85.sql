-- P1.1 通用实体关系：设备/资产/客户/网关之间的有向、带类型与元数据的关系。
-- 设计要点：
--   1. 租户隔离由 tenant_id 承担，唯一约束包含 tenant_id，跨租户关系不会互相覆盖。
--   2. 方向由 (from_type, from_id) -> (to_type, to_id) 表达，relation_type 只描述语义，
--      不隐含对称性；反向关系必须显式写入，不允许由查询层"脑补"出来。
--   3. 自环（同一实体指向自己）由 CHECK 直接拒绝，避免生成无意义的关系环。

CREATE TABLE IF NOT EXISTS public.entity_relations (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     varchar(36) NOT NULL,
    from_type     varchar(32) NOT NULL,
    from_id       varchar(36) NOT NULL,
    relation_type varchar(64) NOT NULL,
    to_type       varchar(32) NOT NULL,
    to_id         varchar(36) NOT NULL,
    metadata      jsonb NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT entity_relations_no_self_loop
        CHECK (NOT (from_type = to_type AND from_id = to_id)),
    CONSTRAINT entity_relations_unique_edge
        UNIQUE (tenant_id, from_type, from_id, relation_type, to_type, to_id)
);

-- 按起点查关系（看板/详情页正查）。
CREATE INDEX IF NOT EXISTS idx_entity_relations_from
    ON public.entity_relations (tenant_id, from_type, from_id, relation_type);

-- 按终点反查（"谁指向我"）。
CREATE INDEX IF NOT EXISTS idx_entity_relations_to
    ON public.entity_relations (tenant_id, to_type, to_id, relation_type);
