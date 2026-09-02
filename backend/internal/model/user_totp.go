// 文件用途：2FA（ROADMAP C7）模型——user_totp 状态与一次性恢复码。
// 边界：secret 仅存 AES-GCM 密文（服务端派生密钥），明文永不落库/入日志。
package model

import "time"

// UserTOTP 用户 TOTP 绑定状态（一用户一行）。
type UserTOTP struct {
	UserID       string     `gorm:"column:user_id;primaryKey" json:"user_id"`
	SecretCipher string     `gorm:"column:secret_cipher;not null" json:"-"`
	Enabled      bool       `gorm:"column:enabled;not null;default:false" json:"enabled"`
	LastUsedStep int64      `gorm:"column:last_used_step;not null;default:0" json:"-"`
	CreatedAt    *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName UserTOTP's table name
func (*UserTOTP) TableName() string { return "user_totp" }

// UserTOTPRecoveryCode 一次性恢复码（存 SHA-256，用后即废）。
type UserTOTPRecoveryCode struct {
	ID        string     `gorm:"column:id;primaryKey" json:"id"`
	UserID    string     `gorm:"column:user_id;not null" json:"user_id"`
	CodeHash  string     `gorm:"column:code_hash;not null" json:"-"`
	UsedAt    *time.Time `gorm:"column:used_at" json:"used_at"`
	CreatedAt *time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName UserTOTPRecoveryCode's table name
func (*UserTOTPRecoveryCode) TableName() string { return "user_totp_recovery_codes" }
