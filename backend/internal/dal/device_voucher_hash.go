// 文件用途：devices.voucher_hash 列的统一写入与存量回填（凭证哈希存储 Phase 1，
// 见 references/backend-hardening-plan.md 车道1）。
// 核心逻辑：gen 模型（model.Device/query.Device）无 VoucherHash 字段且生成文件不手改，
// 写入侧采用二段式——插入/更新原样走 gen，同事务内经 raw gorm 补 UPDATE voucher_hash；
// 存量行由 BackfillDeviceVoucherHash 分批幂等回填。
// 关键注意事项：存储哈希=缓存键算法（utils.VoucherStorageHash，跨服务契约，与
// mqtt-broker/plugin/aetherlink/db.go 双模式匹配同源）；phase2 停写明文后 voucher_hash
// 将成为唯一匹配依据，任何写入 voucher 的路径都必须同时落本列。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

// deviceVoucherHashBackfillBatchSize 单批扫描/写回的设备行数。
const deviceVoucherHashBackfillBatchSize = 500

type deviceVoucherHashBackfillRow struct {
	ID      string `gorm:"column:id"`
	Voucher string `gorm:"column:voucher"`
}

// writeVoucherHashWithTx 在调用方既有事务内为每台设备补写 voucher_hash 列。
// 供创建路径（createDevicesWithDefaultRootGroup）与凭证更新路径共用；
// 空 voucher 行跳过（与回填口径一致：voucher 非空才参与哈希匹配）。
func writeVoucherHashWithTx(tx *gorm.DB, devices []*model.Device) error {
	for _, device := range devices {
		if device == nil || device.ID == "" || device.Voucher == "" {
			continue
		}
		if err := tx.Exec("UPDATE devices SET voucher_hash = ? WHERE id = ?",
			utils.VoucherStorageHash(device.Voucher), device.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

// WriteVoucherHashInQueryTx 供 service 层在 query.Q.Transaction 的 gen 事务内
// 二段式补写 voucher_hash（gen 模型无该字段，raw UPDATE 是唯一写入通道）。
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
