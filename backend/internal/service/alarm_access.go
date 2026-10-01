// alarm_access.go 告警域的访问守卫与租户归一：alarm_config / alarm_info / alarm_history
// 三类聚合共用的加载+校验 helper、错误包装与列表作用域解析。
// 核心逻辑：先走 requireSupportedScopeAuthority / ensureActiveAlarmInfoOwnerScope 等
// 既有角色门禁，再按聚合的租户（或设备属主）规则判定；not-found 掩码与错误码逐字保留。
// 关键注意事项：alarmHistoryDeviceIDsForAccess 解析失败按无权限 fail-closed；
// alarmListScopes 的 allTenants(nil) 与 expandTenantIDScope(self∪子孙) 语义是 DAL 契约，禁止顺手清理。
package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"aetherlink-iot/backend/internal/authz"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

const (
	alarmConfigWritePermissionMessage  = "no permission to modify alarm config"
	alarmInfoWritePermissionMessage    = "no permission to modify alarm info"
	alarmInfoOwnerScopeMessage         = "active alarm info has no device ownership scope; use owner-scoped alarm history"
	alarmHistoryReadPermissionMessage  = "no permission to query alarm history"
	alarmHistoryWritePermissionMessage = "no permission to modify alarm history"
	alarmHistoryRetentionMessage       = "alarm history is retained for audit and cannot be deleted"
	alarmListReadPermissionMessage     = "no permission to query alarms"
)

// wrapAlarmDBError 将底层数据库异常统一包装成带 SQL 上下文的业务错误。
func wrapAlarmDBError(err error) error {
	return dbError(err)
}

// wrapAlarmHistoryLoadError 区分"查不到这条告警历史"与"数据库出错"。
// 前者是 404：当成 101001 会让客户端把不存在的 id 判成系统故障并重试，
// 同时把 ORM 的 "record not found" 原文带进响应体。
func wrapAlarmHistoryLoadError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.NewWithMessage(errcode.CodeNotFound, "alarm history not found")
	}
	return wrapAlarmDBError(err)
}

func ensureAlarmConfigWriteAccess(id string, claims *utils.UserClaims) (*model.AlarmConfig, error) {
	alarmConfig, err := dal.GetAlarmByID(id)
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	if err := ensureAlarmTenantAccess(alarmConfig.TenantID, claims, alarmConfigWritePermissionMessage); err != nil {
		return nil, err
	}
	return alarmConfig, nil
}

func ensureAlarmInfoWriteAccess(id string, claims *utils.UserClaims) (*model.AlarmInfo, error) {
	if err := ensureActiveAlarmInfoOwnerScope(claims); err != nil {
		return nil, err
	}
	alarmInfo, err := dal.GetAlarmInfoByID(id)
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	if err := ensureAlarmTenantAccess(alarmInfo.TenantID, claims, alarmInfoWritePermissionMessage); err != nil {
		return nil, err
	}
	return alarmInfo, nil
}

func ensureAlarmHistoryReadAccess(id string, claims *utils.UserClaims) (*model.AlarmHistory, error) {
	if err := requireSupportedScopeAuthority(claims, alarmHistoryReadPermissionMessage); err != nil {
		return nil, err
	}
	history, err := dal.GetAlarmHistoryByID(id)
	if err != nil {
		return nil, wrapAlarmHistoryLoadError(err)
	}
	if err := ensureLoadedAlarmHistoryReadAccess(history, claims); err != nil {
		return nil, err
	}
	return history, nil
}

func ensureLoadedAlarmHistoryReadAccess(history *model.AlarmHistory, claims *utils.UserClaims) error {
	if err := requireSupportedScopeAuthority(claims, alarmHistoryReadPermissionMessage); err != nil {
		return err
	}
	if history == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, alarmHistoryReadPermissionMessage)
	}
	if !authz.HasRole(claims, authz.TenantUser) {
		if err := ensureAlarmTenantAccess(history.TenantID, claims, alarmHistoryReadPermissionMessage); err != nil {
			return err
		}
		return nil
	}
	if err := authz.CheckTenant(claims, history.TenantID, alarmHistoryReadPermissionMessage); err != nil {
		return err
	}
	for _, deviceID := range alarmHistoryDeviceIDsForAccess(history.AlarmDeviceList) {
		// Alarm-history lists are owner-scoped for tenant users. Reuse the device
		// owner-only guard here instead of the shared-read guard so direct ID reads
		// cannot reveal rows that the same user would not see in the list.
		if _, err := ensureTelemetryDeviceWriteAccess(deviceID, claims); err == nil {
			return nil
		}
	}
	return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query alarm history")
}

