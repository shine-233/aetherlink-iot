// alarm.go 负责告警领域的核心服务编排。
//
// 主要职责：
// 1. 维护告警规则、告警记录、告警历史及其租户权限边界。
// 2. 处理告警状态流转、通知分组关联和外部通知副作用。
// 3. 为后端接口、前端告警页、设备详情与 ThingsVis 提供告警相关数据转换。
//
// 静态审查建议：
// 1. 文件职责较重，后续可按“规则配置”“历史查询”“通知投递”逐步拆分辅助函数。
// 2. 权限校验已集中到若干 helper，新增入口时应优先复用，避免出现租户绕过。
// 3. 部分错误消息仍为英文，如需统一文案，建议后续集中整理错误码与提示语映射。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

type Alarm struct{}

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
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}

func ensureAlarmTenantAccess(resourceTenantID string, claims *utils.UserClaims, permissionMessage string) error {
	if err := requireSupportedScopeAuthority(claims, permissionMessage); err != nil {
		return err
	}
	if claims.Authority == constant.SYS_ADMIN || resourceTenantID == claims.TenantID {
		return nil
	}
	return errcode.NewWithMessage(errcode.CodeNoPermission, permissionMessage)
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

// alarm_info is a legacy active-alert table without a device relationship.
// Until the schema records affected devices, TENANT_USER access cannot be
// narrowed to owner_user_id safely and must fail closed instead of exposing
// the whole tenant. Owner-scoped device alarms remain available via history.
func ensureActiveAlarmInfoOwnerScope(claims *utils.UserClaims) error {
	if claims == nil || (claims.Authority != constant.TENANT_ADMIN && claims.Authority != constant.SYS_ADMIN) {
		return errcode.NewWithMessage(errcode.CodeNoPermission, alarmInfoOwnerScopeMessage)
	}
	return nil
}

func ensureAlarmHistoryReadAccess(id string, claims *utils.UserClaims) (*model.AlarmHistory, error) {
	if err := requireSupportedScopeAuthority(claims, alarmHistoryReadPermissionMessage); err != nil {
		return nil, err
	}
	history, err := dal.GetAlarmHistoryByID(id)
	if err != nil {
		return nil, wrapAlarmDBError(err)
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
	if claims.Authority != constant.TENANT_USER {
		if err := ensureAlarmTenantAccess(history.TenantID, claims, alarmHistoryReadPermissionMessage); err != nil {
			return err
		}
		return nil
	}
	if history.TenantID != claims.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, alarmHistoryReadPermissionMessage)
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
		return nil, wrapAlarmDBError(err)
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
	if claims.Authority != constant.TENANT_USER {
		if err := ensureAlarmTenantAccess(history.TenantID, claims, alarmHistoryWritePermissionMessage); err != nil {
			return err
		}
		return nil
	}
	if history.TenantID != claims.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, alarmHistoryWritePermissionMessage)
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
	if claims.Authority == constant.SYS_ADMIN {
		return requestTenantID, nil
	}

	if strings.TrimSpace(claims.TenantID) == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "tenant id is required")
	}
	if requestTenantID != "" && requestTenantID != claims.TenantID {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query alarms for another tenant")
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

func buildAlarmConfigUpdate(req *model.UpdateAlarmConfigReq, oldConfig *model.AlarmConfig, claims *utils.UserClaims) (*model.AlarmConfig, error) {
	data := &model.AlarmConfig{
		ID:        req.ID,
		UpdatedAt: time.Now().UTC(),
		TenantID:  oldConfig.TenantID,
		Remark:    req.Remark,
	}

	if req.Name != nil {
		data.Name = *req.Name
	}
	if req.Description != nil {
		data.Description = req.Description
	}
	if req.AlarmLevel != nil {
		normalizedLevel, err := normalizeAlarmConfigLevel(*req.AlarmLevel)
		if err != nil {
			return nil, err
		}
		data.AlarmLevel = normalizedLevel
	}
	if req.NotificationGroupID != nil {
		data.NotificationGroupID = *req.NotificationGroupID
		if err := validateAlarmNotificationGroupTenant(data.NotificationGroupID, oldConfig.TenantID, claims); err != nil {
			return nil, err
		}
	}
	if req.Enabled != nil {
		data.Enabled = *req.Enabled
	}
	if req.TriggerDuration != nil {
		if err := validateAlarmTriggerDuration(req.TriggerDuration); err != nil {
			return nil, err
		}
		data.TriggerDuration = normalizeAlarmTriggerDuration(req.TriggerDuration)
	} else {
		data.TriggerDuration = oldConfig.TriggerDuration
	}

	return data, nil
}

func executeAlarmNotificationPayload(alarmConfig *model.AlarmConfig, alertData map[string]interface{}) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(alertData); err != nil {
		logrus.Error("encode alarm notification payload failed: ", err)
		return
	}

	alertJSON := strings.TrimSpace(buffer.String())
	GroupApp.NotificationServicesConfig.ExecuteNotification(alarmConfig.NotificationGroupID, alertJSON, alarmConfig.TenantID)
}

