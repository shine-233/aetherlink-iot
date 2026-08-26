// 文件用途：维护用户邮箱验证码、换绑邮箱和告警邮箱配置服务。
// 核心逻辑：发送/校验验证码，更新用户邮箱，并在 additional_info 中维护告警邮箱集合。
// 关键注意事项：邮箱流程涉及验证码安全和告警接收人，需防止重放、无效地址和跨租户更新。
// 重构建议：拆分验证码存储、邮件发送和告警邮箱 value object，补齐锁定、事务和发送失败测试。
// sys_user_email.go handles user email verification and warning recipients.
//
// Purpose: send verification codes, change account email addresses, and manage tenant warning-email recipients stored in user additional_info.
// Core logic: masks verification codes in logs, validates current/new email workflows, normalizes recipient lists, and keeps warning recipients tenant-scoped.
// Important notes: verification codes and recipient lists are sensitive, so changes must avoid logging secrets and must preserve tenant isolation.
// Refactor suggestion: move email-code delivery and warning-recipient storage behind interfaces to simplify retry and failure-path tests.
package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"

	"gorm.io/gorm"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

const userWarningEmailsKey = "warning_emails"

// 验证码防滥用参数：发送频率按邮箱维度限流，校验失败次数有上限以防 6 位数字码被暴力枚举。
const (
	verificationCodeMaxAttempts  = 5
	verificationCodeSendInterval = 60 * time.Second
	verificationCodeValidityTTL  = 5 * time.Minute
)

func verificationCodeSendLimitKey(email string) string {
	return "email:" + email + ":code_send_limit"
}

func verificationCodeAttemptsKey(email string) string {
	return "email:" + email + ":code_attempts"
}

func verificationCodeEmailBody(code, language string) string {
	lang := "en-US"
	if normalized, err := normalizePreferredLanguage(language); err == nil {
		lang = normalized
	}

	switch lang {
	case "zh-CN":
		return fmt.Sprintf("您的验证码是 %s", code)
	case "fr-FR":
		return fmt.Sprintf("Votre code de verification est %s", code)
	case "es-ES":
		return fmt.Sprintf("Su codigo de verificacion es %s", code)
	default:
		return fmt.Sprintf("Your verification code is %s", code)
	}
}

// maskVerificationCode 对验证码进行脱敏处理，仅保留前2位和后1位
func maskVerificationCode(code string) string {
	if len(code) <= 3 {
		return strings.Repeat("*", len(code))
	}
	return code[:2] + strings.Repeat("*", len(code)-3) + code[len(code)-1:]
}

// @description 发送验证码
func (userService *User) GetVerificationCode(email, isRegister, language string) error {
	// 按邮箱维度限制发送频率：防止公开接口被刷发真实邮件（邮箱轰炸成本转移给部署者）。
	sent, err := global.REDIS.SetNX(context.Background(), verificationCodeSendLimitKey(email), 1, verificationCodeSendInterval).Result()
	if err != nil {
		return errcode.WithData(errcode.CodeCacheError, map[string]interface{}{
			"operation": "check_verification_send_limit",
			"email":     email,
			"error":     err.Error(),
		})
	}
	if !sent {
		return errcode.New(errcode.CodeRateLimit)
	}

	user, err := dal.GetUsersByEmail(email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_user",
			"email":     email,
			"error":     err.Error(),
		})
	}

	// 邮箱验证相关错误应归类到用户模块
	switch {
	case user == nil && isRegister != "1":
		return errcode.New(errcode.CodeEmailNotFound) // 用户邮箱不存在
	case user != nil && isRegister == "1":
		return errcode.New(200008) // 新增: 用户邮箱已注册
	}

	verificationCode, err := common.GenerateNumericCode(6)
	if err != nil {
		return errcode.WithData(200009, map[string]interface{}{ // 新增: 验证码生成失败
			"email": email,
		})
	}

	err = global.REDIS.Set(context.Background(), email+"_code", verificationCode, verificationCodeValidityTTL).Err()
	if err != nil {
		return errcode.WithData(errcode.CodeCacheError, map[string]interface{}{
			"operation": "save_verification_code",
			"email":     email,
			"error":     err.Error(),
		})
	}
	// 新码生成后重置该邮箱的校验失败计数，避免旧计数误伤新验证码。
	global.REDIS.Del(context.Background(), verificationCodeAttemptsKey(email))

	if err := userService.deliverVerificationCodeEmail(context.Background(), email, verificationCode, language); err != nil {
		return errcode.WithData(200010, map[string]interface{}{ // 新增: 验证码邮件发送失败
			"email": email,
			"error": err.Error(),
		})
	}

	// 只有 adapter 明确确认投递后才记录“已发送”；生产 SMTP 失败不会降级成本地成功。
	_ = maskVerificationCode(verificationCode)
	logrus.Info("verification email sent")
	return nil
}

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
	if claims == nil || strings.TrimSpace(claims.ID) == "" {
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

	newEmail := strings.ToLower(strings.TrimSpace(req.NewEmail))
	if newEmail == "" {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "new_email is required")
	}
	if strings.EqualFold(currentEmail, newEmail) {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "new email must be different from current email")
	}
	return newEmail, nil
}

