package service

import (
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/initialize"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

func (*OTA) RecordOTAProgress(deviceID string, params map[string]interface{}) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil
	}
	if params == nil {
		params = map[string]interface{}{}
	}

	progressUpdate, ok := resolveOTAProgressUpdate(params)
	if !ok {
		return nil
	}

	taskDetail, err := findActiveOTAUpgradeTaskDetail(deviceID)
	if err != nil {
		return err
	}
	if taskDetail == nil {
		return nil
	}

	if err := applyOTAProgressUpdate(taskDetail, progressUpdate); err != nil {
		return err
	}

	return updateOTACurrentVersionIfCompleted(deviceID, progressUpdate)
}

func findActiveOTAUpgradeTaskDetail(deviceID string) (*model.OtaUpgradeTaskDetail, error) {
	taskDetail, err := query.OtaUpgradeTaskDetail.Where(
		query.OtaUpgradeTaskDetail.DeviceID.Eq(deviceID),
		query.OtaUpgradeTaskDetail.Status.In(
			model.OtaUpgradeTaskDetailStatusPushed,
			model.OtaUpgradeTaskDetailStatusUpgrading,
		),
	).Order(query.OtaUpgradeTaskDetail.UpdatedAt.Desc()).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return taskDetail, nil
}

func applyOTAProgressUpdate(taskDetail *model.OtaUpgradeTaskDetail, progressUpdate otaProgressUpdate) error {
	if !otaProgressUpdateChanged(taskDetail, progressUpdate) {
		return nil
	}
	// A device may repeat an earlier queued/pushed frame after it has already
	// entered upgrading. Never let that regress a persisted rollout row.
	if progressUpdate.hasStatus && progressUpdate.status == model.OtaUpgradeTaskDetailStatusPending {
		return nil
	}

	updates := map[string]interface{}{
		"updated_at": time.Now().UTC(),
	}
	if progressUpdate.hasStatus {
		updates["status"] = progressUpdate.status
	}
	if progressUpdate.hasProgress {
		updates["steps"] = gorm.Expr("GREATEST(COALESCE(steps, 0), ?)", progressUpdate.progress)
	}
	if progressUpdate.description != "" {
		updates["status_description"] = progressUpdate.description
	}

	allowedCurrentStatuses := []int16{
		model.OtaUpgradeTaskDetailStatusPushed,
		model.OtaUpgradeTaskDetailStatusUpgrading,
	}
	if progressUpdate.hasStatus && progressUpdate.status == model.OtaUpgradeTaskDetailStatusPushed {
		allowedCurrentStatuses = []int16{model.OtaUpgradeTaskDetailStatusPushed}
	}
	return global.DB.Model(&model.OtaUpgradeTaskDetail{}).
		Where("id = ? AND status IN ?", taskDetail.ID, allowedCurrentStatuses).
		Updates(updates).
		Error
}

func otaProgressUpdateChanged(taskDetail *model.OtaUpgradeTaskDetail, progressUpdate otaProgressUpdate) bool {
	if progressUpdate.hasStatus && taskDetail.Status != progressUpdate.status {
		return true
	}
	if progressUpdate.hasProgress && (taskDetail.Step == nil || *taskDetail.Step != progressUpdate.progress) {
		return true
	}
	if progressUpdate.description != "" {
		return taskDetail.StatusDescription == nil || *taskDetail.StatusDescription != progressUpdate.description
	}
	return false
}

func updateOTACurrentVersionIfCompleted(deviceID string, progressUpdate otaProgressUpdate) error {
	if progressUpdate.version == "" || !otaProgressReachedCompletion(progressUpdate) {
		return nil
	}
	device, err := query.Device.Select(query.Device.CurrentVersion).Where(query.Device.ID.Eq(deviceID)).First()
	if err != nil {
		return err
	}
	if device.CurrentVersion != nil && strings.TrimSpace(*device.CurrentVersion) == progressUpdate.version {
		return nil
	}
	if _, err := query.Device.Where(query.Device.ID.Eq(deviceID)).Update(query.Device.CurrentVersion, progressUpdate.version); err != nil {
		return err
	}
	initialize.DelDeviceCache(deviceID)
	return nil
}
