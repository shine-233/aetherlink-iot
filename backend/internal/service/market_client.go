// 文件用途：维护市场服务 HTTP 客户端和模板包数据转换逻辑。
// 核心功能：封装市场 API 请求、鉴权 token、响应解析、本地安装与发布流程需要的数据映射。
// 注意事项：市场服务属于外部依赖，未配置或仍使用占位地址时必须显式失败；出站调用受总超时和轻量熔断保护。
// 审查建议：若引入重试，只能针对幂等操作并配合退避和抖动，避免在故障期间放大流量。
//
// It calls external or internal market APIs, imports templates/packages, and
// maps marketplace responses into backend domain models. Network and payload
// changes should be isolated and tested because install flows depend on them.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"

	"github.com/golang-jwt/jwt/v4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// 市场服务角色常量，用于安装回调时向市场侧传递授权上下文。
const (
	MarketRoleOrgAdmin   = "org_admin"
	MarketRoleSuperAdmin = "super_admin"
)

// MarketClient handles communication with the external market API.
type MarketClient struct {
	// enabled is set by NewMarketClient. A nil value is reserved for isolated
	// same-package HTTP contract tests that construct the client directly.
	enabled    *bool
	baseURL    string
	httpClient *http.Client
}

const marketFallbackBaseURL = "https://market.example.com"

var (
	// ErrMarketServiceUnavailable indicates network/connectivity/base URL issues.
	ErrMarketServiceUnavailable = errors.New("market service unavailable")
	// ErrMarketRequestRejected indicates non-200 HTTP responses (except explicit not-found).
	ErrMarketRequestRejected = errors.New("market request rejected")
	// ErrMarketInvalidResponse indicates unexpected or malformed market response payloads.
	ErrMarketInvalidResponse = errors.New("market response invalid")
)

func getMarketBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(viper.GetString("market.base_url")), "/")
}

func isConfiguredMarketBaseURL(rawURL string) bool {
	baseURL := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if baseURL == "" || baseURL == marketFallbackBaseURL {
		return false
	}
	parsed, err := url.Parse(baseURL)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// NewMarketClient creates a client from the current configuration. Market is
// fail-closed: a missing market.enabled key is treated as disabled.
func NewMarketClient() *MarketClient {
	enabled := viper.GetBool("market.enabled")
	return &MarketClient{
		enabled: &enabled,
		baseURL: getMarketBaseURL(),
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: newMarketCircuitBreakerTransport(http.DefaultTransport),
		},
	}
}

