package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"aetherlink-iot/backend/pkg/safehttp"
	"aetherlink-iot/backend/third_party/others/http_client"
)

// 租户可控的 Webhook / IM 地址不得指向回环、内网或云元数据地址（SSRF）。
func TestResolveWebhookEndpointRejectsNonPublicHosts(t *testing.T) {
	for _, rawURL := range []string{
		"http://127.0.0.1:8080/hook",
		"http://localhost/hook",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.5/hook",
		"http://192.168.1.10:6379/",
		"http://[::1]:9999/hook",
		"https://user:pw@172.16.3.4/hook",
	} {
		t.Run(rawURL, func(t *testing.T) {
			_, _, err := resolveWebhookEndpoint(rawURL)
			if !errors.Is(err, ErrWebhookProviderUnavailable) {
				t.Fatalf("resolveWebhookEndpoint(%q) err = %v, want ErrWebhookProviderUnavailable", rawURL, err)
			}
		})
	}
}

func TestNotificationSendWebhookMessageRejectsLoopbackBeforePersistence(t *testing.T) {
	err := (&NotificationServicesConfig{}).sendWebhookMessage("http://127.0.0.1:1/hook", "secret", `{}`, "tenant-1")
	if !errors.Is(err, ErrWebhookProviderUnavailable) {
		t.Fatalf("err = %v, want ErrWebhookProviderUnavailable", err)
	}
}

// 前置校验之外的兜底：即使某个地址绕过了字面量检查（例如域名解析到内网），
// 共享客户端也必须在拨号时拒绝，且请求绝不能到达内网服务。
func TestTenantWebhookHTTPClientBlocksLoopbackAtDialTime(t *testing.T) {
	var hits atomic.Int32
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer internal.Close()

	err := http_client.SendSignedRequestWithClient(context.Background(), tenantWebhookHTTPClient, internal.URL, `{}`, "secret")
	if !errors.Is(err, safehttp.ErrUnsafeWebhookURL) {
		t.Fatalf("signed webhook err = %v, want ErrUnsafeWebhookURL", err)
	}

	_, err = postJSONWithTimeout(context.Background(), internal.URL, []byte(`{}`))
	if !errors.Is(err, safehttp.ErrUnsafeWebhookURL) {
		t.Fatalf("IM post err = %v, want ErrUnsafeWebhookURL", err)
	}

	if got := hits.Load(); got != 0 {
		t.Fatalf("internal server received %d requests, want 0", got)
	}
}