func (*Alarm) CreateAlarmConfig(req *model.CreateAlarmConfigReq, claims *utils.UserClaims) (data *model.AlarmConfig, err error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to create alarm config")
	}
	data = &model.AlarmConfig{}
	t := time.Now().UTC()
	data.ID = uuid.New()
	data.Name = req.Name
	data.Description = req.Description
	normalizedLevel, err := normalizeAlarmConfigLevel(req.AlarmLevel)
	if err != nil {
		return nil, err
	}
	data.AlarmLevel = normalizedLevel
	data.NotificationGroupID = req.NotificationGroupID
	if err := validateAlarmNotificationGroupTenant(data.NotificationGroupID, claims.TenantID, claims); err != nil {
		return nil, err
	}
	data.CreatedAt = t
	data.UpdatedAt = t
	data.TenantID = claims.TenantID
	data.Remark = req.Remark
	data.Enabled = req.Enabled
	if err := validateAlarmTriggerDuration(req.TriggerDuration); err != nil {
		return nil, err
	}
	data.TriggerDuration = normalizeAlarmTriggerDuration(req.TriggerDuration)

	err = dal.CreateAlarmConfig(data)
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	return
}

// DeleteAlarmConfig deletes one alarm configuration and clears related caches.
// Existing alarm history is retained when the rule is removed.
func (*Alarm) DeleteAlarmConfig(id string, claims *utils.UserClaims) (err error) {
	if _, err = ensureAlarmConfigWriteAccess(id, claims); err != nil {
		return err
	}
	err = dal.DeleteAlarmConfig(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	_ = dal.DeleteAlarmNameCache(id)
	go func() {
		if err := initialize.NewAlarmCache().DeleteByAlarmId(id); err != nil {
			logrus.Error("DeleteAlarmConfig failed to clear alarm cache: ", err)
		}
	}()
	return
}

// UpdateAlarmConfig updates alarm rule fields after tenant and notification-group checks.
func (*Alarm) UpdateAlarmConfig(req *model.UpdateAlarmConfigReq, claims *utils.UserClaims) (data *model.AlarmConfig, err error) {
	oldConfig, err := ensureAlarmConfigWriteAccess(req.ID, claims)
	if err != nil {
		return nil, err
	}
	data, err = buildAlarmConfigUpdate(req, oldConfig, claims)
	if err != nil {
		return nil, err
	}
	err = dal.UpdateAlarmConfig(data)
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	// 结构体 Updates 会跳过零值，把持续时长显式改回 0（立即触发）时需要补一次单列更新。
	if req.TriggerDuration != nil && data.TriggerDuration == 0 {
		if err := dal.UpdateAlarmConfigTriggerDuration(req.ID, 0); err != nil {
			return nil, wrapAlarmDBError(err)
		}
	}
	go func() {
		if err := dal.DeleteAlarmNameCache(req.ID); err != nil {
			logrus.Error("UpdateAlarmConfig failed to clear alarm cache: ", err)
		}
	}()
	data, err = dal.GetAlarmByID(req.ID)
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	return data, nil
}

// GetAlarmConfigListByPage returns paged alarm rule configuration for the caller tenant scope.
func (*Alarm) GetAlarmConfigListByPage(req *model.GetAlarmConfigListByPageReq, claims *utils.UserClaims) (data map[string]interface{}, err error) {
	tenantID, err := normalizeAlarmListTenantID(req.TenantID, claims)
	if err != nil {
		return nil, err
	}
	req.TenantID = tenantID
	// ROADMAP A1：SYS_ADMIN 未指定租户时显式授权全租户视角，其余空租户在 DAL fail-closed。
	allTenants := claims.Authority == constant.SYS_ADMIN && strings.TrimSpace(tenantID) == ""
	total, list, err := dal.GetAlarmConfigListByPageForScopes(req, allTenants, alarmListScopes(allTenants, tenantID))
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	data = make(map[string]interface{})
	data["total"] = total
	data["list"] = list
	return
}

// UpdateAlarmInfo marks one alarm as processed by the current user.
func (*Alarm) UpdateAlarmInfo(req *model.UpdateAlarmInfoReq, claims *utils.UserClaims) (alarmInfo *model.AlarmInfo, err error) {
	alarmInfo, err = ensureAlarmInfoWriteAccess(req.Id, claims)
	if err != nil {
		return nil, err
	}
	alarmInfo.Processor = &claims.ID
	if req.ProcessingResult != nil && *req.ProcessingResult != "" {
		alarmInfo.ProcessingResult = *req.ProcessingResult
	}
	err = dal.UpdateAlarmInfo(alarmInfo)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return
}

// UpdateAlarmInfoBatch marks multiple alarms as processed after tenant consistency checks.
func (*Alarm) UpdateAlarmInfoBatch(req *model.UpdateAlarmInfoBatchReq, claims *utils.UserClaims) error {
	if len(req.Id) == 0 {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"id": "id is empty",
		})
	}
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify alarm info")
	}
	targetTenantID := ""
	for _, id := range req.Id {
		alarmInfo, err := ensureAlarmInfoWriteAccess(id, claims)
		if err != nil {
			return err
		}
		if targetTenantID == "" {
			targetTenantID = alarmInfo.TenantID
		} else if targetTenantID != alarmInfo.TenantID {
			return errcode.NewWithMessage(errcode.CodeNoPermission, "alarm info batch tenant mismatch")
		}
	}
	err := dal.UpdateAlarmInfoBatch(req, claims.ID, targetTenantID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return err
}

