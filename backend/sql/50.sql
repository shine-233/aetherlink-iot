-- 设备凭证哈希存储 Phase 1（车道1，设计见 references/backend-hardening-plan.md）。
-- 边界说明：
--   1. 本迁移只新增 devices.voucher_hash 列 + 索引，不做任何数据回填；
--      存量行的摘要由 Go 侧启动逻辑补齐（backend/internal/dal/device_voucher_hash.go，
--      在 CheckVersion 迁移完成后调用）。回填不走 pgcrypto/纯 SQL，因为 broker 侧存在
--      多候选键序兼容匹配（mqtt-broker/plugin/aetherlink/db.go 的
--      deviceVoucherLookupCandidates），纯 SQL 无法语义等价复刻该展开。
--   2. voucher 明文列本阶段原样保留，作为双模式匹配窗口（哈希优先、明文兜底）的
--      兜底列；Phase 2 停写明文并观测归零后，由后续迁移置空并 DROP 明文列。
ALTER TABLE public.devices
    ADD COLUMN IF NOT EXISTS voucher_hash varchar(64);

CREATE INDEX IF NOT EXISTS idx_devices_voucher_hash ON public.devices (voucher_hash);

COMMENT ON COLUMN public.devices.voucher_hash IS '设备凭证 SHA-256 十六进制摘要（64 字符），存储哈希=缓存键算法（跨服务契约）；Phase 1 双模式窗口内 voucher 明文列仍为匹配兜底';
