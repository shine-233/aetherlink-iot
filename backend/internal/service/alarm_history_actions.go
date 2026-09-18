package service

import (
	"strings"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func applyAlarmHistoryAction(
	id string,
	claims *utils.UserClaims,
	action func(id, tenantID, operatorID string) (*model.AlarmHistoryActionResp, error),
) (*model.AlarmHistoryActionResp, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "alarm history id is required")
	}
	history, err := ensureAlarmHistoryWriteAccess(id, claims)
	if err != nil {
		return nil, err
	}
	data, err := action(id, history.TenantID, claims.ID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, nil
}

func applyAlarmHistoryActionWithNote(
	id string,
	claims *utils.UserClaims,
	note string,
	action func(id, tenantID, operatorID, note string) (*model.AlarmHistoryActionResp, error),
) (*model.AlarmHistoryActionResp, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "alarm history id is required")
	}
	history, err := ensureAlarmHistoryWriteAccess(id, claims)
	if err != nil {
		return nil, err
	}
	data, err := action(id, history.TenantID, claims.ID, strings.TrimSpace(note))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, nil
}

// ClearAlarmHistory 针对指定告警记录执行清除操作（对齐 ThingsBoard 4.3 告警清除生命周期）。
func (*Alarm) ClearAlarmHistory(id string, note string, claims *utils.UserClaims) (*model.AlarmHistoryActionResp, error) {
	return applyAlarmHistoryActionWithNote(id, claims, note, func(id, tenantID, operatorID, note string) (*model.AlarmHistoryActionResp, error) {
		return dal.ClearAlarmHistory(id, tenantID, operatorID, note)
	})
}