func ensureChangeEmailTargetAvailable(newEmail string) error {
	if existing, err := dal.GetUsersByEmail(newEmail); err == nil && existing != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "new email is already registered")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "check_new_email",
			"email":     newEmail,
			"error":     err.Error(),
		})
	}
	return nil
}

func verifyChangeEmailCode(ctx context.Context, newEmail, currentEmail, providedCode string) (string, error) {
	codeFound := false
	normalizedCode := strings.TrimSpace(providedCode)
	for _, email := range changeEmailCodeCandidateEmails(newEmail, currentEmail) {
		if err := ensureVerificationCodeAttemptsAllowed(ctx, email); err != nil {
			return "", err
		}
		verificationCode, codeErr := global.REDIS.Get(ctx, changeEmailVerificationCodeKey(email)).Result()
		if codeErr != nil {
			continue
		}
		codeFound = true
		if subtle.ConstantTimeCompare([]byte(verificationCode), []byte(normalizedCode)) == 1 {
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
	for _, email := range []string{newEmail, strings.ToLower(strings.TrimSpace(currentEmail))} {
		if email != "" {
			candidates = append(candidates, email)
		}
	}
	return candidates
}

func changeEmailVerificationCodeKey(email string) string {
	return email + "_code"
}

// ensureVerificationCodeAttemptsAllowed 在比对验证码前检查失败次数；超过上限视为验证码已失效。
func ensureVerificationCodeAttemptsAllowed(ctx context.Context, email string) error {
	attempts, err := global.REDIS.Get(ctx, verificationCodeAttemptsKey(email)).Int()
	if err != nil {
		// 计数不存在或不可解析时按"无失败记录"处理，不阻断正常校验。
		return nil
	}
	if attempts >= verificationCodeMaxAttempts {
		return errcode.New(200011)
	}
	return nil
}

// registerVerificationCodeFailure 记录一次验证码校验失败；达到上限时立即作废该验证码。
func registerVerificationCodeFailure(ctx context.Context, email string) {
	attemptsKey := verificationCodeAttemptsKey(email)
	attempts := global.REDIS.Incr(ctx, attemptsKey).Val()
	if attempts == 1 {
		global.REDIS.Expire(ctx, attemptsKey, verificationCodeValidityTTL)
	}
	if attempts >= verificationCodeMaxAttempts {
		global.REDIS.Del(ctx, changeEmailVerificationCodeKey(email))
	}
}

func updateUserEmail(ctx context.Context, user *model.User, newEmail string) error {
	updates := map[string]interface{}{
		"email":      newEmail,
		"updated_at": time.Now(),
	}
	if user.Name == nil || strings.EqualFold(strings.TrimSpace(*user.Name), user.Email) {
		updates["name"] = newEmail
	}

	result, err := query.User.WithContext(ctx).Where(query.User.ID.Eq(user.ID)).Updates(updates)
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
	_ = global.REDIS.Set(ctx, loginEmailTokenKey(newEmail), oldToken, 7*24*time.Hour).Err()
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

func (*User) GetWarningEmails(claims *utils.UserClaims) ([]string, error) {
	if claims == nil || strings.TrimSpace(claims.ID) == "" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	if !canManageTenantWarningEmails(claims) {
		// Tenant-wide recipient addresses are administrator configuration. Device
		// owners still receive alarms through warningEmailsForOwnedDevices, but an
		// ordinary account must not learn the tenant administrator's addresses.
		return []string{}, nil
	}
	user, err := loadWarningEmailOwnerUser(claims)
	if err != nil {
		return nil, err
	}
	return warningEmailsFromUser(user), nil
}

func (*User) UpdateWarningEmails(ctx context.Context, req *model.WarningEmailReq, claims *utils.UserClaims) ([]string, error) {
	if claims == nil || strings.TrimSpace(claims.ID) == "" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	if !canManageTenantWarningEmails(claims) {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to update tenant warning emails")
	}
	if req == nil {
		req = &model.WarningEmailReq{}
	}

	emails, err := normalizeWarningEmails(req.Emails)
	if err != nil {
		return nil, err
	}

	user, err := loadWarningEmailOwnerUser(claims)
	if err != nil {
		return nil, err
	}

	additional := userAdditionalInfoMap(user.AdditionalInfo)
	additional[userWarningEmailsKey] = emails
	bytes, err := json.Marshal(additional)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}
	if _, err := query.User.WithContext(ctx).Where(query.User.ID.Eq(user.ID)).Update(query.User.AdditionalInfo, string(bytes)); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "update_warning_emails",
			"user_id":   user.ID,
			"error":     err.Error(),
		})
	}
	return emails, nil
}

