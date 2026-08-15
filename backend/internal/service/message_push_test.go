// 文件用途：验证消息推送配置和远端回调发送行为。
// 核心逻辑：构造推送配置、告警上下文和 fake 回调，断言 URL 校验、请求体和历史记录。
// 关键注意事项：推送测试要防止泄露凭据或依赖真实网络，失败响应也应有清晰日志边界。
// 重构建议：抽出推送发送器和历史仓储，补齐重试、超时、权限拒绝和日志脱敏断言。
package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func TestMessagePushConfigRejectsNilRequestBeforeDAL(t *testing.T) {
	err := (&MessagePush{}).SetMessagePushConfig(nil, &utils.UserClaims{Authority: constant.SYS_ADMIN})
	assertErrCode(t, err, errcode.CodeParamError, "message push config is required")
}

func TestMessagePushManageRejectsInvalidRequestsBeforeDAL(t *testing.T) {
	service := &MessagePush{}

	cases := []struct {
		name        string
		err         error
		wantMessage string
	}{
		{
			name:        "create nil request",
			err:         service.CreateMessagePush(nil, "user-a"),
			wantMessage: "message push registration is required",
		},
		{
			name:        "create blank push id",
			err:         service.CreateMessagePush(&model.CreateMessagePushReq{PushId: "   ", DeviceType: "ios"}, "user-a"),
			wantMessage: "push id is required",
		},
		{
			name:        "create blank device type",
			err:         service.CreateMessagePush(&model.CreateMessagePushReq{PushId: "push-a", DeviceType: "\t"}, "user-a"),
			wantMessage: "device type is required",
		},
		{
			name:        "logout nil request",
			err:         service.MessagePushMangeLogout(nil, "user-a"),
			wantMessage: "message push logout is required",
		},
		{
			name:        "logout blank push id",
			err:         service.MessagePushMangeLogout(&model.MessagePushMangeLogoutReq{PushId: "\n"}, "user-a"),
			wantMessage: "push id is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertErrCode(t, tc.err, errcode.CodeParamError, tc.wantMessage)
		})
	}
}

func TestMessagePushResponseLogMessageDoesNotEchoResponseBody(t *testing.T) {
	rawResponse := `{"errCode":500,"token":"secret-token","message":"full provider body"}`

	got := messagePushResponseLogMessage(len(rawResponse))
	if strings.Contains(got, "secret-token") || strings.Contains(got, "full provider body") || strings.Contains(got, rawResponse) {
		t.Fatalf("response log message leaked raw response: %q", got)
	}
	want := fmt.Sprintf("message push response classified, response_bytes:%d", len(rawResponse))
	if got != want {
		t.Fatalf("response log message = %q, want length-only summary", got)
	}
}

func TestMessagePushDebugFieldValueDoesNotEchoStringCode(t *testing.T) {
	got := messagePushDebugFieldValue("secret-provider-code")
	if got == "secret-provider-code" {
		t.Fatalf("debug field value leaked raw string code")
	}
	if got != "string" {
		t.Fatalf("debug field value = %#v, want type-only string marker", got)
	}

	if numeric := messagePushDebugFieldValue(float64(500)); numeric != float64(500) {
		t.Fatalf("numeric debug field value = %#v, want numeric code preserved", numeric)
	}
}

func TestDeliverMessagePushHTTPSendsJSON(t *testing.T) {
	payload := []byte(`{"title":"alarm","content":"offline"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(body) != string(payload) {
			t.Errorf("body = %q, want %q", body, payload)
		}
		_, _ = io.WriteString(w, `{"errCode":0}`)
	}))
	defer server.Close()

	got, err := deliverMessagePushHTTP(server.URL, payload)
	if err != nil {
		t.Fatalf("deliverMessagePushHTTP returned error: %v", err)
	}
	if got != `{"errCode":0}` {
		t.Fatalf("response = %q, want provider response", got)
	}
}

func TestDeliverMessagePushHTTPClassifiesProviderFailureWithoutBodyLeak(t *testing.T) {
	const secretBody = "provider-secret-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, secretBody)
	}))
	defer server.Close()

	_, err := deliverMessagePushHTTP(server.URL, []byte(`{}`))
	if !errors.Is(err, ErrMessagePushExternalUnavailable) {
		t.Fatalf("error = %v, want ErrMessagePushExternalUnavailable", err)
	}
	if strings.Contains(err.Error(), secretBody) {
		t.Fatalf("error leaked provider response body: %q", err)
	}
}

func TestDeliverMessagePushHTTPRejectsOversizedResponseWithoutBodyLeak(t *testing.T) {
	const secretSuffix = "provider-secret-suffix"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", int(messagePushMaxResponseBytes))+secretSuffix)
	}))
	defer server.Close()

	_, err := deliverMessagePushHTTP(server.URL, []byte(`{}`))
	if !errors.Is(err, ErrMessagePushExternalUnavailable) {
		t.Fatalf("error = %v, want ErrMessagePushExternalUnavailable", err)
	}
	if strings.Contains(err.Error(), secretSuffix) {
		t.Fatalf("error leaked oversized provider response: %q", err)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("exceeds %d bytes", messagePushMaxResponseBytes)) {
		t.Fatalf("error = %q, want stable response limit", err)
	}
}

func TestDeliverMessagePushHTTPClassifiesNetworkFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint := server.URL
	server.Close()

	_, err := deliverMessagePushHTTP(endpoint, []byte(`{}`))
	if !errors.Is(err, ErrMessagePushExternalUnavailable) {
		t.Fatalf("error = %v, want ErrMessagePushExternalUnavailable", err)
	}
}

func TestClassifyMessagePushDeliveryError(t *testing.T) {
	t.Run("disabled is skipped", func(t *testing.T) {
		result := classifyMessagePushDeliveryError(fmt.Errorf("config: %w", ErrMessagePushDisabled))
		if !result.skipped || result.status != 0 || result.errMessage != "" {
			t.Fatalf("disabled result = %#v, want skipped without failure audit", result)
		}
	})

	t.Run("external failure uses stable reason", func(t *testing.T) {
		result := classifyMessagePushDeliveryError(fmt.Errorf("send: %w", ErrMessagePushExternalUnavailable))
		if result.skipped || result.status != messagePushStatusFailed || result.errMessage != messagePushExternalUnavailableReason {
			t.Fatalf("external result = %#v, want stable external-unavailable reason", result)
		}
	})

	t.Run("local failure does not expose original error", func(t *testing.T) {
		const sensitiveError = "database password appeared in driver error"
		result := classifyMessagePushDeliveryError(errors.New(sensitiveError))
		if result.skipped || result.status != messagePushStatusFailed || result.errMessage != messagePushDeliveryFailedReason {
			t.Fatalf("local result = %#v, want stable delivery-failed reason", result)
		}
		if strings.Contains(result.errMessage, sensitiveError) {
			t.Fatalf("delivery result leaked original error: %q", result.errMessage)
		}
	})
}
