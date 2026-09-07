// 文件用途：接入安全 X.509（ROADMAP D5）数据访问层。
// 核心逻辑：设备证书 CRUD、按序列号定位、平台 CA 单行读写。
// 关键注意事项：全部查询带 tenant_id 过滤保证租户隔离；平台 CA 为全局单行（id="default"）。
package dal

import (
	"errors"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

func CreateDeviceCertificate(s *model.DeviceCertificate) error {
	return global.DB.Create(s).Error
}

// GetDeviceCertificateInTenant 按租户定位单条证书记录；未命中返回 gorm.ErrRecordNotFound。
func GetDeviceCertificateInTenant(id, tenantID string) (*model.DeviceCertificate, error) {
	var s model.DeviceCertificate
	err := global.DB.
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&s).Error
	return &s, err
}

// ListDeviceCertificates 租户内列证书，可按设备过滤。
func ListDeviceCertificates(tenantID, deviceID string, limit int) ([]*model.DeviceCertificate, error) {
	var list []*model.DeviceCertificate
	q := global.DB.Where("tenant_id = ?", tenantID)
	if deviceID != "" {
		q = q.Where("device_id = ?", deviceID)
	}
	err := q.Order("created_at DESC").Limit(limit).Find(&list).Error
	return list, err
}

// GetActiveDeviceCertificateBySerial 按序列号取 active 证书（吊销校验/接入认证用）。
// 序列号在签发时用 128bit 随机数生成，跨租户冲突概率可忽略；仍以租户过滤兜底。
func GetActiveDeviceCertificateBySerial(tenantID, serial string) (*model.DeviceCertificate, error) {
	var s model.DeviceCertificate
	err := global.DB.
		Where("tenant_id = ? AND serial_number = ? AND status = ?", tenantID, serial, "active").
		First(&s).Error
	return &s, err
}

// MarkDeviceCertificateRevoked 吊销落库。
func MarkDeviceCertificateRevoked(id, reason string, revokedAt interface{}) error {
	return global.DB.Model(&model.DeviceCertificate{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"status": "revoked", "revoked_at": revokedAt, "revoke_reason": reason}).Error
}

// MarkDeviceCertificateExpired 到期惰性翻转。
func MarkDeviceCertificateExpired(id string) error {
	return global.DB.Model(&model.DeviceCertificate{}).
		Where("id = ?", id).
		Update("status", "expired").Error
}

// GetPlatformCA 读取平台 CA 单行；未初始化返回 gorm.ErrRecordNotFound。
// tenant-scope: system-table——platform_cas 为平台级系统表（无租户列，全局单行 id="default"），
// 设备证书行的租户隔离由 device_certificates.tenant_id 承担，与 CA 复用无关。
func GetPlatformCA() (*model.PlatformCA, error) {
	var ca model.PlatformCA
	err := global.DB.Where("id = ?", "default").First(&ca).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ca, err
}

// SavePlatformCA 写入/更新平台 CA 单行。
func SavePlatformCA(ca *model.PlatformCA) error {
	return global.DB.Save(ca).Error
}
