// flat_json.go: fast path for decoding the common telemetry/attribute payload
// shape, a flat JSON object of numbers, booleans, nulls and plain strings.
//
// encoding/json into map[string]interface{} costs ~4 allocations per key
// (reflect.New, key string, boxed value, map growth). Here the payload is
// copied into one string, keys and string values are substrings of it, and
// only numbers/booleans are boxed.
//
// Contract: the result is identical to json.Unmarshal into
// map[string]interface{}. Anything outside the fast shape (nested values,
// escaped or non-UTF-8 strings, numbers outside float64, non-object top level,
// invalid JSON) returns ok=false and the caller falls back to encoding/json, so
// error behavior is unchanged.
package uplink

import (
	"encoding/json"
	"strconv"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
)

// decodeJSONObjectOrRaw decodes payload into a JSON object map (flat fast path,
// then encoding/json). Payloads that are not a JSON object are logged with
// warnMsg and wrapped as {"_raw": <value>} so they are still stored.
func decodeJSONObjectOrRaw(logger *logrus.Logger, deviceID string, payload []byte, warnMsg string) map[string]interface{} {
	if dataMap, ok := decodeFlatJSONObject(payload); ok {
		return dataMap
	}
	var dataMap map[string]interface{}
	if err := json.Unmarshal(payload, &dataMap); err != nil {
		logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"payload":   string(payload),
			"error":     err,
		}).Warn(warnMsg)

		dataMap = map[string]interface{}{
			"_raw": parseRawJSONValue(payload),
		}
	}
	return dataMap
}

// decodeFlatJSONObject decodes payload when it is a flat JSON object.
func decodeFlatJSONObject(payload []byte) (map[string]interface{}, bool) {
	if !json.Valid(payload) {
		return nil, false
	}
	// One copy; every key and string value below is a substring of it.
	s := string(payload)
	i := skipJSONSpace(s, 0)
	if i >= len(s) || s[i] != '{' {
		return nil, false
	}
	i++
	out := make(map[string]interface{}, estimateFlatJSONKeys(s))
	for {
		i = skipJSONSpace(s, i)
		if s[i] == '}' {
			return out, true
		}
		if s[i] == ',' {
			i = skipJSONSpace(s, i+1)
		}
		key, next, ok := readPlainJSONString(s, i)
		if !ok {
			return nil, false
		}
		i = skipJSONSpace(s, next)
		i = skipJSONSpace(s, i+1) // ':' (json.Valid guarantees it)

		var value interface{}
		switch c := s[i]; c {
		case '"':
			str, end, ok := readPlainJSONString(s, i)
			if !ok {
				return nil, false
			}
			value, i = str, end
		case 't':
			value, i = true, i+4
		case 'f':
			value, i = false, i+5
		case 'n':
			value, i = nil, i+4
		case '{', '[':
			return nil, false
		default:
			end := i
			for end < len(s) && isJSONNumberByte(s[end]) {
				end++
			}
			f, err := strconv.ParseFloat(s[i:end], 64)
			if err != nil {
				return nil, false // out of float64 range: let encoding/json report it
			}
			value, i = f, end
		}
		out[key] = value
	}
}

// readPlainJSONString reads the string starting at the opening quote s[i]. It
// rejects escapes and invalid UTF-8, which encoding/json would rewrite.
func readPlainJSONString(s string, i int) (string, int, bool) {
	start := i + 1
	ascii := true
	for j := start; j < len(s); j++ {
		switch c := s[j]; {
		case c == '"':
			str := s[start:j]
			if !ascii && !utf8.ValidString(str) {
				return "", 0, false
			}
			return str, j + 1, true
		case c == '\\':
			return "", 0, false
		case c >= utf8.RuneSelf:
			ascii = false
		}
	}
	return "", 0, false
}

func skipJSONSpace(s string, i int) int {
	for i < len(s) {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

func isJSONNumberByte(c byte) bool {
	return (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E'
}

// estimateFlatJSONKeys sizes the map from the ':' count (exact for flat
// objects whose strings contain no ':'), avoiding map growth.
func estimateFlatJSONKeys(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			n++
		}
	}
	return n
}
