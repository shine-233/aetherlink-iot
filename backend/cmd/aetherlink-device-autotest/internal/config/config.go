// 文件用途：装载并校验设备自动测试工具的 YAML 配置。
// 核心逻辑：读取 MQTT、设备、网关、数据库、API 和测试参数，并生成运行所需的连接配置。
// 关键注意事项：配置校验只覆盖启动前必需字段，不证明 broker、API 或数据库真实可达。
// 重构建议：可拆分直连与网关专用校验，并补充默认值、敏感字段脱敏和本地配置一致性测试。

/*
Purpose: 装载并校验设备自动测试工具的 YAML 配置。
Core logic: 读取配置文件，反序列化 MQTT、设备、网关、数据库、API 和测试参数，并生成 PostgreSQL DSN。
Important notes: Validate 只做运行前必需字段检查，不验证 broker/API/database 是否真实可达；外部集成测试启动时仍会单独探测环境。
Refactor suggestion: 可拆分直连与网关专用校验，并为默认值、敏感字段脱敏和本地配置一致性增加表驱动测试。
*/
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 总配置结构
type Config struct {
	DeviceType string         `yaml:"device_type"` // "direct" 或 "gateway"
	MQTT       MQTTConfig     `yaml:"mqtt"`
	Device     DeviceConfig   `yaml:"device"`
	Gateway    GatewayConfig  `yaml:"gateway"` // 网关配置(当 device_type="gateway" 时使用)
	Database   DatabaseConfig `yaml:"database"`
	API        APIConfig      `yaml:"api"`
	Test       TestConfig     `yaml:"test"`
}

// MQTTConfig MQTT配置
type MQTTConfig struct {
	Broker       string `yaml:"broker"`
	ClientID     string `yaml:"client_id"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	QoS          byte   `yaml:"qos"`
	CleanSession bool   `yaml:"clean_session"`
	KeepAlive    int    `yaml:"keep_alive"`
}

// DeviceConfig 设备配置
type DeviceConfig struct {
	DeviceID     string `yaml:"device_id"`
	DeviceNumber string `yaml:"device_number"`
}

// SubDeviceConfig 子设备配置
type SubDeviceConfig struct {
	SubDeviceNumber string `yaml:"sub_device_number"`
	DeviceID        string `yaml:"device_id"`
	Description     string `yaml:"description"`
}

// SubGatewayConfig 子网关配置
type SubGatewayConfig struct {
	SubGatewayNumber string            `yaml:"sub_gateway_number"`
	DeviceID         string            `yaml:"device_id"`
	Description      string            `yaml:"description"`
	SubDevices       []SubDeviceConfig `yaml:"sub_devices"`
}

// GatewayConfig 网关配置
type GatewayConfig struct {
	SubDevices  []SubDeviceConfig  `yaml:"sub_devices"`
	SubGateways []SubGatewayConfig `yaml:"sub_gateways"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	DBName       string `yaml:"dbname"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	SSLMode      string `yaml:"sslmode"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

// APIConfig API配置
type APIConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
	Timeout int    `yaml:"timeout"`
}

