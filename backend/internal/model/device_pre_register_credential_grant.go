// 文件用途：预注册凭证"一次性下载许可"的模型与校验（ROADMAP P0.5）。
// 核心逻辑：一批次一个 pending 许可 → 下载时条件更新为 consumed → 此后不可再下载。
// 关键注意事项：
//  1. "只出现一次"必须由**数据库条件更新**守，不能靠应用层先查再改。
//     两个请求并发下载同一许可时，应用层的 if 会让两边都看到 pending 然后都放行；
//     条件更新下只有一个 RowsAffected=1，另一个必然失败。
//  2. 一批次同时只能有一个 pending 许可（部分唯一索引）。否则"重复签发"
//     就能无限次拿到明文库，一次性形同虚设。
//  3. consumed_by / consumed_at 是审计面：谁在什么时候取走了这批凭证必须留痕，
//     事后无法追责的明文下发等于没有门禁。
//  4. 本表不消除 devices.voucher 里的明文（broker 的 MQTT 基础认证要读），
//     它限制的是**明文被下发的次数**，不是库里有没有明文。
package model

import (
	"errors"
	"strings"
	"time"
)

// TableNameDevicePreRegisterCredentialGrant 预注册凭证下载许可表名。
const TableNameDevicePreRegisterCredentialGrant = "device_pre_register_credential_grants"

// 许可状态。
const (
	CredentialGrantStatusPending  = "pending"
	CredentialGrantStatusConsumed = "consumed"
	CredentialGrantStatusExpired  = "expired"
	CredentialGrantStatusRevoked  = "revoked"
)

// 校验错误。
var (
	// ErrCredentialGrantBatchRequired 批次号为空。
	ErrCredentialGrantBatchRequired = errors.New("batch number is required")
	// ErrCredentialGrantBatchTooLong 批次号超长。
	ErrCredentialGrantBatchTooLong = errors.New("batch number exceeds 36 characters")
	// ErrCredentialGrantUnknownStatus 未知状态。
	ErrCredentialGrantUnknownStatus = errors.New("unknown credential grant status")
)

// CredentialGrantReq 签发一次性凭证下载许可的请求。
// 只收批次号：凭证明文的下发粒度是"一批"，不允许按单台设备挑着下——
// 挑着下会让"这批凭证被谁取走了"无法整体审计。
type CredentialGrantReq struct {
	BatchNumber string `json:"batch_number" form:"batch_number" validate:"required,max=36"`
}

// DevicePreRegisterCredentialGrant 预注册凭证的一次性下载许可。
type DevicePreRegisterCredentialGrant struct {
	ID          string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID    string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	BatchNumber string     `gorm:"column:batch_number;not null" json:"batch_number"`
	DeviceCount int        `gorm:"column:device_count;not null" json:"device_count"`
	Status      string     `gorm:"column:status;not null" json:"status"`
	ExpiresAt   time.Time  `gorm:"column:expires_at;not null" json:"expires_at"`
	CreatedBy   string     `gorm:"column:created_by" json:"created_by"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	ConsumedBy  *string    `gorm:"column:consumed_by" json:"consumed_by"`
	ConsumedAt  *time.Time `gorm:"column:consumed_at" json:"consumed_at"`
}

// TableName 指定表名。
func (DevicePreRegisterCredentialGrant) TableName() string {
	return TableNameDevicePreRegisterCredentialGrant
}

// IsCredentialGrantStatus 判断状态是否在词表内。
func IsCredentialGrantStatus(status string) bool {
	switch status {
	case CredentialGrantStatusPending,
		CredentialGrantStatusConsumed,
		CredentialGrantStatusExpired,
		CredentialGrantStatusRevoked:
		return true
	default:
		return false
	}
}

// ValidateCredentialGrantBatchNumber 校验批次号非空且不超长。
func ValidateCredentialGrantBatchNumber(batch string) error {
	trimmed := strings.TrimSpace(batch)
	if trimmed == "" {
		return ErrCredentialGrantBatchRequired
	}
	if len(trimmed) > 36 {
		return ErrCredentialGrantBatchTooLong
	}
	return nil
}

// ValidateCredentialGrant 校验许可行自身的字段自洽。
func ValidateCredentialGrant(g *DevicePreRegisterCredentialGrant) error {
	if g == nil {
		return ErrCredentialGrantBatchRequired
	}
	if err := ValidateCredentialGrantBatchNumber(g.BatchNumber); err != nil {
		return err
	}
	if strings.TrimSpace(g.TenantID) == "" {
		return errors.New("tenant id is required")
	}
	if !IsCredentialGrantStatus(g.Status) {
		return ErrCredentialGrantUnknownStatus
	}
	// 已消费必须有消费者与时间：只写 status 不留痕，审计就断了。
	if g.Status == CredentialGrantStatusConsumed {
		if g.ConsumedBy == nil || strings.TrimSpace(*g.ConsumedBy) == "" || g.ConsumedAt == nil {
			return errors.New("consumed grant must carry consumed_by and consumed_at")
		}
	}
	return nil
}