func loadWarningEmailOwnerUser(claims *utils.UserClaims) (*model.User, error) {
	if claims == nil || strings.TrimSpace(claims.ID) == "" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}

	var tenantAdmin *model.User
	tenantID := strings.TrimSpace(claims.TenantID)
	if tenantID != "" {
		admin, err := dal.GetTenantAdmin(tenantID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"operation": "load_warning_email_tenant_admin",
				"tenant_id": tenantID,
				"error":     err.Error(),
			})
		}
		tenantAdmin = admin
	}

	currentUser, err := dal.GetUsersById(claims.ID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "load_warning_email_user",
			"user_id":   claims.ID,
			"error":     err.Error(),
		})
	}

	owner := pickWarningEmailOwnerUser(claims, tenantAdmin, currentUser)
	if tenantID != "" && owner == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "tenant admin warning email owner not found")
	}

	return owner, nil
}

func canManageTenantWarningEmails(claims *utils.UserClaims) bool {
	if claims == nil {
		return false
	}
	switch strings.TrimSpace(claims.Authority) {
	case constant.SYS_ADMIN, constant.TENANT_ADMIN:
		return true
	default:
		return false
	}
}

func pickWarningEmailOwnerUser(claims *utils.UserClaims, tenantAdmin *model.User, currentUser *model.User) *model.User {
	if claims != nil && strings.TrimSpace(claims.TenantID) != "" {
		return tenantAdmin
	}
	return currentUser
}

func warningEmailsForTenant(tenantID string) []string {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil
	}
	user, err := dal.GetTenantAdmin(tenantID)
	if err != nil || user == nil {
		return nil
	}
	return warningEmailsFromUser(user)
}

