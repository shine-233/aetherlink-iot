// File purpose: TB-12 设备认领 DAL——令牌行的写入、条件消费与设备租户转移。
// Core logic: 一次性与并发安全都落在数据库条件更新（WHERE status='active'），
// 租户转移落在行锁 + 条件更新（WHERE tenant_id = 签发租户），RowsAffected 判定成败。
// Key notes: 唯一允许的"全局寻址"查询是按 device_number 找设备（redeem 的入口），
// 已用 tenant-scope: 标记显式说明；其余查询全部显式携带租户过滤。
package dal

import (
	"errors"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrDeviceClaimNoRow 条件更新未命中（并发下被人抢先、或状态已不再是 active）。
var ErrDeviceClaimNoRow = errors.New("device claim conditional update matched no rows")

// WithDeviceClaimTransaction 在一个事务里跑认领的多步操作（消费令牌 + 租户转移），
// 任一步失败整体回滚——"令牌被消费但设备没转移"比全失败更糟。
func WithDeviceClaimTransaction(fn func(tx *gorm.DB) error) error {
	return global.DB.Transaction(fn)
}

// GetDeviceClaimSourceByIDAndTenant 签发前的设备归属校验：设备必须属于签发方租户。
// 不存在与不属于返回同一错误，由服务层统一按 not found 处理（不泄露存在性）。
func GetDeviceClaimSourceByIDAndTenant(tenantID, deviceID string) (*model.Device, error) {
	var device model.Device
	err := global.DB.Where("id = ? AND tenant_id = ?", deviceID, tenantID).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

// InsertDeviceClaimToken 写入一条新令牌（调用方需已完成同设备 active 行的 replaced 覆盖）。
func InsertDeviceClaimToken(tx *gorm.DB, token *model.DeviceClaimToken) error {
	return tx.Create(token).Error
}

// ReplaceActiveDeviceClaimTokens 把该设备当前 active 的令牌全部标记为 replaced
//（重新签发覆盖旧行），返回受影响行数。
func ReplaceActiveDeviceClaimTokens(tx *gorm.DB, deviceID string) (int64, error) {
	res := tx.Model(&model.DeviceClaimToken{}).
		Where("device_id = ? AND status = ?", deviceID, model.DeviceClaimStatusActive).
		Update("status", model.DeviceClaimStatusReplaced)
	return res.RowsAffected, res.Error
}

// ListDeviceClaimTokensByDevice 签发方回查某设备的令牌历史（管理视图，无明文无哈希）。
func ListDeviceClaimTokensByDevice(tenantID, deviceID string) ([]model.DeviceClaimToken, error) {
	var rows []model.DeviceClaimToken
	err := global.DB.
		Where("tenant_id = ? AND device_id = ?", tenantID, deviceID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

// GetDeviceClaimTokenForUpdate 撤销场景：行锁读一条属于签发租户的 active 令牌。
func GetDeviceClaimTokenForUpdate(tx *gorm.DB, tenantID, tokenID string) (*model.DeviceClaimToken, error) {
	var row model.DeviceClaimToken
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND id = ?", tenantID, tokenID).
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// RevokeDeviceClaimToken 条件撤销：只有 active 可撤销，终态（consumed/replaced）不可。
func RevokeDeviceClaimToken(tx *gorm.DB, tokenID string) error {
	res := tx.Model(&model.DeviceClaimToken{}).
		Where("id = ? AND status = ?", tokenID, model.DeviceClaimStatusActive).
		Update("status", model.DeviceClaimStatusRevoked)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDeviceClaimNoRow
	}
	return nil
}

// ConsumeDeviceClaimToken 条件消费：并发 redeem 下至多一条成功（一次性落点）。
// 过期判定在这里（expires_at > now），过期/已消费/已撤销/已替换都是 RowsAffected=0。
func ConsumeDeviceClaimToken(tx *gorm.DB, tokenID, byTenantID, byUserID string) error {
	res := tx.Model(&model.DeviceClaimToken{}).
		Where("id = ? AND status = ? AND expires_at > ?",
			tokenID, model.DeviceClaimStatusActive, time.Now()).
		Updates(map[string]interface{}{
			"status":                model.DeviceClaimStatusConsumed,
			"consumed_by_tenant_id": byTenantID,
			"consumed_by_user_id":   byUserID,
			"consumed_at":           time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDeviceClaimNoRow
	}
	return nil
}

// DeviceClaimTarget redeem 寻址结果。
type DeviceClaimTarget struct {
	ID       string
	TenantID string
}

// GetDeviceClaimTargetByNumber 按全局唯一 device_number 寻址待认领设备。
//
// tenant-scope: caller-enforced —— redeem 必须按 device_number 全局寻址（认领方
// 不知道设备内部 ID 与原租户）；租户边界由服务层事务把关：设备属于认领方自己
// 租户时直接拒绝，转移动作是行锁 + WHERE tenant_id = 签发租户 的条件更新。
func GetDeviceClaimTargetByNumber(deviceNumber string) (*DeviceClaimTarget, error) {
	var row struct {
		ID       string
		TenantID string
	}
	err := global.DB.Table("devices").
		Select("id, tenant_id").
		Where("device_number = ?", deviceNumber).
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &DeviceClaimTarget{ID: row.ID, TenantID: row.TenantID}, nil
}

// GetDeviceClaimTokenActiveForDeviceByNumber 行锁读取某设备的 active 令牌
//（redeem 场景：认领方凭 device_number 寻址后，服务层在事务内锁定消费）。
//
// tenant-scope: caller-enforced —— 与 GetDeviceClaimTargetByNumber 同一链路，
// 原租户边界由随后的租户转移条件更新把关，本查询只按设备定位令牌。
func GetDeviceClaimTokenActiveForDeviceByNumber(tx *gorm.DB, deviceID string) (*model.DeviceClaimToken, error) {
	var row model.DeviceClaimToken
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("device_id = ? AND status = ?", deviceID, model.DeviceClaimStatusActive).
		Order("created_at DESC").
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// TransferDeviceTenant 认领成功的租户转移：行锁 + 条件更新，
// 只有设备仍属于签发租户时才生效（防签发方并发把设备删走/转移走）。
// 转移同时清空 owner_user_id——原租户的"设备拥有者用户"对认领方没有意义。
func TransferDeviceTenant(tx *gorm.DB, deviceID, fromTenantID, toTenantID string) error {
	res := tx.Exec(
		"UPDATE devices SET tenant_id = ?, owner_user_id = NULL, update_at = ? "+
			"WHERE id = ? AND tenant_id = ?",
		toTenantID, time.Now(), deviceID, fromTenantID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDeviceClaimNoRow
	}
	return nil
}
