-- 56.sql: 规则链可视化编辑器（ROADMAP B2）所需的 graph 列增量。
-- 背景：53.sql 已按三表方案（rule_chains / rule_chain_nodes / rule_chain_edges）建表；
--       B2 编辑器采用「整图 JSONB」读写模型（rule_chains.graph 存 {nodes:[],edges:[]}），
--       本迁移为既有表补齐 graph 列，使 graph 模型 DAL 与三表底座共存。
-- 备注：原规划在 55.sql，因 55.sql 已预留给 Modbus 点表（B1），重排为 56.sql。
-- 边界：幂等（ADD COLUMN IF NOT EXISTS）；不迁移存量节点/边数据（53 方案从未接入写入路径）。
-- 回滚：ALTER TABLE rule_chains DROP COLUMN graph;（编辑器功能随之不可用，三表数据不受影响）

ALTER TABLE rule_chains
    ADD COLUMN IF NOT EXISTS graph JSONB NOT NULL DEFAULT '{"nodes":[],"edges":[]}';

CREATE INDEX IF NOT EXISTS idx_rule_chains_tenant_enabled
    ON rule_chains (tenant_id, enabled);