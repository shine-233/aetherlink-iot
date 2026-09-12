// 文件用途：devices.voucher_hash 列的统一写入、明文停写收口与存量回填
// （凭证哈希存储 Phase 1/2b，见 references/backend-hardening-plan.md 车道1）。
// 核心逻辑：gen 模型（model.Device/query.Device）无 VoucherHash 字段且生成文件不手改，
// 写入侧经 raw gorm 在调用方事务内补 UPDATE voucher_hash；Phase 2b 起同一收口点把
// voucher 明文列置空串（停写明文），并把创建/轮换时的明文暂存 24h 网页测试缓存
// （device_credential_cache.go）；存量行由 BackfillDeviceVoucherHash 分批幂等回填。
// 关键注意事项：存储哈希=缓存键算法（utils.VoucherStorageHash，跨服务契约，与
// mqtt-broker/plugin/aetherlink/db.go 双模式匹配同源）；停写明文后 voucher_hash
// 是唯一匹配依据，任何写入 voucher 的路径都必须同时落本列。回填路径只补 hash、
// 不动明文列也不写测试缓存（Phase 2 dual-mode 窗口内存量行仍靠明文兜底匹配）。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

// deviceVoucherHashBackfillBatchSize 单批扫描/写回的设备行数。
const deviceVoucherHashBackfillBatchSize = 500

// deviceVoucherPurgeBatchSize 单批清理存量明文的行数。
const deviceVoucherPurgeBatchSize = 500

type deviceVoucherHashBackfillRow struct {
	ID      string `gorm:"column:id"`
	Voucher string `gorm:"column:voucher"`
}

// writeVoucherHashWithTx 在调用方既有事务内为每台设备补写 voucher_hash 列，
// 并在同一收口点完成 Phase 2b 的两件事：
//  1. 停写明文——voucher 列统一置空串（列 NOT NULL DEFAULT ”），即使调用方传入的
//     struct/语句仍携带明文，提交后的行状态也不含明文；
//  2. 追加网页测试缓存写入（逐设备 StoreDeviceCredentialTestCache）。
//
// 供创建路径（createDevicesWithDefaultRootGroup）与凭证更新路径共用；
// 空 voucher 行跳过（与回填口径一致：voucher 非空才参与哈希匹配）。
// 测试缓存是 UX 增强，不是一致性依赖：写失败仅 Warn 不阻断主流程。
func writeVoucherHashWithTx(tx *gorm.DB, devices []*model.Device) error {
	for _, device := range devices {
		if device == nil || device.ID == "" || device.Voucher == "" {
			continue
		}
		if err := tx.Exec("UPDATE devices SET voucher_hash = ?, voucher = '' WHERE id = ?",
			utils.VoucherStorageHash(device.Voucher), device.ID).Error; err != nil {
			return err
		}
		StoreDeviceCredentialTestCache(device.ID, device.Voucher)
	}
	return nil
}

// WriteVoucherHashInQueryTx 供 service 层在 query.Q.Transaction 的 gen 事务内补写
// voucher_hash 并停写明文（gen 模型无 VoucherHash 字段，raw UPDATE 是唯一写入通道）。
func WriteVoucherHashInQueryTx(tx *query.Query, devices []*model.Device) error {
	return writeVoucherHashWithTx(tx.Device.UnderlyingDB(), devices)
}

// BackfillDeviceVoucherHash 为存量设备行一次性补齐 voucher_hash。
// 以 voucher_hash IS NULL 为进度游标：幂等可重入，重复执行只补缺口、已回填行零写入；
// 分批（500 行/批）避免长事务与大结果集。50.sql 迁移后由 initialize.PgInit 调用，
// 失败由调用方告警且不阻断启动（双模式下明文列仍是有效兜底，下次启动自动续跑）。
func BackfillDeviceVoucherHash(db *gorm.DB) error {
	if db == nil {
		return gorm.ErrInvalidDB
	}
	for {
		var rows []deviceVoucherHashBackfillRow
		if err := db.Raw(
			"SELECT id, voucher FROM devices WHERE voucher <> '' AND voucher_hash IS NULL LIMIT ?",
			deviceVoucherHashBackfillBatchSize,
		).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			// 带 IS NULL 条件重申进度游标，防御并发实例重复回填时的覆盖竞争。
			if err := db.Exec(
				"UPDATE devices SET voucher_hash = ? WHERE id = ? AND voucher_hash IS NULL",
				utils.VoucherStorageHash(row.Voucher), row.ID,
			).Error; err != nil {
				return err
			}
		}
		if len(rows) < deviceVoucherHashBackfillBatchSize {
			return nil
		}
	}
}

// PurgeDeviceVoucherPlaintext 清理存量行的明文凭证（P0.5"凭证仅出现一次"的收尾）。
// 回填（BackfillDeviceVoucherHash）只补 hash、不动明文，双模式窗口内存量明文一直留在
// devices.voucher 列里；窗口关闭后需要这一步把它清掉，否则"停写明文"只对新增/轮换生效。
//
// 硬安全约束：**只清理已经有 voucher_hash 的行**。voucher_hash 是停写明文后唯一的
// 匹配依据，缺 hash 的行一旦清掉明文，凭证就不可恢复地丢失（设备再也连不上、运维也拿不回），
// 所以这类行一律跳过，交给回填补完 hash 后的下一轮处理——宁可留着明文，也不能删掉唯一副本。
//
// 写入用 (id, voucher_hash IS NOT NULL, voucher=原值) 三条件做 CAS：并发的凭证轮换或回填
// 会让条件不再匹配，此时跳过而不是误清。函数幂等，已清理行（voucher=''）不再入选。
// maxRows<=0 表示不限；返回实际被清理的行数。
func PurgeDeviceVoucherPlaintext(db *gorm.DB, maxRows int) (int64, error) {
	if db == nil {
		return 0, gorm.ErrInvalidDB
	}
	if maxRows <= 0 {
		maxRows = int(^uint(0) >> 1) // 不限：交给批次循环与"无进展即停"收口
	}

	var purged int64
	for processed := 0; processed < maxRows; {
		batch := maxRows - processed
		if batch > deviceVoucherPurgeBatchSize {
			batch = deviceVoucherPurgeBatchSize
		}
		var rows []deviceVoucherHashBackfillRow
		if err := db.Raw(
			"SELECT id, voucher FROM devices WHERE voucher <> '' AND voucher_hash IS NOT NULL LIMIT ?",
			batch,
		).Scan(&rows).Error; err != nil {
			return purged, err
		}
		if len(rows) == 0 {
			return purged, nil
		}

		batchPurged := int64(0)
		for _, row := range rows {
			processed++
			result := db.Exec(
				"UPDATE devices SET voucher = '' WHERE id = ? AND voucher_hash IS NOT NULL AND voucher = ?",
				row.ID, row.Voucher,
			)
			if result.Error != nil {
				return purged, result.Error
			}
			purged += result.RowsAffected
			batchPurged += result.RowsAffected
		}
		// 本批一条都没清掉：剩余行正在被并发改写，交下一轮，避免原地空转。
		if batchPurged == 0 {
			return purged, nil
		}
		if len(rows) < batch {
			return purged, nil
		}
	}
	return purged, nil
}
