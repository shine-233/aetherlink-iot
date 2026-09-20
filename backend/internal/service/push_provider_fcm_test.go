package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testServiceAccountJSON 生成一份可用的服务账号 JSON（测试专用密钥）。
func testServiceAccountJSON(t *testing.T, privateKeyPEM string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"project_id":   "test-project",
		"client_email": "push@test-project.iam.gserviceaccount.com",
		"private_key":  privateKeyPEM,
	})
	if err != nil {
		t.Fatalf("marshal service account: %v", err)
	}
	return string(raw)
}

func testPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// newTestFCM 起一个假的 OAuth2 + FCM 端点，返回配置好的 Provider。
func newTestFCM(t *testing.T, sendStatus int, sendBody string) (PushProvider, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "test-access-token",
			"expires_in":   3600,
		})
	})
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(sendStatus)
		_, _ = w.Write([]byte(sendBody))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	provider, err := NewFCMProvider(FCMConfig{
		ProjectID:          "test-project",
		ServiceAccountJSON: testServiceAccountJSON(t, testPrivateKeyPEM(t)),
		TokenURL:           srv.URL + "/token",
		SendURL:            srv.URL + "/send",
	})
	if err != nil {
		t.Fatalf("NewFCMProvider: %v", err)
	}
	return provider, srv
}

func TestNewFCMProviderRejectsIncompleteConfig(t *testing.T) {
	key := testPrivateKeyPEM(t)
	cases := []struct {
		name string
		cfg  FCMConfig
		want error
	}{
		{name: "no project", cfg: FCMConfig{ServiceAccountJSON: testServiceAccountJSON(t, key)}, want: ErrFCMNotConfigured},
		{name: "no credentials", cfg: FCMConfig{ProjectID: "p"}, want: ErrFCMNotConfigured},
		{name: "bad json", cfg: FCMConfig{ProjectID: "p", ServiceAccountJSON: "{not json"}, want: ErrFCMBadAccount},
		{
			name: "missing private key",
			cfg:  FCMConfig{ProjectID: "p", ServiceAccountJSON: `{"client_email":"a@b.c"}`},
			want: ErrFCMBadAccount,
		},
		{
			name: "not pem",
			cfg:  FCMConfig{ProjectID: "p", ServiceAccountJSON: testServiceAccountJSON(t, "not-a-pem")},
			want: ErrFCMBadAccount,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewFCMProvider(tc.cfg); !isErr(err, tc.want) {
				t.Fatalf("NewFCMProvider(%s) = %v, want %v", tc.name, err, tc.want)
			}
		})
	}
}

func isErr(err, target error) bool {
	return err != nil && strings.Contains(err.Error(), target.Error())
}

func TestFCMSupportsAndroidOnly(t *testing.T) {
	provider, _ := newTestFCM(t, http.StatusOK, `{"name":"x"}`)
	if provider.Name() != PushProviderFCM {
		t.Fatalf("name = %q, want %q", provider.Name(), PushProviderFCM)
	}
	if !provider.Supports("android") {
		t.Fatal("fcm must support android")
	}
	// iOS 走 APNs，本项目尚未实现；报支持会让 iOS 令牌被送进 FCM 然后失败。
	if provider.Supports("ios") {
		t.Fatal("fcm must not claim ios support")
	}
	if provider.Supports("h5") {
		t.Fatal("fcm must not claim h5 support")
	}
}

