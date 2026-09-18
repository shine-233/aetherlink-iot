// 文件用途：通用 Secrets Storage（ROADMAP TB-18）业务服务层。
// 核心逻辑：
//   - 密钥录入与更新：强校验 Key 命名空间并使用 pkg/secrets AES-256-GCM 信封加密，AAD 绑定租户 ID；
//   - 读模型脱敏：普通查询和详情仅出不可逆 MaskPreview，绝对不暴露密文和明文；
//   - 高危解密（Reveal）：严格权限门禁，使用 secrets.Open 解密，审计记录入库，绝不在日志回显明文；
//   - 轮换重加密（Reseal）：检测旧版本密钥并用 active_key_id 重塑信封；
//   - 内部下游解析（ResolveSecret）：提供 ${secret.KEY} 动态解密能力，供规则链、Webhook 和通知等下游复用。
// 关键注意事项：任何密钥材料错误、缺失或越权一律 fail closed。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/secrets"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SecretService struct{}

const (
	secretMaskHead = 4
)

// tenantScope 返回租户过滤范围。超管（SYS_ADMIN）若未绑定特定租户可跨租户查看，其他角色严格隔离。
func tenantScope(claims *utils.UserClaims) string {
	if claims == nil {
		return ""
	}
	if claims.Authority == "SYS_ADMIN" {
		// 超管默认不限制租户范围，若 claims.TenantID 为非系统占位符则尊重
		return claims.TenantID
	}
	return claims.TenantID
}

// toSecretResp 转换出参，计算脱敏和重加密需求。
func toSecretResp(s *model.SysSecret, needsReseal bool) *model.SecretResp {
	if s == nil {
		return nil
	}
	return &model.SecretResp{
		ID:          s.ID,
		TenantID:    s.TenantID,
		Key:         s.Key,
		Name:        s.Name,
		SecretType:  s.SecretType,
		Description: s.Description,
		MaskPreview: s.MaskPreview,
		KeyID:       s.KeyID,
		NeedsReseal: needsReseal,
		CreatedAt:   s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   s.UpdatedAt.Format(time.RFC3339),
	}
}

// validateSecretType 校验 secret_type 是否合法。
func validateSecretType(st string) bool {
	switch st {
	case model.SecretTypeGeneric,
		model.SecretTypeApiKey,
		model.SecretTypeToken,
		model.SecretTypePassword,
		model.SecretTypeCertificate,
		model.SecretTypeOAuth2:
		return true
	default:
		return false
	}
}

// CreateSecret 创建通用密钥。
func (SecretService) CreateSecret(ctx context.Context, req *model.CreateSecretReq, claims *utils.UserClaims) (*model.SecretResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized")
	}
	key := strings.TrimSpace(req.Key)
	if !model.SecretKeyRegex.MatchString(key) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "invalid secret key format: only letters, numbers, _, -, . allowed (1-64 chars)")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "secret name is required")
	}
	rawVal := strings.TrimSpace(req.Value)
	if rawVal == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "secret value is required")
	}
	secType := strings.ToUpper(strings.TrimSpace(req.SecretType))
	if secType == "" {
		secType = model.SecretTypeGeneric
	}
	if !validateSecretType(secType) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("invalid secret_type: %s", secType))
	}

	tenantID := claims.TenantID
	// 检查租户命名空间下的 key 唯一性
	exists, err := dal.CheckKeyExists(ctx, tenantID, key, "")
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, "failed to check key uniqueness: "+err.Error())
	}
	if exists {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("secret key %q already exists in this tenant", key))
	}

	// 使用租户 ID 作为 AAD 信封加密
	encrypted, err := secrets.Seal(rawVal, tenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, "failed to encrypt secret: "+err.Error())
	}
	keyID, _ := secrets.KeyIDOf(encrypted)
	maskPreview := secrets.Mask(rawVal, secretMaskHead)

	now := time.Now()
	s := &model.SysSecret{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		Key:            key,
		Name:           name,
		SecretType:     secType,
		Description:    strings.TrimSpace(req.Description),
		EncryptedValue: encrypted,
		MaskPreview:    maskPreview,
		KeyID:          keyID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := dal.CreateSecret(ctx, s); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, "failed to save secret: "+err.Error())
	}

	return toSecretResp(s, false), nil
}

// GetSecret 获取密钥详情（脱敏）。
func (SecretService) GetSecret(ctx context.Context, id string, claims *utils.UserClaims) (*model.SecretResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized")
	}
	s, err := dal.GetSecretByID(ctx, tenantScope(claims), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "secret not found")
		}
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
	}
	return toSecretResp(s, secrets.NeedsReseal(s.EncryptedValue)), nil
}

// ListSecrets 分页与条件查询密钥（全部脱敏）。
func (SecretService) ListSecrets(ctx context.Context, req *model.SecretListReq, claims *utils.UserClaims) (*model.SecretPageResult, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized")
	}
	list, total, err := dal.ListSecrets(ctx, tenantScope(claims), req)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
	}

	resps := make([]model.SecretResp, 0, len(list))
	for _, s := range list {
		resps = append(resps, *toSecretResp(&s, secrets.NeedsReseal(s.EncryptedValue)))
	}
	return &model.SecretPageResult{
		List:  resps,
		Total: total,
	}, nil
}

