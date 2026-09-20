// 文件用途：预注册清理分流的定向证据（ROADMAP P0.5）。
// 覆盖：跨租户 fail closed、已激活设备不可删、空批次清理幂等不报错、混合集合正确分流。
// 说明：本文件为纯逻辑证据，不依赖数据库。
// 导出面（Excel + MaskVoucher 脱敏）的用例在 device_preregister_export_test.go，不在本文件。
package service

import (
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
)

// newPreRegisterCleanupFixture 造一台预注册设备；激活态与租户可按用例调整。
func newPreRegisterCleanupFixture(id, number, tenantID, activateFlag string) *model.Device {
	name := "fixture-" + number
	createdAt := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	return &model.Device{
		ID:           id,
		DeviceNumber: number,
		Name:         &name,
		Voucher:      `{"username":"fixture-` + number + `"}`,
		TenantID:     tenantID,
		ActivateFlag: activateFlag,
		CreatedAt:    &createdAt,
	}
}

func TestClassifyPreRegisterCleanupRejectsCrossTenant(t *testing.T) {
	devices := []*model.Device{
		newPreRegisterCleanupFixture("d1", "PR-A", "tenant-other", preRegisterActivateFlag),
	}
	plan, err := classifyPreRegisterCleanup(devices, "tenant-1")
	if err == nil {
		t.Fatalf("cross-tenant device must fail closed, got plan %+v", plan)
	}
	if plan != nil {
		t.Fatalf("plan must be nil on cross-tenant rejection, got %+v", plan)
	}
}

func TestClassifyPreRegisterCleanupBlocksActivatedDevices(t *testing.T) {
	devices := []*model.Device{
		newPreRegisterCleanupFixture("d1", "PR-A", "t1", preRegisterActivateFlag),
		newPreRegisterCleanupFixture("d2", "PR-B", "t1", "active"),
	}
	plan, err := classifyPreRegisterCleanup(devices, "t1")
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(plan.deletable) != 1 || plan.deletable[0] != "d1" {
		t.Fatalf("deletable = %v, want [d1]", plan.deletable)
	}
	if len(plan.blockedActivated) != 1 || plan.blockedActivated[0] != "PR-B" {
		t.Fatalf("blockedActivated = %v, want [PR-B]", plan.blockedActivated)
	}
}

func TestClassifyPreRegisterCleanupEmptyIsIdempotent(t *testing.T) {
	plan, err := classifyPreRegisterCleanup(nil, "t1")
	if err != nil {
		t.Fatalf("cleaning an empty batch must not error: %v", err)
	}
	if !plan.isEmpty() {
		t.Fatalf("empty batch plan should be empty: %+v", plan)
	}
}

func TestClassifyPreRegisterCleanupAllActivatedDeletesNothing(t *testing.T) {
	devices := []*model.Device{
		newPreRegisterCleanupFixture("d1", "PR-A", "t1", "active"),
		newPreRegisterCleanupFixture("d2", "PR-B", "t1", "active"),
	}
	plan, err := classifyPreRegisterCleanup(devices, "t1")
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(plan.deletable) != 0 {
		t.Fatalf("activated devices must never be deletable, got %v", plan.deletable)
	}
	if len(plan.blockedActivated) != 2 {
		t.Fatalf("blockedActivated = %v, want 2 entries", plan.blockedActivated)
	}
}
