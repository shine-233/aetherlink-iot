// 文件用途：计算字段引擎的遥测 payload 按需解码（热路径）。
// 核心逻辑：规则集在装载期汇总出"会读到的键"；每条消息只把这些键解出为 float64/bool，
// 其余键仅做跳过扫描，不再为每个键分配 string + interface{} + 中间 map。
// 关键注意事项：结果必须与 decodeFlatPayload 在 wanted 键上逐键一致——
//   - 非法 JSON / 顶层非对象 / 数值溢出 float64 → 整体返回 nil（与 json.Unmarshal 失败一致）；
//   - 重复键后者覆盖前者（与 encoding/json 解到 map 一致）；
//   - 键含转义字符或非法 UTF-8 时回退到 decodeFlatPayload，避免手写反转义带来的语义漂移。
package calcfield

import (
	"encoding/json"
	"strconv"
	"unicode/utf8"
	"unsafe"
)

// decodePayloadSubset 只解码 wanted 中列出的键；wanted 为空返回 nil。
// wanted 的 value 是规范键本身（key==value），用于让结果 map 复用装载期驻留的字符串。
func decodePayloadSubset(raw []byte, wanted map[string]string) map[string]interface{} {
	if len(raw) == 0 || len(wanted) == 0 {
		return nil
	}
	if !json.Valid(raw) {
		return nil
	}
	s := payloadScanner{data: raw}
	s.skipSpace()
	if s.pos >= len(raw) || raw[s.pos] != '{' {
		return nil
	}
	s.pos++

	var flat map[string]interface{}
	for {
		s.skipSpace()
		if raw[s.pos] == '}' {
			break
		}
		keyBytes, plain := s.readKey()
		if !plain {
			return filterSubset(decodeFlatPayload(raw), wanted)
		}
		s.skipSpace()
		s.pos++ // ':'
		s.skipSpace()

		key, want := lookupWanted(wanted, keyBytes)
		value, scalar, ok := s.readValue(want)
		if !ok {
			return nil // 数值超出 float64 范围：json.Unmarshal 会整体失败
		}
		if want {
			if scalar {
				if flat == nil {
					flat = make(map[string]interface{}, len(wanted))
				}
				flat[key] = value
			} else if flat != nil {
				delete(flat, key) // 重复键且后者非标量：decodeFlatPayload 会把它过滤掉
			}
		}

		s.skipSpace()
		if raw[s.pos] == ',' {
			s.pos++
			continue
		}
		break // '}'
	}
	return flat
}

// lookupWanted 用 wanted 里已驻留的字符串作为结果 map 的键，避免为每条消息重新分配 key。
// probe 指向消息缓冲区，只用于查找，绝不被结果 map 持有。
func lookupWanted(wanted map[string]string, keyBytes []byte) (string, bool) {
	probe := unsafe.String(unsafe.SliceData(keyBytes), len(keyBytes))
	key, ok := wanted[probe]
	return key, ok
}

func filterSubset(flat map[string]interface{}, wanted map[string]string) map[string]interface{} {
	if flat == nil {
		return nil
	}
	for key := range flat {
		if _, ok := wanted[key]; !ok {
			delete(flat, key)
		}
	}
	if len(flat) == 0 {
		return nil
	}
	return flat
}

// payloadScanner 在已通过 json.Valid 的输入上前进，因此不再重复做语法校验。
type payloadScanner struct {
	data []byte
	pos  int
}

func (s *payloadScanner) skipSpace() {
	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case ' ', '\t', '\n', '\r':
			s.pos++
		default:
			return
		}
	}
}

// readKey 读取对象键（当前位置为 '"'）。plain=false 表示含转义或非法 UTF-8，需要回退。
func (s *payloadScanner) readKey() ([]byte, bool) {
	s.pos++ // opening quote
	start := s.pos
	ascii := true
	for {
		c := s.data[s.pos]
		if c == '"' {
			key := s.data[start:s.pos]
			s.pos++
			if !ascii && !utf8.Valid(key) {
				return nil, false
			}
			return key, true
		}
		if c == '\\' {
			return nil, false
		}
		if c >= utf8.RuneSelf {
			ascii = false
		}
		s.pos++
	}
}

// readValue 读取一个值。decode=false 时只跳过（但仍校验数值范围以保持失败语义）。
// 返回 (值, 是否为数值/布尔标量, 是否成功)。
func (s *payloadScanner) readValue(decode bool) (interface{}, bool, bool) {
	switch c := s.data[s.pos]; {
	case c == '"':
		s.skipString()
		return nil, false, true
	case c == '{' || c == '[':
		return nil, false, s.skipComposite()
	case c == 't':
		s.pos += 4
		return true, true, true
	case c == 'f':
		s.pos += 5
		return false, true, true
	case c == 'n':
		s.pos += 4
		return nil, false, true
	default:
		return s.readNumber(decode)
	}
}

// readNumber 读取数值。跳过模式下只对可能溢出的数值（含指数或超长）调用 ParseFloat。
func (s *payloadScanner) readNumber(decode bool) (interface{}, bool, bool) {
	start := s.pos
	hasExp := false
	for s.pos < len(s.data) {
		b := s.data[s.pos]
		if (b >= '0' && b <= '9') || b == '-' || b == '+' || b == '.' {
			s.pos++
			continue
		}
		if b == 'e' || b == 'E' {
			hasExp = true
			s.pos++
			continue
		}
		break
	}
	if !decode && !hasExp && s.pos-start < 300 {
		// 无指数且不足 300 位的十进制数不可能溢出 float64。
		return nil, false, true
	}
	text := unsafe.String(unsafe.SliceData(s.data[start:s.pos]), s.pos-start)
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, false, false
	}
	if !decode {
		return nil, false, true
	}
	return f, true, true
}

func (s *payloadScanner) skipString() {
	s.pos++ // opening quote
	for {
		switch s.data[s.pos] {
		case '\\':
			s.pos += 2
		case '"':
			s.pos++
			return
		default:
			s.pos++
		}
	}
}

// skipComposite 跳过嵌套对象/数组；内部数值同样做溢出校验（json.Unmarshal 对嵌套溢出也会失败）。
func (s *payloadScanner) skipComposite() bool {
	depth := 0
	for {
		switch c := s.data[s.pos]; {
		case c == '"':
			s.skipString()
			continue
		case c == '-' || (c >= '0' && c <= '9'):
			if _, _, ok := s.readNumber(false); !ok {
				return false
			}
			continue
		case c == '{' || c == '[':
			depth++
		case c == '}' || c == ']':
			depth--
			if depth == 0 {
				s.pos++
				return true
			}
		}
		s.pos++
	}
}
