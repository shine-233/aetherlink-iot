// 文件用途：Modbus TCP 插件配置模型与加载（ROADMAP B1）。
// 核心逻辑：声明平台 MQTT 上报通道、每台设备的 Modbus 目标与寄存器点表。
// 关键注意事项：插件以「设备自身的 MQTT 凭证」连接平台，设备归属由 broker 认证链路绑定，
//   因此每台设备需要独立的 username/password；点表支持 holding/input/coil/discrete 四类。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	DefaultPollIntervalSeconds = 10
	DefaultTargetPort          = 502
	DefaultTimeoutMs           = 3000
	DefaultTelemetryTopic      = "devices/telemetry"
	DefaultCommandTopicPrefix  = "devices/command"
)

// Config 插件根配置。
type Config struct {
	MQTT                MQTTConfig     `json:"mqtt"`
	Platform            PlatformConfig `json:"platform"`
	PollIntervalSeconds int            `json:"poll_interval_seconds"`
	HealthAddr          string         `json:"health_addr"`
	Devices             []DeviceConfig `json:"devices"`
}

// PlatformConfig 平台 HTTP API 拉取点表的连接信息（OpenAPI Key 鉴权）。
type PlatformConfig struct {
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key"`
	TimeoutMilli int    `json:"timeout_milli"`
}

// MQTTConfig 平台 broker 连接与 topic 约定。
type MQTTConfig struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	TelemetryTopic     string `json:"telemetry_topic"`
	CommandTopicPrefix string `json:"command_topic_prefix"`
}

// DeviceConfig 一台被采集的 Modbus 设备（对应平台上一台真实设备凭证）。
// use_platform_profile=true 时从平台拉取 target+registers 覆盖本地回退配置；
// 凭证（username/password）永远只存本地，平台侧点表不包含任何凭证字段。
type DeviceConfig struct {
	DeviceNumber       string          `json:"device_number"`
	Username           string          `json:"username"`
	Password           string          `json:"password"`
	ClientID           string          `json:"client_id"`
	UsePlatformProfile bool            `json:"use_platform_profile"`
	Target             TargetConfig    `json:"target"`
	Registers          []RegisterPoint `json:"registers"`
}

// TargetConfig Modbus TCP 从站目标。
type TargetConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	UnitID    uint8  `json:"unit_id"`
	TimeoutMs int    `json:"timeout_ms"`
}

// Load 从 JSON 文件读取并校验配置。
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate 校验必填项与点表合法性，填充默认值。
func (c *Config) Validate() error {
	if strings.TrimSpace(c.MQTT.Host) == "" {
		return fmt.Errorf("mqtt.host is required")
	}
	if c.MQTT.Port <= 0 {
		c.MQTT.Port = 1883
	}
	if strings.TrimSpace(c.MQTT.TelemetryTopic) == "" {
		c.MQTT.TelemetryTopic = DefaultTelemetryTopic
	}
	if strings.TrimSpace(c.MQTT.CommandTopicPrefix) == "" {
		c.MQTT.CommandTopicPrefix = DefaultCommandTopicPrefix
	}
	if c.PollIntervalSeconds <= 0 {
		c.PollIntervalSeconds = DefaultPollIntervalSeconds
	}
	if c.HealthAddr == "" {
		c.HealthAddr = ":8090"
	}
	if len(c.Devices) == 0 {
		return fmt.Errorf("devices must not be empty")
	}
	seenDevice := map[string]bool{}
	for i := range c.Devices {
		if err := c.Devices[i].validate(i, seenDevice); err != nil {
			return err
		}
	}
	return nil
}

func (d *DeviceConfig) validate(index int, seenDevice map[string]bool) error {
	if strings.TrimSpace(d.DeviceNumber) == "" {
		return fmt.Errorf("devices[%d].device_number is required", index)
	}
	if seenDevice[d.DeviceNumber] {
		return fmt.Errorf("devices[%d].device_number %q duplicated", index, d.DeviceNumber)
	}
	seenDevice[d.DeviceNumber] = true
	if !d.UsePlatformProfile {
		if strings.TrimSpace(d.Target.Host) == "" {
			return fmt.Errorf("devices[%d].target.host is required", index)
		}
	}
	if len(d.Registers) == 0 && !d.UsePlatformProfile {
		return fmt.Errorf("devices[%d].registers must not be empty (or enable use_platform_profile)", index)
	}
	if d.Target.Port <= 0 {
		d.Target.Port = DefaultTargetPort
	}
	if d.Target.TimeoutMs <= 0 {
		d.Target.TimeoutMs = DefaultTimeoutMs
	}
	seenKey := map[string]bool{}
	for j := range d.Registers {
		r := &d.Registers[j]
		if err := r.Normalize(); err != nil {
			return fmt.Errorf("devices[%d].registers[%d]: %w", index, j, err)
		}
		if seenKey[r.Key] {
			return fmt.Errorf("devices[%d].registers[%d].key %q duplicated", index, j, r.Key)
		}
		seenKey[r.Key] = true
	}
	return nil
}

// PollInterval 轮询间隔。
func (c *Config) PollInterval() time.Duration {
	return time.Duration(c.PollIntervalSeconds) * time.Second
}

// FindWritable 按点表键名查找可写寄存器（命令下发用）。
func (d *DeviceConfig) FindWritable(key string) (*RegisterPoint, bool) {
	for i := range d.Registers {
		if d.Registers[i].Key == key && d.Registers[i].Writable {
			return &d.Registers[i], true
		}
	}
	return nil, false
}