// CheckUserExists checks if a user with the given email exists in the market.
func (c *MarketClient) CheckUserExists(ctx context.Context, email string) (bool, error) {
	endpoint, err := c.marketEndpoint("/api/account/auth/user/exists")
	if err != nil {
		return false, err
	}
	query := endpoint.Query()
	query.Set("email", email)
	endpoint.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create check user request: %w", err)
	}

	statusCode, bodyBytes, err := c.readMarketResponse(
		httpReq,
		func(err error) error {
			return fmt.Errorf("%w: %v", ErrMarketServiceUnavailable, err)
		},
		func(err error) error {
			return fmt.Errorf("%w: failed to read check user response: %v", ErrMarketInvalidResponse, err)
		},
	)
	if err != nil {
		return false, err
	}

	if statusCode == http.StatusNotFound {
		return false, nil
	}

	// 其他非 200 响应视为请求失败；只有明确的“用户不存在”语义会返回 false。
	if statusCode != http.StatusOK {
		if isUserNotFoundResponse(statusCode, bodyBytes) {
			return false, nil
		}
		return false, fmt.Errorf("%w: status=%d body=%s", ErrMarketRequestRejected, statusCode, compactMarketBody(bodyBytes))
	}

	exists, err := parseExistsFromBody(bodyBytes)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func parseExistsFromBody(bodyBytes []byte) (bool, error) {
	bodyBytes = bytes.TrimSpace(bodyBytes)
	if len(bodyBytes) == 0 {
		return false, fmt.Errorf("%w: empty response body", ErrMarketInvalidResponse)
	}

	var result struct {
		Exists *bool           `json:"exists"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return false, fmt.Errorf("%w: failed to parse check user response: %v", ErrMarketInvalidResponse, err)
	}

	if result.Exists != nil {
		return *result.Exists, nil
	}

	if len(result.Data) > 0 && string(result.Data) != "null" {
		var existsBool bool
		if err := json.Unmarshal(result.Data, &existsBool); err == nil {
			return existsBool, nil
		}

		var nested struct {
			Exists *bool `json:"exists"`
		}
		if err := json.Unmarshal(result.Data, &nested); err == nil && nested.Exists != nil {
			return *nested.Exists, nil
		}
	}

	return false, fmt.Errorf("%w: missing exists field, body=%s", ErrMarketInvalidResponse, compactMarketBody(bodyBytes))
}

func compactMarketBody(bodyBytes []byte) string {
	body := strings.TrimSpace(string(bodyBytes))
	if body == "" {
		return "<empty>"
	}

	const maxBodyLen = 256
	if len(body) > maxBodyLen {
		return body[:maxBodyLen] + "..."
	}
	return body
}

func isUserNotFoundResponse(statusCode int, bodyBytes []byte) bool {
	if statusCode == http.StatusNotFound {
		return true
	}

	// 仅对常见客户端错误识别“用户不存在”，避免把服务端故障误判为不存在。
	if statusCode != http.StatusBadRequest && statusCode != http.StatusUnprocessableEntity {
		return false
	}

	lowerBody := strings.ToLower(strings.TrimSpace(string(bodyBytes)))
	if lowerBody != "" {
		if strings.Contains(lowerBody, "not found") ||
			strings.Contains(lowerBody, "not exists") ||
			strings.Contains(lowerBody, "email not found") ||
			strings.Contains(lowerBody, "\u4e0d\u5b58\u5728") ||
			strings.Contains(lowerBody, "\u672a\u6ce8\u518c") {
			return true
		}
	}

	var errResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Error   string `json:"error"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		return false
	}

	if errResp.Code == 200015 {
		return true
	}

	msg := strings.ToLower(strings.TrimSpace(errResp.Message + " " + errResp.Error + " " + errResp.Msg))
	if msg == "" {
		return false
	}

	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "not exists") ||
		strings.Contains(msg, "email not found") ||
		strings.Contains(msg, "\u4e0d\u5b58\u5728") ||
		strings.Contains(msg, "\u672a\u6ce8\u518c")
}

func (c *MarketClient) marketEndpoint(apiPath string) (*url.URL, error) {
	// Production clients always carry an explicit enable decision. A nil value
	// is reserved for isolated same-package HTTP contract tests.
	if c.enabled != nil && !*c.enabled {
		return nil, fmt.Errorf("%w: market integration is disabled", ErrMarketServiceUnavailable)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(c.baseURL), "/")
	if baseURL == "" || baseURL == marketFallbackBaseURL {
		return nil, fmt.Errorf("%w: market.base_url is not configured", ErrMarketServiceUnavailable)
	}
	if !isConfiguredMarketBaseURL(baseURL) {
		return nil, fmt.Errorf("%w: invalid market base_url", ErrMarketServiceUnavailable)
	}

	endpoint, err := url.Parse(baseURL + apiPath)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid market base_url: %v", ErrMarketServiceUnavailable, err)
	}
	return endpoint, nil
}

func (c *MarketClient) readMarketResponse(httpReq *http.Request, requestErr func(error) error, readErr func(error) error) (int, []byte, error) {
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, nil, requestErr(err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, readErr(err)
	}

	return resp.StatusCode, bodyBytes, nil
}

// Login authenticates with the market service to get an access token.
func (c *MarketClient) Login(ctx context.Context, username, password string) (string, error) {
	reqBody := map[string]string{
		"username": username,
		"password": password,
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal login body: %w", err)
	}

	endpoint, err := c.marketEndpoint("/api/account/auth/login")
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	statusCode, bodyBytes, err := c.readMarketResponse(
		httpReq,
		func(err error) error {
			return fmt.Errorf("login request failed: %w", err)
		},
		func(err error) error {
			return fmt.Errorf("failed to read login response: %w", err)
		},
	)
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		var errResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		json.Unmarshal(bodyBytes, &errResp)
		if errResp.Message != "" {
			return "", fmt.Errorf("%s", errResp.Message)
		}
		if errResp.Error != "" {
			return "", fmt.Errorf("%s", errResp.Error)
		}
		return "", fmt.Errorf("login failed with status: %d", statusCode)
	}

	var loginResp model.MarketLoginRsp
	if err := json.Unmarshal(bodyBytes, &loginResp); err != nil {
		return "", fmt.Errorf("failed to parse login response: %w", err)
	}

	return loginResp.Token, nil
}