// GetAlarmInfoListByPage returns paged active alarm records for the caller tenant scope.
func (*Alarm) GetAlarmInfoListByPage(req *model.GetAlarmInfoListByPageReq, claims *utils.UserClaims) (data map[string]interface{}, err error) {
	if err := ensureActiveAlarmInfoOwnerScope(claims); err != nil {
		return nil, err
	}
	tenantID, err := normalizeAlarmListTenantID(req.TenantID, claims)
	if err != nil {
		return nil, err
	}
	req.TenantID = tenantID
	// ROADMAP A1：SYS_ADMIN 未指定租户时显式授权全租户视角，其余空租户在 DAL fail-closed。
	allTenants := claims.Authority == constant.SYS_ADMIN && strings.TrimSpace(tenantID) == ""
	total, list, err := dal.GetAlarmInfoListByPageForScopes(req, allTenants, alarmListScopes(allTenants, tenantID))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	data = make(map[string]interface{})
	data["total"] = total
	data["list"] = list
	return
}

// GetAlarmHisttoryListByPage returns paged alarm history for a tenant or permitted device.
func (*Alarm) GetAlarmHisttoryListByPage(req *model.GetAlarmHisttoryListByPage, claims *utils.UserClaims) (data map[string]interface{}, err error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "alarm history request is required")
	}
	if err := validateAlarmHistoryStatus(req); err != nil {
		return nil, err
	}
	if err := validateAlarmHistoryType(req); err != nil {
		return nil, err
	}
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query alarm history")
	}
	if err := requireSystemAdminAllTenantsScope(
		req.AllTenants,
		claims,
		"all-tenants alarm history is only available to system administrators",
	); err != nil {
		return nil, err
	}
	tenantID := claims.TenantID
	if req.DeviceId != nil && strings.TrimSpace(*req.DeviceId) != "" {
		device, err := ensureTelemetryDeviceReadAccess(*req.DeviceId, claims)
		if err != nil {
			return nil, err
		}
		tenantID = device.TenantID
	}
	total, list, err := dal.GetAlarmHistoryListByPageForScopes(req, alarmListScopes(req.AllTenants, tenantID), deviceOwnerUserIDFilterForClaims(claims))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	data = make(map[string]interface{})
	data["total"] = total
	data["list"] = list
	return
}

func validateAlarmHistoryMonthlyTrendYear(year int) error {
	if year < 2000 || year > 2100 {
		return errcode.NewWithMessage(errcode.CodeParamError, "year must be between 2000 and 2100")
	}
	return nil
}

func resolveAlarmHistoryMonthlyTrendLocation(raw string) (*time.Location, string, error) {
	timezone := strings.TrimSpace(raw)
	if timezone == "" {
		timezone = "UTC"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, "", errcode.NewWithMessage(errcode.CodeParamError, "timezone must be a valid IANA time zone")
	}
	return location, timezone, nil
}