// warningEmailsForOwnedDevices prefers the registered warning-email owner of
// one device-owner scope. Aggregated alarms that span multiple owners, contain
// missing devices, or have no explicit owner safely fall back to the tenant
// administrator instead of disclosing one owner's device data to another.
func warningEmailsForOwnedDevices(tenantID string, deviceIDs ...string) []string {
	tenantID = strings.TrimSpace(tenantID)
	normalizedDeviceIDs := make([]string, 0, len(deviceIDs))
	seenDeviceIDs := make(map[string]struct{}, len(deviceIDs))
	for _, rawDeviceID := range deviceIDs {
		deviceID := strings.TrimSpace(rawDeviceID)
		if deviceID == "" {
			continue
		}
		if _, exists := seenDeviceIDs[deviceID]; exists {
			continue
		}
		seenDeviceIDs[deviceID] = struct{}{}
		normalizedDeviceIDs = append(normalizedDeviceIDs, deviceID)
	}
	if tenantID == "" || len(normalizedDeviceIDs) == 0 {
		return warningEmailsForTenant(tenantID)
	}

	devicesByID, err := dal.GetDevicesByIDsUnscoped(normalizedDeviceIDs)
	if err != nil || len(devicesByID) != len(normalizedDeviceIDs) {
		return warningEmailsForTenant(tenantID)
	}

	ownerUserID := ""
	for _, deviceID := range normalizedDeviceIDs {
		device := devicesByID[deviceID]
		if device == nil || strings.TrimSpace(device.TenantID) != tenantID || device.OwnerUserID == nil {
			return warningEmailsForTenant(tenantID)
		}
		currentOwnerUserID := strings.TrimSpace(*device.OwnerUserID)
		if currentOwnerUserID == "" {
			return warningEmailsForTenant(tenantID)
		}
		if ownerUserID == "" {
			ownerUserID = currentOwnerUserID
			continue
		}
		if ownerUserID != currentOwnerUserID {
			return warningEmailsForTenant(tenantID)
		}
	}

	owner, err := dal.GetUsersById(ownerUserID)
	if err != nil || owner == nil || strings.TrimSpace(SafeDeref(owner.TenantID)) != tenantID {
		return warningEmailsForTenant(tenantID)
	}
	if emails := warningEmailsFromUser(owner); len(emails) > 0 {
		return emails
	}
	return warningEmailsForTenant(tenantID)
}

func warningEmailsFromUser(user *model.User) []string {
	if user == nil {
		return nil
	}
	if emails := warningEmailsFromAdditionalInfo(user.AdditionalInfo); len(emails) > 0 {
		return emails
	}
	emails, err := normalizeWarningEmails([]string{user.Email})
	if err != nil {
		return nil
	}
	return emails
}

func userAdditionalInfoMap(raw *string) map[string]interface{} {
	result := map[string]interface{}{}
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return result
	}
	if err := json.Unmarshal([]byte(*raw), &result); err != nil {
		return map[string]interface{}{}
	}
	return result
}

func warningEmailsFromAdditionalInfo(raw *string) []string {
	additional := userAdditionalInfoMap(raw)
	value, ok := additional[userWarningEmailsKey]
	if !ok {
		return nil
	}
	switch items := value.(type) {
	case []interface{}:
		emails := make([]string, 0, len(items))
		for _, item := range items {
			emails = append(emails, fmt.Sprint(item))
		}
		normalized, _ := normalizeWarningEmails(emails)
		return normalized
	case []string:
		normalized, _ := normalizeWarningEmails(items)
		return normalized
	case string:
		normalized, _ := normalizeWarningEmails(strings.Split(items, ","))
		return normalized
	default:
		return nil
	}
}

func normalizeWarningEmails(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, item := range values {
		email := strings.TrimSpace(item)
		if email == "" {
			continue
		}
		address, err := mail.ParseAddress(email)
		if err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "emails contains an invalid email address")
		}
		normalized := strings.ToLower(strings.TrimSpace(address.Address))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result, nil
}
