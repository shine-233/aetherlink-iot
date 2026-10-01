// 文件用途：计算字段规则集（装载期预计算的热路径元数据）。
// 核心逻辑：模板规则在缓存刷新时编译一次，同时汇总出所有规则会读取的 payload 键；
// processMessage 据此只解码需要的键，模板无规则时完全不解码 payload。
// 关键注意事项：若任一规则不依赖具体键（related_agg、零变量表达式）却受"payload 非空"门控，
// 则必须回退全量解码以保持原有门控语义（fullDecode=true）。
package calcfield

import "encoding/json"

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
// 编码与原先逐条 json.Marshal(map{key: value}) 一致，只是合并为一次 Marshal、一条存储消息。
func marshalDerivedBatch(keys []string, values []interface{}) ([]byte, error) {
	if len(keys) == 1 {
		return json.Marshal(map[string]interface{}{keys[0]: values[0]})
	}
	batch := make(map[string]interface{}, len(keys))
	for i, key := range keys {
		batch[key] = values[i] // 同名输出键后者覆盖前者，与逐条写入的最终值一致
	}
	return json.Marshal(batch)
}
