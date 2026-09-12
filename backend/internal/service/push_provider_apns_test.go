package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testAPNsKeyP8 生成一份可用的 .p8（P-256 EC 私钥，PKCS8 PEM）。
func testAPNsKeyP8(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ec key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal ec key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// newTestAPNs 起一个开启 HTTP/2 的假 APNs 端点。
//
// 为什么必须开 h2：APNs 对 HTTP/1.1 直接返回 400。若测试跑在 h1 上，
// 就会去验证一条真实环境根本走不通的路径。
func newTestAPNs(t *testing.T, status int, body string) (PushProvider, *httptest.Server, *apnsCapture) {
	t.Helper()
	capture := &apnsCapture{}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.proto = r.Proto
		capture.topic = r.Header.Get("apns-topic")
		capture.auth = r.Header.Get("Authorization")
		capture.path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&capture.payload)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)

	provider, err := NewAPNsProvider(APNsConfig{
		KeyID:        "TESTKEYID",
		TeamID:       "TESTTEAMID",
		Topic:        "com.example.aetherlink",
		PrivateKeyP8: testAPNsKeyP8(t),
		BaseURL:      srv.URL,
		HTTPClient:   srv.Client(), // 信任测试证书
	})
	if err != nil {
		t.Fatalf("NewAPNsProvider: %v", err)
	}
	return provider, srv, capture
}

type apnsCapture struct {
	proto   string
	topic   string
	auth    string
	path    string
	payload map[string]interface{}
}

func TestNewAPNsProviderRejectsIncompleteConfig(t *testing.T) {
	key := testAPNsKeyP8(t)
	full := APNsConfig{KeyID: "k", TeamID: "t", Topic: "com.x", PrivateKeyP8: key}
	cases := []struct {
		name string
		cfg  APNsConfig
		want error
	}{
		{name: "missing key id", cfg: APNsConfig{TeamID: "t", Topic: "com.x", PrivateKeyP8: key}, want: ErrAPNSNotConfigured},
		{name: "missing team id", cfg: APNsConfig{KeyID: "k", Topic: "com.x", PrivateKeyP8: key}, want: ErrAPNSNotConfigured},
		{name: "missing topic", cfg: APNsConfig{KeyID: "k", TeamID: "t", PrivateKeyP8: key}, want: ErrAPNSNotConfigured},
		{name: "missing key", cfg: APNsConfig{KeyID: "k", TeamID: "t", Topic: "com.x"}, want: ErrAPNSNotConfigured},
		{name: "not pem", cfg: APNsConfig{KeyID: "k", TeamID: "t", Topic: "com.x", PrivateKeyP8: "nope"}, want: ErrAPNSBadKey},
		{name: "rsa key instead of ec", cfg: APNsConfig{KeyID: "k", TeamID: "t", Topic: "com.x", PrivateKeyP8: testPrivateKeyPEM(t)}, want: ErrAPNSBadKey},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewAPNsProvider(tc.cfg); !isErr(err, tc.want) {
				t.Fatalf("NewAPNsProvider(%s) = %v, want %v", tc.name, err, tc.want)
			}
		})
	}
	// 完整配置必须能构造出来。
	if _, err := NewAPNsProvider(full); err != nil {
		t.Fatalf("complete config must work: %v", err)
	}
}

func TestAPNsSupportsIOSOnly(t *testing.T) {
	provider, _, _ := newTestAPNs(t, http.StatusOK, `{}`)
	if provider.Name() != PushProviderAPNS {
		t.Fatalf("name = %q, want %q", provider.Name(), PushProviderAPNS)
	}
	if !provider.Supports("ios") {
		t.Fatal("apns must support ios")
	}
	if provider.Supports("android") {
		t.Fatal("apns must not claim android support")
	}
}

