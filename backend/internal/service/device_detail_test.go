// 文件用途：锁定设备详情读面的空快照错误映射行为。
// 核心逻辑：dal.GetDeviceDetail 的 Scan 对零行结果不报错、返回缺 id 键的空 map；
// loadDeviceDetail 必须把这种空快照显式映射为资源不存在业务错误，
// 杜绝"HTTP 200 空壳 data"被前端渲染成空白详情页（gen 继承链读到陈旧快照时同样触发）。
// 关键注意事项：错误码迁移批次（2026-08，承接 PR #123 回滚后的正式迁移）；
// 业务码契约与遥测访问守卫保持一致（100404 device not found）。
// 重构建议：若 GetDeviceDetail 后续改为显式返回 not-found 错误，可移除本处的空 map 判断。
package service

import (
	"testing"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/errcode"
)

func TestLoadDeviceDetailMapsEmptySnapshotToRecordNotFound(t *testing.T) {
	setupDeviceServiceTestDB(t)

	// 前置事实：不播种任何设备时，GetDeviceDetail 的 Scan 不报错，只返回缺 id 键的空 map。
	snapshot, err := dal.GetDeviceDetail("ghost-device")
	if err != nil {
		t.Fatalf("GetDeviceDetail should scan zero rows without error: %v", err)
	}
	if _, ok := snapshot["id"]; ok {
		t.Fatal("precondition broken: expected empty snapshot without id key")
	}

	data, err := loadDeviceDetail("ghost-device")
	if err == nil {
		t.Fatalf("loadDeviceDetail must reject empty snapshot, got HTTP 200 空壳 data: %#v", data)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("loadDeviceDetail error type = %T, want *errcode.Error", err)
	}
	if appErr.Code != errcode.CodeNotFound || appErr.CustomMsg != "device not found" {
		t.Fatalf("loadDeviceDetail error = code %d message %q, want code %d message %q",
			appErr.Code, appErr.CustomMsg, errcode.CodeNotFound, "device not found")
	}
	if data != nil {
		t.Fatalf("loadDeviceDetail should return nil data on empty snapshot, got %#v", data)
	}
}

func TestLoadDeviceDetailKeepsExistingSnapshotUnchanged(t *testing.T) {
	db := setupDeviceServiceTestDB(t)

	// 对照：正常快照仍原样透传，id 键齐全，不受空快照判断影响。
	createDeviceServiceDevice(t, db, "detail-existing", "detail-existing-number", "tenant-a", "", time.Now().UTC())

	data, err := loadDeviceDetail("detail-existing")
	if err != nil {
		t.Fatalf("loadDeviceDetail returned error for existing device: %v", err)
	}
	if data == nil || data["id"] != "detail-existing" {
		t.Fatalf("loadDeviceDetail snapshot broken: %#v", data)
	}
}
