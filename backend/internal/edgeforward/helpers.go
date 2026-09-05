// 文件用途：edgeforward 内部助手（JSON 编码与缓冲条目构造）。
package edgeforward

import (
	"bytes"
	"encoding/json"
	"strconv"
)

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// appendJSONString 追加一个 JSON 字符串字面量（含引号与转义）。
func appendJSONString(buf []byte, s string) []byte {
	b, _ := json.Marshal(s)
	return append(buf, b...)
}

func appendInt64(buf []byte, n int64) []byte {
	return append(buf, strconv.FormatInt(n, 10)...)
}

// isJSONObject 判断 payload 是否已是 JSON 对象/数组（否则按字符串内嵌）。
func isJSONObject(b []byte) bool {
	t := bytes.TrimSpace(b)
	return len(t) > 0 && (t[0] == '{' || t[0] == '[')
}