// GetAlarmHistoryMonthlyTrend returns twelve stable month buckets for the selected calendar year.
func (*Alarm) GetAlarmHistoryMonthlyTrend(req *model.AlarmHistoryMonthlyTrendReq, claims *utils.UserClaims) (*model.AlarmHistoryMonthlyTrendResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query alarm history")
	}
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "monthly alarm trend request is required")
	}
	if err := requireSystemAdminAllTenantsScope(
		req.AllTenants,
		claims,
		"all-tenants alarm trend is only available to system administrators",
	); err != nil {
		return nil, err
	}
	if err := validateAlarmHistoryMonthlyTrendYear(req.Year); err != nil {
		return nil, err
	}
	location, timezone, err := resolveAlarmHistoryMonthlyTrendLocation(req.Timezone)
	if err != nil {
		return nil, err
	}
	startTime := time.Date(req.Year, time.January, 1, 0, 0, 0, 0, location)
	endTime := startTime.AddDate(1, 0, 0)
	points, err := dal.GetAlarmHistoryMonthlyTrend(
		claims.TenantID,
		deviceOwnerUserIDFilterForClaims(claims),
		startTime.UTC(),
		endTime.UTC(),
		timezone,
		req.AllTenants,
	)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return &model.AlarmHistoryMonthlyTrendResp{
		Year:   req.Year,
		Months: points,
	}, nil
}

// normalizeAlarmConfigLevel 校验并归一告警规则的级别。
// alarm_config.alarm_level 是 varchar(10) 且此前只有 required 校验，任意字符串都能落库；
// 前端 alarmSeverityLabel 在匹配不到选项时会原样回显该值，等于把脏数据带到界面上。
// 这里与 alarm_history 的 H/M/L 口径保持一致（配置侧不接受表示"恢复正常"的 N）。
func normalizeAlarmConfigLevel(alarmLevel string) (string, error) {
	normalized := strings.TrimSpace(alarmLevel)
	switch normalized {
	case "H", "M", "L":
		return normalized, nil
	default:
		return "", errcode.NewWithMessage(errcode.CodeParamError, "unsupported alarm_level")
	}
}

func validateAlarmHistoryStatus(req *model.GetAlarmHisttoryListByPage) error {
	if req == nil || req.AlarmStatus == nil {
		return nil
	}
	alarmStatus := strings.TrimSpace(*req.AlarmStatus)
	*req.AlarmStatus = alarmStatus
	switch alarmStatus {
	case "", "H", "M", "L", "N", model.AlarmHistoryQueryStatusActive:
		return nil
	default:
		return errcode.NewWithMessage(errcode.CodeParamError, "unsupported alarm_status")
	}
}

func validateAlarmHistoryType(req *model.GetAlarmHisttoryListByPage) error {
	if req == nil || req.AlarmType == nil {
		return nil
	}
	alarmType := strings.TrimSpace(*req.AlarmType)
	*req.AlarmType = alarmType
	if alarmType == "" {
		return nil
	}
	switch alarmType {
	case "temperature_alarm", "switch_alarm", "warranty_alarm", "pressure_alarm", "PT":
		return nil
	default:
		return errcode.NewWithMessage(errcode.CodeParamError, "unsupported alarm_type")
	}
}

func (*Alarm) AlarmHistoryDescUpdate(req *model.AlarmHistoryDescUpdateReq, claims *utils.UserClaims) (err error) {
	history, err := ensureAlarmHistoryWriteAccess(req.AlarmHistoryId, claims)
	if err != nil {
		return err
	}
	err = dal.AlarmHistoryDescUpdate(req, history.TenantID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return
}

// AcknowledgeAlarmHistory marks an alarm history row as acknowledged.
func (*Alarm) AcknowledgeAlarmHistory(id string, claims *utils.UserClaims) (*model.AlarmHistoryActionResp, error) {
	return applyAlarmHistoryAction(id, claims, dal.AcknowledgeAlarmHistory)
}

// ResetAlarmHistory clears the acknowledged state for an alarm history row.
func (*Alarm) ResetAlarmHistory(id string, claims *utils.UserClaims) (*model.AlarmHistoryActionResp, error) {
	return applyAlarmHistoryAction(id, claims, dal.ResetAlarmHistory)
}

// BatchAlarmHistoryAction applies acknowledge/reset to selected history rows and reports partial failures.
func (*Alarm) BatchAlarmHistoryAction(req *model.AlarmHistoryBatchActionReq, claims *utils.UserClaims) (*model.AlarmHistoryBatchActionResp, error) {
	plan, err := buildAlarmHistoryBatchActionPlan(req)
	if err != nil {
		return nil, err
	}
	return executeAlarmHistoryBatchActionPlan(plan, claims), nil
}

func (*Alarm) GetDeviceAlarmStatus(req *model.GetDeviceAlarmStatusReq, claims *utils.UserClaims) (bool, error) {
	device, err := ensureTelemetryDeviceReadAccess(req.DeviceId, claims)
	if err != nil {
		return false, err
	}
	active, err := dal.GetDeviceAlarmStatus(req, device.TenantID)
	if err != nil {
		return false, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "get_device_alarm_status",
			"error":     err.Error(),
		})
	}
	return active, nil
}

