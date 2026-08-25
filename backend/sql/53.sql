-- 53.sql: 设备 Modbus 点表（ROADMAP B1 前端配置界面配套存储）
-- 背景：modbus-plugin 的寄存器点表原先只能靠本地 config.json 下发；
--       平台侧持久化后，前端可在线编辑点表，插件经 OpenAPI Key 拉取。
-- 边界：profile 只存 target + registers 等映射信息，绝不存设备凭证
--       （username/password 仍只在插件本地配置/密钥管理中）。
-- 回滚：DROP TABLE device_modbus_profiles;（插件可回退本地文件模式）

CREATE TABLE IF NOT EXISTS device_modbus_profiles (
    device_id VARCHAR(36) PRIMARY KEY,
    profile JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by VARCHAR(36)
);

COMMENT ON TABLE device_modbus_profiles IS '设备 Modbus 点表（target+registers 映射；不含凭证），供前端编辑与 modbus-plugin 拉取';
