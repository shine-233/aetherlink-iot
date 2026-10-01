// 文件用途：日志值的控制字符净化，防止外部可控内容伪造日志行（CodeQL go/log-injection）。
// 核心逻辑：CR/LF/TAB 转义为可见形式，其余 C0/C1 控制字符剔除；内容不截断、语义不变。
// 关键注意事项：只用于日志输出路径，不得替代业务层校验；结构化字段的值同样需要净化。
package utils

import "strings"

var logUnsafeReplacer = strings.NewReplacer(
	"\r", `\r`,
	"\n", `\n`,
	"\t", `\t`,
)

// SanitizeForLog 返回可安全写入单行日志的字符串。
func SanitizeForLog(s string) string {
	if !strings.ContainsFunc(s, func(r rune) bool {
		return r == '\r' || r == '\n' || r == '\t' || (r < 0x20 && r >= 0) || r == 0x7f
	}) {
		return s
	}
	s = logUnsafeReplacer.Replace(s)
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}