func (*Alarm) GetConfigByDevice(req *model.GetDeviceAlarmStatusReq, claims *utils.UserClaims) ([]model.AlarmConfig, error) {
	device, err := ensureTelemetryDeviceReadAccess(req.DeviceId, claims)
	if err != nil {
		return nil, err
	}
	data, err := dal.GetConfigByDevice(req, device.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, nil
}

// GetAlarmInfoHistoryByID returns one alarm history detail after access checks.
func (*Alarm) GetAlarmInfoHistoryByID(id string, claims *utils.UserClaims) (map[string]interface{}, error) {
	if _, err := ensureAlarmHistoryReadAccess(id, claims); err != nil {
		return nil, err
	}
	alarmInfo, err := dal.GetAlarmInfoHistoryByID(id, deviceOwnerUserIDFilterForClaims(claims))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return alarmInfo, nil
}

// GetAlarmDeviceCountsByTenant preserves the default tenant/owner-scoped contract.
func (a *Alarm) GetAlarmDeviceCountsByTenant(claims *utils.UserClaims) (*model.AlarmDeviceCountsResponse, error) {
	return a.GetAlarmDeviceCounts(&model.AlarmDeviceCountsReq{}, claims)
}

// GetAlarmDeviceCounts optionally expands the aggregate to every tenant for an
// explicitly authorized system administrator.
func (a *Alarm) GetAlarmDeviceCounts(req *model.AlarmDeviceCountsReq, claims *utils.UserClaims) (*model.AlarmDeviceCountsResponse, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query alarm counts")
	}
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "alarm counts request is required")
	}
	if err := requireSystemAdminAllTenantsScope(
		req.AllTenants,
		claims,
		"all-tenants alarm counts are only available to system administrators",
	); err != nil {
		return nil, err
	}
	ctx := context.Background()
	db := &dal.LatestDeviceAlarmQuery{}
	ownerUserID := deviceOwnerUserIDFilterForClaims(claims)
	totalCount, err := db.CountDevicesByScopeAndStatus(ctx, claims.TenantID, ownerUserID, req.AllTenants)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "count_alarm_devices",
			"error":     err.Error(),
		})
	}
	activeAlarmTotal, err := dal.CountActiveAlarmHistoryByScope(claims.TenantID, ownerUserID, req.AllTenants)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "count_active_alarm_history",
			"error":     err.Error(),
		})
	}
	alarmHistoryTotal, err := dal.CountAlarmHistoryByScope(claims.TenantID, ownerUserID, req.AllTenants)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "count_alarm_history",
			"error":     err.Error(),
		})
	}
	return &model.AlarmDeviceCountsResponse{
		AlarmDeviceTotal:  int64(totalCount),
		ActiveAlarmTotal:  activeAlarmTotal,
		AlarmHistoryTotal: alarmHistoryTotal,
	}, nil
}

// DeleteAlarmHistory keeps the legacy DELETE contract explicit while enforcing
// the customer retention rule that every triggered alarm remains auditable.
// Authorization still runs first so callers cannot use the endpoint to probe
// another tenant's or another owner's alarm-history IDs.
func (*Alarm) DeleteAlarmHistory(id string, claims *utils.UserClaims) error {
	_, err := ensureAlarmHistoryWriteAccess(id, claims)
	if err != nil {
		return err
	}
	return errcode.NewWithMessage(errcode.CodeOpDenied, alarmHistoryRetentionMessage)
}

// alarmListScopes 返回告警列表查询的层级作用域：allTenants(系统管理员全量) 返回 nil，否则 self∪祖先。
func alarmListScopes(allTenants bool, self string) []string {
	if allTenants {
		return nil
	}
	return expandTenantIDScope(self)
}
