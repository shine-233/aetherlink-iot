// 文件用途：覆盖RDI 校验测试相关 API 行为的 Go 测试。
// 核心逻辑：构造 Gin 路由或测试上下文，验证接口契约、参数处理和关键响应。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的路由/请求构造器。
package api

import (
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestRDIRequestValidationProtectsDeviceActivationHistoryAndCommandInputs(t *testing.T) {
	validHistory := model.RDIHistoryReq{
		Key:         "temperature_1",
		StartTime:   1719000000,
		EndTime:     1719003600,
		Page:        intPtr(1),
		PageSize:    intPtr(50),
		ExportExcel: boolPtr(false),
	}
	if err := ValidateStruct(validHistory); err != nil {
		t.Fatalf("valid RDIHistoryReq returned error: %v", err)
	}

	tests := []struct {
		name string
		req  interface{}
		want string
	}{
		{
			name: "activation requires PID number",
			req:  model.ActivateRDIDeviceReq{Name: "Kitchen RDI"},
			want: "Field 'PIDNumber' is required",
		},
		{
			name: "command requires identifier",
			req:  model.RDICommandReq{Params: map[string]interface{}{"value": true}},
			want: "Field 'Identifier' is required",
		},
		{
			name: "history requires data key",
			req:  model.RDIHistoryReq{StartTime: 1719000000, EndTime: 1719003600},
			want: "Field 'Key' is required",
		},
		{
			name: "history export format is constrained",
			req: model.RDIHistoryReq{
				Key:          "temperature_1",
				StartTime:    1719000000,
				EndTime:      1719003600,
				ExportFormat: stringPtr("pdf"),
			},
			want: "Field 'ExportFormat' failed validation (Must be one of: excel, csv)",
		},
		{
			name: "shared device page size has an upper bound",
			req:  model.RDISharedDeviceListReq{Page: 1, PageSize: 201},
			want: "Field 'PageSize' failed validation (Must be at most 200)",
		},
		{
			name: "share token device id length is bounded",
			req:  model.RDIShareTokenReq{DeviceID: strings.Repeat("d", 37), ExpiresIn: 3600},
			want: "Field 'DeviceID' failed validation (At most 36 characters)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStructLang(tt.req, "en-US")
			if err == nil {
				t.Fatal("ValidateStructLang expected RDI request validation error")
			}
			if err.Error() != tt.want {
				t.Fatalf("ValidateStructLang error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestRDISharedDeviceListValidationAllowsOptionalFiltersAndPagination(t *testing.T) {
	req := model.RDISharedDeviceListReq{
		DeviceID:   "device-1",
		DeviceName: "RDI cabinet",
		Page:       2,
		PageSize:   100,
	}

	if err := ValidateStruct(req); err != nil {
		t.Fatalf("valid RDISharedDeviceListReq returned error: %v", err)
	}
}

func intPtr(value int) *int {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
