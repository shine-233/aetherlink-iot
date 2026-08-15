// 文件用途：覆盖 CreateDevice 对空白 device_config_id 的归一化，确保空字符串不会
//
//	被写进 devices.device_config_id 而触发 fk_device_config_id 外键约束。
//
// 背景：normalizeCreateDeviceConfigID 原先只在 buildCreateDeviceContext 内部调用，
//
//	而该函数按值接收 req，归一化结果被丢弃，导致 `"device_config_id": ""` 的创建请求
//	在真实 Postgres 上以 23503 外键错误失败（自动化用例 seed 设备时大面积 500）。
//	本测试针对该语义做变异可证的回归覆盖：把 device_create.go 里的归一化调用删掉，
//	TestApplyCreateDeviceRequestFieldsKeepsBlankConfigIDNil 必须失败。
package service

import (
	"strconv"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
)

func TestNormalizeCreateDeviceConfigIDBlankBecomesNil(t *testing.T) {
	cases := []struct {
		name string
		in   *string
	}{
		{name: "empty string", in: strPtr("")},
		{name: "spaces", in: strPtr("   ")},
		{name: "tab", in: strPtr("\t")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeCreateDeviceConfigID(tc.in); got != nil {
				t.Fatalf("expected blank config id to normalize to nil, got %q", *got)
			}
		})
	}
}

func TestNormalizeCreateDeviceConfigIDPreservesRealValue(t *testing.T) {
	id := "cfg-0001"
	got := normalizeCreateDeviceConfigID(&id)
	if got == nil {
		t.Fatal("expected a real config id to survive normalization")
	}
	if *got != id {
		t.Fatalf("config id mutated: want %q got %q", id, *got)
	}

	if nilIn := normalizeCreateDeviceConfigID(nil); nilIn != nil {
		t.Fatalf("nil input must stay nil, got %q", *nilIn)
	}
}

// TestApplyCreateDeviceRequestFieldsKeepsBlankConfigIDNil is the mutation-sensitive
// guard: it hands the RAW request (blank config id, exactly what the HTTP layer
// binds) to production code and asserts the value written onto the Device model,
// which is what the foreign key sees. The test must NOT normalize the input
// itself — doing so is what made an earlier version of this test pass against the
// broken source. Delete the normalizeCreateDeviceConfigID call in
// applyCreateDeviceRequestFields and this test must fail.
func TestApplyCreateDeviceRequestFieldsKeepsBlankConfigIDNil(t *testing.T) {
	for _, blank := range []string{"", "   ", "\t"} {
		t.Run("blank="+strconv.Quote(blank), func(t *testing.T) {
			req := model.CreateDeviceReq{
				Name:           strPtr("fk-guard-device"),
				DeviceConfigId: strPtr(blank),
			}

			var device model.Device
			applyCreateDeviceRequestFields(&device, req)

			if device.DeviceConfigID != nil {
				t.Fatalf("blank device_config_id must persist as NULL, got %q "+
					"(this reintroduces the fk_device_config_id 23503 failure)", *device.DeviceConfigID)
			}
			if device.ActivateFlag != "active" {
				t.Fatalf("unrelated create defaults regressed: activate_flag=%q", device.ActivateFlag)
			}
			if device.IsEnabled != "enabled" {
				t.Fatalf("new devices must be enabled when active, got is_enabled=%q", device.IsEnabled)
			}
		})
	}
}

func TestApplyCreateDeviceRequestFieldsKeepsProvidedConfigID(t *testing.T) {
	req := model.CreateDeviceReq{
		Name:           strPtr("bound-device"),
		DeviceConfigId: strPtr("cfg-real-42"),
	}

	var device model.Device
	applyCreateDeviceRequestFields(&device, req)

	if device.DeviceConfigID == nil {
		t.Fatal("a real device_config_id must be persisted, got NULL")
	}
	if *device.DeviceConfigID != "cfg-real-42" {
		t.Fatalf("device_config_id mutated: got %q", *device.DeviceConfigID)
	}
}

// TestLoadCreateDeviceConfigSkipsLookupForBlankID documents that the blank case
// must not attempt a device-config access check at all (nil in -> nil out, no
// DB/claims dependency), so an unbound device create stays a pure insert.
func TestLoadCreateDeviceConfigSkipsLookupForBlankID(t *testing.T) {
	cfg, err := loadCreateDeviceConfig(normalizeCreateDeviceConfigID(strPtr("  ")), nil)
	if err != nil {
		t.Fatalf("blank config id must not error: %v", err)
	}
	if cfg != nil {
		t.Fatal("blank config id must not resolve a device config")
	}
}

func TestBuildCreateDeviceVoucherUnboundDeviceGetsBasicCredentials(t *testing.T) {
	voucher := buildCreateDeviceVoucher(nil, nil)
	if !strings.Contains(voucher, `"username"`) || !strings.Contains(voucher, `"password"`) {
		t.Fatalf("unbound device should get basic username/password voucher, got %s", voucher)
	}
}
