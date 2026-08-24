-- 51.sql: 凭证哈希存储 Phase 2b 配套——唯一性从明文列迁移到哈希列。
-- 背景：Phase 2b 起 devices.voucher 对新行一律写空串（停写明文），而 1.sql 遗留的
-- devices_unique_1 UNIQUE(voucher) 约束会使第二台及之后的空串行必然违反唯一约束
-- （CI compose lane run 32724039702 / 32729258881 的 POST /device 101001 实证）。
-- 处理：
--   1) 删除明文列上的唯一约束。凭证唯一性语义由 voucher_hash 唯一索引承接；
--      明文列在 dual-mode 观测期结束、后续迁移置空并 DROP 后不再承载任何匹配语义。
--   2) 将 Phase 1 建的普通索引 idx_devices_voucher_hash 升级为 UNIQUE 索引。
--      PG 对 NULL 不做唯一冲突，存量未回填行不受影响；不同凭证哈希天然互异，
--      与原 UNIQUE(voucher) 的业务意图（防两台设备持有同一凭证）等价。
-- 回滚：重建 devices_unique_1 前必须先清理空串行并恢复明文唯一性，否则必然失败。

ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_unique_1;

DROP INDEX IF EXISTS idx_devices_voucher_hash;
CREATE UNIQUE INDEX idx_devices_voucher_hash ON devices(voucher_hash);
