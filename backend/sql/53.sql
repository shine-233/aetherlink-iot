-- Phase B2: 规则链编辑器后端持久化
-- 规则链主表
CREATE TABLE IF NOT EXISTS rule_chains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(128) NOT NULL,
    description TEXT DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    tenant_id VARCHAR(36) NOT NULL,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rc_tenant ON rule_chains(tenant_id);

-- 规则链节点表
CREATE TABLE IF NOT EXISTS rule_chain_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id UUID NOT NULL REFERENCES rule_chains(id) ON DELETE CASCADE,
    node_key VARCHAR(64) NOT NULL,
    node_type VARCHAR(20) NOT NULL,
    subtype VARCHAR(64) NOT NULL DEFAULT '',
    label VARCHAR(128) DEFAULT '',
    config JSONB NOT NULL DEFAULT '{}',
    position_x DOUBLE PRECISION NOT NULL DEFAULT 0,
    position_y DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rcn_chain ON rule_chain_nodes(chain_id);

-- 规则链连接表（DAG 边）
CREATE TABLE IF NOT EXISTS rule_chain_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id UUID NOT NULL REFERENCES rule_chains(id) ON DELETE CASCADE,
    source_node_key VARCHAR(64) NOT NULL,
    target_node_key VARCHAR(64) NOT NULL,
    label VARCHAR(64) DEFAULT ''
);
CREATE INDEX idx_rce_chain ON rule_chain_edges(chain_id);
