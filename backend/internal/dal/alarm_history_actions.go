package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func acknowledgeAlarmHistory(id, tenantID, userID, note string) (*model.AlarmHistoryActionResp, error) {
	history, err := actionAlarmHistoryForAction(id, tenantID)
	if err != nil {
		return nil, err
	}
	return acknowledgeLoadedAlarmHistory(history, tenantID, userID, note)
}

func AcknowledgeLoadedAlarmHistoryWithNote(history *model.AlarmHistory, userID, note string) (*model.AlarmHistoryActionResp, error) {
	if history == nil {
		return nil, errors.New("alarm history is required")
	}
	return acknowledgeLoadedAlarmHistory(history, history.TenantID, userID, note)
}

func acknowledgeLoadedAlarmHistory(history *model.AlarmHistory, tenantID, userID, note string) (*model.AlarmHistoryActionResp, error) {
	ackAt := time.Now().UTC().Format(time.RFC3339)
	remark := actionAlarmHistoryAcknowledgeRemark(history.Remark, userID, ackAt, note)
	if err := actionUpdateAlarmHistoryRemark(history.ID, tenantID, remark); err != nil {
		return nil, err
	}
	history.Remark = &remark

	resp := &model.AlarmHistoryActionResp{
		ID:             history.ID,
		AlarmStatus:    history.AlarmStatus,
		Remark:         &remark,
		AcknowledgedBy: &userID,
		AcknowledgedAt: &ackAt,
	}
	if strings.TrimSpace(note) != "" {
		resp.ActionNote = &note
	}
	return resp, nil
}

func resetAlarmHistory(id, tenantID, userID, note string) (*model.AlarmHistoryActionResp, error) {
	history, err := actionAlarmHistoryForAction(id, tenantID)
	if err != nil {
		return nil, err
	}
	return resetLoadedAlarmHistory(history, tenantID, userID, note)
}

func ResetLoadedAlarmHistoryWithNote(history *model.AlarmHistory, userID, note string) (*model.AlarmHistoryActionResp, error) {
	if history == nil {
		return nil, errors.New("alarm history is required")
	}
	return resetLoadedAlarmHistory(history, history.TenantID, userID, note)
}

func resetLoadedAlarmHistory(history *model.AlarmHistory, tenantID, userID, note string) (*model.AlarmHistoryActionResp, error) {
	resetFromStatus := strings.ToUpper(strings.TrimSpace(history.AlarmStatus))
	if resetFromStatus != "H" && resetFromStatus != "M" && resetFromStatus != "L" {
		return nil, errors.New("only active alarm history can be reset")
	}
	resetAt := time.Now().UTC().Format(time.RFC3339)
	remark := actionAlarmHistoryResetRemark(history.Remark, userID, resetAt, resetFromStatus, note)
	if err := actionApplyAlarmHistoryReset(history.ID, tenantID, remark); err != nil {
		return nil, err
	}
	history.AlarmStatus = "N"
	history.Remark = &remark

	resp := &model.AlarmHistoryActionResp{
		ID:          history.ID,
		AlarmStatus: "N",
		Remark:      &remark,
		ResetBy:     &userID,
		ResetAt:     &resetAt,
	}
	if strings.TrimSpace(note) != "" {
		resp.ActionNote = &note
	}
	return resp, nil
}

func actionAlarmHistoryForAction(id, tenantID string) (*model.AlarmHistory, error) {
	return actionAlarmHistoryRecordByID(id, tenantID).First()
}

func actionAlarmHistoryRecordByID(id, tenantID string) query.IAlarmHistoryDo {
	return query.AlarmHistory.Where(
		query.AlarmHistory.ID.Eq(id),
		query.AlarmHistory.TenantID.Eq(tenantID),
	)
}

func actionUpdateAlarmHistoryRemark(id, tenantID, remark string) error {
	result, err := actionAlarmHistoryRecordByID(id, tenantID).
		UpdateColumn(query.AlarmHistory.Remark, remark)
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errors.New("acknowledge alarm failed")
	}
	return nil
}

func actionApplyAlarmHistoryReset(id, tenantID, remark string) error {
	result, err := actionAlarmHistoryRecordByID(id, tenantID).
		Where(query.AlarmHistory.AlarmStatus.In("H", "M", "L")).
		Updates(actionAlarmHistoryResetUpdates(remark))
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errors.New("reset alarm failed")
	}
	return nil
}

func actionAlarmHistoryAcknowledgeRemark(raw *string, userID, ackAt, note string) string {
	fields := map[string]interface{}{
		"acknowledged":    true,
		"acknowledged_by": userID,
		"acknowledged_at": ackAt,
	}
	if strings.TrimSpace(note) != "" {
		fields["acknowledge_note"] = note
	}
	return actionMergeAlarmHistoryRemark(raw, fields)
}

func actionAlarmHistoryResetRemark(raw *string, userID, resetAt, resetFromStatus, note string) string {
	fields := map[string]interface{}{
		"reset":             true,
		"reset_by":          userID,
		"reset_at":          resetAt,
		"reset_from_status": resetFromStatus,
	}
	if strings.TrimSpace(note) != "" {
		fields["reset_note"] = note
	}
	return actionMergeAlarmHistoryRemark(raw, fields)
}

func actionAlarmHistoryResetUpdates(remark string) map[string]interface{} {
	return map[string]interface{}{
		"alarm_status": "N",
		"remark":       remark,
	}
}

func actionMergeAlarmHistoryRemark(raw *string, fields map[string]interface{}) string {
	remark := make(map[string]interface{})
	if raw != nil && strings.TrimSpace(*raw) != "" {
		if err := json.Unmarshal([]byte(*raw), &remark); err != nil {
			remark["previous_remark"] = *raw
		}
	}
	for key, value := range fields {
		remark[key] = value
	}
	bytes, err := json.Marshal(remark)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}
