package service

import (
	"strings"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

type alarmHistoryBatchActionFn func(id, tenantID, operatorID, note string) (*model.AlarmHistoryActionResp, error)
type loadedAlarmHistoryBatchActionFn func(history *model.AlarmHistory, operatorID, note string) (*model.AlarmHistoryActionResp, error)

type alarmHistoryBatchActionPlan struct {
	action      string
	note        string
	ids         []string
	apply       alarmHistoryBatchActionFn
	applyLoaded loadedAlarmHistoryBatchActionFn
}

func buildAlarmHistoryBatchActionPlan(req *model.AlarmHistoryBatchActionReq) (*alarmHistoryBatchActionPlan, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "alarm history batch action request is required")
	}
	action := strings.TrimSpace(req.Action)
	note := ""
	if req.Note != nil {
		note = strings.TrimSpace(*req.Note)
	}
	if len(req.IDs) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "alarm history ids are required")
	}

	var actionFn alarmHistoryBatchActionFn
	var loadedActionFn loadedAlarmHistoryBatchActionFn
	switch action {
	case "acknowledge":
		actionFn = dal.AcknowledgeAlarmHistoryWithNote
		loadedActionFn = dal.AcknowledgeLoadedAlarmHistoryWithNote
	case "reset":
		actionFn = dal.ResetAlarmHistoryWithNote
		loadedActionFn = dal.ResetLoadedAlarmHistoryWithNote
	default:
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "unsupported alarm history action")
	}

	return &alarmHistoryBatchActionPlan{
		action:      action,
		note:        note,
		ids:         req.IDs,
		apply:       actionFn,
		applyLoaded: loadedActionFn,
	}, nil
}

func executeAlarmHistoryBatchActionPlan(
	plan *alarmHistoryBatchActionPlan,
	claims *utils.UserClaims,
) *model.AlarmHistoryBatchActionResp {
	resp := &model.AlarmHistoryBatchActionResp{
		Action:  plan.action,
		Results: make([]model.AlarmHistoryBatchActionItemResp, 0, len(plan.ids)),
	}
	historiesByID, preloadErr := preloadAlarmHistoryBatchActionHistories(plan.ids)

	for _, rawID := range plan.ids {
		id := strings.TrimSpace(rawID)
		item := model.AlarmHistoryBatchActionItemResp{ID: id}
		data, err := applyPreloadedAlarmHistoryBatchAction(id, claims, plan, historiesByID, preloadErr)
		if err != nil {
			item.Error = err.Error()
			resp.FailureCount++
		} else {
			item.OK = true
			item.History = data
			resp.SuccessCount++
		}
		resp.Results = append(resp.Results, item)
	}

	return resp
}

func preloadAlarmHistoryBatchActionHistories(rawIDs []string) (map[string]*model.AlarmHistory, error) {
	ids := make([]string, 0, len(rawIDs))
	seen := make(map[string]struct{}, len(rawIDs))
	for _, rawID := range rawIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	histories, err := dal.GetAlarmHistoriesByIDs(ids)
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	byID := make(map[string]*model.AlarmHistory, len(histories))
	for _, history := range histories {
		if history == nil {
			continue
		}
		byID[history.ID] = history
	}
	return byID, nil
}

func applyPreloadedAlarmHistoryBatchAction(
	id string,
	claims *utils.UserClaims,
	plan *alarmHistoryBatchActionPlan,
	historiesByID map[string]*model.AlarmHistory,
	preloadErr error,
) (*model.AlarmHistoryActionResp, error) {
	if id == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "alarm history id is required")
	}
	if preloadErr != nil {
		return applyAlarmHistoryActionWithNote(id, claims, plan.note, plan.apply)
	}
	history := historiesByID[id]
	if history == nil {
		return applyAlarmHistoryActionWithNote(id, claims, plan.note, plan.apply)
	}
	if err := ensureLoadedAlarmHistoryWriteAccess(history, claims); err != nil {
		return nil, err
	}
	data, err := plan.applyLoaded(history, claims.ID, plan.note)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, nil
}
