// 文件用途：验证第三方 HTTP 基础请求和签名 Webhook 的可测试行为。
// 核心逻辑：使用 httptest 覆盖 JSON Header、HMAC 签名、成功响应和错误状态码处理。
// 关键注意事项：测试只覆盖本地 HTTP 交互，不代表真实协议插件服务可用。
// 重构建议：后续可补充超时、调用方未关闭 Body 的静态检查，以及敏感日志脱敏断言。
package http_client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateHMACUsesSHA256HexSignature(t *testing.T) {
	signature := generateHMAC(`{"device":"rdi-001"}`, "shared-secret")

	const expected = "ba2a1d1de3a907fa4ab846d97598bf71d922aeb567d53b560ae6c26c0f5a1a64"
	if signature != expected {
		t.Fatalf("unexpected signature: got %s want %s", signature, expected)
	}
}

func TestGetReturnsBodyOnlyForHTTP200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"online"}`))
			return
		}
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	body, err := Get(server.URL + "/ok")
	if err != nil {
		t.Fatalf("Get ok returned error: %v", err)
	}
	if string(body) != `{"status":"online"}` {
		t.Fatalf("unexpected body: %s", string(body))
	}

	if _, err := Get(server.URL + "/unavailable"); err == nil {
		t.Fatal("Get should fail for non-200 status")
	}
}

func TestPostJsonSendsJSONContentTypeAndPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type = %q, want application/json", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(body) != `{"cmd":"restart"}` {
			t.Fatalf("payload = %s", string(body))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	resp, err := PostJson(server.URL, []byte(`{"cmd":"restart"}`))
	if err != nil {
		t.Fatalf("PostJson returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
}

func TestSignedRequestIncludesSignatureAndRejectsHTTPErrorStatus(t *testing.T) {
	const payload = `{"alarm":"ack"}`
	const secret = "alarm-secret"
	expectedSignature := "sha256=" + generateHMAC(payload, secret)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Signature-256") != expectedSignature {
			t.Fatalf("signature header = %q, want %q", r.Header.Get("X-Signature-256"), expectedSignature)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type = %q, want application/json", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read signed body: %v", err)
		}
		if string(body) != payload {
			t.Fatalf("signed payload = %s", string(body))
		}
		http.Error(w, "rejected", http.StatusForbidden)
	}))
	defer server.Close()

	err := SendSignedRequestWithTimeout(context.Background(), server.URL, payload, secret)
	if err == nil {
		t.Fatal("signed request should return an error for HTTP 403")
	}
}

func TestSignedRequestRejectsRedirectWithoutForwardingPayload(t *testing.T) {
	redirectTargetCalls := 0
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer redirectTarget.Close()

	configuredEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusTemporaryRedirect)
	}))
	defer configuredEndpoint.Close()

	err := SendSignedRequestWithTimeout(context.Background(), configuredEndpoint.URL, `{}`, "secret")
	if err == nil {
		t.Fatal("signed request should treat HTTP 307 as a delivery failure")
	}
	if redirectTargetCalls != 0 {
		t.Fatalf("redirect target received %d requests, want 0", redirectTargetCalls)
	}
}
