// 文件用途：接入安全 X.509（ROADMAP D5）服务层——设备证书生命周期与平台 CA。
// 核心逻辑：
//   - 平台 CA：首次签发时自举生成 ECDSA P-256 自签 CA（IsCA=true），PEM 持久化 platform_cas 单行。
//   - 签发：为设备生成 ECDSA P-256 密钥对并用 CA 签发客户端证书（ExtKeyUsage ClientAuth，
//     供 broker mTLS）；私钥仅在签发响应返回一次，平台只存证书。
//   - 吊销/轮换/到期：状态机 active→revoked/expired；校验时按序列号查库判吊销。
//   - 校验：证书链验证（对平台 CA）+ 有效期 + 库内吊销状态，返回 device_id/tenant_id。
// 关键注意事项：
//   - 证书不绑定具体 hostname；接入方以证书链+序列号+库内状态为准（IoT 客户端证书惯例）。
//   - 平台 CA 私钥当前存库（本地开发栈可接受）；生产必须迁移 KMS/HSM 并加密静态存储，
//     届时仅需替换 loadPlatformCA/savePlatformCA 两个函数的实现，上层生命周期逻辑不变。
//   - Organization 字段携带租户 ID，便于离线侧（如 broker 日志）粗粒度归属识别；
//     权威租户边界仍以 device_certificates 行的 tenant_id 为准。
package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeviceCertificateService 接入安全 X.509 业务服务。
type DeviceCertificateService struct{}

const (
	deviceCertDefaultValidityDays = 365
	deviceCertMaxValidityDays     = 3650
	deviceCertCAID                = "default"
)

// ensurePlatformCA 返回平台 CA；不存在时自举生成并持久化。
func ensurePlatformCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	row, err := dal.GetPlatformCA()
	if err == nil {
		caBlock, _ := pem.Decode([]byte(row.Certificate))
		if caBlock == nil {
			return nil, nil, errcode.NewWithMessage(errcode.CodeParamError, "platform CA PEM parse failed")
		}
		caCert, err := x509.ParseCertificate(caBlock.Bytes)
		if err != nil {
			return nil, nil, errcode.NewWithMessage(errcode.CodeParamError, "platform CA parse failed: "+err.Error())
		}
		keyBlock, _ := pem.Decode([]byte(row.PrivateKey))
		if keyBlock == nil {
			return nil, nil, errcode.NewWithMessage(errcode.CodeParamError, "platform CA key PEM parse failed")
		}
		caKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, nil, errcode.NewWithMessage(errcode.CodeParamError, "platform CA key parse failed: "+err.Error())
		}
		ecKey, ok := caKey.(*ecdsa.PrivateKey)
		if !ok {
			return nil, nil, errcode.NewWithMessage(errcode.CodeParamError, "platform CA key is not ECDSA")
		}
		return caCert, ecKey, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	// 自举：生成自签 CA。
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, errcode.NewWithMessage(errcode.CodeParamError, "generate CA key failed: "+err.Error())
	}
	now := time.Now()
	serial, err := randSerial()
	if err != nil {
		return nil, nil, errcode.NewWithMessage(errcode.CodeParamError, "generate CA serial failed: "+err.Error())
	}
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "AetherLink Device CA", Organization: []string{"AetherLink"}},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, errcode.NewWithMessage(errcode.CodeParamError, "create CA cert failed: "+err.Error())
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, nil, errcode.NewWithMessage(errcode.CodeParamError, "parse CA cert failed: "+err.Error())
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(caKey)
	if err != nil {
		return nil, nil, errcode.NewWithMessage(errcode.CodeParamError, "marshal CA key failed: "+err.Error())
	}
	row = &model.PlatformCA{
		ID:          deviceCertCAID,
		Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})),
		PrivateKey:  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})),
		NotBefore:   tpl.NotBefore,
		NotAfter:    tpl.NotAfter,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := dal.SavePlatformCA(row); err != nil {
		return nil, nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return caCert, caKey, nil
}

// randSerial 生成 128bit 正随机序列号（RFC 5280 要求为正整数）。
func randSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	s, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// issueCertForDevice 用平台 CA 为设备签发客户端证书，落库并返回（含一次性私钥）。
func issueCertForDevice(tenantID, deviceID, commonName string, validityDays int) (*model.IssueDeviceCertificateResp, error) {
	if validityDays <= 0 {
		validityDays = deviceCertDefaultValidityDays
	}
	if validityDays > deviceCertMaxValidityDays {
		validityDays = deviceCertMaxValidityDays
	}
	caCert, caKey, err := ensurePlatformCA()
	if err != nil {
		return nil, err
	}
	devKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "generate device key failed: "+err.Error())
	}
	serial, err := randSerial()
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "generate device serial failed: "+err.Error())
	}
	if commonName == "" {
		commonName = deviceID
	}
	now := time.Now()
	notBefore := now.Add(-time.Minute)
	notAfter := now.AddDate(0, 0, validityDays)
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{tenantID},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tpl, caCert, &devKey.PublicKey, caKey)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "create device cert failed: "+err.Error())
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyDER, err := x509.MarshalPKCS8PrivateKey(devKey)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "marshal device key failed: "+err.Error())
	}
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	sum := sha256.Sum256(certDER)

	row := &model.DeviceCertificate{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		DeviceID:     deviceID,
		SerialNumber: serial.Text(16),
		Fingerprint:  hex.EncodeToString(sum[:]),
		CommonName:   commonName,
		Certificate:  certPEM,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		Status:       "active",
		IssuedAt:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := dal.CreateDeviceCertificate(row); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return &model.IssueDeviceCertificateResp{
		ID:           row.ID,
		DeviceID:     row.DeviceID,
		SerialNumber: row.SerialNumber,
		Fingerprint:  row.Fingerprint,
		CommonName:   row.CommonName,
		Certificate:  certPEM,
		PrivateKey:   keyPEM,
		NotBefore:    notBefore.Format(time.RFC3339),
		NotAfter:     notAfter.Format(time.RFC3339),
	}, nil
}

