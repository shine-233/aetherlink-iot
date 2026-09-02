// 文件用途：集中提供 LIKE 模糊查询通配符转义工具。
// 核心逻辑：将用户输入中的 %、_ 与反斜杠转义为字面量，防止通配符注入与全表扫描式模糊攻击。
// 关键注意事项：PostgreSQL 默认转义符为反斜杠；所有把用户输入拼进 LIKE 模式的位置必须经过本函数。
package dal

import "strings"

// EscapeLikePattern 转义用户输入，使其在 SQL LIKE 模式中按字面量匹配。
// 约定：先转义反斜杠本身，再转义 % 和 _，与 PostgreSQL 默认转义规则一致。
func EscapeLikePattern(input string) string {
	escaped := strings.ReplaceAll(input, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, "%", `\%`)
	escaped = strings.ReplaceAll(escaped, "_", `\_`)
	return escaped
}

// ContainsLikePattern 返回包裹两侧 % 的字面量包含匹配模式。
func ContainsLikePattern(input string) string {
	return "%" + EscapeLikePattern(input) + "%"
}
