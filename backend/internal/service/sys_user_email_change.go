// 文件用途：账号换绑邮箱流程。
// 核心逻辑：校验新邮箱 → 确认未被占用 → 依次用新邮箱/当前邮箱的验证码比对 → 更新邮箱 → 作废验证码并迁移登录 token。
// 关键注意事项：验证码比对必须常量时间且计入失败次数；换绑不改变租户和设备归属，返回的 devices_migrated 只是租户设备计数。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/errcode"

	"gorm.io/gorm"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"
)

// changeEmailLoginTokenTTL 与登录流程写入的邮箱 token 有效期一致。
const changeEmailLoginTokenTTL = 7 * 24 * time.Hour

// ChangeEmail verifies the new email and keeps the current tenant/device ownership intact.
func (*User) ChangeEmail(ctx context.Context, req *model.ChangeEmailReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	user, err := requireChangeEmailUser(claims)
	if err != nil {
		return nil, err
	}

	newEmail, err := validateChangeEmailRequest(req, user.Email)
	if err != nil {
		return nil, err
	}
	if err := ensureChangeEmailTargetAvailable(newEmail); err != nil {
		return nil, err
	}

	matchedCodeEmail, err := verifyChangeEmailCode(ctx, newEmail, user.Email, req.VerifyCode)
	if err != nil {
		return nil, err
	}

	if err := updateUserEmail(ctx, user, newEmail); err != nil {
		return nil, err
	}

	clearChangeEmailVerificationCode(ctx, matchedCodeEmail)
	migrateChangeEmailLoginToken(ctx, user.Email, newEmail)

	deviceCount, err := countTenantDevicesForEmailChange(ctx, claims.TenantID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"old_email":        user.Email,
		"new_email":        newEmail,
		"tenant_id":        claims.TenantID,
		"devices_migrated": deviceCount,
	}, nil
}

func requireChangeEmailUser(claims *utils.UserClaims) (*model.User, error) {
	if !hasUserClaimsIdentity(claims) {
		return nil, errcode.New(errcode.CodeNoPermission)
	}

	user, err := dal.GetUsersById(claims.ID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": claims.ID,
		})
	}
	return user, nil
}

func validateChangeEmailRequest(req *model.ChangeEmailReq, currentEmail string) (string, error) {
	if req == nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "change email request is required")
	}

	newEmail := normalizeChangeEmailAddress(req.NewEmail)
	if newEmail == "" {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "new_email is required")
	}
	if strings.EqualFold(currentEmail, newEmail) {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "new email must be different from current email")
	}
	return newEmail, nil
}

func normalizeChangeEmailAddress(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ensureChangeEmailTargetAvailable(newEmail string) error {
	existing, err := dal.GetUsersByEmail(newEmail)
	switch {
	case err == nil && existing != nil:
		return errcode.NewWithMessage(errcode.CodeParamError, "new email is already registered")
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "check_new_email",
			"email":     newEmail,
			"error":     err.Error(),
		})
	}
	return nil
}

// verifyChangeEmailCode 依次尝试新邮箱、当前邮箱下的验证码（兼容旧站“向当前邮箱发码”的流程），
// 返回命中验证码所属的邮箱。任一候选失败次数超限即直接拒绝；
// 存在验证码但不匹配返回 200012，所有候选都没有验证码返回 200011。
func verifyChangeEmailCode(ctx context.Context, newEmail, currentEmail, providedCode string) (string, error) {
	codeFound := false
	normalizedCode := strings.TrimSpace(providedCode)
	for _, email := range changeEmailCodeCandidateEmails(newEmail, currentEmail) {
		if err := ensureVerificationCodeAttemptsAllowed(ctx, email); err != nil {
			return "", err
		}
		storedCode, codeErr := global.REDIS.Get(ctx, changeEmailVerificationCodeKey(email)).Result()
		if codeErr != nil {
			continue
		}
		codeFound = true
		if verificationCodesEqual(storedCode, normalizedCode) {
			return email, nil
		}
		registerVerificationCodeFailure(ctx, email)
	}
	if codeFound {
		return "", errcode.New(200012)
	}
	return "", errcode.New(200011)
}

func changeEmailCodeCandidateEmails(newEmail, currentEmail string) []string {
	candidates := make([]string, 0, 2)
	for _, email := range []string{newEmail, normalizeChangeEmailAddress(currentEmail)} {
		if email != "" {
			candidates = append(candidates, email)
		}
	}
	return candidates
}

// changeEmailVerificationCodeKey 与 GetVerificationCode 写入的 key 必须一致，统一委托给 verificationCodeKey。
func changeEmailVerificationCodeKey(email string) string {
	return verificationCodeKey(email)
}

// changeEmailUserUpdates 构造换绑邮箱的列更新：用户名仍等于旧邮箱（或未设置）时一并改为新邮箱。
func changeEmailUserUpdates(user *model.User, newEmail string, now time.Time) map[string]interface{} {
	updates := map[string]interface{}{
		"email":      newEmail,
		"updated_at": now,
	}
	if user.Name == nil || strings.EqualFold(strings.TrimSpace(*user.Name), user.Email) {
		updates["name"] = newEmail
	}
	return updates
}

func updateUserEmail(ctx context.Context, user *model.User, newEmail string) error {
	result, err := query.User.WithContext(ctx).Where(query.User.ID.Eq(user.ID)).Updates(changeEmailUserUpdates(user, newEmail, time.Now()))
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "change_email",
			"user_id":   user.ID,
			"error":     err.Error(),
		})
	}
	if result.RowsAffected == 0 {
		return errcode.NewWithMessage(errcode.CodeNotFound, "user not found")
	}
	return nil
}

func clearChangeEmailVerificationCode(ctx context.Context, matchedCodeEmail string) {
	if matchedCodeEmail != "" {
		_ = global.REDIS.Del(ctx, changeEmailVerificationCodeKey(matchedCodeEmail)).Err()
	}
}

func migrateChangeEmailLoginToken(ctx context.Context, oldEmail, newEmail string) {
	oldToken, err := global.REDIS.Get(ctx, loginEmailTokenKey(oldEmail)).Result()
	if err != nil || oldToken == "" {
		return
	}
	_ = global.REDIS.Set(ctx, loginEmailTokenKey(newEmail), oldToken, changeEmailLoginTokenTTL).Err()
	_ = global.REDIS.Del(ctx, loginEmailTokenKey(oldEmail)).Err()
}

func countTenantDevicesForEmailChange(ctx context.Context, tenantID string) (int64, error) {
	deviceCount, err := query.Device.WithContext(ctx).Where(query.Device.TenantID.Eq(tenantID)).Count()
	if err != nil {
		return 0, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "count_migrated_devices",
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
	}
	return deviceCount, nil
}
