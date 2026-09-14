// 文件用途：预注册凭证一次性下载许可的数据访问（ROADMAP P0.5）。
// 核心逻辑：签发许可、按租户读取、条件更新消费、过期收口。
// 关键注意事项：
//  1. 消费必须是**带当前状态的条件更新**（WHERE status='pending'），靠 RowsAffected
//     判定成败。先 SELECT 再 UPDATE 在高并发下会让两个请求都读到 pending 而双双放行，
//     "只下载一次"当场失效。
//  2. 所有查询强制带 tenant_id：跨租户表现为未命中，而不是"存在但无权限"——
//     后者等于告诉调用方这个批次在别的租户里存在。
//  3. 一个批次同时只能有一个 pending 许可，由数据库部分唯一索引保证（95.sql），
//     应用层不做"先查有没有再决定插不插"。
package dal

import (
	"errors"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// CreateCredentialGrant 签发一条下载许可。
// 同一租户同一批次若已存在 pending 许可，数据库唯一索引会拒绝，错误原样返回
// （调用方负责转成"该批次有待消费许可"）。
func CreateCredentialGrant(g *model.DevicePreRegisterCredentialGrant) error {
	return global.DB.Create(g).Error
}

// GetCredentialGrantInTenant 读取租户内的一条许可；未命中返回 gorm.ErrRecordNotFound。
func GetCredentialGrantInTenant(id, tenantID string) (*model.DevicePreRegisterCredentialGrant, error) {
	var row model.DevicePreRegisterCredentialGrant
	err := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// FindPendingCredentialGrant 查找某批次当前待消费的许可；没有返回 nil, nil。
func FindPendingCredentialGrant(tenantID, batchNumber string) (*model.DevicePreRegisterCredentialGrant, error) {
	var row model.DevicePreRegisterCredentialGrant
	err := global.DB.
		Where("tenant_id = ? AND batch_number = ? AND status = ?",
			tenantID, batchNumber, model.CredentialGrantStatusPending).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// ConsumeCredentialGrant 消费许可：条件更新 pending -> consumed 并记录消费者与时间。
// RowsAffected=0 表示已被别人消费或已失效，调用方必须据此拒绝下发明文。
func ConsumeCredentialGrant(id, tenantID, consumedBy string, consumedAt time.Time) (int64, error) {
	res := global.DB.Model(&model.DevicePreRegisterCredentialGrant{}).
		Where("id = ? AND tenant_id = ? AND status = ?",
			id, tenantID, model.CredentialGrantStatusPending).
		Updates(map[string]interface{}{
			"status":      model.CredentialGrantStatusConsumed,
			"consumed_by": consumedBy,
			"consumed_at": consumedAt,
		})
	return res.RowsAffected, res.Error
}

// ListPreRegisterDevicesByBatch 按租户与批次读取预注册设备（含凭证明文）。
// 只用于一次性下载通道——这是**唯一**一个把 devices.voucher 原样读出的路径，
// 调用方必须先确认许可消费成功。按 device_number 排序保证结果稳定可比对。
func ListPreRegisterDevicesByBatch(tenantID, batchNumber string) ([]model.Device, error) {
	var rows []model.Device
	err := global.DB.
		Where("tenant_id = ? AND batch_number = ? AND activate_flag = ?",
			tenantID, batchNumber, "inactive").
		Order("device_number ASC").
		Find(&rows).Error
	return rows, err
}

// ExpireCredentialGrantByID 把单条过期许可收口为 expired，返回受影响行数。
func ExpireCredentialGrantByID(id, tenantID string) (int64, error) {
	res := global.DB.Model(&model.DevicePreRegisterCredentialGrant{}).
		Where("id = ? AND tenant_id = ? AND status = ?",
			id, tenantID, model.CredentialGrantStatusPending).
		Updates(map[string]interface{}{"status": model.CredentialGrantStatusExpired})
	return res.RowsAffected, res.Error
}

// ExpireCredentialGrantsBefore 把过期仍未消费的许可收口为 expired，返回收口行数。
// 不做物理删除：许可行是"谁曾经可以取走这批凭证"的审计事实，删掉就无从追责。
func ExpireCredentialGrantsBefore(now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}
	res := global.DB.Model(&model.DevicePreRegisterCredentialGrant{}).
		Where("status = ? AND expires_at <= ?", model.CredentialGrantStatusPending, now).
		Limit(limit).
		Updates(map[string]interface{}{"status": model.CredentialGrantStatusExpired})
	return res.RowsAffected, res.Error
}
