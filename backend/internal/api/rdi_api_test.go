// 文件用途：覆盖RDI 接口测试相关 API 行为的 Go 测试。
// 核心逻辑：构造 Gin 路由或测试上下文，验证接口契约、参数处理和关键响应。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的路由/请求构造器。
package api

import (
	"net/http"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
)

// Source-structure contracts below guard declarations only. They are not
// evidence of HTTP routing, authentication, status codes, or response bodies.
func TestRDISourceStructureContractDeclaresActivationConfigCommandFirmwareAndSharing(t *testing.T) {
	requireAPIMethods(t, "rdi.go", "RDIApi",
		"ThingModel",
		"ActivateDevice",
		"DeviceConfig",
		"DeviceHistory",
		"LatestFirmware",
		"UpdateDeviceConfig",
		"SendCommand",
		"CreateShareToken",
		"AcceptSharedDevice",
		"SharedDevices",
		"SharedDeviceConfig",
	)
}

func TestRDISourceStructureContractDeclaresDeviceAndTokenInputHelpers(t *testing.T) {
	requireAPIIdentifiers(t, "rdi.go",
		"BindAndValidate",
		"devicePathID",
		"Param",
		"Set",
	)
	requireAPIStringLiterals(t, "rdi.go",
		"token",
	)
}

func TestRDISendCommandHandlerRejectsMissingIdentifier(t *testing.T) {
	status, got := performAPIValidationRequest(
		t,
		http.MethodPost,
		"/api/v1/rdi/devices/device-1/commands",
		`{"params":{"value":true}}`,
		(&RDIApi{}).SendCommand,
	)

	if status != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", status, http.StatusOK)
	}
	if got.Code != errcode.CodeParamError {
		t.Fatalf("response code = %d, want %d", got.Code, errcode.CodeParamError)
	}
	const wantMessage = "Field 'Identifier' is required"
	if got.Message != wantMessage {
		t.Fatalf("response message = %q, want %q", got.Message, wantMessage)
	}
	if got.Data != nil {
		t.Fatalf("response data = %#v, want omitted", got.Data)
	}
}
