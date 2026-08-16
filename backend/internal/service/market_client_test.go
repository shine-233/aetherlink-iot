// 文件用途：验证市场客户端的 HTTP 调用、鉴权和响应映射。
// 核心逻辑：使用 httptest 覆盖 token、模板包、错误响应和 payload 解码分支。
// 关键注意事项：市场接口是外部依赖，测试不能访问真实网络，需固定超时、状态码和错误体语义。
// 重构建议：抽出 HTTP client 和重试策略接口，补齐超时、鉴权失效、坏 JSON 和幂等安装边界。
package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"github.com/spf13/viper"
)

func TestNewMarketClientRequiresExplicitEnable(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	client := NewMarketClient()
	_, err := client.marketEndpoint("/api/market/templates")
	if !errors.Is(err, ErrMarketServiceUnavailable) {
		t.Fatalf("marketEndpoint() error = %v, want ErrMarketServiceUnavailable", err)
	}
	if !strings.Contains(err.Error(), "market integration is disabled") {
		t.Fatalf("marketEndpoint() error = %v, want disabled diagnostic", err)
	}
}

func TestNewMarketClientRequiresConfiguredBaseURLWhenEnabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("market.enabled", true)

	client := NewMarketClient()
	if client.baseURL != "" {
		t.Fatalf("baseURL = %q, want empty when market.base_url is not configured", client.baseURL)
	}
	_, err := client.marketEndpoint("/api/market/templates")
	if !errors.Is(err, ErrMarketServiceUnavailable) {
		t.Fatalf("marketEndpoint() error = %v, want ErrMarketServiceUnavailable", err)
	}
	if !strings.Contains(err.Error(), "market.base_url is not configured") {
		t.Fatalf("marketEndpoint() error = %v, want configured base URL diagnostic", err)
	}
}

func TestParseExistsFromBody(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		want      bool
		wantError bool
	}{
		{
			name: "flat exists",
			body: `{"exists":true,"email":"fixture@example.com"}`,
			want: true,
		},
		{
			name: "nested exists in data",
			body: `{"code":200,"data":{"exists":false}}`,
			want: false,
		},
		{
			name: "boolean data",
			body: `{"code":200,"data":true}`,
			want: true,
		},
		{
			name:      "missing exists",
			body:      `{"code":200,"data":{"email":"fixture@example.com"}}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseExistsFromBody([]byte(tt.body))
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestCheckUserExists_ResponseClassify(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		want      bool
		wantError error
	}{
		{
			name:   "not found",
			status: http.StatusNotFound,
			body:   `{"message":"not found"}`,
			want:   false,
		},
		{
			name:   "bad request but user not exists",
			status: http.StatusBadRequest,
			body:   `{"message":"email not found"}`,
			want:   false,
		},
		{
			name:      "non-200 rejected",
			status:    http.StatusInternalServerError,
			body:      `{"message":"internal error"}`,
			wantError: ErrMarketRequestRejected,
		},
		{
			name:      "invalid body",
			status:    http.StatusOK,
			body:      `not-json`,
			wantError: ErrMarketInvalidResponse,
		},
		{
			name:   "ok exists",
			status: http.StatusOK,
			body:   `{"exists":true}`,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := &MarketClient{
				baseURL:    server.URL,
				httpClient: server.Client(),
			}

			got, err := client.CheckUserExists(context.Background(), "fixture@example.com")
			if tt.wantError != nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, tt.wantError) {
					t.Fatalf("expected error %v, got %v", tt.wantError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestMarketTemplateQueriesEncodeParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch r.URL.Path {
		case "/api/market/templates":
			if query.Get("name") == "RDI Sensor & Alarm" {
				if query.Get("version") != "v1.0+prod" {
					t.Fatalf("version query = %q", query.Get("version"))
				}
				_, _ = w.Write([]byte(`{"data":{"total":1}}`))
				return
			}

			if query.Get("keyword") != "RDI Sensor & Alarm" {
				t.Fatalf("keyword query = %q", query.Get("keyword"))
			}
			if query.Get("category") != "工业/农业" {
				t.Fatalf("category query = %q", query.Get("category"))
			}
			if query.Get("sort_by") != "latest" {
				t.Fatalf("sort_by query = %q", query.Get("sort_by"))
			}
			if strings.Contains(r.URL.RawQuery, "RDI Sensor & Alarm") {
				t.Fatalf("raw query was not encoded: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"data":[],"total":0,"page":1,"page_size":12}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := &MarketClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	exists, err := client.CheckTemplateExists(context.Background(), "token", "RDI Sensor & Alarm", "v1.0+prod")
	if err != nil {
		t.Fatalf("CheckTemplateExists() error = %v", err)
	}
	if !exists {
		t.Fatal("CheckTemplateExists() = false, want true")
	}

	if _, err := client.ListMarketTemplates(context.Background(), "RDI Sensor & Alarm", "工业/农业", "latest", 1, 12); err != nil {
		t.Fatalf("ListMarketTemplates() error = %v", err)
	}
}

func TestCheckUserExistsSendsEmailQueryToExpectedEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/account/auth/user/exists" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("email"); got != "fixture+ops@example.com" {
			t.Fatalf("email query = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":{"exists":true}}`))
	}))
	defer server.Close()

	client := &MarketClient{
		baseURL:    server.URL + "/",
		httpClient: server.Client(),
	}

	got, err := client.CheckUserExists(context.Background(), "fixture+ops@example.com")
	if err != nil {
		t.Fatalf("CheckUserExists() error = %v", err)
	}
	if !got {
		t.Fatal("CheckUserExists() = false, want true")
	}
}