// PublishTemplate publishes a template to the market.
func (c *MarketClient) PublishTemplate(ctx context.Context, token string, userID string, req *model.PublishTemplateReq) (*model.MarketPublishApiResponse, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	endpoint, err := c.marketEndpoint("/api/market/templates/publish")
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	if userID != "" {
		httpReq.Header.Set("X-User-Id", userID)
	}

	statusCode, bodyBytes, err := c.readMarketResponse(
		httpReq,
		func(err error) error {
			return fmt.Errorf("http request failed: %w", err)
		},
		func(err error) error {
			return fmt.Errorf("failed to read response body: %w", err)
		},
	)
	if err != nil {
		return nil, err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: publish status=%d body=%s", ErrMarketRequestRejected, statusCode, compactMarketBody(bodyBytes))
	}

	var apiResp model.MarketPublishApiResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("%w: failed to parse publish response: %v", ErrMarketInvalidResponse, err)
	}

	return &apiResp, nil
}

// CheckTemplateExists checks if a template with the given name+version already exists on the market.
func (c *MarketClient) CheckTemplateExists(ctx context.Context, token string, name, version string) (bool, error) {
	endpoint, err := c.marketEndpoint("/api/market/templates")
	if err != nil {
		return false, err
	}
	query := endpoint.Query()
	query.Set("name", name)
	query.Set("version", version)
	endpoint.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint.String(), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create check request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	statusCode, bodyBytes, err := c.readMarketResponse(
		httpReq,
		func(err error) error {
			return fmt.Errorf("check request failed: %w", err)
		},
		func(err error) error {
			return fmt.Errorf("failed to read check response: %w", err)
		},
	)
	if err != nil {
		return false, err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return false, fmt.Errorf("%w: template lookup status=%d body=%s", ErrMarketRequestRejected, statusCode, compactMarketBody(bodyBytes))
	}

	var result struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return false, fmt.Errorf("%w: failed to parse template lookup response: %v", ErrMarketInvalidResponse, err)
	}

	return result.Data.Total > 0, nil
}

// ListMarketTemplates fetches the list of templates from the market (public, no token needed).
func (c *MarketClient) ListMarketTemplates(ctx context.Context, keyword, category, sortBy string, page, pageSize int) (interface{}, error) {
	endpoint, err := c.marketEndpoint("/api/market/templates")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("page", fmt.Sprintf("%d", page))
	query.Set("page_size", fmt.Sprintf("%d", pageSize))
	if keyword != "" {
		query.Set("keyword", keyword)
	}
	if category != "" {
		query.Set("category", category)
	}
	if sortBy != "" {
		query.Set("sort_by", sortBy)
	}
	endpoint.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list request: %w", err)
	}

	_, bodyBytes, err := c.readMarketResponse(
		httpReq,
		func(err error) error {
			return fmt.Errorf("list request failed: %w", err)
		},
		func(err error) error {
			return fmt.Errorf("failed to read list response: %w", err)
		},
	)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	return flattenMarketTemplateListResponse(result, page, pageSize), nil
}

func flattenMarketTemplateListResponse(result map[string]interface{}, requestedPage, requestedPageSize int) map[string]interface{} {
	dataMap, _ := result["data"].(map[string]interface{})
	list := marketTemplateListPayload(result["data"])

	flattened := map[string]interface{}{
		"list":      list,
		"total":     len(list),
		"page":      requestedPage,
		"page_size": requestedPageSize,
	}

	setMarketListField(flattened, "total", result, dataMap, "total")
	setMarketListField(flattened, "page", result, dataMap, "page")
	setMarketListField(flattened, "page_size", result, dataMap, "page_size", "pageSize")

	return flattened
}

func marketTemplateListPayload(data interface{}) []interface{} {
	switch payload := data.(type) {
	case []interface{}:
		return payload
	case map[string]interface{}:
		for _, key := range []string{"list", "items", "records", "templates"} {
			if list, ok := payload[key].([]interface{}); ok {
				return list
			}
		}
	}
	return []interface{}{}
}

func setMarketListField(dst map[string]interface{}, dstKey string, top, nested map[string]interface{}, sourceKeys ...string) {
	for _, key := range sourceKeys {
		if value, ok := top[key]; ok && value != nil {
			dst[dstKey] = value
			return
		}
	}
	for _, key := range sourceKeys {
		if value, ok := nested[key]; ok && value != nil {
			dst[dstKey] = value
			return
		}
	}
}