// TestAPNsSendUsesHTTP2 断言请求确实走了 HTTP/2。
// 这条是 APNs 与其它 Provider 最大的差别所在，值得单独锁住。
func TestAPNsSendUsesHTTP2(t *testing.T) {
	provider, _, capture := newTestAPNs(t, http.StatusOK, `{}`)
	if err := provider.Send(context.Background(), PushMessage{
		Token: "ios-device-token", Title: "T", Body: "B", Platform: "ios",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if capture.proto != "HTTP/2.0" {
		t.Fatalf("request protocol = %q, want HTTP/2.0 (APNs rejects HTTP/1.1)", capture.proto)
	}
	if capture.topic != "com.example.aetherlink" {
		t.Fatalf("apns-topic = %q, want the configured bundle id", capture.topic)
	}
	if !strings.HasPrefix(capture.auth, "bearer ") {
		t.Fatalf("Authorization = %q, want a bearer JWT", capture.auth)
	}
	if capture.path != "/3/device/ios-device-token" {
		t.Fatalf("path = %q, want /3/device/{token}", capture.path)
	}
}

// TestAPNsPayloadShape 断言 aps 结构与自定义字段的位置。
// APNs 要求自定义数据放在 aps 之外；放错位置整条推送会被拒。
func TestAPNsPayloadShape(t *testing.T) {
	provider, _, capture := newTestAPNs(t, http.StatusOK, `{}`)
	if err := provider.Send(context.Background(), PushMessage{
		Token: "t", Title: "告警", Body: "温度过高",
		Data: map[string]interface{}{"device_id": "d1", "count": 2}, Platform: "ios",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	aps, ok := capture.payload["aps"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload has no aps: %v", capture.payload)
	}
	alert, _ := aps["alert"].(map[string]interface{})
	if alert["title"] != "告警" || alert["body"] != "温度过高" {
		t.Fatalf("alert = %v, want title/body", alert)
	}
	// 自定义字段必须在 aps 之外，且被收敛成字符串。
	if capture.payload["device_id"] != "d1" {
		t.Fatalf("device_id = %v, want d1", capture.payload["device_id"])
	}
	if capture.payload["count"] != "2" {
		t.Fatalf("count = %v, want the string \"2\"", capture.payload["count"])
	}
}

// TestAPNsClassifiesRetryableErrors 429/5xx 必须可重试。
func TestAPNsClassifiesRetryableErrors(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		provider, _, _ := newTestAPNs(t, status, `{"reason":"busy"}`)
		err := provider.Send(context.Background(), PushMessage{Token: "t", Platform: "ios"})
		if err == nil {
			t.Fatalf("status %d must produce an error", status)
		}
		if IsPushTerminal(err) {
			t.Fatalf("status %d must be retryable, got terminal: %v", status, err)
		}
	}
}

// TestAPNsClassifiesTerminalErrors 4xx（除 429）必须终态。
// 410 是 APNs 表示"令牌对该话题已失效"的码，是最典型的重试无意义场景。
func TestAPNsClassifiesTerminalErrors(t *testing.T) {
	for _, status := range []int{410, http.StatusBadRequest, http.StatusForbidden, http.StatusRequestEntityTooLarge} {
		provider, _, _ := newTestAPNs(t, status, `{"reason":"Unregistered"}`)
		err := provider.Send(context.Background(), PushMessage{Token: "t", Platform: "ios"})
		if !IsPushTerminal(err) {
			t.Fatalf("status %d must be terminal, got %v", status, err)
		}
	}
}

// TestAPNsRejectsEmptyTokenAsTerminal 空令牌是终态。
func TestAPNsRejectsEmptyTokenAsTerminal(t *testing.T) {
	provider, _, _ := newTestAPNs(t, http.StatusOK, `{}`)
	if err := provider.Send(context.Background(), PushMessage{Token: " ", Platform: "ios"}); !IsPushTerminal(err) {
		t.Fatalf("empty token must be terminal, got %v", err)
	}
}

// TestAPNsReusesSignedJWT JWT 必须复用：每条推送都重新签名是白白的 CPU 开销。
func TestAPNsReusesSignedJWT(t *testing.T) {
	provider, _, _ := newTestAPNs(t, http.StatusOK, `{}`)
	concrete, ok := provider.(*apnsProvider)
	if !ok {
		t.Fatal("provider is not *apnsProvider")
	}
	first, err := concrete.authToken()
	if err != nil {
		t.Fatalf("authToken: %v", err)
	}
	second, err := concrete.authToken()
	if err != nil {
		t.Fatalf("authToken again: %v", err)
	}
	if first != second {
		t.Fatal("JWT must be reused until it expires")
	}
	// JWT 必须是三段，且头部带 kid（APNs 靠 kid 选密钥）。
	if parts := len(strings.Split(first, ".")); parts != 3 {
		t.Fatalf("JWT has %d parts, want 3", parts)
	}
	headerJSON, err := jwtSegment(first, 0)
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if headerJSON["kid"] != "TESTKEYID" {
		t.Fatalf("JWT header kid = %v, want TESTKEYID", headerJSON["kid"])
	}
	if headerJSON["alg"] != "ES256" {
		t.Fatalf("JWT header alg = %v, want ES256", headerJSON["alg"])
	}
}

func TestAssemblePushRegistersBothProviders(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)

	cfg := PushWiringConfig{
		FCM:  FCMConfig{ProjectID: "p", ServiceAccountJSON: testServiceAccountJSON(t, testPrivateKeyPEM(t))},
		APNs: APNsConfig{KeyID: "k", TeamID: "t", Topic: "com.x", PrivateKeyP8: testAPNsKeyP8(t), BaseURL: srv.URL, HTTPClient: srv.Client()},
	}
	svc, configured, err := AssemblePush(cfg)
	if err != nil || !configured {
		t.Fatalf("AssemblePush = (%v, %t, %v)", svc, configured, err)
	}
	if svc.registry.Len() != 2 {
		t.Fatalf("registry has %d providers, want 2", svc.registry.Len())
	}
	if _, err := svc.registry.ForPlatform("android"); err != nil {
		t.Fatalf("android lookup: %v", err)
	}
	if _, err := svc.registry.ForPlatform("ios"); err != nil {
		t.Fatalf("ios lookup: %v", err)
	}
	// h5 没有系统级推送通道：必须明确失败，不能随便挑一个 Provider 发出去。
	if _, err := svc.registry.ForPlatform("h5"); err == nil {
		t.Fatal("h5 must have no provider")
	}
}

// TestAssemblePushOneChannelOnly 只配一个通道时，另一个平台没有 Provider。
// 半配置状态必须如实暴露，不能让 iOS 的推送被"找不到通道"静默吞掉。
func TestAssemblePushOneChannelOnly(t *testing.T) {
	svc, configured, err := AssemblePush(PushWiringConfig{
		FCM: FCMConfig{ProjectID: "p", ServiceAccountJSON: testServiceAccountJSON(t, testPrivateKeyPEM(t))},
	})
	if err != nil || !configured {
		t.Fatalf("AssemblePush = (%v, %t, %v)", svc, configured, err)
	}
	if _, err := svc.registry.ForPlatform("ios"); err == nil {
		t.Fatal("ios must have no provider when APNs is not configured")
	}
}

// TestAssemblePushInvalidCredentialsBlock 凭据非法必须报错（阻断启动），
// 而不是静默降级成"不发推送"。
func TestAssemblePushInvalidCredentialsBlock(t *testing.T) {
	cases := []struct {
		name string
		cfg  PushWiringConfig
	}{
		{name: "bad fcm json", cfg: PushWiringConfig{FCM: FCMConfig{ProjectID: "p", ServiceAccountJSON: "{bad"}}},
		{name: "bad apns key", cfg: PushWiringConfig{APNs: APNsConfig{KeyID: "k", TeamID: "t", Topic: "com.x", PrivateKeyP8: "not-pem"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := AssemblePush(tc.cfg); err == nil {
				t.Fatalf("%s must produce an error", tc.name)
			}
		})
	}
}

// TestAssemblePushNothingConfigured 全没配时返回 nil 服务（不是空壳）。
func TestAssemblePushNothingConfigured(t *testing.T) {
	svc, configured, err := AssemblePush(PushWiringConfig{})
	if err != nil || configured || svc != nil {
		t.Fatalf("AssemblePush() = (%v, %t, %v), want (nil, false, nil)", svc, configured, err)
	}
}

func TestAPNsStatusLabelCoversKnownCodes(t *testing.T) {
	if strings.Contains(apnsStatusLabel(410), "410") {
		t.Fatal("410 label should explain the reason, not repeat the code")
	}
	if !strings.Contains(apnsStatusLabel(410), "no longer active") {
		t.Fatalf("410 label = %q", apnsStatusLabel(410))
	}
}

// jwtSegment 解出 JWT 的第 i 段（测试辅助）。
func jwtSegment(token string, i int) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) <= i {
		return nil, errors.New("segment out of range")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[i])
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
