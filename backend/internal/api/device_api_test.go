// 文件用途：覆盖设备接口测试相关 API 行为的 Go 测试。
// 核心逻辑：构造 Gin 路由或测试上下文，验证接口契约、参数处理和关键响应。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的路由/请求构造器。
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiresponse "aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/gin-gonic/gin"
)

// These tests are lightweight source-structure guards. They do not prove HTTP
// routing, authentication, status codes, response bodies, or device behavior.

func TestDeviceSourceStructureContractDeclaresCRUDActivationAndDeviceDetails(t *testing.T) {
	requireAPIMethods(t, "device.go", "DeviceApi",
		"CreateDevice",
		"DeleteDevice",
		"UpdateDevice",
		"ActiveDevice",
		"HandleDeviceByID",
		"HandleDeviceListByPage",
		"DeviceConnectForm",
		"DeviceConnect",
		"UpdateDeviceVoucher",
	)
}

func TestDeviceSourceStructureContractDeclaresTopicMappingsDebugAndServiceAccess(t *testing.T) {
	requireAPIMethods(t, "device_topic_mapping.go", "DeviceTopicMappingApi",
		"CreateDeviceTopicMapping",
		"GetDeviceTopicMappings",
		"UpdateDeviceTopicMapping",
		"DeleteDeviceTopicMapping",
	)
	requireAPIMethods(t, "device_debug.go", "DeviceDebugApi",
		"SetDeviceDebug",
		"GetDeviceDebugStatus",
		"GetDeviceDebugLogs",
	)
	requireAPIMethods(t, "device_mqtt_debug.go", "DeviceDebugApi",
		"OpenMQTTDebugSession",
		"GetMQTTDebugSession",
		"ApplyMQTTDebugCommand",
		"CloseMQTTDebugSession",
	)
	requireAPIMethods(t, "device.go", "DeviceApi",
		"HandleTenantTelemetryData",
		"GetDeviceStatusHistory",
		"HandleDeviceSelector",
	)
	requireAPIMethods(t, "device_connection_diagnostics.go", "DeviceApi",
		"GetDeviceDiagnostics",
		"GetDeviceConnectionDiagnostics",
	)
	requireAPIStringLiterals(t, "device_connection_diagnostics.go",
		"debug_log_limit",
		"debug_log_limit must be an integer",
	)
	requireAPIStringLiterals(t, "../service/device_connection_diagnostics.go",
		`json:"debug_log_limit,omitempty"`,
		`json:"evaluated_at"`,
		`json:"conclusion"`,
		`json:"next_actions"`,
		`json:"online"`,
		`json:"debug"`,
		`json:"diagnostics"`,
		`json:"partial_results,omitempty"`,
	)
}

func TestDeviceSourceStructureContractDeclaresOnboardingConnectionGuide(t *testing.T) {
	requireAPIMethods(t, "device_connection_guide.go", "DeviceApi",
		"GetDeviceConnectionGuide",
	)
	requireAPIStringLiterals(t, "device_connection_guide.go",
		"debug_log_limit",
		"command_log_limit",
	)
	requireAPIStringLiterals(t, "../model/device_connection_guide.http.go",
		`json:"device_id"`,
		`json:"evaluated_at"`,
		`json:"access"`,
		`json:"readiness"`,
		`json:"last_connection_error,omitempty"`,
		`json:"twin_summary,omitempty"`,
		`json:"command_summary,omitempty"`,
		`json:"next_steps"`,
		`json:"partial_results,omitempty"`,
	)
}

func TestDeviceActiveHandlerRejectsMissingDeviceNumber(t *testing.T) {
	status, got := performAPIValidationRequest(
		t,
		http.MethodPut,
		"/api/v1/device/active",
		`{"name":"Lab device"}`,
		(&DeviceApi{}).ActiveDevice,
	)

	if status != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", status, http.StatusOK)
	}
	if got.Code != errcode.CodeParamError {
		t.Fatalf("response code = %d, want %d", got.Code, errcode.CodeParamError)
	}
	const wantMessage = "Field 'DeviceNumber' is required"
	if got.Message != wantMessage {
		t.Fatalf("response message = %q, want %q", got.Message, wantMessage)
	}
	if got.Data != nil {
		t.Fatalf("response data = %#v, want omitted", got.Data)
	}
}

func performAPIValidationRequest(
	t *testing.T,
	method string,
	target string,
	body string,
	handler gin.HandlerFunc,
) (int, apiresponse.Response) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	responseHandler := &apiresponse.Handler{
		ErrManager: errcode.NewErrorManager("", ""),
	}
	router := gin.New()
	router.Use(responseHandler.Middleware())
	router.Handle(method, target, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", "en-US")
	router.ServeHTTP(recorder, request)

	var payload apiresponse.Response
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response body %q: %v", recorder.Body.String(), err)
	}
	return recorder.Code, payload
}