func marketTemplatePath(marketTemplateID, suffix string) string {
	return "/api/market/templates/" + url.PathEscape(marketTemplateID) + suffix
}

// GetMarketTemplateDetail fetches a single template's detail from the market.
func (c *MarketClient) GetMarketTemplateDetail(ctx context.Context, marketTemplateID string) (interface{}, error) {
	endpoint, err := c.marketEndpoint(marketTemplatePath(marketTemplateID, ""))
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create detail request: %w", err)
	}

	_, bodyBytes, err := c.readMarketResponse(
		httpReq,
		func(err error) error {
			return fmt.Errorf("detail request failed: %w", err)
		},
		func(err error) error {
			return fmt.Errorf("failed to read detail response: %w", err)
		},
	)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse detail response: %w", err)
	}

	// 提取内层数据，去掉市场服务的 code 包装。
	if data, ok := result["data"].(map[string]interface{}); ok {
		// 如果包含 template 和 versions，则合并为前端详情页更容易消费的结构。
		if tpl, ok := data["template"].(map[string]interface{}); ok {
			if vers, ok := data["versions"]; ok {
				tpl["versions"] = vers
			}
			return tpl, nil
		}
		return data, nil
	}

	return result, nil
}

// DownloadTemplate downloads the full thing-model definition from the market.
func (c *MarketClient) DownloadTemplate(ctx context.Context, token string, marketTemplateID string, version string) (*model.MarketTemplateFullData, error) {
	endpoint, err := c.marketEndpoint(marketTemplatePath(marketTemplateID, "/download"))
	if err != nil {
		return nil, err
	}
	if version != "" {
		query := endpoint.Query()
		query.Set("version", version)
		endpoint.RawQuery = query.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	statusCode, bodyBytes, err := c.readMarketResponse(
		httpReq,
		func(err error) error {
			return fmt.Errorf("download request failed: %w", err)
		},
		func(err error) error {
			return fmt.Errorf("failed to read download response: %w", err)
		},
	)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d: %s", statusCode, string(bodyBytes))
	}

	var result struct {
		Code int                          `json:"code"`
		Data model.MarketTemplateFullData `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse download response: %w", err)
	}

	logrus.Debugf("[MarketClient] DownloadTemplate: MarketTemplateID=%s, VersionID=%s, Version=%s, Name=%s",
		marketTemplateID, result.Data.VersionID, result.Data.Version, result.Data.Name)

	return &result.Data, nil
}

// ExtractUserIDFromMarketToken parses the market (Keycloak) JWT and returns the subject (user_id).
// The market credit account is keyed by this ID, not by the IoT platform's user ID.
func (c *MarketClient) ExtractUserIDFromMarketToken(tokenString string) (string, error) {
	if tokenString == "" {
		return "", fmt.Errorf("empty token")
	}
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil || token == nil {
		return "", fmt.Errorf("parse market token: %w", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", fmt.Errorf("token missing sub claim")
	}
	return sub, nil
}

// InstallTemplate notifies the market service that a template has been installed.
func (c *MarketClient) InstallTemplate(ctx context.Context, token string, marketTemplateID string, versionID string, userID string, orgID string) error {
	endpoint, err := c.marketEndpoint(marketTemplatePath(marketTemplateID, "/install"))
	if err != nil {
		return err
	}
	reqBody := map[string]string{
		"version_id": versionID,
	}
	reqBytes, _ := json.Marshal(reqBody)

	logrus.Debugf("[MarketClient] InstallTemplate: URL=%s, MarketTemplateID=%s, VersionID=%s, UserID=%s, OrgID=%s", endpoint.String(), marketTemplateID, versionID, userID, orgID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(reqBytes))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	if userID != "" {
		httpReq.Header.Set("X-User-Id", userID)
		httpReq.Header.Set("X-Org-Id", orgID)
		// 市场安装通知允许安装者以 org_admin 和 super_admin 角色操作。
		httpReq.Header.Set("X-Roles", MarketRoleOrgAdmin+","+MarketRoleSuperAdmin)
	}

	statusCode, bodyBytes, err := c.readMarketResponse(
		httpReq,
		func(err error) error {
			return err
		},
		func(err error) error {
			return err
		},
	)
	if err != nil {
		return err
	}

	if statusCode != http.StatusCreated && statusCode != http.StatusOK {
		return fmt.Errorf("install notification failed with status %d: %s", statusCode, string(bodyBytes))
	}

	return nil
}