// UpdateSecret 更新密钥元数据或重新加密更新值。
func (SecretService) UpdateSecret(ctx context.Context, id string, req *model.UpdateSecretReq, claims *utils.UserClaims) (*model.SecretResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized")
	}
	s, err := dal.GetSecretByID(ctx, tenantScope(claims), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "secret not found")
		}
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
	}

	if name := strings.TrimSpace(req.Name); name != "" {
		s.Name = name
	}
	if secType := strings.ToUpper(strings.TrimSpace(req.SecretType)); secType != "" {
		if !validateSecretType(secType) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("invalid secret_type: %s", secType))
		}
		s.SecretType = secType
	}
	if req.Description != "" {
		s.Description = strings.TrimSpace(req.Description)
	}

	// 若提供了新明文，重新信封加密并更新 MaskPreview
	if rawVal := strings.TrimSpace(req.Value); rawVal != "" {
		encrypted, err := secrets.Seal(rawVal, s.TenantID)
		if err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeSystemError, "failed to encrypt secret: "+err.Error())
		}
		keyID, _ := secrets.KeyIDOf(encrypted)
		s.EncryptedValue = encrypted
		s.KeyID = keyID
		s.MaskPreview = secrets.Mask(rawVal, secretMaskHead)
	}

	if err := dal.UpdateSecret(ctx, s); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, "failed to update secret: "+err.Error())
	}

	return toSecretResp(s, secrets.NeedsReseal(s.EncryptedValue)), nil
}

// DeleteSecret 删除密钥。
func (SecretService) DeleteSecret(ctx context.Context, id string, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized")
	}
	err := dal.DeleteSecret(ctx, tenantScope(claims), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.NewWithMessage(errcode.CodeNotFound, "secret not found")
		}
		return errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
	}
	return nil
}

// RevealSecret 明文解密查看（防泄密审计闸门）。
func (SecretService) RevealSecret(ctx context.Context, id string, claims *utils.UserClaims) (*model.RevealSecretResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized")
	}
	// 深度安全防线：只有系统管理员和租户管理员允许调用解密
	if claims.Authority != "SYS_ADMIN" && claims.Authority != "TENANT_ADMIN" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "permission denied: cannot reveal secret")
	}

	s, err := dal.GetSecretByID(ctx, tenantScope(claims), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "secret not found")
		}
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
	}

	plain, err := secrets.Open(s.EncryptedValue, s.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, "failed to decrypt secret: "+err.Error())
	}

	// 记录安全审查日志（切记：绝不记录解密明文到审计日志！）
	auditPath := "/api/v1/secrets/" + id + "/reveal"
	auditName := "SECRET_REVEAL"
	auditReq := fmt.Sprintf("secret_key=%s,secret_id=%s", s.Key, s.ID)
	auditResp := "revealed_successfully"
	auditLog := &model.OperationLog{
		ID:              uuid.New().String(),
		IP:              "127.0.0.1",
		Path:            &auditPath,
		UserID:          claims.ID,
		Name:            &auditName,
		CreatedAt:       time.Now(),
		TenantID:        claims.TenantID,
		RequestMessage:  &auditReq,
		ResponseMessage: &auditResp,
	}
	if global.DB != nil {
		_ = global.DB.Table("operation_logs").Create(auditLog).Error
	}

	return &model.RevealSecretResp{
		ID:    s.ID,
		Key:   s.Key,
		Value: plain,
	}, nil
}

// ResealSecret 在线轮换重加密。
func (SecretService) ResealSecret(ctx context.Context, id string, claims *utils.UserClaims) (*model.SecretResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized")
	}
	if claims.Authority != "SYS_ADMIN" && claims.Authority != "TENANT_ADMIN" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "permission denied")
	}

	s, err := dal.GetSecretByID(ctx, tenantScope(claims), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "secret not found")
		}
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
	}

	plain, err := secrets.Open(s.EncryptedValue, s.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, "failed to decrypt for reseal: "+err.Error())
	}

	// 用当前 active_key_id 重新 Seal
	newEncrypted, err := secrets.Seal(plain, s.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, "failed to reseal secret: "+err.Error())
	}
	newKeyID, _ := secrets.KeyIDOf(newEncrypted)

	s.EncryptedValue = newEncrypted
	s.KeyID = newKeyID
	if err := dal.UpdateSecret(ctx, s); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, "failed to update resealed secret: "+err.Error())
	}

	return toSecretResp(s, false), nil
}

// ResolveSecret 通用下游凭据解析器。
// 支持解析 "${secret.KEY_NAME}"、"secret:KEY_NAME" 或 "KEY_NAME"。
// 优先解析本租户下的密钥，未找到时可兜底解析系统公共密钥（tenant_id=""）。
// 杜绝在配置中硬编码明文凭证，按需解密后直接返回明文。
func ResolveSecret(ctx context.Context, tenantID string, keyOrRef string) (string, error) {
	raw := strings.TrimSpace(keyOrRef)
	if raw == "" {
		return "", errors.New("empty secret key or reference")
	}

	key := raw
	if strings.HasPrefix(raw, "${secret.") && strings.HasSuffix(raw, "}") {
		key = strings.TrimSuffix(strings.TrimPrefix(raw, "${secret."), "}")
	} else if strings.HasPrefix(raw, "secret:") {
		key = strings.TrimPrefix(raw, "secret:")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("malformed secret reference: key is empty")
	}

	// 1. 尝试查找本租户的密钥
	s, err := dal.GetSecretByKey(ctx, tenantID, key)
	if err != nil && tenantID != "" {
		// 2. 兜底尝试查找系统级密钥
		s, err = dal.GetSecretByKey(ctx, "", key)
	}
	if err != nil {
		return "", fmt.Errorf("secret %q not found: %w", key, err)
	}

	// 解密信封密文（使用密钥记录本身的 TenantID 作为 AAD）
	plain, err := secrets.Open(s.EncryptedValue, s.TenantID)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret %q: %w", key, err)
	}
	return plain, nil
}
