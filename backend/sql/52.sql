-- 52.sql: 设备影子消息表（ROADMAP Phase A3——离线命令缓存）
-- 背景：设备离线时下发命令会静默丢失（fire-and-forget 入队后无法送达）。
--       影子消息提供"离线缓存、上线自动投递、TTL 过期"的消息队列语义，
--       与 DeviceTwin 的 desired/reported 状态视图互补（状态 vs 消息）。
-- 处理：
--   1) 建 device_shadow_messages 表：status 取值 pending/delivered/expired/canceled；
--      payload 为 JSONB；expires_at 由 ttl_seconds 计算写入。
--   2) 部分索引只覆盖 pending 行：上线投递与到期扫描都走该窄索引。
-- 回滚：DROP TABLE device_shadow_messages;（功能为增量增强，无存量数据依赖）

CREATE TABLE IF NOT EXISTS device_shadow_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id VARCHAR(36) NOT NULL,
    message_type VARCHAR(20) NOT NULL DEFAULT 'command',
    payload JSONB NOT NULL DEFAULT '{}',
    ttl_seconds INT NOT NULL DEFAULT 86400,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_by VARCHAR(36),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL
);

COMMENT ON TABLE device_shadow_messages IS '设备影子消息（离线命令缓存）：设备上线时投递 pending 消息，TTL 过期自动标记';
COMMENT ON COLUMN device_shadow_messages.status IS 'pending=待投递 delivered=已投递 expired=已过期 canceled=已取消';

CREATE INDEX IF NOT EXISTS idx_dsm_device_pending
    ON device_shadow_messages (device_id, created_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_dsm_expiry_sweep
    ON device_shadow_messages (expires_at)
    WHERE status = 'pending';
