// 文件用途：验证 LIKE 通配符转义工具的字面量语义与边界输入。
// 核心逻辑：钉死 %、_、反斜杠的转义顺序契约（先反斜杠后通配符），防止回归为可注入模式。
// 关键注意事项：LIKE 安全边界测试；任何绕过 EscapeLikePattern 的拼接都应被视为缺陷。
package dal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeLikePatternEscapesWildcardsAndBackslash(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain text unchanged", input: "device-A1", want: "device-A1"},
		{name: "percent wildcard escaped", input: "100%", want: `100\%`},
		{name: "underscore wildcard escaped", input: "dev_ice", want: `dev\_ice`},
		{name: "backslash escaped first", input: `a\b`, want: `a\\b`},
		{name: "mixed injection attempt", input: `%_\`, want: `\%\_\\`},
		{name: "empty string stays empty", input: "", want: ""},
		{name: "cjk text unchanged", input: "温度传感器", want: "温度传感器"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, EscapeLikePattern(tc.input))
		})
	}
}

func TestContainsLikePatternWrapsEscapedValue(t *testing.T) {
	assert.Equal(t, `%foo\_bar\%baz%`, ContainsLikePattern("foo_bar%baz"))
	assert.Equal(t, "%%", ContainsLikePattern(""))
}

// 回归防线：确保转义后的值拼进 LIKE 模式后不再改变模式结构。
func TestEscapedPatternKeepsLikeShapeStable(t *testing.T) {
	pattern := fmt.Sprintf("%%%s%%", EscapeLikePattern(`a%b_c\d`))
	assert.Equal(t, `%a\%b\_c\\d%`, pattern)
}
