package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/assert"
)

func TestDeviceDeleteDeviceCleansOTATaskDetailsBeforeDeviceRow(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	deviceID := "device-delete-ota-detail"
	tenantID := "tenant-a"
	taskID := "ota-delete-task"
	detailID := "ota-delete-detail"

	if err := db.Create(&model.Device{
		ID:           deviceID,
		DeviceNumber: "delete-ota-detail-number",
		Voucher:      "delete-ota-detail-voucher",
		TenantID:     tenantID,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&model.OtaUpgradeTask{
		ID:                  taskID,
		Name:                "delete device OTA task",
		OtaUpgradePackageID: "ota-package",
		CreatedAt:           now,
	}).Error; err != nil {
		t.Fatalf("create OTA task: %v", err)
	}
	if err := db.Create(&model.OtaUpgradeTaskDetail{
		ID:               detailID,
		OtaUpgradeTaskID: taskID,
		DeviceID:         deviceID,
		Status:           model.OtaUpgradeTaskDetailStatusPending,
		UpdatedAt:        &now,
	}).Error; err != nil {
		t.Fatalf("create OTA task detail: %v", err)
	}

	err := (&Device{}).DeleteDevice(deviceID, &utils.UserClaims{TenantID: tenantID})

	assert.NoError(t, err)
	assertDeviceServiceRowCount(t, db, &model.OtaUpgradeTaskDetail{}, "id = ?", detailID, 0)
	assertDeviceServiceRowCount(t, db, &model.Device{}, "id = ?", deviceID, 0)
}