// IssueDeviceCertificate 为设备签发证书（ROADMAP D5）。
func (DeviceCertificateService) IssueDeviceCertificate(req *model.IssueDeviceCertificateReq, claims *utils.UserClaims) (*model.IssueDeviceCertificateResp, error) {
	return issueCertForDevice(claims.TenantID, req.DeviceID, req.CommonName, req.ValidityDays)
}

// ListDeviceCertificates 租户内列证书，可按设备过滤，默认最多 100 条。
func (DeviceCertificateService) ListDeviceCertificates(deviceID string, limit int, claims *utils.UserClaims) ([]*model.DeviceCertificate, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	list, err := dal.ListDeviceCertificates(claims.TenantID, deviceID, limit)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	lazilyExpire(list)
	return list, nil
}

// GetDeviceCertificate 租户内取单条证书详情。
func (DeviceCertificateService) GetDeviceCertificate(id string, claims *utils.UserClaims) (*model.DeviceCertificate, error) {
	row, err := dal.GetDeviceCertificateInTenant(id, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "device certificate not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	lazilyExpire([]*model.DeviceCertificate{row})
	return row, nil
}

// RevokeDeviceCertificate 吊销证书（人工吊销）。
func (DeviceCertificateService) RevokeDeviceCertificate(req *model.RevokeDeviceCertificateReq, claims *utils.UserClaims) (*model.DeviceCertificate, error) {
	row, err := dal.GetDeviceCertificateInTenant(req.ID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "device certificate not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if row.Status != "active" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device certificate is not active: "+row.Status)
	}
	reason := req.Reason
	if reason == "" {
		reason = "revoked by user"
	}
	if err := dal.MarkDeviceCertificateRevoked(row.ID, reason, time.Now()); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return dal.GetDeviceCertificateInTenant(row.ID, claims.TenantID)
}

// RenewDeviceCertificate 轮换：为同一设备签发新证书并吊销旧证书，返回新证书（含一次性私钥）。
func (DeviceCertificateService) RenewDeviceCertificate(req *model.RenewDeviceCertificateReq, claims *utils.UserClaims) (*model.IssueDeviceCertificateResp, error) {
	old, err := dal.GetDeviceCertificateInTenant(req.ID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "device certificate not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	resp, err := issueCertForDevice(claims.TenantID, old.DeviceID, old.CommonName, req.ValidityDays)
	if err != nil {
		return nil, err
	}
	if old.Status == "active" {
		if err := dal.MarkDeviceCertificateRevoked(old.ID, "renewed by "+resp.ID, time.Now()); err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
	}
	return resp, nil
}

// VerifyDeviceCertificate 校验 PEM 证书：链验证（对平台 CA）+ 有效期 + 库内吊销状态。
// 供 broker mTLS 认证插件与人工校验复用；broker 进程内集成时直接调用本函数即可（同库依赖）。
func (DeviceCertificateService) VerifyDeviceCertificate(req *model.VerifyDeviceCertificateReq, claims *utils.UserClaims) (*model.VerifyDeviceCertificateResp, error) {
	return verifyDeviceCertificatePEM(req.Certificate, claims.TenantID)
}

// verifyDeviceCertificatePEM 核心校验：不依赖 claims 的版本供内部复用。
func verifyDeviceCertificatePEM(certPEM, tenantID string) (*model.VerifyDeviceCertificateResp, error) {
	invalid := func(reason string) (*model.VerifyDeviceCertificateResp, error) {
		return &model.VerifyDeviceCertificateResp{Valid: false, Reason: reason}, nil
	}
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return invalid("certificate PEM parse failed")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return invalid("certificate parse failed: " + err.Error())
	}
	caCert, _, err := ensurePlatformCA()
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	if _, err := cert.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		return invalid("chain verify failed: " + err.Error())
	}
	if cert.NotAfter.Before(time.Now()) {
		return invalid("certificate expired")
	}
	// 租户边界：优先信任证书 Organization 携带的租户；显式传入 tenantID 时以传入值为准。
	certTenant := tenantID
	if certTenant == "" && len(cert.Subject.Organization) > 0 {
		certTenant = cert.Subject.Organization[0]
	}
	if certTenant == "" {
		return invalid("tenant unknown")
	}
	row, err := dal.GetActiveDeviceCertificateBySerial(certTenant, cert.SerialNumber.Text(16))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invalid("certificate revoked or unknown serial")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return &model.VerifyDeviceCertificateResp{
		Valid:    true,
		DeviceID: row.DeviceID,
		TenantID: row.TenantID,
	}, nil
}

// lazilyExpire 将已过期但仍标 active 的行翻转为 expired（读路径惰性收敛状态）。
func lazilyExpire(list []*model.DeviceCertificate) {
	now := time.Now()
	for _, row := range list {
		if row.Status == "active" && row.NotAfter.Before(now) {
			if err := dal.MarkDeviceCertificateExpired(row.ID); err == nil {
				row.Status = "expired"
			}
		}
	}
}