// TestFCMSendSuccess 成功路径：拿到令牌 → 发出 → 无错误。
func TestFCMSendSuccess(t *testing.T) {
	provider, _ := newTestFCM(t, http.StatusOK, `{"name":"projects/x/messages/1"}`)
	if err := provider.Send(context.Background(), PushMessage{
		Token: "device-token", Title: "告警", Body: "温度过高", Platform: "android",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

// TestFCMSendClassifiesRetryableErrors 429/5xx 必须判为可重试。
// 判成终态会让一次限流直接把推送判死；判成可重试才会按退避重来。
func TestFCMSendClassifiesRetryableErrors(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		provider, _ := newTestFCM(t, status, `{"error":"busy"}`)
		err := provider.Send(context.Background(), PushMessage{Token: "t", Platform: "android"})
		if err == nil {
			t.Fatalf("status %d must produce an error", status)
		}
		if IsPushTerminal(err) {
			t.Fatalf("status %d must be retryable, got terminal: %v", status, err)
		}
	}
}

// TestFCMSendClassifiesTerminalErrors 4xx（除 429）必须判为终态。
// 典型场景是令牌失效（404/UNREGISTERED）：重试一万次也不会成功，
// 只会把"令牌失效"这个事实埋进最后一次 last_error。
func TestFCMSendClassifiesTerminalErrors(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusUnauthorized, http.StatusBadRequest, http.StatusForbidden} {
		provider, _ := newTestFCM(t, status, `{"error":{"status":"UNREGISTERED"}}`)
		err := provider.Send(context.Background(), PushMessage{Token: "t", Platform: "android"})
		if !IsPushTerminal(err) {
			t.Fatalf("status %d must be terminal, got %v", status, err)
		}
	}
}

// TestFCMSendRejectsEmptyTokenAsTerminal 空令牌是终态：没有目标地址，重试无意义。
func TestFCMSendRejectsEmptyTokenAsTerminal(t *testing.T) {
	provider, _ := newTestFCM(t, http.StatusOK, `{}`)
	err := provider.Send(context.Background(), PushMessage{Token: "  ", Platform: "android"})
	if !IsPushTerminal(err) {
		t.Fatalf("empty token must be terminal, got %v", err)
	}
}

// TestFCMTokenExchangeFailureIsRetryable 换令牌失败属于可重试：
// 通常是网络或对方临时故障，不该把整条推送判死。
func TestFCMTokenExchangeFailureIsRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server_error"}`))
	}))
	t.Cleanup(srv.Close)

	provider, err := NewFCMProvider(FCMConfig{
		ProjectID:          "test-project",
		ServiceAccountJSON: testServiceAccountJSON(t, testPrivateKeyPEM(t)),
		TokenURL:           srv.URL + "/token",
		SendURL:            srv.URL + "/send",
	})
	if err != nil {
		t.Fatalf("NewFCMProvider: %v", err)
	}
	sendErr := provider.Send(context.Background(), PushMessage{Token: "t", Platform: "android"})
	if sendErr == nil {
		t.Fatal("token exchange failure must produce an error")
	}
	if IsPushTerminal(sendErr) {
		t.Fatalf("token exchange failure must be retryable, got terminal: %v", sendErr)
	}
}

// TestFCMReusesAccessToken 令牌必须缓存：每条推送都换票会把配额耗在换票上。
func TestFCMReusesAccessToken(t *testing.T) {
	tokenCalls := 0
	sendCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		tokenCalls++
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "tok", "expires_in": 3600})
	})
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		sendCalls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	provider, err := NewFCMProvider(FCMConfig{
		ProjectID:          "test-project",
		ServiceAccountJSON: testServiceAccountJSON(t, testPrivateKeyPEM(t)),
		TokenURL:           srv.URL + "/token",
		SendURL:            srv.URL + "/send",
	})
	if err != nil {
		t.Fatalf("NewFCMProvider: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := provider.Send(context.Background(), PushMessage{Token: "t", Platform: "android"}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if tokenCalls != 1 {
		t.Fatalf("token exchange calls = %d, want 1 (token must be cached)", tokenCalls)
	}
	if sendCalls != 3 {
		t.Fatalf("send calls = %d, want 3", sendCalls)
	}
}

