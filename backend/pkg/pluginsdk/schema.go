// 配置 Schema 校验：与 internal/service/widget_schema.go 同语义的 JSON Schema 子集，
// 刻意独立实现——SDK 是叶子包，插件作者不必拉起服务端即可编译与自测。
//
// 支持的关键字：type / enum / required / properties / additionalProperties(仅布尔) /
// items / minLength / maxLength / pattern / minimum / maximum /
// exclusiveMinimum / exclusiveMaximum / minItems / maxItems。
// schema 编译期即拒绝未知关键字：静默忽略的约束等于没有约束，比报错危险得多。
package pluginsdk

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// schemaError 带路径的校验错误，便于插件作者定位。
type schemaError struct {
	Path    string
	Message string
}

func (e *schemaError) Error() string {
	if e.Path == "$" {
		return "pluginsdk: schema: " + e.Message
	}
	return "pluginsdk: schema: " + e.Path + ": " + e.Message
}

var (
	supportedKeywords = map[string]bool{
		"type": true, "enum": true, "required": true, "properties": true,
		"additionalProperties": true, "items": true, "minLength": true,
		"maxLength": true, "pattern": true, "minimum": true, "maximum": true,
		"exclusiveMinimum": true, "exclusiveMaximum": true, "minItems": true,
		"maxItems": true,
	}
	schemaTypes = map[string]bool{
		"null": true, "boolean": true, "string": true, "number": true,
		"integer": true, "array": true, "object": true,
	}
	numericKeywords = []string{
		"minLength", "maxLength", "minimum", "maximum",
		"exclusiveMinimum", "exclusiveMaximum", "minItems", "maxItems",
	}
)

// CheckConfigSchema 编译期检查 schema 自身（未知关键字、类型、取值合法性）。
func CheckConfigSchema(node map[string]any) error {
	return checkSchemaNode(node, "$")
}

// ValidateConfigValue 按 schema 校验一个配置值。
func ValidateConfigValue(schema map[string]any, config map[string]any) error {
	if config == nil {
		config = map[string]any{}
	}
	return validateSchemaNode(schema, config, "$")
}

// checkSchemaNode 递归检查 schema 自身。
func checkSchemaNode(node map[string]any, path string) error {
	for k := range node {
		if !supportedKeywords[k] {
			return fmt.Errorf("pluginsdk: schema: unsupported keyword %q at %s", k, path)
		}
	}
	if t, ok := node["type"]; ok {
		name, isStr := t.(string)
		if !isStr || !schemaTypes[name] {
			return fmt.Errorf("pluginsdk: schema: bad type %v at %s", t, path)
		}
	}
	for _, k := range numericKeywords {
		if v, ok := node[k]; ok {
			if _, ok := toNumber(v); !ok {
				return fmt.Errorf("pluginsdk: schema: %s must be a number at %s", k, path)
			}
		}
	}
	if p, ok := node["pattern"]; ok {
		s, isStr := p.(string)
		if !isStr {
			return fmt.Errorf("pluginsdk: schema: pattern must be a string at %s", path)
		}
		if _, err := regexp.Compile(s); err != nil {
			return fmt.Errorf("pluginsdk: schema: bad pattern %q at %s: %v", s, path, err)
		}
	}
	if e, ok := node["enum"]; ok {
		list, isArr := e.([]any)
		if !isArr || len(list) == 0 {
			return fmt.Errorf("pluginsdk: schema: enum must be a non-empty array at %s", path)
		}
	}
	if req, ok := node["required"]; ok {
		list, isArr := req.([]any)
		if !isArr {
			return fmt.Errorf("pluginsdk: schema: required must be an array at %s", path)
		}
		for _, item := range list {
			if _, isStr := item.(string); !isStr {
				return fmt.Errorf("pluginsdk: schema: required entries must be strings at %s", path)
			}
		}
	}
	if ap, ok := node["additionalProperties"]; ok {
		if _, isBool := ap.(bool); !isBool {
			return fmt.Errorf("pluginsdk: schema: additionalProperties only supports true/false at %s", path)
		}
	}
	if props, ok := node["properties"]; ok {
		m, isMap := props.(map[string]any)
		if !isMap {
			return fmt.Errorf("pluginsdk: schema: properties must be an object at %s", path)
		}
		names := make([]string, 0, len(m))
		for n := range m {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			sub, isMap := m[n].(map[string]any)
			if !isMap {
				return fmt.Errorf("pluginsdk: schema: properties.%s must be an object at %s", n, path)
			}
			if err := checkSchemaNode(sub, path+".properties."+n); err != nil {
				return err
			}
		}
	}
	if items, ok := node["items"]; ok {
		sub, isMap := items.(map[string]any)
		if !isMap {
			return fmt.Errorf("pluginsdk: schema: items must be a schema object at %s", path)
		}
		if err := checkSchemaNode(sub, path+".items"); err != nil {
			return err
		}
	}
	return nil
}

// validateSchemaNode 按 schema 校验一个值。
func validateSchemaNode(node map[string]any, value any, path string) error {
	if t, ok := node["type"].(string); ok {
		if !typeMatches(t, value) {
			return &schemaError{Path: path, Message: fmt.Sprintf("expected %s, got %s", t, jsonKind(value))}
		}
	}
	if e, ok := node["enum"]; ok {
		if list, isArr := e.([]any); isArr && !enumContains(list, value) {
			return &schemaError{Path: path, Message: "value is not one of " + enumLiteral(list)}
		}
	}
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
		return validateStringNode(node, typed, path)
	case []any:
		return validateArrayNode(node, typed, path)
	case map[string]any:
		return validateObjectNode(node, typed, path)
	case bool:
		return nil
	default:
		if num, ok := toNumber(value); ok {
			return validateNumberNode(node, num, path)
		}
	}
	return nil
}

