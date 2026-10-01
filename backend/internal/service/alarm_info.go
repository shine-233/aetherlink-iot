// alarm_info.go 活跃告警（alarm_info）聚合的服务编排：单条/批量处理与分页列表。
// 核心逻辑：写路径先过 ensureAlarmInfoWriteAccess（TENANT_USER 对无设备关联的
// 活跃告警 fail-closed），批量操作校验全部目标同租户后才提交。
// 关键注意事项：处理人固定取 claims.ID；状态变更通过 PublishAlarmEvent 广播给订阅方。
package service

import (
	"context"
	"strings"

	"aetherlink-iot/backend/internal/authz"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

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
		return nil, dbError(err)
	}
	PublishAlarmEvent(context.Background(), alarmInfo.TenantID, map[string]interface{}{
		"type":              "status",
		"alarm_id":          alarmInfo.ID,
		"processing_result": alarmInfo.ProcessingResult,
	})
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
		return dbError(err)
	}
	PublishAlarmEvent(context.Background(), targetTenantID, map[string]interface{}{
		"type":      "status",
		"alarm_ids": req.Id,
	})
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
	allTenants := authz.IsSysAdmin(claims) && strings.TrimSpace(tenantID) == ""
	total, list, err := dal.GetAlarmInfoListByPageForScopes(req, allTenants, alarmListScopes(allTenants, tenantID))
	if err != nil {
		return nil, dbError(err)
	}
	data = make(map[string]interface{})
	data["total"] = total
	data["list"] = list
	return
}
