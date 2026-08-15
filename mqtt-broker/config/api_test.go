// 文件用途：维护 config\api_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPI_Validate(t *testing.T) {
	a := assert.New(t)

	tt := []struct {
		cfg   API
		valid bool
	}{
		{
			cfg: API{
				GRPC: []*Endpoint{
					{
						Address: "udp://127.0.0.1",
					},
				},
				HTTP: []*Endpoint{
					{},
				},
			},
			valid: false,
		},
		{
			cfg: API{
				GRPC: []*Endpoint{
					{
						Address: "tcp://127.0.0.1:1234",
					},
				},
				HTTP: []*Endpoint{
					{
						Address: "udp://127.0.0.1",
					},
				},
			},
			valid: false,
		},
		{
			cfg: API{
				GRPC: []*Endpoint{
					{
						Address: "tcp://127.0.0.1:1234",
					},
				},
			},
			valid: true,
		},
		{
			cfg: API{
				GRPC: []*Endpoint{
					{
						Address: "tcp://127.0.0.1:1234",
					},
				},
				HTTP: []*Endpoint{
					{
						Address: "tcp://127.0.0.1:1235",
					},
				},
			},
			valid: false,
		},
		{
			cfg: API{
				GRPC: []*Endpoint{
					{
						Address: "unix:///var/run/gmqttd.sock",
					},
				},
				HTTP: []*Endpoint{
					{
						Address: "tcp://127.0.0.1:1235",
						Map:     "unix:///var/run/gmqttd.sock",
					},
				},
			},
			valid: true,
		},
	}
	for _, v := range tt {
		err := v.cfg.Validate()
		if v.valid {
			a.NoError(err)
		} else {
			a.Error(err)
		}
	}

}
