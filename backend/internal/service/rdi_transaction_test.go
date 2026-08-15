package service

import (
	"context"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
	"gorm.io/gorm"
)

func TestUpdateRDIConfigReleasesTransactionAfterPermissionDenial(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	configID := createDeviceServiceConfig(t, db, "rdi-config-rollback", "tenant-a", "1")
	createDeviceServiceOwnedDevice(
		t,
		db,
		"rdi-device-rollback",
		"rdi-device-number-rollback",
		"tenant-a",
		"owner-a",
		configID,
		time.Now().UTC(),
	)

	_, err := GroupApp.RDI.UpdateDeviceConfig(
		context.Background(),
		"rdi-device-rollback",
		&model.UpdateRDIConfigReq{Config: DefaultRDIConfig()},
		&utils.UserClaims{ID: "admin-b", TenantID: "tenant-b", Authority: constant.TENANT_ADMIN},
	)
	assertRDITransactionError(t, err, errcode.CodeNoPermission)
	assertRDITransactionReleased(t, db, "rdi-device-rollback")
}

func TestLockedRDIShareDeviceReleasesTransactionAfterPanic(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	configID := createDeviceServiceConfig(t, db, "rdi-share-config-rollback", "tenant-a", "1")
	createDeviceServiceOwnedDevice(
		t,
		db,
		"rdi-share-device-rollback",
		"rdi-share-number-rollback",
		"tenant-a",
		"owner-a",
		configID,
		time.Now().UTC(),
	)

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("withLockedRDIShareDevice callback did not panic")
			}
		}()
		_, _ = withLockedRDIShareDevice("rdi-share-device-rollback", func(_ *query.QueryTx, _ *model.Device) (*lockedRDIShareResult, error) {
			panic("forced callback failure")
		})
	}()

	assertRDITransactionReleased(t, db, "rdi-share-device-rollback")
}

func assertRDITransactionError(t *testing.T, err error, wantCode int) {
	t.Helper()
	appErr, ok := err.(*errcode.Error)
	if !ok || appErr.Code != wantCode {
		t.Fatalf("RDI transaction error = %#v, want code %d", err, wantCode)
	}
}

func assertRDITransactionReleased(t *testing.T, db *gorm.DB, deviceID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := db.WithContext(ctx).
		Model(&model.Device{}).
		Where("id = ?", deviceID).
		Update("description", "transaction-released")
	if result.Error != nil {
		t.Fatalf("device row remained locked after RDI request returned: %v", result.Error)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("post-RDI update affected %d rows, want 1", result.RowsAffected)
	}
}
