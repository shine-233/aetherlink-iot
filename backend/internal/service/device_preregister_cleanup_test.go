package service

import (
	"errors"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/utils"
)

// P0.5 清理执行面证据：classifyPreRegisterCleanup 只做分流，
// 此前没有任何地方真正执行删除，也没有 API 端点，
// "清理"是一个永远不会被执行的分流计划。

func cleanupDevice(id, tenantID, activateFlag string) *model.Device {
	return &model.Device{ID: id, TenantID: tenantID, ActivateFlag: activateFlag, DeviceNumber: "n-" + id}
}

func withCleanupOps(devices []*model.Device, removeErr error, deleteFn func(ids []string, tenantID string) (int64, error)) (*PreRegisterCleanupResult, []string, error) {
	original := preRegisterCleanupOps
	defer func() { preRegisterCleanupOps = original }()
	preRegisterCleanupOps.load = func(model.ExportPreRegisterReq, string) ([]*model.Device, error) {
		return devices, nil
	}
	removed := []string{}
	if deleteFn != nil {
		preRegisterCleanupOps.remove = deleteFn
	} else {
		preRegisterCleanupOps.remove = func(ids []string, _ string) (int64, error) {
			if removeErr != nil {
				return 0, removeErr
			}
			removed = append(removed, ids...)
			return int64(len(ids)), nil
		}
	}
	device := &Device{}
	result, err := device.CleanupDevicePreRegister(
		model.ExportPreRegisterReq{ProductID: "p1"},
		&utils.UserClaims{TenantID: "tenant-hq"},
	)
	return result, removed, err
}

// 已激活设备绝不进入删除集合：清理批次时误删在运设备是不可接受的。
func TestCleanupDevicePreRegisterNeverDeletesActivated(t *testing.T) {
	devices := []*model.Device{
		cleanupDevice("d1", "tenant-hq", "inactive"),
		cleanupDevice("d2", "tenant-hq", "active"),
	}
	result, removed, err := withCleanupOps(devices, nil, nil)
	if err != nil {
		t.Fatalf("cleanup error = %v", err)
	}
	if len(removed) != 1 || removed[0] != "d1" {
		t.Fatalf("removed = %v, want only d1 (inactive)", removed)
	}
	if result.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", result.Deleted)
	}
	if len(result.BlockedActivated) != 1 {
		t.Fatalf("blocked_activated = %v, want d2 reported", result.BlockedActivated)
	}
	if result.Scanned != 2 {
		t.Fatalf("scanned = %d, want 2", result.Scanned)
	}
}

// 跨租户数据是安全事件：必须 fail closed，且不能已经删了再说。
func TestCleanupDevicePreRegisterFailsClosedOnCrossTenant(t *testing.T) {
	devices := []*model.Device{
		cleanupDevice("d1", "tenant-hq", "inactive"),
		cleanupDevice("d2", "tenant-other", "inactive"),
	}
	deletedAny := false
	result, _, err := withCleanupOps(devices, nil, func(ids []string, _ string) (int64, error) {
		deletedAny = true
		return int64(len(ids)), nil
	})
	if err == nil {
		t.Fatal("cross-tenant device must fail the cleanup, not be silently filtered")
	}
	if deletedAny {
		t.Fatal("nothing may be deleted when the set contains cross-tenant data")
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil on failure", result)
	}
}

// 已清空批次重复清理必须幂等：返回 0，不报错。
func TestCleanupDevicePreRegisterEmptyIsIdempotent(t *testing.T) {
	result, removed, err := withCleanupOps(nil, nil, nil)
	if err != nil {
		t.Fatalf("cleanup of an empty batch must not error; got %v", err)
	}
	if result == nil || result.Deleted != 0 {
		t.Fatalf("result = %+v, want deleted=0", result)
	}
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none", removed)
	}
}

// 全部已激活：一个都不删，但要如实报告被拦截的设备。
func TestCleanupDevicePreRegisterAllActivatedDeletesNothing(t *testing.T) {
	devices := []*model.Device{cleanupDevice("d1", "tenant-hq", "active")}
	result, removed, err := withCleanupOps(devices, nil, nil)
	if err != nil {
		t.Fatalf("cleanup error = %v", err)
	}
	if len(removed) != 0 || result.Deleted != 0 {
		t.Fatalf("activated devices must not be deleted; removed=%v deleted=%d", removed, result.Deleted)
	}
	if len(result.BlockedActivated) != 1 {
		t.Fatalf("blocked_activated = %v, want d1", result.BlockedActivated)
	}
}

// 删除失败必须上抛，不能假装清理成功。
func TestCleanupDevicePreRegisterPropagatesDeleteFailure(t *testing.T) {
	devices := []*model.Device{cleanupDevice("d1", "tenant-hq", "inactive")}
	result, _, err := withCleanupOps(devices, errors.New("db down"), nil)
	if err == nil {
		t.Fatal("delete failure must surface")
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil when deletion failed", result)
	}
}

// 租户上下文缺失时绝不发起删除。
func TestCleanupDevicePreRegisterRequiresTenantAndProduct(t *testing.T) {
	device := &Device{}
	if _, err := device.CleanupDevicePreRegister(model.ExportPreRegisterReq{ProductID: "p1"}, nil); err == nil {
		t.Fatal("missing claims must be rejected")
	}
	if _, err := device.CleanupDevicePreRegister(model.ExportPreRegisterReq{ProductID: "p1"}, &utils.UserClaims{}); err == nil {
		t.Fatal("empty tenant must be rejected")
	}
	if _, err := device.CleanupDevicePreRegister(model.ExportPreRegisterReq{}, &utils.UserClaims{TenantID: "t"}); err == nil {
		t.Fatal("missing product_id must be rejected")
	}
}
