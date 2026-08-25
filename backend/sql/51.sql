-- 计算字段表：从遥测数据实时派生新指标
CREATE TABLE IF NOT EXISTS calculated_fields (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_config_id VARCHAR(36) NOT NULL REFERENCES device_configs(id),
    output_key VARCHAR(64) NOT NULL,
    expression TEXT NOT NULL,
    unit VARCHAR(32) DEFAULT '',
    description TEXT DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_cf_config ON calculated_fields(device_config_id) WHERE enabled;
-- 唯一约束：同一配置下输出字段名不重复
CREATE UNIQUE INDEX idx_cf_output_key ON calculated_fields(device_config_id, output_key) WHERE enabled;