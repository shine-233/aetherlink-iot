-- 54.sql: 计算字段 v2（对标 ThingsBoard Calculated Fields——从遥测派生新遥测）
-- 背景：51.sql 曾以 device_config_id 维度落过一版 calculated_fields 桩表，该实现
--       从未接线 API/UI/引擎（无任何写入路径），且与当前"租户+设备模板"归属模型不一致。
-- 处理：
--   1) DROP 桩表后按新维度重建同名表，兼容"已跑过桩表"和"未跑过"两类存量库；
--   2) output_key 为派生遥测键名；expression 为 govaluate 安全表达式；
--      enabled 默认关闭，启用后才参与上行计算；
--   3) 索引 (tenant_id) 支撑管理端分页列表；(device_template_id, enabled) 支撑
--      引擎按模板拉取启用字段的热路径查询。
-- 回滚：DROP TABLE calculated_fields;（功能为增量增强，无存量数据依赖）

DROP TABLE IF EXISTS calculated_fields;

CREATE TABLE calculated_fields (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    name VARCHAR(128) NOT NULL,
    device_template_id VARCHAR(36) NOT NULL,
    output_key VARCHAR(128) NOT NULL,
    expression TEXT NOT NULL,
    enabled BOOLEAN DEFAULT FALSE,
    remark VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_calculated_fields_tenant
    ON calculated_fields (tenant_id);

CREATE INDEX idx_calculated_fields_template_enabled
    ON calculated_fields (device_template_id, enabled);

-- 迁移完整性兜底：51.sql 曾在并行合并中被计算字段桩表覆盖，导致
-- DROP CONSTRAINT devices_unique_1 / UNIQUE(voucher_hash) 在全新库上丢失。
-- 以下语句幂等补齐（IF EXISTS/IF NOT EXISTS），老库重复执行无副作用。
ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_unique_1;
DROP INDEX IF EXISTS idx_devices_voucher_hash;
CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_voucher_hash ON devices(voucher_hash);
