package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// This contract uses a known, existing device from another tenant rather than
// an invented ID, so it proves the detail read is tenant-scoped instead of
// merely exercising the not-found path.
func TestDeviceDetailRejectsKnownCrossTenantDeviceID(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	deviceID := "tenant-b-known-device"

	if err := db.Create(&model.Device{
		ID:           deviceID,
		DeviceNumber: "tenant-b-known-number",
		Voucher:      "tenant-b-known-voucher",
		TenantID:     "tenant-b",
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
	}).Error; err != nil {
		t.Fatalf("create known cross-tenant device: %v", err)
	}

	detail, err := (&Device{}).GetDeviceByIDV1(deviceID, &utils.UserClaims{
		ID:        "tenant-a-admin",
		TenantID:  "tenant-a",
		Authority: constant.TENANT_ADMIN,
	})

	if detail != nil {
		t.Fatalf("cross-tenant device detail = %#v, want nil", detail)
	}
	assertDeviceConfigServiceError(t, err, "known cross-tenant device detail", errcode.CodeNoPermission, telemetryReadPermissionMessage)

	// Re-read as the owning tenant to prove the fixture exists and the rejection
	// was authorization-shaped rather than an accidental missing-record result.
	ownedDetail, err := (&Device{}).GetDeviceByIDV1(deviceID, &utils.UserClaims{
		ID:        "tenant-b-admin",
		TenantID:  "tenant-b",
		Authority: constant.TENANT_ADMIN,
	})
	if err != nil {
		t.Fatalf("own-tenant device detail returned error: %v", err)
	}
	if ownedDetail["id"] != deviceID {
		t.Fatalf("own-tenant device detail id = %#v, want %q", ownedDetail["id"], deviceID)
	}
}
