// 文件用途：租户告警邮箱（warning_emails）的读写与告警收件人解析。
// 核心逻辑：告警邮箱存于租户管理员（无租户时为当前用户）的 additional_info.warning_emails；
// 告警投递时优先使用单一设备 owner 的告警邮箱，任何不确定情形都回退到租户管理员。
// 关键注意事项：普通账号不得读到租户管理员的收件地址；跨 owner / 跨租户 / 缺失设备一律回退，防止泄露他人设备数据。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"aetherlink-iot/backend/internal/authz"
	"aetherlink-iot/backend/pkg/errcode"

	"gorm.io/gorm"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"
)

const userWarningEmailsKey = "warning_emails"

// hasUserClaimsIdentity 判断 claims 是否携带可用的用户身份。
func hasUserClaimsIdentity(claims *utils.UserClaims) bool {
	return claims != nil && strings.TrimSpace(claims.ID) != ""
}

func (*User) GetWarningEmails(claims *utils.UserClaims) ([]string, error) {
	if !hasUserClaimsIdentity(claims) {
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
	if !hasUserClaimsIdentity(claims) {
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

	if err := persistWarningEmails(ctx, user, emails); err != nil {
		return nil, err
	}
	return emails, nil
}

// mergeWarningEmailsIntoAdditionalInfo 把告警邮箱写入 additional_info，保留其余已有字段。
func mergeWarningEmailsIntoAdditionalInfo(raw *string, emails []string) (string, error) {
	additional := userAdditionalInfoMap(raw)
	additional[userWarningEmailsKey] = emails
	bytes, err := json.Marshal(additional)
	if err != nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}
	return string(bytes), nil
}

func persistWarningEmails(ctx context.Context, user *model.User, emails []string) error {
	additionalInfo, err := mergeWarningEmailsIntoAdditionalInfo(user.AdditionalInfo, emails)
	if err != nil {
		return err
	}
	if _, err := query.User.WithContext(ctx).Where(query.User.ID.Eq(user.ID)).Update(query.User.AdditionalInfo, additionalInfo); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "update_warning_emails",
			"user_id":   user.ID,
			"error":     err.Error(),
		})
	}
	return nil
}

func loadWarningEmailOwnerUser(claims *utils.UserClaims) (*model.User, error) {
	if !hasUserClaimsIdentity(claims) {
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
	// 角色门禁收编 authz：SYS_ADMIN/TENANT_ADMIN 即 authz.ManagerRoles，
	// 其余（含 TENANT_USER 与未知 authority）fail-closed，与迁移前 switch 等价。
	return authz.HasRole(claims, authz.ManagerRoles...)
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
	normalizedDeviceIDs := normalizeWarningEmailDeviceIDs(deviceIDs)
	if tenantID == "" || len(normalizedDeviceIDs) == 0 {
		return warningEmailsForTenant(tenantID)
	}

	devicesByID, err := dal.GetDevicesByIDsUnscoped(normalizedDeviceIDs)
	if err != nil {
		return warningEmailsForTenant(tenantID)
	}
	ownerUserID, ok := resolveSingleDeviceOwnerUserID(tenantID, normalizedDeviceIDs, devicesByID)
	if !ok {
		return warningEmailsForTenant(tenantID)
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

// normalizeWarningEmailDeviceIDs 去空白、去空值并按首次出现顺序去重。
func normalizeWarningEmailDeviceIDs(deviceIDs []string) []string {
	normalized := make([]string, 0, len(deviceIDs))
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, rawDeviceID := range deviceIDs {
		deviceID := strings.TrimSpace(rawDeviceID)
		if deviceID == "" {
			continue
		}
		if _, exists := seen[deviceID]; exists {
			continue
		}
		seen[deviceID] = struct{}{}
		normalized = append(normalized, deviceID)
	}
	return normalized
}

// resolveSingleDeviceOwnerUserID 仅当所有设备都存在、属于 tenantID 且归属同一个非空 owner 时返回该 owner。
// 任何缺失、跨租户、无 owner 或多 owner 情形返回 ok=false，调用方必须回退到租户管理员。
func resolveSingleDeviceOwnerUserID(tenantID string, deviceIDs []string, devicesByID map[string]*model.Device) (string, bool) {
	if len(deviceIDs) == 0 || len(devicesByID) != len(deviceIDs) {
		return "", false
	}
	ownerUserID := ""
	for _, deviceID := range deviceIDs {
		device := devicesByID[deviceID]
		if device == nil || strings.TrimSpace(device.TenantID) != tenantID || device.OwnerUserID == nil {
			return "", false
		}
		currentOwnerUserID := strings.TrimSpace(*device.OwnerUserID)
		if currentOwnerUserID == "" {
			return "", false
		}
		if ownerUserID == "" {
			ownerUserID = currentOwnerUserID
			continue
		}
		if ownerUserID != currentOwnerUserID {
			return "", false
		}
	}
	return ownerUserID, true
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
	value, ok := userAdditionalInfoMap(raw)[userWarningEmailsKey]
	if !ok {
		return nil
	}
	var candidates []string
	switch items := value.(type) {
	case []interface{}:
		candidates = make([]string, 0, len(items))
		for _, item := range items {
			candidates = append(candidates, fmt.Sprint(item))
		}
	case []string:
		candidates = items
	case string:
		candidates = strings.Split(items, ",")
	default:
		return nil
	}
	// 历史数据中的非法地址整体视为“未配置”，由调用方回退到注册邮箱。
	normalized, _ := normalizeWarningEmails(candidates)
	return normalized
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
