// 文件用途：P3 商业许可证边界的服务接线——离线验证、状态查询、启动门控与设备配额。
// 核心逻辑：
//   - 验证核心在 pkg/license（Ed25519，纯标准库，离线）；本文件负责配置装配与执法点。
//   - 执法点 1：启动门控。license.required=true 时启动必须持有效许可证，否则拒绝启动。
//   - 执法点 2：设备配额。许可证声明 max_devices>0 且当前有效时，CreateDevice 前置检查。
//   - 状态查询：GET /api/v1/license/status（SYS_ADMIN）。
//
// 关键注意事项：
//   - **未配置公钥 = 商业边界未启用**：所有执法点放行并如实报告 enabled=false。
//     默认行为不变是既有部署的兼容底线；要启用边界，部署方必须显式配置公钥与材料。
//   - 材料无效但 required=true 属启动失败；required=false 只记告警——
//     静默忽略等于让"过期"和"没启用"无法区分。
package service

import (
	"errors"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/license"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/spf13/viper"
)

// 许可证配置键。
const (
	licensePublicKeysKey = "license.public_keys"
	licenseMaterialKey   = "license.material"
	licenseRequiredKey   = "license.required"
)

// LicenseService 商业许可证服务。
type LicenseService struct{}

// verifierOnce 公钥表装配一次；配置在运行期不热加载（与 secrets 同口径）。
var (
	verifierOnce   sync.Once
	verifierLoaded *license.Verifier
	verifierErr    error
)

// licenseVerifier 从配置构造验证器；未配置公钥返回 (nil, nil)——边界未启用。
// 配置非法则返回错误，调用方必须如实上抛，不得降级成"未启用"。
func licenseVerifier() (*license.Verifier, error) {
	verifierOnce.Do(func() {
		raw := viper.GetStringMapString(licensePublicKeysKey)
		if len(raw) == 0 {
			return
		}
		verifierLoaded, verifierErr = license.NewVerifier(raw)
	})
	return verifierLoaded, verifierErr
}

// ErrLicenseBoundaryViolated 启动门控失败。
var ErrLicenseBoundaryViolated = errors.New("license: valid license is required but missing or invalid")

// LicenseStatus 许可证状态视图（不暴露材料本身）。
type LicenseStatus struct {
	Enabled     bool     `json:"enabled"`  // 公钥已配置，边界已启用
	Required    bool     `json:"required"` // 是否为启动硬性要求
	Valid       bool     `json:"valid"`    // 当前材料是否通过验证
	Edition     string   `json:"edition,omitempty"`
	IssuedTo    string   `json:"issued_to,omitempty"`
	Features    []string `json:"features,omitempty"`
	MaxDevices  int64    `json:"max_devices,omitempty"`
	MaxTenants  int64    `json:"max_tenants,omitempty"`
	NotBeforeMs int64    `json:"not_before_ms,omitempty"`
	NotAfterMs  int64    `json:"not_after_ms,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"` // 正文 SHA-256，供合同比对
	Reason      string   `json:"reason,omitempty"`      // 未通过时的原因
}

// currentDocument 验证当前配置的材料。未启用边界返回 (nil, nil)。
func (LicenseService) currentDocument() (*license.Document, error) {
	verifier, err := licenseVerifier()
	if err != nil {
		return nil, err
	}
	if verifier == nil {
		return nil, nil
	}
	material := viper.GetString(licenseMaterialKey)
	if material == "" {
		return nil, license.ErrLicenseUnsigned
	}
	doc, _, err := verifier.Parse(material, time.Now())
	return doc, err
}

// GetStatus 查询许可证状态。
func (LicenseService) GetStatus(claims *utils.UserClaims) (*LicenseStatus, error) {
	if claims == nil || claims.Authority != "SYS_ADMIN" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "license status is platform-admin capability")
	}
	verifier, err := licenseVerifier()
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
	}
	status := &LicenseStatus{
		Enabled:  verifier != nil,
		Required: viper.GetBool(licenseRequiredKey),
	}
	doc, derr := LicenseService{}.currentDocument()
	switch {
	case verifier == nil:
		status.Reason = "license boundary is not enabled (no public keys configured)"
	case derr != nil:
		status.Reason = derr.Error()
	default:
		status.Valid = true
		status.Edition = doc.Edition
		status.IssuedTo = doc.IssuedTo
		status.Features = doc.Features
		status.MaxDevices = doc.MaxDevices
		status.MaxTenants = doc.MaxTenants
		status.NotBeforeMs = doc.NotBefore
		status.NotAfterMs = doc.NotAfter
	}
	if status.Valid {
		// fingerprint 在 Parse 成功时才有意义，这里重新解析一次取摘要。
		material := viper.GetString(licenseMaterialKey)
		if _, fp, perr := verifier.Parse(material, time.Now()); perr == nil {
			status.Fingerprint = fp
		}
	}
	return status, nil
}

// EnforceAtStartup 启动门控：required=true 时必须持有效许可证。
func (LicenseService) EnforceAtStartup() error {
	if !viper.GetBool(licenseRequiredKey) {
		return nil
	}
	verifier, err := licenseVerifier()
	if err != nil {
		return err
	}
	if verifier == nil {
		return ErrLicenseBoundaryViolated
	}
	doc, derr := LicenseService{}.currentDocument()
	if derr != nil || doc == nil {
		return ErrLicenseBoundaryViolated
	}
	return nil
}

// enforceDeviceQuota 设备配额执法（CreateDevice 前置）。
// 边界未启用 / 材料无效 / 未声明配额一律放行——配额只在"有效许可证明确声明"时执行，
// 与 P0.7 以来"默认未配置即关闭"的口径一致。
func enforceDeviceQuota() error {
	doc, derr := LicenseService{}.currentDocument()
	if derr != nil || doc == nil || doc.MaxDevices <= 0 {
		return nil
	}
	count, err := dal.CountAllDevices()
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if count >= doc.MaxDevices {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error":       "device quota exhausted by license",
			"max_devices": doc.MaxDevices,
			"current":     count,
		})
	}
	return nil
}
