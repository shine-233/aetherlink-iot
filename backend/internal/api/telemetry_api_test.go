// 文件用途：覆盖遥测接口测试相关 API 行为的 Go 测试。
// 核心逻辑：构造 Gin 路由或测试上下文，验证接口契约、参数处理和关键响应。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的路由/请求构造器。
package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// Source-structure contracts below guard declarations only. They are not
// evidence of HTTP routing, authentication, status codes, or response bodies.
func TestTelemetrySourceStructureContractDeclaresCurrentHistoryStatisticAndSimulation(t *testing.T) {
	requireAPIMethods(t, "telemetry_data.go", "TelemetryDataApi",
		"HandleCurrentData",
		"HandleCurrentDataKeys",
		"ServeHistoryData",
		"ServeHistoryDataByPage",
		"ServeStatisticData",
		"ServeStatisticDataByDeviceId",
		"SimulationTelemetryData",
		"SimulationSend",
		"TelemetryPutMessage",
	)
}

func TestTelemetrySourceStructureContractDeclaresDeadLetterOperations(t *testing.T) {
	requireAPIMethods(t, "telemetry_data.go", "TelemetryDataApi",
		"ServeDeadLetterList",
		"DrainDeadLetters",
		"UpdateDeadLetterStatus",
	)
}

func TestTelemetrySourceStructureContractDeclaresAttributeEventDeadLetterOperations(t *testing.T) {
	requireAPIMethods(t, "attribute_event_dead_letters.go", "TelemetryDataApi",
		"ServeAttributeEventDeadLetterList",
		"DrainAttributeEventDeadLetters",
		"UpdateAttributeEventDeadLetterStatus",
	)
}

func TestAttributeEventDeadLetterStatusUpdateRequiresExpectedStatus(t *testing.T) {
	if err := ValidateStruct(model.UpdateAttributeEventDeadLetterStatusReq{Action: "replay"}); err == nil {
		t.Fatal("missing expected_status should be rejected")
	}
	if err := ValidateStruct(model.UpdateAttributeEventDeadLetterStatusReq{
		Action:         "replay",
		ExpectedStatus: "pending",
	}); err != nil {
		t.Fatalf("valid expected_status should be accepted: %v", err)
	}
}

func TestTelemetryDeadLetterListValidationAcceptsProcessingStatus(t *testing.T) {
	for _, status := range []string{"pending", "processing", "retrying", "resolved", "dead"} {
		req := model.GetTelemetryDeadLetterListReq{
			PageReq: model.PageReq{Page: 1, PageSize: 20},
			Status:  status,
		}
		if err := ValidateStruct(req); err != nil {
			t.Fatalf("status %q should be accepted: %v", status, err)
		}
	}

	req := model.GetTelemetryDeadLetterListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 20},
		Status:  "archived",
	}
	if err := ValidateStruct(req); err == nil {
		t.Fatal("unsupported dead-letter status should be rejected")
	}
}

func TestTelemetrySourceStructureContractDeclaresAuthAndWebsocketBoundaries(t *testing.T) {
	requireAPIMethods(t, "telemetry_data.go", "TelemetryDataApi",
		"ServeCurrentDataByWS",
		"ServeDeviceStatusByWS",
		"ServeCurrentDataByKey",
	)
	requireAPIIdentifiers(t, "telemetry_ws_auth.go",
		"validateToken",
		"validateAPIKey",
		"validateAuth",
	)
	requireAPIIdentifiers(t, "telemetry_ws_stream.go",
		"startTelemetryWSWriter",
		"writeTelemetryWSControl",
		"writeTelemetryWSClose",
		"writeTelemetryWSPongControl",
		"queueTelemetryWSPong",
	)
}

func TestTelemetrySimulationHandlerRejectsMissingCommand(t *testing.T) {
	status, got := performAPIValidationRequest(
		t,
		http.MethodPost,
		"/api/v1/telemetry/datas/simulation",
		`{}`,
		(&TelemetryDataApi{}).SimulationTelemetryData,
	)

	if status != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", status, http.StatusOK)
	}
	if got.Code != errcode.CodeParamError {
		t.Fatalf("response code = %d, want %d", got.Code, errcode.CodeParamError)
	}
	const wantMessage = "Field 'Command' is required"
	if got.Message != wantMessage {
		t.Fatalf("response message = %q, want %q", got.Message, wantMessage)
	}
	if got.Data != nil {
		t.Fatalf("response data = %#v, want omitted", got.Data)
	}
}

func TestTelemetryWSMessageHelpersValidateJSONDeviceAndKeys(t *testing.T) {
	if _, err := parseTelemetryWSMessage([]byte(`{`)); err == nil {
		t.Fatal("parseTelemetryWSMessage accepted invalid JSON")
	}
	if _, err := parseTelemetryWSMessage([]byte(`null`)); err == nil {
		t.Fatal("parseTelemetryWSMessage accepted null JSON")
	}

	msg, err := parseTelemetryWSMessage([]byte(`{"device_id":"device-1","keys":["temp","hum"]}`))
	if err != nil {
		t.Fatalf("parseTelemetryWSMessage returned error: %v", err)
	}
	deviceID, err := telemetryWSDeviceID(msg)
	if err != nil {
		t.Fatalf("telemetryWSDeviceID returned error: %v", err)
	}
	if deviceID != "device-1" {
		t.Fatalf("deviceID = %q, want device-1", deviceID)
	}
	keys, err := telemetryWSKeys(msg)
	if err != nil {
		t.Fatalf("telemetryWSKeys returned error: %v", err)
	}
	if len(keys) != 2 || keys[0] != "temp" || keys[1] != "hum" {
		t.Fatalf("keys = %#v, want temp/hum", keys)
	}
}

