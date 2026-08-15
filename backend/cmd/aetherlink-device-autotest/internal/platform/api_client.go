// 文件用途：封装自动测试对 AetherLink IoT HTTP API 的下行操作。
// 核心逻辑：使用 API 配置发起 JSON 请求，携带 x-api-key，并提供遥测控制、属性设置、属性获取和命令下发方法。
// 关键注意事项：客户端假定响应体符合 APIResponse 且 code=200 表示成功，运行依赖真实 API 服务和有效凭证。
// 重构建议：可注入 http.RoundTripper 并增加 URL/query 编码测试，降低真实服务依赖。

package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"

	"aetherlink-iot/aetherlink-device-autotest/internal/config"
)

// APIResponse API响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// APIClient HTTP API客户端
type APIClient struct {
	config     *config.APIConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewAPIClient 创建API客户端
func NewAPIClient(cfg *config.APIConfig, logger *zap.Logger) *APIClient {
	return &APIClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
		logger: logger,
	}
}

// doRequest 执行HTTP请求
func (c *APIClient) doRequest(method, path string, body interface{}) (*APIResponse, error) {
	// 所有平台下行入口都复用这一层，集中处理鉴权头、序列化和响应解码约定。
	url := c.config.BaseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
		c.logger.Debug("API request",
			zap.String("method", method),
			zap.String("url", url),
			zap.String("body", string(jsonData)))
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Debug("API response",
		zap.Int("status_code", resp.StatusCode),
		zap.String("body", string(respBody)))

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 当前约定 code=200 才算平台业务成功，HTTP 200 但业务失败也会在这里返回错误。
	if apiResp.Code != 200 {
		return &apiResp, fmt.Errorf("API error: code=%d, message=%s", apiResp.Code, apiResp.Message)
	}

	return &apiResp, nil
}

// PublishTelemetryControl 下发遥测控制
func (c *APIClient) PublishTelemetryControl(deviceID string, value interface{}) error {
	// 平台接口当前要求 value 以 JSON 字符串形式透传，而不是直接嵌套对象。
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	body := map[string]interface{}{
		"device_id": deviceID,
		"value":     string(valueJSON), // 转换为字符串
	}

	_, err = c.doRequest("POST", "/api/v1/telemetry/datas/pub", body)
	if err != nil {
		return fmt.Errorf("publish telemetry control failed: %w", err)
	}

	c.logger.Info("Telemetry control published",
		zap.String("device_id", deviceID),
		zap.String("value", string(valueJSON)))

	return nil
}

// PublishAttributeSet 下发属性设置
func (c *APIClient) PublishAttributeSet(deviceID string, value interface{}) error {
	// 与遥测控制保持同一接口口径，value 在请求体中仍是字符串化 JSON。
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	body := map[string]interface{}{
		"device_id": deviceID,
		"value":     string(valueJSON), // 转换为字符串
	}

	_, err = c.doRequest("POST", "/api/v1/attribute/datas/pub", body)
	if err != nil {
		return fmt.Errorf("publish attribute set failed: %w", err)
	}

	c.logger.Info("Attribute set published",
		zap.String("device_id", deviceID),
		zap.String("value", string(valueJSON)))

	return nil
}

// GetAttribute 请求获取属性
func (c *APIClient) GetAttribute(deviceID string, keys []string) error {
	query := url.Values{}
	query.Set("device_id", deviceID)

	if len(keys) > 0 {
		keysJSON, err := json.Marshal(keys)
		if err != nil {
			return fmt.Errorf("failed to marshal keys: %w", err)
		}
		query.Set("keys", string(keysJSON))
	}

	path := fmt.Sprintf("/api/v1/attribute/datas/get?%s", query.Encode())
	_, err := c.doRequest("GET", path, nil)
	if err != nil {
		return fmt.Errorf("get attribute failed: %w", err)
	}

	c.logger.Info("Get attribute request sent",
		zap.String("device_id", deviceID),
		zap.Strings("keys", keys))

	return nil
}

// PublishCommand 下发命令
func (c *APIClient) PublishCommand(deviceID string, identify string, value interface{}) error {
	// 命令值同样走字符串化 JSON；`Identify` 字段大小写也受平台接口契约约束。
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	body := map[string]interface{}{
		"device_id": deviceID,
		"Identify":  identify,
		"value":     string(valueJSON), // 转换为字符串
	}

	_, err = c.doRequest("POST", "/api/v1/command/datas/pub", body)
	if err != nil {
		return fmt.Errorf("publish command failed: %w", err)
	}

	c.logger.Info("Command published",
		zap.String("device_id", deviceID),
		zap.String("identify", identify),
		zap.String("value", string(valueJSON)))

	return nil
}