// TestConfig 测试配置
type TestConfig struct {
	WaitDBSyncSeconds       int    `yaml:"wait_db_sync_seconds"`
	WaitMQTTResponseSeconds int    `yaml:"wait_mqtt_response_seconds"`
	RetryTimes              int    `yaml:"retry_times"`
	LogLevel                string `yaml:"log_level"`
	CommandFailureIdentify  string `yaml:"command_failure_identify"`
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	if err := applyEnvironmentOverrides(&cfg); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// applyEnvironmentOverrides keeps checked-in example YAML safe while making
// local integration runs reproducible against an isolated broker/database.
// Empty variables leave the file value unchanged; secrets are never logged.
func applyEnvironmentOverrides(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if value := strings.TrimSpace(os.Getenv("AUTOTEST_MQTT_BROKER")); value != "" {
		cfg.MQTT.Broker = value
	}
	if value := strings.TrimSpace(os.Getenv("AUTOTEST_MQTT_SERVER")); value != "" {
		port := strings.TrimSpace(os.Getenv("AUTOTEST_MQTT_PORT"))
		if port == "" {
			return fmt.Errorf("AUTOTEST_MQTT_PORT is required when AUTOTEST_MQTT_SERVER is set")
		}
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort <= 0 || parsedPort > 65535 {
			return fmt.Errorf("AUTOTEST_MQTT_PORT must be between 1 and 65535, got %q", port)
		}
		cfg.MQTT.Broker = value + ":" + strconv.Itoa(parsedPort)
	}
	if value := os.Getenv("AUTOTEST_MQTT_CLIENT_ID"); value != "" {
		cfg.MQTT.ClientID = value
	}
	if value := os.Getenv("AUTOTEST_MQTT_USERNAME"); value != "" {
		cfg.MQTT.Username = value
	}
	if value := os.Getenv("AUTOTEST_MQTT_PASSWORD"); value != "" {
		cfg.MQTT.Password = value
	}
	if value := os.Getenv("AUTOTEST_DEVICE_ID"); value != "" {
		cfg.Device.DeviceID = value
	}
	if value := os.Getenv("AUTOTEST_DEVICE_NUMBER"); value != "" {
		cfg.Device.DeviceNumber = value
	}
	if value := os.Getenv("AUTOTEST_API_BASE_URL"); value != "" {
		cfg.API.BaseURL = value
	}
	if value := os.Getenv("AUTOTEST_API_KEY"); value != "" {
		cfg.API.APIKey = value
	}
	if value := strings.TrimSpace(os.Getenv("AUTOTEST_COMMAND_FAILURE_IDENTIFY")); value != "" {
		cfg.Test.CommandFailureIdentify = value
	}
	if value := os.Getenv("AUTOTEST_DATABASE_HOST"); value != "" {
		cfg.Database.Host = value
	}
	if value := os.Getenv("AUTOTEST_DATABASE_NAME"); value != "" {
		cfg.Database.DBName = value
	}
	if value := os.Getenv("AUTOTEST_DATABASE_USER"); value != "" {
		cfg.Database.Username = value
	}
	if value := os.Getenv("AUTOTEST_DATABASE_PASSWORD"); value != "" {
		cfg.Database.Password = value
	}
	if value := os.Getenv("AUTOTEST_DATABASE_SSLMODE"); value != "" {
		cfg.Database.SSLMode = value
	}
	if value := strings.TrimSpace(os.Getenv("AUTOTEST_DATABASE_PORT")); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port <= 0 || port > 65535 {
			return fmt.Errorf("AUTOTEST_DATABASE_PORT must be between 1 and 65535, got %q", value)
		}
		cfg.Database.Port = port
	}
	return nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 设备类型缺省时默认走直连模式，保持最简单的本地配置可以直接启动。
	if c.DeviceType == "" {
		c.DeviceType = "direct" // 默认为直连设备
	}
	if c.DeviceType != "direct" && c.DeviceType != "gateway" {
		return fmt.Errorf("device_type must be 'direct' or 'gateway', got: %s", c.DeviceType)
	}

	// 当前校验只覆盖启动所需字段，不检查 broker、API、数据库是否真实可达。
	if c.MQTT.Broker == "" {
		return fmt.Errorf("mqtt broker is required")
	}
	if c.Device.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}
	if c.Device.DeviceNumber == "" {
		return fmt.Errorf("device_number is required")
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.API.BaseURL == "" {
		return fmt.Errorf("api base_url is required")
	}
	if c.API.APIKey == "" {
		return fmt.Errorf("api api_key is required")
	}
	return nil
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	// DSN 仅负责拼接参数，不在这里做额外转义或敏感信息脱敏。
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.Username, c.Password, c.DBName, c.SSLMode)
}
