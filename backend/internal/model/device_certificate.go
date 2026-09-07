// 文件用途：接入安全 X.509（ROADMAP D5）数据模型与 HTTP 请求体。
// 核心逻辑：设备证书生命周期（签发/轮换/吊销/校验）的存储结构，以及平台 CA 单行存储结构。
// 关键注意事项：
//   - 私钥仅在签发响应中返回一次，平台侧不落库设备私钥；平台 CA 私钥当前存库（本地开发栈），
//     生产部署应迁移至 KMS/HSM 并启用静态加密（见 service/device_certificate.go 注释）。
//   - 证书状态机：active → revoked（人工/轮换）或 active → expired（到期惰性翻转）。
package model

import "time"

const TableNameDeviceCertificate = "device_certificates"
const TableNamePlatformCA = "platform_cas"

// DeviceCertificate 设备 X.509 证书记录（仅存证书，不存私钥）。
type DeviceCertificate struct {
	ID           string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID     string     `gorm:"column:tenant_id;not null;index" json:"tenant_id"`
	DeviceID     string     `gorm:"column:device_id;not null;index" json:"device_id"`
	SerialNumber string     `gorm:"column:serial_number;not null;index" json:"serial_number"`
	Fingerprint  string     `gorm:"column:fingerprint;not null" json:"fingerprint"` // SHA-256 hex(DER)
	CommonName   string     `gorm:"column:common_name;not null" json:"common_name"`
	Certificate  string     `gorm:"column:certificate;type:text;not null" json:"certificate"` // PEM
	NotBefore    time.Time  `gorm:"column:not_before;not null" json:"not_before"`
	NotAfter     time.Time  `gorm:"column:not_after;not null" json:"not_after"`
	Status       string     `gorm:"column:status;not null;default:active" json:"status"` // active/revoked/expired
	IssuedAt     time.Time  `gorm:"column:issued_at;not null" json:"issued_at"`
	RevokedAt    *time.Time `gorm:"column:revoked_at" json:"revoked_at"`
	RevokeReason string     `gorm:"column:revoke_reason" json:"revoke_reason"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*DeviceCertificate) TableName() string { return TableNameDeviceCertificate }

// PlatformCA 平台设备 CA（单行，id 固定 "default"）。
// 全局单 CA：设备证书以 Organization 字段携带租户标识，避免多租户多 CA 的分发复杂度；
// 吊销校验始终落库到租户作用域的 device_certificates 行，租户隔离不受影响。
type PlatformCA struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	Certificate string    `gorm:"column:certificate;type:text;not null" json:"certificate"` // PEM
	PrivateKey  string    `gorm:"column:private_key;type:text;not null" json:"-"`           // PEM PKCS8，禁止出参
	NotBefore   time.Time `gorm:"column:not_before;not null" json:"not_before"`
	NotAfter    time.Time `gorm:"column:not_after;not null" json:"not_after"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*PlatformCA) TableName() string { return TableNamePlatformCA }

// ---- HTTP 请求/响应结构体 ----

// IssueDeviceCertificateReq 为设备签发证书。
type IssueDeviceCertificateReq struct {
	DeviceID     string `json:"device_id" validate:"required,max=36"`
	CommonName   string `json:"common_name" validate:"omitempty,max=255"`
	ValidityDays int    `json:"validity_days" validate:"omitempty,min=1,max=3650"`
}

// IssueDeviceCertificateResp 签发结果；私钥仅此一次返回。
type IssueDeviceCertificateResp struct {
	ID           string `json:"id"`
	DeviceID     string `json:"device_id"`
	SerialNumber string `json:"serial_number"`
	Fingerprint  string `json:"fingerprint"`
	CommonName   string `json:"common_name"`
	Certificate  string `json:"certificate"` // PEM
	PrivateKey   string `json:"private_key"` // PEM，仅签发响应返回一次
	NotBefore    string `json:"not_before"`
	NotAfter     string `json:"not_after"`
}

// RevokeDeviceCertificateReq 吊销证书。
type RevokeDeviceCertificateReq struct {
	ID     string `json:"id" validate:"required"`
	Reason string `json:"reason" validate:"omitempty,max=255"`
}

// RenewDeviceCertificateReq 轮换证书：为同一设备签发新证书并吊销旧证书。
type RenewDeviceCertificateReq struct {
	ID           string `json:"id" validate:"required"` // 旧证书 ID
	ValidityDays int    `json:"validity_days" validate:"omitempty,min=1,max=3650"`
}

// VerifyDeviceCertificateReq 校验 PEM 证书链与吊销状态（broker mTLS 认证后端）。
type VerifyDeviceCertificateReq struct {
	Certificate string `json:"certificate" validate:"required"`
}

// VerifyDeviceCertificateResp 校验结果。
type VerifyDeviceCertificateResp struct {
	Valid    bool   `json:"valid"`
	DeviceID string `json:"device_id"`
	TenantID string `json:"tenant_id"`
	Reason   string `json:"reason,omitempty"`
}
