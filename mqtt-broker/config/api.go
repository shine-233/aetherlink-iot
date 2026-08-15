// 文件用途：维护 config\api.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package config

import (
	"fmt"
	"net"
	"strings"
)

// API is the configuration for API server.
// The API server use gRPC-gateway to provide both gRPC and HTTP endpoints.
type API struct {
	// GRPC is the gRPC endpoint configuration.
	GRPC []*Endpoint `yaml:"grpc"`
	// HTTP is the HTTP endpoint configuration.
	HTTP []*Endpoint `yaml:"http"`
}

// Endpoint represents a gRPC or HTTP server endpoint.
type Endpoint struct {
	// Address is the bind address of the endpoint.
	// Format: [tcp|unix://][<host>]:<port>
	// e.g :
	// * unix:///var/run/gmqttd.sock
	// * tcp://127.0.0.1:8080
	// * :8081 (equal to tcp://:8081)
	Address string `yaml:"address"`
	// Map maps the HTTP endpoint to gRPC endpoint.
	// Must be set if the endpoint is representing a HTTP endpoint.
	Map string `yaml:"map"`
	// TLS is the tls configuration.
	TLS *TLSOptions `yaml:"tls"`
}

var DefaultAPI API

func (a API) validateAddress(address string, fieldName string) error {
	if address == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	epParts := strings.SplitN(address, "://", 2)
	if len(epParts) == 1 && epParts[0] != "" {
		epParts = []string{"tcp", epParts[0]}
	}
	if len(epParts) != 0 {
		switch epParts[0] {
		case "tcp":
			_, _, err := net.SplitHostPort(epParts[1])
			if err != nil {
				return fmt.Errorf("invalid %s: %s", fieldName, err.Error())
			}
		case "unix":
		default:
			return fmt.Errorf("invalid %s schema: %s", fieldName, epParts[0])
		}
	}
	return nil
}

func (a API) Validate() error {
	for _, v := range a.GRPC {
		err := a.validateAddress(v.Address, "endpoint")
		if err != nil {
			return err
		}
	}
	for _, v := range a.HTTP {
		err := a.validateAddress(v.Address, "endpoint")
		if err != nil {
			return err
		}
		err = a.validateAddress(v.Map, "map")
		if err != nil {
			return err
		}
	}
	return nil
}
