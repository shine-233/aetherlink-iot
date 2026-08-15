// 文件用途：验证服务接入设备查询拆分后仍保持规范化、隔离和分组契约。
package dal

import (
	"reflect"
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestNormalizeServiceDeviceNumbers(t *testing.T) {
	got := normalizeServiceDeviceNumbers([]string{" device-2 ", "", "device-1", "device-2", "   ", "device-1"})
	want := []string{"device-2", "device-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeServiceDeviceNumbers() = %#v, want %#v", got, want)
	}
}

func TestServiceAccessDeviceQueriesKeepAccessScope(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	accessA := "access-a"
	accessB := "access-b"
	configA := "config-a"
	devices := []model.Device{
		{ID: "device-a-1", Voucher: "voucher-a-1", TenantID: "tenant-1", IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "number-1", ServiceAccessID: &accessA, DeviceConfigID: &configA},
		{ID: "device-a-2", Voucher: "voucher-a-2", TenantID: "tenant-1", IsEnabled: "enabled", ActivateFlag: "inactive", DeviceNumber: "number-2", ServiceAccessID: &accessA},
		{ID: "device-b-1", Voucher: "voucher-b-1", TenantID: "tenant-1", IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "number-1", ServiceAccessID: &accessB},
	}
	if err := db.Create(&devices).Error; err != nil {
		t.Fatalf("create service access devices: %v", err)
	}

	count, err := CountServiceDevicesByAccessID(accessA)
	if err != nil || count != 2 {
		t.Fatalf("CountServiceDevicesByAccessID() = (%d, %v), want (2, nil)", count, err)
	}
	if count, err := CountServiceDevicesByAccessID(""); err != nil || count != 0 {
		t.Fatalf("empty CountServiceDevicesByAccessID() = (%d, %v), want (0, nil)", count, err)
	}

	list, err := GetServiceDeviceList(accessA)
	if err != nil || len(list) != 2 {
		t.Fatalf("GetServiceDeviceList() len = %d, err = %v, want 2, nil", len(list), err)
	}

	selected, err := GetServiceDeviceListByNumbers(accessA, []string{" number-1 ", "number-1"})
	if err != nil || len(selected) != 1 || selected[0].DeviceNumber != "number-1" || selected[0].DeviceConfigID == nil || *selected[0].DeviceConfigID != configA {
		t.Fatalf("GetServiceDeviceListByNumbers() = %#v, err = %v", selected, err)
	}
	if selected, err := GetServiceDeviceListByNumbers("", []string{"number-1"}); err != nil || selected == nil || len(selected) != 0 {
		t.Fatalf("empty GetServiceDeviceListByNumbers() = %#v, err = %v, want non-nil empty slice", selected, err)
	}

	grouped, err := GetServiceDevicesByAccessIDs([]string{" access-a ", "access-b", "access-a", ""})
	if err != nil {
		t.Fatalf("GetServiceDevicesByAccessIDs(): %v", err)
	}
	if len(grouped[accessA]) != 2 || len(grouped[accessB]) != 1 || len(grouped) != 2 {
		t.Fatalf("GetServiceDevicesByAccessIDs() = %#v", grouped)
	}
	if grouped, err := GetServiceDevicesByAccessIDs(nil); err != nil || grouped == nil || len(grouped) != 0 {
		t.Fatalf("empty GetServiceDevicesByAccessIDs() = %#v, err = %v, want initialized empty map", grouped, err)
	}
}