func TestTelemetryWSMessageHelpersRejectBadDeviceAndKeys(t *testing.T) {
	if _, err := telemetryWSDeviceID(map[string]interface{}{"device_id": "   "}); err == nil {
		t.Fatal("telemetryWSDeviceID accepted blank device_id")
	}
	if _, err := telemetryWSKeys(map[string]interface{}{"keys": []interface{}{}}); err == nil {
		t.Fatal("telemetryWSKeys accepted empty keys")
	}
	if _, err := telemetryWSKeys(map[string]interface{}{"keys": []interface{}{"temp", ""}}); err == nil {
		t.Fatal("telemetryWSKeys accepted blank key")
	}
	if _, err := telemetryWSKeys(map[string]interface{}{"keys": "temp"}); err == nil {
		t.Fatal("telemetryWSKeys accepted non-array keys")
	}
}

func TestQueueTelemetryWSPongUsesSendQueueWhenAvailable(t *testing.T) {
	wsClient := &global.WSClient{
		Send: make(chan []byte, 1),
	}

	queueTelemetryWSPong(wsClient)

	select {
	case got := <-wsClient.Send:
		if string(got) != "pong" {
			t.Fatalf("queued pong = %q, want pong", string(got))
		}
	default:
		t.Fatal("queueTelemetryWSPong did not queue pong")
	}
}

func TestValidateAuthMatrix(t *testing.T) {
	originalTokenValidator := validateTelemetryTokenFn
	originalAPIKeyValidator := validateTelemetryAPIKeyFn
	t.Cleanup(func() {
		validateTelemetryTokenFn = originalTokenValidator
		validateTelemetryAPIKeyFn = originalAPIKeyValidator
	})

	validateTelemetryTokenFn = func(token string) (*utils.UserClaims, error) {
		switch token {
		case "token-ok":
			return &utils.UserClaims{
				ID:        "token-user",
				TenantID:  "tenant-token",
				Authority: "SYS_ADMIN",
			}, nil
		default:
			return nil, errors.New("invalid token")
		}
	}
	validateTelemetryAPIKeyFn = func(apiKey string) (*utils.UserClaims, error) {
		switch apiKey {
		case "key-ok":
			return &utils.UserClaims{
				ID:        "api-user",
				TenantID:  "tenant-api",
				Authority: "TENANT_ADMIN",
			}, nil
		default:
			return nil, errors.New("invalid api key")
		}
	}

	tests := []struct {
		name       string
		msgMap     map[string]interface{}
		wantID     string
		wantTenant string
		wantErr    string
	}{
		{
			name: "token",
			msgMap: map[string]interface{}{
				"token": "token-ok",
			},
			wantID:     "token-user",
			wantTenant: "tenant-token",
		},
		{
			name: "authorization bearer",
			msgMap: map[string]interface{}{
				"authorization": "Bearer token-ok",
			},
			wantID:     "token-user",
			wantTenant: "tenant-token",
		},
		{
			name: "api key alias",
			msgMap: map[string]interface{}{
				"X-API-Key": "key-ok",
			},
			wantID:     "api-user",
			wantTenant: "tenant-api",
		},
		{
			name:    "missing auth",
			msgMap:  map[string]interface{}{},
			wantErr: "authentication failed: token or x-api-key is required",
		},
		{
			name: "invalid token",
			msgMap: map[string]interface{}{
				"token": "token-bad",
			},
			wantErr: "invalid token",
		},
		{
			name: "invalid token plus valid api key fallback",
			msgMap: map[string]interface{}{
				"token":     "token-bad",
				"x_api_key": "key-ok",
			},
			wantID:     "api-user",
			wantTenant: "tenant-api",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claims, err := validateAuth(tc.msgMap)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("validateAuth(%v) returned nil error, want %q", tc.msgMap, tc.wantErr)
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("validateAuth(%v) error = %q, want %q", tc.msgMap, err.Error(), tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("validateAuth(%v) returned error: %v", tc.msgMap, err)
			}
			if claims == nil {
				t.Fatalf("validateAuth(%v) returned nil claims", tc.msgMap)
			}
			if claims.ID != tc.wantID || claims.TenantID != tc.wantTenant {
				t.Fatalf("validateAuth(%v) claims = %+v, want ID=%q TenantID=%q", tc.msgMap, claims, tc.wantID, tc.wantTenant)
			}
		})
	}
}

func TestResolveSimulationClientIPUsesRemoteAddrInsteadOfRequestHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("GET", "/api/v1/telemetry/datas/simulation?device_id=dev-1", nil)
	req.Host = "preview.local:8080"
	req.RemoteAddr = "203.0.113.8:4567"
	ctx.Request = req

	got := resolveSimulationClientIP(ctx)
	if got != "203.0.113.8" {
		t.Fatalf("resolveSimulationClientIP() = %q, want remote IP without host port", got)
	}
}

func TestResolveSimulationClientIPHandlesNilContext(t *testing.T) {
	if got := resolveSimulationClientIP(nil); got != "" {
		t.Fatalf("resolveSimulationClientIP(nil) = %q, want empty string", got)
	}
}
