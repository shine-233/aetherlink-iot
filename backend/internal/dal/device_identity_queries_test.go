// 文件用途：锁定设备身份唯一性预检的精确匹配、批量去重和排除自身契约。
package dal

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestCheckDeviceNumberExistsUsesGlobalExactMatch(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	if err := db.Create(&model.Device{
		ID: "identity-device-1", Voucher: "voucher-1", TenantID: "tenant-a", IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "device-1",
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	if exists, err := CheckDeviceNumberExists("device-1"); err != nil || !exists {
		t.Fatalf("existing device number = (%v, %v), want (true, nil)", exists, err)
	}
	if exists, err := CheckDeviceNumberExists(" device-1 "); err != nil || exists {
		t.Fatalf("trimmed device number = (%v, %v), want (false, nil)", exists, err)
	}
	if exists, err := CheckDeviceNumberExists("missing"); err != nil || exists {
		t.Fatalf("missing device number = (%v, %v), want (false, nil)", exists, err)
	}
}

func TestCheckDeviceNumbersExistsKeepsExactBatchContract(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	devices := []model.Device{
		{ID: "identity-device-1", Voucher: "voucher-1", TenantID: "tenant-a", IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "device-1"},
		{ID: "identity-device-2", Voucher: "voucher-2", TenantID: "tenant-b", IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "device-2"},
	}
	if err := db.Create(&devices).Error; err != nil {
		t.Fatalf("create devices: %v", err)
	}

	got, err := CheckDeviceNumbersExists([]string{"device-1", "", "device-1", "device-2", "missing", " device-1 "})
	if err != nil {
		t.Fatalf("CheckDeviceNumbersExists(): %v", err)
	}
	if !got["device-1"] || !got["device-2"] || got["missing"] || got[" device-1 "] || got[""] {
		t.Fatalf("CheckDeviceNumbersExists() = %#v", got)
	}
	if got, err := CheckDeviceNumbersExists(nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty CheckDeviceNumbersExists() = %#v, err = %v, want initialized empty map", got, err)
	}
}

func TestCheckVoucherExistsExcludesCurrentDevice(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	devices := []model.Device{
		{ID: "identity-device-1", Voucher: "voucher-a", TenantID: "tenant-a", IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "device-1"},
		{ID: "identity-device-2", Voucher: "voucher-b", TenantID: "tenant-a", IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "device-2"},
	}
	if err := db.Create(&devices).Error; err != nil {
		t.Fatalf("create devices: %v", err)
	}

	cases := []struct {
		name    string
		voucher string
		exclude string
		want    bool
	}{
		{name: "own voucher excluded", voucher: "voucher-a", exclude: "identity-device-1", want: false},
		{name: "own voucher without exclusion", voucher: "voucher-a", exclude: "", want: true},
		{name: "other voucher remains conflict", voucher: "voucher-b", exclude: "identity-device-1", want: true},
		{name: "missing voucher", voucher: "missing", exclude: "identity-device-1", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CheckVoucherExists(tc.voucher, tc.exclude)
			if err != nil || got != tc.want {
				t.Fatalf("CheckVoucherExists(%q, %q) = (%v, %v), want (%v, nil)", tc.voucher, tc.exclude, got, err, tc.want)
			}
		})
	}
}
