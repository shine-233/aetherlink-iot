// File purpose: TB-12 设备认领（Device Claiming）模型与请求/响应契约。
// Core logic: 认领令牌行 + 签发/撤销/赎回三种请求形状。
// Key notes:
//   - 明文 claim_key 永不落库：库里只有 SHA-256 哈希（device_claim_tokens.claim_key_hash）。
//   - 有效状态 active 是「status='active' 且未过期」的组合语义，读取方负责按 expires_at 判定。
package model

import "time"

// DeviceClaimToken 认领令牌行（表 device_claim_tokens，见 112.sql）。
type DeviceClaimToken struct {
	ID                  string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID            string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	DeviceID            string     `gorm:"column:device_id;not null" json:"device_id"`
	DeviceNumber        string     `gorm:"column:device_number;not null" json:"device_number"`
	ClaimKeyHash        string     `gorm:"column:claim_key_hash;not null" json:"-"`
	Status              string     `gorm:"column:status;not null;default:active" json:"status"`
	ExpiresAt           time.Time  `gorm:"column:expires_at;not null" json:"expires_at"`
	PreviousTenantID    *string    `gorm:"column:previous_tenant_id" json:"previous_tenant_id,omitempty"`
	ConsumedByTenantID  *string    `gorm:"column:consumed_by_tenant_id" json:"consumed_by_tenant_id,omitempty"`
	ConsumedByUserID    *string    `gorm:"column:consumed_by_user_id" json:"consumed_by_user_id,omitempty"`
	ConsumedAt          *time.Time `gorm:"column:consumed_at" json:"consumed_at,omitempty"`
	CreatedAt           time.Time  `gorm:"column:created_at;not null;default:now()" json:"created_at"`
}

// TableName 指向 112.sql 建的表。
func (DeviceClaimToken) TableName() string { return "device_claim_tokens" }

// 认领令牌状态（112.sql CHECK 同源）。
const (
	DeviceClaimStatusActive   = "active"
	DeviceClaimStatusConsumed = "consumed"
	DeviceClaimStatusRevoked  = "revoked"
	DeviceClaimStatusReplaced = "replaced"
)

// IssueDeviceClaimTokenReq 签发认领令牌。
type IssueDeviceClaimTokenReq struct {
	DeviceID string `json:"device_id" validate:"required,max=36"`
	// TTLSeconds 令牌有效期（秒）。缺省 72h；测试用小值验证过期路径。
	TTLSeconds int `json:"ttl_seconds" validate:"omitempty,min=1,max=2592000"`
}

// RedeemDeviceClaimReq 认领设备。
type RedeemDeviceClaimReq struct {
	DeviceNumber string `json:"device_number" validate:"required,max=64"`
	ClaimKey     string `json:"claim_key" validate:"required,min=16,max=128"`
}

// IssueDeviceClaimTokenResp 签发响应：claim_key 是**唯一一次**明文出现。
type IssueDeviceClaimTokenResp struct {
	TokenID      string     `json:"token_id"`
	DeviceID     string     `json:"device_id"`
	DeviceNumber string     `json:"device_number"`
	ClaimKey     string     `json:"claim_key"`
	ExpiresAt    time.Time  `json:"expires_at"`
	CreatedAt    time.Time  `json:"created_at"`
	Previous     *time.Time `json:"previous_expires_at,omitempty"`
}

// DeviceClaimTokenView 管理视图（无明文、无哈希，只有元数据与生效状态）。
type DeviceClaimTokenView struct {
	TokenID        string     `json:"token_id"`
	DeviceID       string     `json:"device_id"`
	DeviceNumber   string     `json:"device_number"`
	Status         string     `json:"status"`
	Effective      string     `json:"effective"`
	ExpiresAt      time.Time  `json:"expires_at"`
	ConsumedByTID  *string    `json:"consumed_by_tenant_id,omitempty"`
	ConsumedAt     *time.Time `json:"consumed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// RedeemDeviceClaimResp 认领成功响应。
type RedeemDeviceClaimResp struct {
	DeviceID      string    `json:"device_id"`
	DeviceNumber  string    `json:"device_number"`
	TokenID       string    `json:"token_id"`
	PreviousTenID string    `json:"previous_tenant_id"`
	ClaimedAt     time.Time `json:"claimed_at"`
}