// alarmDeviceListLogPreview 截取设备列表原始 JSON 的前 64 字节，用于日志定位脏数据。
func alarmDeviceListLogPreview(raw string) string {
	if len(raw) > 64 {
		return raw[:64]
	}
	return raw
}

func alarmHistoryDeviceIDsForAccess(raw string) []string {
	var ids []string
	if strings.TrimSpace(raw) == "" {
		return ids
	}
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		// 解析失败会得到空设备列表并按无权限处理（fail-closed），
		// 这里记录错误和原始片段，便于定位无法授权的历史记录。
		logrus.Warnf("alarm history device list 解析失败: err=%v raw_prefix=%q", err, alarmDeviceListLogPreview(raw))
	}
	return ids
}

func ensureAlarmHistoryWriteAccess(id string, claims *utils.UserClaims) (*model.AlarmHistory, error) {
	if err := requireSupportedScopeAuthority(claims, alarmHistoryWritePermissionMessage); err != nil {
		return nil, err
	}
	history, err := dal.GetAlarmHistoryByID(id)
	if err != nil {
		return nil, wrapAlarmHistoryLoadError(err)
	}
	if err := ensureLoadedAlarmHistoryWriteAccess(history, claims); err != nil {
		return nil, err
	}
	return history, nil
}

func ensureLoadedAlarmHistoryWriteAccess(history *model.AlarmHistory, claims *utils.UserClaims) error {
	if err := requireSupportedScopeAuthority(claims, alarmHistoryWritePermissionMessage); err != nil {
		return err
	}
	if history == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, alarmHistoryWritePermissionMessage)
	}
	if !authz.HasRole(claims, authz.TenantUser) {
		if err := ensureAlarmTenantAccess(history.TenantID, claims, alarmHistoryWritePermissionMessage); err != nil {
			return err
		}
		return nil
	}
	if err := authz.CheckTenant(claims, history.TenantID, alarmHistoryWritePermissionMessage); err != nil {
		return err
	}
	deviceIDs := alarmHistoryDeviceIDsForAccess(history.AlarmDeviceList)
	if len(deviceIDs) == 0 {
		return errcode.NewWithMessage(errcode.CodeNoPermission, alarmHistoryWritePermissionMessage)
	}
	for _, deviceID := range deviceIDs {
		if _, err := ensureTelemetryDeviceWriteAccess(deviceID, claims); err != nil {
			return errcode.NewWithMessage(errcode.CodeNoPermission, alarmHistoryWritePermissionMessage)
		}
	}
	return nil
}

func normalizeAlarmListTenantID(requestTenantID string, claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, alarmListReadPermissionMessage)
	}

	requestTenantID = strings.TrimSpace(requestTenantID)
	if authz.IsSysAdmin(claims) {
		return requestTenantID, nil
	}

	if strings.TrimSpace(claims.TenantID) == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "tenant id is required")
	}
	if requestTenantID != "" {
		if err := authz.CheckTenant(claims, requestTenantID, "no permission to query alarms for another tenant"); err != nil {
			return "", err
		}
	}
	return claims.TenantID, nil
}

func validateAlarmNotificationGroupTenant(notificationGroupID, tenantID string, claims *utils.UserClaims) error {
	if notificationGroupID == "" {
		return nil
	}
	notificationGroup, err := ensureNotificationGroupReadAccess(notificationGroupID, claims)
	if err != nil {
		return err
	}
	if notificationGroup.TenantID != tenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "alarm config and notification group tenant mismatch")
	}
	return nil
}

// alarmListScopes 返回告警列表查询的层级作用域：allTenants(系统管理员全量) 返回 nil，否则 self∪子孙（自上而下）。
func alarmListScopes(allTenants bool, self string) []string {
	if allTenants {
		return nil
	}
	return expandTenantIDScope(self)
}
