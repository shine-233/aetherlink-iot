// 文件用途：计算字段规则集（装载期预计算的热路径元数据）。
// 核心逻辑：模板规则在缓存刷新时编译一次，同时汇总出所有规则会读取的 payload 键；
// processMessage 据此只解码需要的键，模板无规则时完全不解码 payload。
// 关键注意事项：若任一规则不依赖具体键（related_agg、零变量表达式）却受"payload 非空"门控，
// 则必须回退全量解码以保持原有门控语义（fullDecode=true）。
package calcfield

import (
	"encoding/json"
	"math"
	"strconv"
	"unicode/utf8"
)

type ruleSet struct {
	rules []compiledRule
	// wanted 为全部规则会读取的键（value 与 key 相同，用作驻留字符串）。
	wanted map[string]string
	// fullDecode 表示存在不依赖具体键的规则，需要按原逻辑全量解码判空。
	fullDecode bool
}

func newRuleSet(rules []compiledRule) ruleSet {
	set := ruleSet{rules: rules}
	if len(rules) == 0 {
		return set
	}
	wanted := make(map[string]string)
	add := func(key string) { wanted[key] = key }
	for i := range rules {
		rule := &rules[i]
		switch rule.fieldType {
		case "", FieldTypeSimple:
			if len(rule.variables) == 0 {
				set.fullDecode = true
			}
			for _, v := range rule.variables {
				add(v)
			}
		case FieldTypeTimeseries, FieldTypePropagation:
			add(rule.advanced.SourceKey)
		case FieldTypeGeofence:
			add(rule.advanced.LatKey)
			add(rule.advanced.LngKey)
		default:
			// related_agg / alarm 等：不读或在运行期才决定读哪些键，按原逻辑全量解码。
			set.fullDecode = true
		}
	}
	set.wanted = wanted
	return set
}

// decode 返回本规则集需要的扁平 payload；nil/空表示没有可参与运算的值。
func (s ruleSet) decode(raw []byte) map[string]interface{} {
	if s.fullDecode {
		return decodeFlatPayload(raw)
	}
	return decodePayloadSubset(raw, s.wanted)
}

// marshalDerivedBatch 把同一条源消息的全部 simple 派生结果编码为一个 JSON 对象，
// 输出与 json.Marshal(map[string]interface{}) 逐字节一致（键按字节序排序、同名键后者覆盖、
// 浮点格式与 HTML 转义规则同 encoding/json），但不经反射与中间 map。
// 值类型超出 float64/bool/string、或键/字符串需要转义时回退 json.Marshal，结果与错误行为不变。
func marshalDerivedBatch(keys []string, values []interface{}) ([]byte, error) {
	if out, ok := appendDerivedBatchJSON(nil, keys, values); ok {
		return out, nil
	}
	batch := make(map[string]interface{}, len(keys))
	for i, key := range keys {
		batch[key] = values[i] // 同名输出键后者覆盖前者，与逐条写入的最终值一致
	}
	return json.Marshal(batch)
}

func appendDerivedBatchJSON(dst []byte, keys []string, values []interface{}) ([]byte, bool) {
	// 选出每个键最后一次出现的下标，并按键字节序插入排序（规则数通常个位数）。
	var stack [8]int
	order := stack[:0]
	for i := range keys {
		if !isPlainJSONString(keys[i]) {
			return nil, false
		}
		replaced := false
		for j, idx := range order {
			if keys[idx] == keys[i] {
				order[j] = i
				replaced = true
				break
			}
		}
		if !replaced {
			order = append(order, i)
		}
	}
	for a := 1; a < len(order); a++ {
		for b := a; b > 0 && keys[order[b]] < keys[order[b-1]]; b-- {
			order[b], order[b-1] = order[b-1], order[b]
		}
	}

	size := 2
	for _, idx := range order {
		size += len(keys[idx]) + 28
	}
	if dst == nil {
		dst = make([]byte, 0, size)
	}
	dst = append(dst, '{')
	for n, idx := range order {
		if n > 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, '"')
		dst = append(dst, keys[idx]...)
		dst = append(dst, '"', ':')
		switch v := values[idx].(type) {
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, false // json.Marshal 报 UnsupportedValueError
			}
			dst = appendJSONFloat64(dst, v)
		case bool:
			dst = strconv.AppendBool(dst, v)
		case string:
			if !isPlainJSONString(v) {
				return nil, false
			}
			dst = append(dst, '"')
			dst = append(dst, v...)
			dst = append(dst, '"')
		default:
			return nil, false
		}
	}
	return append(dst, '}'), true
}

// appendJSONFloat64 与 encoding/json 的 float64 编码完全一致。
func appendJSONFloat64(dst []byte, f float64) []byte {
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	dst = strconv.AppendFloat(dst, f, format, -1, 64)
	if format == 'e' {
		// e-09 -> e-9
		if n := len(dst); n >= 4 && dst[n-4] == 'e' && dst[n-3] == '-' && dst[n-2] == '0' {
			dst[n-2] = dst[n-1]
			dst = dst[:n-1]
		}
	}
	return dst
}

// isPlainJSONString 判断 s 在 encoding/json（含默认 HTML 转义）下是否原样输出。
func isPlainJSONString(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c >= utf8.RuneSelf || c == '"' || c == '\\' || c == '<' || c == '>' || c == '&' {
			return false
		}
	}
	return true
}