func TestMarketClientLoginPostsCredentialsAndReturnsToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/account/auth/login" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type = %q", got)
		}
		_, _ = w.Write([]byte(`{"token":"market-token","expires_at":3600}`))
	}))
	defer server.Close()

	client := &MarketClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	token, err := client.Login(context.Background(), "admin@example.com", "secret")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if token != "market-token" {
		t.Fatalf("token = %q", token)
	}
}

func TestMarketClientLoginRedactsRejectedCredentialDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid credentials"}`))
	}))
	defer server.Close()

	client := &MarketClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	_, err := client.Login(context.Background(), "admin@example.com", "wrong")
	if err == nil {
		t.Fatal("expected rejected login to return an error")
	}
	if !strings.Contains(err.Error(), "login status=401") {
		t.Fatalf("unexpected login error: %v", err)
	}
	if strings.Contains(err.Error(), "invalid credentials") {
		t.Fatalf("login error leaked upstream body: %v", err)
	}
}

func TestPublishTemplateSendsHeadersBodyAndParsesResponse(t *testing.T) {
	var gotAuth string
	var gotContentType string
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/market/templates/publish" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll(body) error = %v", err)
		}
		gotAuth = r.Header.Get("Authorization")
		if got := r.Header.Get("X-User-Id"); got != "" {
			t.Fatalf("X-User-Id header = %q, want omitted for opaque bearer token", got)
		}
		gotContentType = r.Header.Get("Content-Type")
		gotBody = string(bodyBytes)
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"published":true}}`))
	}))
	defer server.Close()

	client := &MarketClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	resp, err := client.PublishTemplate(context.Background(), "market-token", &model.PublishTemplateReq{
		Name:        "AetherLink Sensor",
		Brand:       "AetherLink",
		Model:       "AL-1",
		Category:    "industrial-sensor",
		Version:     "1.0.0",
		Description: "publish payload",
		DeviceConfig: &model.DeviceConfigPayload{
			Name:         "cfg",
			DeviceType:   "1",
			ProtocolType: "mqtt",
		},
		TemplateDefinition: map[string]interface{}{
			"telemetry": []interface{}{map[string]interface{}{"data_identifier": "temp"}},
		},
		PluginDependencies: []model.PluginDependency{{
			PluginName: "mqtt",
			PluginType: "protocol",
			Required:   true,
		}},
	})
	if err != nil {
		t.Fatalf("PublishTemplate() error = %v", err)
	}
	if gotAuth != "Bearer market-token" {
		t.Fatalf("Authorization header = %q, want Bearer market-token", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type header = %q, want application/json", gotContentType)
	}
	if !strings.Contains(gotBody, `"name":"AetherLink Sensor"`) ||
		!strings.Contains(gotBody, `"protocol_type":"mqtt"`) ||
		!strings.Contains(gotBody, `"plugin_name":"mqtt"`) {
		t.Fatalf("publish body = %s, want request payload fields", gotBody)
	}
	if resp.Code != 0 || resp.Message != "ok" {
		t.Fatalf("PublishTemplate() response = %#v, want parsed success payload", resp)
	}
}

func TestPublishTemplateOmitsOptionalUserHeaderAndRejectsBadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-User-Id"); got != "" {
			t.Fatalf("X-User-Id header = %q, want omitted", got)
		}
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client := &MarketClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	_, err := client.PublishTemplate(context.Background(), "market-token", &model.PublishTemplateReq{Name: "test"})
	if err == nil {
		t.Fatal("PublishTemplate() should fail on invalid JSON response")
	}
	if !errors.Is(err, ErrMarketInvalidResponse) {
		t.Fatalf("PublishTemplate() error = %v, want ErrMarketInvalidResponse", err)
	}
}

func TestDownloadTemplateEncodesVersionAndClassifiesFailureModes(t *testing.T) {
	t.Run("success with version query", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			if r.URL.Path != "/api/market/templates/template-1/download" {
				t.Fatalf("path = %s", r.URL.Path)
			}
			if got := r.URL.Query().Get("version"); got != "1.0.0+prod" {
				t.Fatalf("version query = %q, want 1.0.0+prod", got)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer market-token" {
				t.Fatalf("Authorization header = %q, want Bearer market-token", got)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"name":"AetherLink Template","version_id":"ver-1","version":"1.0.0+prod","device_config":{"name":"cfg","device_type":"1"}}}`))
		}))
		defer server.Close()

		client := &MarketClient{
			baseURL:    server.URL,
			httpClient: server.Client(),
		}

		got, err := client.DownloadTemplate(context.Background(), "market-token", "template-1", "1.0.0+prod")
		if err != nil {
			t.Fatalf("DownloadTemplate() error = %v", err)
		}
		if got.Name != "AetherLink Template" || got.VersionID != "ver-1" || got.Version != "1.0.0+prod" {
			t.Fatalf("DownloadTemplate() payload = %#v, want parsed market template", got)
		}
		if got.DeviceConfig == nil || got.DeviceConfig.Name != "cfg" {
			t.Fatalf("DownloadTemplate() device config = %#v, want parsed nested device config", got.DeviceConfig)
		}
	})

	t.Run("non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`gateway down`))
		}))
		defer server.Close()

		client := &MarketClient{
			baseURL:    server.URL,
			httpClient: server.Client(),
		}

		_, err := client.DownloadTemplate(context.Background(), "market-token", "template-1", "")
		if err == nil {
			t.Fatal("DownloadTemplate() should fail on non-200")
		}
		if !strings.Contains(err.Error(), "download failed with status 502") {
			t.Fatalf("DownloadTemplate() error = %v, want non-200 diagnostic", err)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{`))
		}))
		defer server.Close()

		client := &MarketClient{
			baseURL:    server.URL,
			httpClient: server.Client(),
		}

		_, err := client.DownloadTemplate(context.Background(), "market-token", "template-1", "")
		if err == nil {
			t.Fatal("DownloadTemplate() should fail on malformed JSON")
		}
		if !strings.Contains(err.Error(), "failed to parse download response") {
			t.Fatalf("DownloadTemplate() error = %v, want parse failure", err)
		}
	})
}

func TestInstallTemplateSendsBearerOnlyAndAcceptsCreatedOrOK(t *testing.T) {
	t.Run("created without client supplied identity headers", func(t *testing.T) {
		var gotBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/api/market/templates/template-1/install" {
				t.Fatalf("path = %s", r.URL.Path)
			}
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll(body) error = %v", err)
			}
			gotBody = string(bodyBytes)
			if got := r.Header.Get("Authorization"); got != "Bearer market-token" {
				t.Fatalf("Authorization header = %q, want Bearer market-token", got)
			}
			for _, header := range []string{"X-User-Id", "X-Org-Id", "X-Roles"} {
				if got := r.Header.Get(header); got != "" {
					t.Fatalf("%s header = %q, want omitted", header, got)
				}
			}
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		client := &MarketClient{
			baseURL:    server.URL,
			httpClient: server.Client(),
		}

		if err := client.InstallTemplate(context.Background(), "market-token", "template-1", "ver-1"); err != nil {
			t.Fatalf("InstallTemplate() error = %v", err)
		}
		if !strings.Contains(gotBody, `"version_id":"ver-1"`) {
			t.Fatalf("install body = %s, want version_id payload", gotBody)
		}
	})

	t.Run("ok without optional user headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("X-User-Id"); got != "" {
				t.Fatalf("X-User-Id header = %q, want omitted", got)
			}
			if got := r.Header.Get("X-Org-Id"); got != "" {
				t.Fatalf("X-Org-Id header = %q, want omitted", got)
			}
			if got := r.Header.Get("X-Roles"); got != "" {
				t.Fatalf("X-Roles header = %q, want omitted", got)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := &MarketClient{
			baseURL:    server.URL,
			httpClient: server.Client(),
		}

		if err := client.InstallTemplate(context.Background(), "market-token", "template-1", ""); err != nil {
			t.Fatalf("InstallTemplate() error = %v", err)
		}
	})

	t.Run("non-2xx rejection", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`forbidden`))
		}))
		defer server.Close()

		client := &MarketClient{
			baseURL:    server.URL,
			httpClient: server.Client(),
		}

		err := client.InstallTemplate(context.Background(), "market-token", "template-1", "ver-1")
		if err == nil {
			t.Fatal("InstallTemplate() should fail on non-2xx")
		}
		if !strings.Contains(err.Error(), "install notification failed with status 403") {
			t.Fatalf("InstallTemplate() error = %v, want non-2xx diagnostic", err)
		}
	})
}

func TestListMarketTemplatesFlattensMarketPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":[{"name":"RDI"}],"total":1,"page":2,"page_size":5}`))
	}))
	defer server.Close()

	client := &MarketClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	raw, err := client.ListMarketTemplates(context.Background(), "", "", "", 2, 5)
	if err != nil {
		t.Fatalf("ListMarketTemplates() error = %v", err)
	}

	got, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T", raw)
	}
	if got["total"] != float64(1) || got["page"] != float64(2) || got["page_size"] != float64(5) {
		t.Fatalf("pagination fields not preserved: %#v", got)
	}
	if _, ok := got["data"]; ok {
		t.Fatalf("market list payload should expose list, not raw data: %#v", got)
	}
	if _, ok := got["code"]; ok {
		t.Fatalf("market list payload should not leak raw market code: %#v", got)
	}
	list, ok := got["list"].([]interface{})
	if !ok || len(list) != 1 {
		t.Fatalf("list not flattened from data: %#v", got)
	}
	row, ok := list[0].(map[string]interface{})
	if !ok {
		t.Fatalf("list row type = %T, want object", list[0])
	}
	if row["name"] != "RDI" {
		t.Fatalf("list row business fields not preserved: %#v", row)
	}
}

