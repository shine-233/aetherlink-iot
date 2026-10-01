// 文件用途: 锁定 GetDeviceDetail 的父设备（网关）名称折叠查询契约。
// 核心逻辑: 子设备（parent_id 非空）必须一次往返内带出 gateway_device_name；
// 根设备（parent_id 为空）必须完全不带该键，保持与折叠前一致的"键缺省"语义。
// 关键注意事项: 悬空 parent_id（父设备已被删除）走 LEFT JOIN 后应得到 NULL 结果而不是
// 整体报错——这是本次收敛 N+1 查询时刻意引入的加固行为，需要显式覆盖，不能漏测。
// 重构建议: 若后续 Device 自连接别名策略变化，优先核对本文件的裸 SQL 字符串匹配是否仍适用。
package dal

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestGetDeviceDetailJoinsParentDeviceNameForChildDevice(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	now := time.Now().UTC()

	parent := &model.Device{
		ID:           "gateway-parent",
		Voucher:      `{"username":"gateway-parent"}`,
		TenantID:     "tenant-1",
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
		DeviceNumber: "gateway-parent",
		Name:         stringPtr("Gateway One"),
	}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent device: %v", err)
	}

	parentID := "gateway-parent"
	child := &model.Device{
		ID:           "child-device",
		Voucher:      `{"username":"child-device"}`,
		TenantID:     "tenant-1",
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
		DeviceNumber: "child-device",
		Name:         stringPtr("Child One"),
		ParentID:     &parentID,
	}
	if err := db.Create(child).Error; err != nil {
		t.Fatalf("create child device: %v", err)
	}

	data, err := GetDeviceDetail("child-device")
	if err != nil {
		t.Fatalf("GetDeviceDetail returned error: %v", err)
	}
	gatewayName, ok := data["gateway_device_name"]
	if !ok {
		t.Fatalf("expected gateway_device_name key present for child device, got %#v", data)
	}
	if gatewayName != "Gateway One" {
		t.Fatalf("gateway_device_name = %#v, want %q", gatewayName, "Gateway One")
	}
}

func TestGetDeviceDetailOmitsGatewayNameForRootDevice(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	now := time.Now().UTC()

	root := &model.Device{
		ID:           "root-device",
		Voucher:      `{"username":"root-device"}`,
		TenantID:     "tenant-1",
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
		DeviceNumber: "root-device",
		Name:         stringPtr("Root One"),
	}
	if err := db.Create(root).Error; err != nil {
		t.Fatalf("create root device: %v", err)
	}

	data, err := GetDeviceDetail("root-device")
	if err != nil {
		t.Fatalf("GetDeviceDetail returned error: %v", err)
	}
	if _, ok := data["gateway_device_name"]; ok {
		t.Fatalf("expected gateway_device_name key absent for root device, got %#v", data["gateway_device_name"])
	}
}

func TestGetDeviceDetailDanglingParentIDYieldsNilGatewayNameWithoutError(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	now := time.Now().UTC()

	danglingParentID := "deleted-gateway"
	orphan := &model.Device{
		ID:           "orphan-device",
		Voucher:      `{"username":"orphan-device"}`,
		TenantID:     "tenant-1",
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
		DeviceNumber: "orphan-device",
		Name:         stringPtr("Orphan One"),
		ParentID:     &danglingParentID,
	}
	if err := db.Create(orphan).Error; err != nil {
		t.Fatalf("create orphan device: %v", err)
	}

	// 加固行为核验：悬空 parent_id（父设备不存在）走 LEFT JOIN 后应得到
	// gateway_device_name=nil 而不是让整个调用失败，这是本次收敛刻意引入的
	// 更稳健行为，不能回退成旧的"整体报错"模式。
	data, err := GetDeviceDetail("orphan-device")
	if err != nil {
		t.Fatalf("GetDeviceDetail should not fail for dangling parent_id, got err: %v", err)
	}
	if _, ok := data["id"]; !ok {
		t.Fatalf("expected non-empty snapshot for orphan device, got %#v", data)
	}
	gatewayName, ok := data["gateway_device_name"]
	if !ok {
		t.Fatalf("expected gateway_device_name key present (nil) for dangling parent_id, got %#v", data)
	}
	if gatewayName != nil {
		t.Fatalf("gateway_device_name = %#v, want nil for dangling parent_id", gatewayName)
	}
}
