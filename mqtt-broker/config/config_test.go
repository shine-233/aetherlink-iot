// 文件用途：维护 config\config_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseConfig(t *testing.T) {
	var tt = []struct {
		caseName string
		fileName string
		hasErr   bool
		expected Config
	}{
		{
			caseName: "defaultConfig",
			fileName: "",
			hasErr:   false,
			expected: DefaultConfig(),
		},
	}

	for _, v := range tt {
		t.Run(v.caseName, func(t *testing.T) {
			a := assert.New(t)
			c, err := ParseConfig(v.fileName)
			if v.hasErr {
				a.NotNil(err)
			} else {
				a.Nil(err)
			}
			a.Equal(v.expected, c)
		})
	}
}