func TestListMarketTemplatesDefaultsMissingDataToEmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"total":0}`))
	}))
	defer server.Close()

	client := &MarketClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	raw, err := client.ListMarketTemplates(context.Background(), "", "", "", 1, 12)
	if err != nil {
		t.Fatalf("ListMarketTemplates() error = %v", err)
	}

	got := raw.(map[string]interface{})
	list, ok := got["list"].([]interface{})
	if !ok || len(list) != 0 {
		t.Fatalf("missing data should become empty list, got %#v", got["list"])
	}
	if got["page"] != 1 || got["page_size"] != 12 {
		t.Fatalf("missing pagination should default to request values, got %#v", got)
	}
}

func TestListMarketTemplatesFlattensNestedPaginationPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"name":"RDI Nested"}],"total":7,"page":3,"pageSize":20}}`))
	}))
	defer server.Close()

	client := &MarketClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	raw, err := client.ListMarketTemplates(context.Background(), "", "", "", 1, 12)
	if err != nil {
		t.Fatalf("ListMarketTemplates() error = %v", err)
	}

	got := raw.(map[string]interface{})
	if got["total"] != float64(7) || got["page"] != float64(3) || got["page_size"] != float64(20) {
		t.Fatalf("nested pagination not flattened: %#v", got)
	}
	list, ok := got["list"].([]interface{})
	if !ok || len(list) != 1 {
		t.Fatalf("nested items not flattened into list: %#v", got)
	}
	row, ok := list[0].(map[string]interface{})
	if !ok || row["name"] != "RDI Nested" {
		t.Fatalf("nested item fields not preserved: %#v", list)
	}
}

