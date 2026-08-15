// 文件用途：覆盖告警接口测试相关 API 行为的 Go 测试。
// 核心逻辑：构造 Gin 路由或测试上下文，验证接口契约、参数处理和关键响应。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的路由/请求构造器。
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware "aetherlink-iot/backend/internal/middleware"
	apiresponse "aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/gin-gonic/gin"
)

// These source-structure guards only prove that declarations remain present.
// They do not prove routing, authentication, status codes, response bodies, or
// alarm behavior; executable HTTP evidence lives in the tests below.

func TestAlarmSourceStructureContractDeclaresConfigHistoryAcknowledgeAndReset(t *testing.T) {
	requireAPIMethods(t, "alarm.go", "AlarmApi",
		"CreateAlarmConfig",
		"DeleteAlarmConfig",
		"UpdateAlarmConfig",
		"ServeAlarmConfigListByPage",
		"HandleAlarmInfoListByPage",
		"HandleAlarmHisttoryListByPage",
		"HandleAlarmHistoryMonthlyTrend",
		"AlarmHistoryDescUpdate",
		"AcknowledgeAlarmHistory",
		"ResetAlarmHistory",
		"HandleConfigByDevice",
		"DeleteAlarmHistory",
	)
}

func TestAlarmNotificationSourceStructureContractDeclaresNotificationSurfaces(t *testing.T) {
	requireAPIMethods(t, "notification_group.go", "NotificationGroupApi",
		"CreateNotificationGroup",
		"DeleteNotificationGroup",
		"UpdateNotificationGroup",
	)
	requireAPIMethods(t, "notification_histories.go", "NotificationHistoryApi",
		"HandleNotificationHistoryListByPage",
	)
	requireAPIMethods(t, "notification_services_config.go", "NotificationServicesConfigApi",
		"SaveNotificationServicesConfig",
		"HandleNotificationServicesConfig",
		"SendTestEmail",
	)
	requireAPIMethods(t, "email_templates.go", "NotificationServicesConfigApi",
		"ListEmailTemplates",
		"CreateEmailTemplate",
		"UpdateEmailTemplate",
		"DeleteEmailTemplate",
		"SetDefaultEmailTemplate",
		"PreviewEmailTemplate",
	)
}

func TestAlarmCreateConfigHandlerRejectsMissingName(t *testing.T) {
	status, got := performAPIValidationRequest(
		t,
		http.MethodPost,
		"/api/v1/alarm/config",
		`{"alarm_level":"H"}`,
		(&AlarmApi{}).CreateAlarmConfig,
	)

	requireAlarmParamErrorResponse(t, status, got, "Field 'Name' is required")
}

func TestAlarmBatchHistoryActionHandlerRejectsMissingAction(t *testing.T) {
	status, got := performAPIValidationRequest(
		t,
		http.MethodPut,
		"/api/v1/alarm/info/history/batch-action",
		`{"ids":["alarm-history-1"]}`,
		(&AlarmApi{}).BatchAlarmHistoryAction,
	)

	requireAlarmParamErrorResponse(t, status, got, "Field 'Action' is required")
}

func TestAlarmProtectedHandlerRejectsMissingAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responseHandler := &apiresponse.Handler{
		ErrManager: errcode.NewErrorManager("", ""),
	}
	router := gin.New()
	router.Use(responseHandler.Middleware())
	router.Use(middleware.JWTAuth())
	router.GET("/api/v1/alarm/info", (&AlarmApi{}).HandleAlarmInfoListByPage)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/alarm/info?page=1&page_size=10",
		nil,
	)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("HTTP status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	var got middleware.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&got); err != nil {
		t.Fatalf("decode response body %q: %v", recorder.Body.String(), err)
	}
	if got.Code != middleware.ErrCodeNoAuth {
		t.Fatalf("response code = %d, want %d", got.Code, middleware.ErrCodeNoAuth)
	}
	const wantMessage = "missing authentication (x-token or x-api-key required)"
	if got.Message != wantMessage {
		t.Fatalf("response message = %q, want %q", got.Message, wantMessage)
	}
}

func requireAlarmParamErrorResponse(
	t *testing.T,
	status int,
	got apiresponse.Response,
	wantMessage string,
) {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", status, http.StatusOK)
	}
	if got.Code != errcode.CodeParamError {
		t.Fatalf("response code = %d, want %d", got.Code, errcode.CodeParamError)
	}
	if got.Message != wantMessage {
		t.Fatalf("response message = %q, want %q", got.Message, wantMessage)
	}
	if got.Data != nil {
		t.Fatalf("response data = %#v, want omitted", got.Data)
	}
}
