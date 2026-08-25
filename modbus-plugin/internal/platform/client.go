// 文件用途：从平台拉取设备点表（ROADMAP B1 前端配置界面的插件侧消费）。
// 核心逻辑：GET /api/v1/device/modbus/profile/number/{deviceNumber}（x-api-key 鉴权），
//   将返回的 target/registers 覆盖到本地设备配置；凭证字段由平台侧拒绝，本地文件也不下发。
// 关键注意事项：拉取失败不致命——保留本地回退配置并返回错误供调用方记录。
package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/config"
)

// Client 平台 HTTP API 客户端。
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// NewClient 创建平台客户端。
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: timeout},
	}
}

type platformEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

type platformProfilePayload struct {
	DeviceID string          `json:"device_id"`
	Profile  json.RawMessage `json:"profile"`
}

// FetchProfile 拉取并合并点表到设备配置。返回是否发生了覆盖。
func (c *Client) FetchProfile(ctx context.Context, device *config.DeviceConfig) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/device/modbus/profile/number/%s", c.BaseURL, device.DeviceNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("x-api-key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("platform profile http %d", resp.StatusCode)
	}
	var envelope platformEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false, fmt.Errorf("platform envelope: %w", err)
	}
	if envelope.Code != 200 || len(envelope.Data) == 0 {
		return false, fmt.Errorf("platform profile code %d", envelope.Code)
	}
	var payload platformProfilePayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return false, fmt.Errorf("platform payload: %w", err)
	}
	if len(payload.Profile) == 0 {
		return false, fmt.Errorf("platform profile is empty")
	}
	var profile struct {
		Target    *config.TargetConfig   `json:"target"`
		Registers []config.RegisterPoint `json:"registers"`
	}
	if err := json.Unmarshal(payload.Profile, &profile); err != nil {
		return false, fmt.Errorf("profile shape: %w", err)
	}
	changed := false
	if profile.Target != nil && strings.TrimSpace(profile.Target.Host) != "" {
		device.Target = *profile.Target
		if device.Target.Port <= 0 {
			device.Target.Port = config.DefaultTargetPort
		}
		if device.Target.TimeoutMs <= 0 {
			device.Target.TimeoutMs = config.DefaultTimeoutMs
		}
		changed = true
	}
	if len(profile.Registers) > 0 {
		for i := range profile.Registers {
			if err := profile.Registers[i].Normalize(); err != nil {
				return changed, fmt.Errorf("register %d: %w", i, err)
			}
		}
		device.Registers = profile.Registers
		changed = true
	}
	return changed, nil
}