func TestGetMarketTemplateDetailMergesTemplateAndVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/market/templates/template-1" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"template":{"id":"template-1","name":"RDI"},"versions":[{"version_id":"version-1","version":"1.0.0","description":"stable"}]}}`))
	}))
	defer server.Close()

	client := &MarketClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	raw, err := client.GetMarketTemplateDetail(context.Background(), "template-1")
	if err != nil {
		t.Fatalf("GetMarketTemplateDetail() error = %v", err)
	}

	got, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T", raw)
	}
	if got["id"] != "template-1" || got["name"] != "RDI" {
		t.Fatalf("template fields not returned: %#v", got)
	}
	if versions, ok := got["versions"].([]interface{}); !ok || len(versions) != 1 {
		t.Fatalf("versions not merged into template: %#v", got)
	} else {
		version, ok := versions[0].(map[string]interface{})
		if !ok {
			t.Fatalf("version row type = %T, want object", versions[0])
		}
		if version["version_id"] != "version-1" || version["version"] != "1.0.0" || version["description"] != "stable" {
			t.Fatalf("version metadata not preserved: %#v", version)
		}
	}
}

func TestMarketIntegrationDisabledPreventsOutboundRequests(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("market.enabled", false)
	viper.Set("market.base_url", "http://127.0.0.1:1")

	client := NewMarketClient()
	client.httpClient.Timeout = 50 * time.Millisecond

	_, err := client.CheckUserExists(context.Background(), "fixture@example.com")
	if !errors.Is(err, ErrMarketServiceUnavailable) {
		t.Fatalf("CheckUserExists() error = %v, want ErrMarketServiceUnavailable", err)
	}
	if !strings.Contains(err.Error(), "market integration is disabled") {
		t.Fatalf("CheckUserExists() error = %v, want disabled diagnostic", err)
	}
}

func TestCheckTemplateExistsClassifiesExternalFailures(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantError error
	}{
		{name: "server rejection", status: http.StatusBadGateway, body: `gateway down`, wantError: ErrMarketRequestRejected},
		{name: "invalid success payload", status: http.StatusOK, body: `not-json`, wantError: ErrMarketInvalidResponse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := &MarketClient{baseURL: server.URL, httpClient: server.Client()}
			_, err := client.CheckTemplateExists(context.Background(), "token", "fixture", "1.0.0")
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("CheckTemplateExists() error = %v, want %v", err, tt.wantError)
			}
		})
	}
}

func TestMarketTemplatePathEscapesTemplateID(t *testing.T) {
	got := marketTemplatePath("folder/template?draft#1", "/download")
	want := "/api/market/templates/folder%2Ftemplate%3Fdraft%231/download"
	if got != want {
		t.Fatalf("marketTemplatePath() = %q, want %q", got, want)
	}
}
