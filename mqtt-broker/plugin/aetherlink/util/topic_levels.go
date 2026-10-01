// 文件用途：ACL 白名单匹配共用的主题分层工具。
// 核心逻辑：把主题按 '/' 切成子串写入调用方栈上的定长数组，免去每次校验 strings.Split 的堆分配。
// 关键注意事项：白名单模式最多 maxPatternLevels 层（包初始化时断言）；主题层数超过该值
// 必然与所有模式层数不等，splitTopicLevels 返回 ok=false，调用方直接判为不匹配，语义与逐条比对层数一致。
package util

import "strings"

// maxPatternLevels 白名单模式的最大层数上限（当前最长模式为 5 层）。
const maxPatternLevels = 8

// topicLevels 承载一次分层结果的栈上缓冲。
type topicLevels [maxPatternLevels]string

// splitTopicLevels 按 strings.Split 语义切分 topic 到 buf；层数超过上限时返回 ok=false。
func splitTopicLevels(topic string, buf *topicLevels) (parts []string, ok bool) {
	n := 0
	for {
		if n == maxPatternLevels {
			return nil, false
		}
		pos := strings.IndexByte(topic, '/')
		if pos < 0 {
			buf[n] = topic
			return buf[:n+1], true
		}
		buf[n] = topic[:pos]
		topic = topic[pos+1:]
		n++
	}
}

// mustFitLevels 包初始化时校验模式层数不超过栈缓冲上限，防止新增长模式后被静默判为不匹配。
func mustFitLevels(pattern string) []string {
	parts := strings.Split(pattern, "/")
	if len(parts) > maxPatternLevels {
		panic("util: ACL topic pattern exceeds maxPatternLevels: " + pattern)
	}
	return parts
}
