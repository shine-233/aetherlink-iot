// 文件用途：维护 plugin\admin\config.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package admin

import (
	"errors"
	"net"
)

// Config is the configuration for the admin plugin.
type Config struct {
	HTTP HTTPConfig `yaml:"http"`
	GRPC GRPCConfig `yaml:"grpc"`
}

// HTTPConfig is the configuration for http endpoint.
type HTTPConfig struct {
	// Enable indicates whether to expose http endpoint.
	Enable bool `yaml:"enable"`
	// Addr is the address that the http server listen on.
	Addr string `yaml:"http_addr"`
}

// GRPCConfig is the configuration for gRPC endpoint.
type GRPCConfig struct {
	// Addr is the address that the gRPC server listen on.
	Addr string `yaml:"http_addr"`
}

// Validate validates the configuration, and return an error if it is invalid.
func (c *Config) Validate() error {
	if c.HTTP.Enable {
		_, _, err := net.SplitHostPort(c.HTTP.Addr)
		if err != nil {
			return errors.New("invalid http_addr")
		}
	}
	_, _, err := net.SplitHostPort(c.GRPC.Addr)
	if err != nil {
		return errors.New("invalid grpc_addr")
	}
	return nil
}

// DefaultConfig is the default configuration.
var DefaultConfig = Config{
	HTTP: HTTPConfig{
		Enable: true,
		Addr:   "127.0.0.1:8083",
	},
	GRPC: GRPCConfig{
		Addr: "unix://./gmqttd.sock",
	},
}

func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type cfg Config
	var v = &struct {
		Admin cfg `yaml:"admin"`
	}{
		Admin: cfg(DefaultConfig),
	}
	if err := unmarshal(v); err != nil {
		return err
	}
	emptyGRPC := GRPCConfig{}
	if v.Admin.GRPC == emptyGRPC {
		v.Admin.GRPC = DefaultConfig.GRPC
	}
	emptyHTTP := HTTPConfig{}
	if v.Admin.HTTP == emptyHTTP {
		v.Admin.HTTP = DefaultConfig.HTTP
	}
	empty := cfg(Config{})
	if v.Admin == empty {
		v.Admin = cfg(DefaultConfig)
	}
	*c = Config(v.Admin)
	return nil
}