// typeMatches 判断值是否符合声明的 JSON 类型。
func typeMatches(want string, value any) bool {
	switch want {
	case "null":
		return value == nil
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "number":
		_, ok := toNumber(value)
		return ok
	case "integer":
		num, ok := toNumber(value)
		return ok && math.Trunc(num) == num && !math.IsInf(num, 0)
	}
	return false
}

// validateStringNode 字符串约束：长度与正则。
func validateStringNode(node map[string]any, value, path string) error {
	length := utf8.RuneCountInString(value)
	if n, ok := numberKeyword(node, "minLength"); ok && float64(length) < n {
		return &schemaError{Path: path, Message: fmt.Sprintf("length %d is below minLength %s", length, formatNumber(n))}
	}
	if n, ok := numberKeyword(node, "maxLength"); ok && float64(length) > n {
		return &schemaError{Path: path, Message: fmt.Sprintf("length %d exceeds maxLength %s", length, formatNumber(n))}
	}
	if p, ok := node["pattern"].(string); ok {
		matched, err := regexp.MatchString(p, value)
		if err != nil {
			return fmt.Errorf("pluginsdk: schema: bad pattern %q: %v", p, err)
		}
		if !matched {
			return &schemaError{Path: path, Message: fmt.Sprintf("does not match pattern %s", p)}
		}
	}
	return nil
}

// validateNumberNode 数值约束：最小/最大（含开区间）。
func validateNumberNode(node map[string]any, value float64, path string) error {
	if n, ok := numberKeyword(node, "minimum"); ok && value < n {
		return &schemaError{Path: path, Message: fmt.Sprintf("%s is below minimum %s", formatNumber(value), formatNumber(n))}
	}
	if n, ok := numberKeyword(node, "maximum"); ok && value > n {
		return &schemaError{Path: path, Message: fmt.Sprintf("%s exceeds maximum %s", formatNumber(value), formatNumber(n))}
	}
	if n, ok := numberKeyword(node, "exclusiveMinimum"); ok && value <= n {
		return &schemaError{Path: path, Message: fmt.Sprintf("%s must be greater than %s", formatNumber(value), formatNumber(n))}
	}
	if n, ok := numberKeyword(node, "exclusiveMaximum"); ok && value >= n {
		return &schemaError{Path: path, Message: fmt.Sprintf("%s must be less than %s", formatNumber(value), formatNumber(n))}
	}
	return nil
}

// validateArrayNode 数组约束：元素 schema 与长度。
func validateArrayNode(node map[string]any, value []any, path string) error {
	if n, ok := numberKeyword(node, "minItems"); ok && float64(len(value)) < n {
		return &schemaError{Path: path, Message: fmt.Sprintf("%d item(s) is below minItems %s", len(value), formatNumber(n))}
	}
	if n, ok := numberKeyword(node, "maxItems"); ok && float64(len(value)) > n {
		return &schemaError{Path: path, Message: fmt.Sprintf("%d item(s) exceeds maxItems %s", len(value), formatNumber(n))}
	}
	if items, ok := node["items"].(map[string]any); ok {
		for i, item := range value {
			if err := validateSchemaNode(items, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateObjectNode 对象约束：必填、已知属性、属性值。
func validateObjectNode(node map[string]any, value map[string]any, path string) error {
	if req, ok := node["required"].([]any); ok {
		for _, r := range req {
			name, isStr := r.(string)
			if !isStr {
				continue
			}
			if _, present := value[name]; !present {
				return &schemaError{Path: path + "." + name, Message: "is required"}
			}
		}
	}
	props, _ := node["properties"].(map[string]any)
	allowAdditional := true
	if ap, ok := node["additionalProperties"].(bool); ok {
		allowAdditional = ap
	}
	names := make([]string, 0, len(value))
	for n := range value {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		sub, declared := props[n].(map[string]any)
		if !declared {
			if !allowAdditional {
				return &schemaError{Path: path + "." + n, Message: "is not an allowed property"}
			}
			continue
		}
		if err := validateSchemaNode(sub, value[n], path+"."+n); err != nil {
			return err
		}
	}
	return nil
}

// numberKeyword 取数字型关键字。
func numberKeyword(node map[string]any, key string) (float64, bool) {
	v, ok := node[key]
	if !ok {
		return 0, false
	}
	return toNumber(v)
}

// toNumber 把 JSON 数字的各种 Go 表示归一化成 float64。
func toNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// formatNumber 让错误信息里的数字可读（10 而不是 1e+01）。
func formatNumber(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) && math.Abs(f) < 1e15 {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}

// jsonKind 返回值的 JSON 类型名，用于错误信息。
func jsonKind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	if _, ok := toNumber(v); ok {
		return "number"
	}
	return fmt.Sprintf("%T", v)
}

// enumContains 判断值是否命中 enum。
func enumContains(list []any, value any) bool {
	for _, item := range list {
		if jsonEqual(item, value) {
			return true
		}
	}
	return false
}

// enumLiteral 把 enum 渲染进错误信息。
func enumLiteral(list []any) string {
	parts := make([]string, 0, len(list))
	for _, item := range list {
		raw, err := json.Marshal(item)
		if err != nil {
			parts = append(parts, fmt.Sprintf("%v", item))
			continue
		}
		parts = append(parts, string(raw))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// jsonEqual 按 JSON 语义比较两个值（数字跨 Go 类型可比，对象/数组递归）。
func jsonEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if av, ok := toNumber(a); ok {
		if bv, ok := toNumber(b); ok {
			return av == bv
		}
		return false
	}
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !jsonEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			other, present := bv[k]
			if !present || !jsonEqual(v, other) {
				return false
			}
		}
		return true
	}
	return false
}
