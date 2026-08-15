package service

import (
	"fmt"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"
)

// Action 6 cancels a pending, pushed, or upgrading task.
// Action 1 retries a failed task and pushes the OTA package again.
func (o *OTA) UpdateOTAUpgradeTaskStatus(req *model.UpdateOTAUpgradeTaskStatusReq, claims *utils.UserClaims) error {
	taskDetail, err := loadOTAUpgradeTaskDetailForStatusUpdate(req.Id, claims)
	if err != nil {
		return err
	}
	if err := validateOTAUpgradeTaskStatusAction(taskDetail, req.Action); err != nil {
		return err
	}

	switch req.Action {
	case model.OtaUpgradeTaskDetailStatusCanceled:
		return cancelOTAUpgradeTaskDetail(taskDetail)
	case model.OtaUpgradeTaskDetailStatusPending:
		return o.PushOTAUpgradePackage(taskDetail)
	default:
		return nil
	}
}

func loadOTAUpgradeTaskDetailForStatusUpdate(id string, claims *utils.UserClaims) (*model.OtaUpgradeTaskDetail, error) {
	taskDetail, err := query.OtaUpgradeTaskDetail.Where(query.OtaUpgradeTaskDetail.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}
	if _, err := ensureOTATaskAccess(taskDetail.OtaUpgradeTaskID, claims); err != nil {
		return nil, err
	}
	if _, err := ensureTelemetryDeviceWriteAccess(taskDetail.DeviceID, claims); err != nil {
		return nil, err
	}
	return taskDetail, nil
}

func validateOTAUpgradeTaskStatusAction(taskDetail *model.OtaUpgradeTaskDetail, action int16) error {
	if taskDetail.Status == model.OtaUpgradeTaskDetailStatusSucceeded || taskDetail.Status == model.OtaUpgradeTaskDetailStatusCanceled {
		return fmt.Errorf("the task status cannot be modified")
	}
	if action == model.OtaUpgradeTaskDetailStatusCanceled && taskDetail.Status == model.OtaUpgradeTaskDetailStatusFailed {
		return fmt.Errorf("the task status cannot be modified")
	}
	if action == model.OtaUpgradeTaskDetailStatusPending && taskDetail.Status != model.OtaUpgradeTaskDetailStatusFailed {
		return fmt.Errorf("the task is upgrading")
	}
	return nil
}

func cancelOTAUpgradeTaskDetail(taskDetail *model.OtaUpgradeTaskDetail) error {
	description := "MANUALLY_CANCELED"
	affected, err := updateOTAUpgradeTaskDetailIfStatus(
		taskDetail,
		[]int16{
			model.OtaUpgradeTaskDetailStatusPending,
			model.OtaUpgradeTaskDetailStatusPushed,
			model.OtaUpgradeTaskDetailStatusUpgrading,
		},
		map[string]interface{}{
			"status":             model.OtaUpgradeTaskDetailStatusCanceled,
			"status_description": description,
			"updated_at":         time.Now().UTC(),
		},
	)
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("the ota task detail state changed before cancellation")
	}
	taskDetail.Status = model.OtaUpgradeTaskDetailStatusCanceled
	taskDetail.StatusDescription = &description
	return nil
}