// TestFCMSendsExpectedPayload 断言请求体带上令牌与通知内容。
// 只验证"能发出去"不够——发了个空消息体也算成功。
func TestFCMSendsExpectedPayload(t *testing.T) {
	var got map[string]interface{}
	var auth string
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "tok", "expires_in": 3600})
	})
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	provider, err := NewFCMProvider(FCMConfig{
		ProjectID:          "test-project",
		ServiceAccountJSON: testServiceAccountJSON(t, testPrivateKeyPEM(t)),
		TokenURL:           srv.URL + "/token",
		SendURL:            srv.URL + "/send",
	})
	if err != nil {
		t.Fatalf("NewFCMProvider: %v", err)
	}
	if err := provider.Send(context.Background(), PushMessage{
		Token: "device-token", Title: "T", Body: "B",
		Data: map[string]interface{}{"device_id": "d1", "count": 3}, Platform: "android",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if auth != "Bearer tok" {
		t.Fatalf("Authorization = %q, want %q", auth, "Bearer tok")
	}
	message, ok := got["message"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload has no message: %v", got)
	}
	if message["token"] != "device-token" {
		t.Fatalf("message.token = %v, want device-token", message["token"])
	}
	notification, _ := message["notification"].(map[string]interface{})
	if notification["title"] != "T" || notification["body"] != "B" {
		t.Fatalf("notification = %v, want title=T body=B", notification)
	}
	// FCM 的 data 只收字符串：数字必须被转成字符串，否则整条请求会被 400 拒掉。
	data, _ := message["data"].(map[string]interface{})
	if data["device_id"] != "d1" {
		t.Fatalf("data.device_id = %v, want d1", data["device_id"])
	}
	if data["count"] != "3" {
		t.Fatalf("data.count = %v, want the string \"3\"", data["count"])
	}
}

// TestAssemblePushUnconfiguredReturnsNoService 没配凭据时必须返回 nil 服务。
// 返回一个空 Provider 注册表的服务会让能力矩阵报 Push=true 而每次发送都失败。
func TestAssemblePushUnconfiguredReturnsNoService(t *testing.T) {
	svc, configured, err := AssemblePush(PushWiringConfig{})
	if err != nil {
		t.Fatalf("AssemblePush without credentials must not error: %v", err)
	}
	if configured || svc != nil {
		t.Fatalf("AssemblePush() = (%v, %t), want (nil, false)", svc, configured)
	}
}

// TestAssemblePushRejectsBadCredentials 配了凭据但非法时必须报错并阻断启动，
// 而不是静默降级成"不发推送"——那会让运维以为推送是通的。
func TestAssemblePushRejectsBadCredentials(t *testing.T) {
	bad := PushWiringConfig{FCM: FCMConfig{ProjectID: "p", ServiceAccountJSON: "{bad}"}}
	if _, _, err := AssemblePush(bad); err == nil {
		t.Fatal("bad credentials must produce an error")
	}
}

func TestAssemblePushWiresFCMProvider(t *testing.T) {
	svc, configured, err := AssemblePush(PushWiringConfig{
		FCM: FCMConfig{
			ProjectID:          "test-project",
			ServiceAccountJSON: testServiceAccountJSON(t, testPrivateKeyPEM(t)),
		},
	})
	if err != nil || !configured || svc == nil {
		t.Fatalf("AssemblePush = (%v, %t, %v), want a service", svc, configured, err)
	}
	// 装配出来的服务必须真的能通过注册表找到 android 的 Provider。
	if svc.registry == nil {
		t.Fatal("push service has no registry")
	}
	if _, err := svc.registry.ForPlatform("android"); err != nil {
		t.Fatalf("android provider lookup: %v", err)
	}
	if _, err := svc.registry.ForPlatform("ios"); err == nil {
		t.Fatal("ios must have no provider until APNs is implemented")
	}
}

// TestPushTerminalErrorIsDetectedThroughWrapping errors.As 必须能穿透包装，
// 否则终态判定会在错误被 fmt.Errorf 包一层之后失效。
func TestPushTerminalErrorIsDetectedThroughWrapping(t *testing.T) {
	wrapped := &PushTerminalError{Reason: "unregistered"}
	if !IsPushTerminal(wrapped) {
		t.Fatal("terminal error must be detected")
	}
	if IsPushTerminal(ErrFCMSendFailed) {
		t.Fatal("plain error must not be treated as terminal")
	}
}

// TestIsPushTerminalNil nil 必须判 false，不能因为断言写错就 panic。
func TestIsPushTerminalNil(t *testing.T) {
	if IsPushTerminal(nil) {
		t.Fatal("nil must not be terminal")
	}
}
