// alarm_history.go 告警历史（alarm_history）聚合的服务编排：分页查询、月度趋势、
// 描述更新、确认/重置（单条与批量）、按设备的状态/配置查询与计数聚合。
// 核心逻辑：读路径按 requireSystemAdminAllTenantsScope + alarmListScopes + 设备属主过滤
// 三层钳制；TENANT_USER 的直读/直写必须逐一通过设备写守卫（与列表可见性一致）。
// 关键注意事项：历史不可删除（DeleteAlarmHistory 恒返 CodeOpDenied 保留审计契约）；
// 月度趋势固定返回 12 个月桶，时区必须是合法 IANA 名称。
package service

import (
	"context"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

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
		return nil, dbError(err)
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
		return nil, dbError(err)
	}
	return &model.AlarmHistoryMonthlyTrendResp{
		Year:   req.Year,
		Months: points,
	}, nil
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
		return dbError(err)
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
		return nil, dbError(err)
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
		return nil, dbError(err)
	}
	return alarmInfo, nil
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
