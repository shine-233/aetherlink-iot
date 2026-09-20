// 文件用途：Widget 配置的 JSON Schema 子集校验（ROADMAP P1.3「Widget schema 真实字段」）。
// 核心逻辑：把 WidgetDefinition.Schema（以及命令的 ParamsSchema）编译成校验器，
// 在「注册时」和「画布保存时」两处各校验一次。
//
// 关键注意事项（这一层存在的全部理由就是"不许假装校验过"）：
//  1. **只认白名单里的关键字，遇到不认识的关键字直接报错**。
//     静默跳过不认识的关键字，等于给一份根本没检查过的配置盖上"已校验"的章。
//     所以不支持的组合必须在**注册阶段**就失败（Register 返回错误），
//     而不是等到用户保存画布时才炸——那时已经有一堆画布用了这个 Widget。
//  2. 未注册的 Widget 类型**跳过校验并返回 nil**，不是"通过校验"。
//     它会在画布解析阶段被判为 unknown（渲染占位符），校验阶段没有它的 schema，
//     无从校验。把"没法校验"说成"校验通过"是假成功；这里两者都不做，交给解析层定性。
//  3. 数值一律按 JSON 数字比较（float64 / 整型 / json.Number 都归一化）。
//     Go 里 int 与 float64 直接相等比较会在配置来自不同来源时给出相反结论。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	// ErrWidgetSchemaBadJSON schema 字符串不是合法 JSON。
	ErrWidgetSchemaBadJSON = errors.New("widget schema is not valid JSON")
	// ErrWidgetSchemaNotObject schema 顶层必须是 JSON 对象。
	ErrWidgetSchemaNotObject = errors.New("widget schema must be a JSON object")
	// ErrWidgetSchemaBadType type 取值不在支持集合内，或不是字符串。
	ErrWidgetSchemaBadType = errors.New("widget schema declares an unsupported type")
	// ErrWidgetSchemaUnsupportedKeyword 使用了白名单外的关键字（见文件头注意事项 1）。
	ErrWidgetSchemaUnsupportedKeyword = errors.New("widget schema uses an unsupported keyword")
	// ErrWidgetSchemaBadKeyword 关键字本身支持，但取值非法。
	ErrWidgetSchemaBadKeyword = errors.New("widget schema keyword has an invalid value")
	// ErrWidgetSchemaBadPattern pattern 不是可编译的正则。
	ErrWidgetSchemaBadPattern = errors.New("widget schema declares an invalid pattern")
	// ErrWidgetConfigNotObject 配置必须是 JSON 对象。
	ErrWidgetConfigNotObject = errors.New("widget config must be a JSON object")
	// ErrWidgetSchemaNotCompiled 校验器未编译（nil）。
	ErrWidgetSchemaNotCompiled = errors.New("widget schema is not compiled")
)

// widgetSchemaSupportedKeywords 本实现支持的关键字白名单。
//
// 新增支持一个关键字 = 同时实现它的校验语义；只把它加进白名单而不实现，
// 等于宣布支持而实际放行。二者必须同一次提交完成。
var widgetSchemaSupportedKeywords = map[string]bool{
	// 结构
	"type": true, "properties": true, "required": true, "additionalProperties": true,
	"items": true, "minItems": true, "maxItems": true,
	// 取值
	"enum":    true,
	"minimum": true, "maximum": true, "exclusiveMinimum": true, "exclusiveMaximum": true,
	"minLength": true, "maxLength": true, "pattern": true,
	// 纯注解：不影响取值合法性，允许出现且被忽略
	"$schema": true, "title": true, "description": true, "default": true,
}

// widgetSchemaTypes 支持的 type 取值。
var widgetSchemaTypes = map[string]bool{
	"object": true, "array": true, "string": true,
	"number": true, "integer": true, "boolean": true, "null": true,
}

// widgetSchemaNumericKeywords 取值必须是数字的关键字。
var widgetSchemaNumericKeywords = []string{
	"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum",
	"minLength", "maxLength", "minItems", "maxItems",
}

// WidgetSchemaError 一次配置校验失败，带 JSON 路径。
type WidgetSchemaError struct {
	Path    string
	Message string
}

// Error 实现 error。
func (e *WidgetSchemaError) Error() string {
	if e == nil {
		return ""
	}
	if e.Path == "" || e.Path == "$" {
		return "widget config: " + e.Message
	}
	return fmt.Sprintf("widget config at %s: %s", e.Path, e.Message)
}

// WidgetSchema 编译后的 Widget 配置校验器。
type WidgetSchema struct {
	root map[string]any
}

// CompileWidgetSchema 编译 schema 字符串；不合法或含不支持的关键字时返回错误。
func CompileWidgetSchema(schemaJSON string) (*WidgetSchema, error) {
	trimmed := strings.TrimSpace(schemaJSON)
	if trimmed == "" || trimmed[0] != '{' {
		return nil, ErrWidgetSchemaNotObject
	}
	dec := json.NewDecoder(strings.NewReader(trimmed))
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWidgetSchemaBadJSON, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("%w: trailing content after the schema object", ErrWidgetSchemaBadJSON)
	}
	if err := checkSchemaNode(root, "$"); err != nil {
		return nil, err
	}
	return &WidgetSchema{root: root}, nil
}

// Validate 校验任意 JSON 值。
func (s *WidgetSchema) Validate(value any) error {
	if s == nil || s.root == nil {
		return ErrWidgetSchemaNotCompiled
	}
	return validateSchemaNode(s.root, value, "$")
}

// ValidateConfig 校验 Widget 配置对象。nil 视作空对象（required 会因此失败）。
func (s *WidgetSchema) ValidateConfig(config map[string]any) error {
	if s == nil || s.root == nil {
		return ErrWidgetSchemaNotCompiled
	}
	if config == nil {
		config = map[string]any{}
	}
	return validateSchemaNode(s.root, config, "$")
}

// checkSchemaNode 递归检查 schema 自身是否只用了受支持的关键字与取值。
func checkSchemaNode(node map[string]any, path string) error {
	for k := range node {
		if !widgetSchemaSupportedKeywords[k] {
			return fmt.Errorf("%w: %q at %s", ErrWidgetSchemaUnsupportedKeyword, k, path)
		}
	}
	if t, ok := node["type"]; ok {
		name, isStr := t.(string)
		if !isStr || !widgetSchemaTypes[name] {
			return fmt.Errorf("%w: %v at %s", ErrWidgetSchemaBadType, t, path)
		}
	}
	for _, k := range widgetSchemaNumericKeywords {
		if v, ok := node[k]; ok {
			if _, ok := asNumber(v); !ok {
				return fmt.Errorf("%w: %s must be a number at %s", ErrWidgetSchemaBadKeyword, k, path)
			}
		}
	}
	if p, ok := node["pattern"]; ok {
		s, isStr := p.(string)
		if !isStr {
			return fmt.Errorf("%w: pattern must be a string at %s", ErrWidgetSchemaBadKeyword, path)
		}
		if _, err := regexp.Compile(s); err != nil {
			return fmt.Errorf("%w: %q at %s: %v", ErrWidgetSchemaBadPattern, s, path, err)
		}
	}
	if e, ok := node["enum"]; ok {
		list, isArr := e.([]any)
		if !isArr || len(list) == 0 {
			return fmt.Errorf("%w: enum must be a non-empty array at %s", ErrWidgetSchemaBadKeyword, path)
		}
	}
	if req, ok := node["required"]; ok {
		list, isArr := req.([]any)
		if !isArr {
			return fmt.Errorf("%w: required must be an array at %s", ErrWidgetSchemaBadKeyword, path)
		}
		for _, item := range list {
			if _, isStr := item.(string); !isStr {
				return fmt.Errorf("%w: required entries must be strings at %s", ErrWidgetSchemaBadKeyword, path)
			}
		}
	}
	if ap, ok := node["additionalProperties"]; ok {
		// 只支持布尔：additionalProperties 取 schema 对象时语义是"用这份 schema 校验
		// 其余字段"，本实现不做；与其静默放行，不如让注册直接失败。
		if _, isBool := ap.(bool); !isBool {
			return fmt.Errorf("%w: additionalProperties only supports true/false at %s", ErrWidgetSchemaBadKeyword, path)
		}
	}
	if props, ok := node["properties"]; ok {
		m, isMap := props.(map[string]any)
		if !isMap {
			return fmt.Errorf("%w: properties must be an object at %s", ErrWidgetSchemaBadKeyword, path)
		}
		names := make([]string, 0, len(m))
		for n := range m {
			names = append(names, n)
		}
		sort.Strings(names) // 稳定顺序：错误信息不随 map 遍历顺序抖动
		for _, n := range names {
			sub, isMap := m[n].(map[string]any)
			if !isMap {
				return fmt.Errorf("%w: properties.%s must be an object at %s", ErrWidgetSchemaBadKeyword, n, path)
			}
			if err := checkSchemaNode(sub, path+".properties."+n); err != nil {
				return err
			}
		}
	}
	if items, ok := node["items"]; ok {
		sub, isMap := items.(map[string]any)
		if !isMap {
			return fmt.Errorf("%w: items must be a schema object at %s", ErrWidgetSchemaBadKeyword, path)
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
			return &WidgetSchemaError{Path: path, Message: fmt.Sprintf("expected %s, got %s", t, jsonKind(value))}
		}
	}
	if e, ok := node["enum"]; ok {
		if list, isArr := e.([]any); isArr && !enumContains(list, value) {
			return &WidgetSchemaError{Path: path, Message: "value is not one of " + enumLiteral(list)}
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
		if num, ok := asNumber(value); ok {
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
		_, ok := asNumber(value)
		return ok
	case "integer":
		num, ok := asNumber(value)
		return ok && math.Trunc(num) == num && !math.IsInf(num, 0)
	}
	return false
}

// validateStringNode 字符串约束：长度与正则。
func validateStringNode(node map[string]any, value, path string) error {
	length := utf8.RuneCountInString(value)
	if n, ok := numberKeyword(node, "minLength"); ok && float64(length) < n {
		return &WidgetSchemaError{Path: path, Message: fmt.Sprintf("length %d is below minLength %s", length, formatNumber(n))}
	}
	if n, ok := numberKeyword(node, "maxLength"); ok && float64(length) > n {
		return &WidgetSchemaError{Path: path, Message: fmt.Sprintf("length %d exceeds maxLength %s", length, formatNumber(n))}
	}
	if p, ok := node["pattern"].(string); ok {
		matched, err := regexp.MatchString(p, value)
		if err != nil {
			// schema 编译期已校验过可编译性；走到这里说明 schema 被绕过注册直接改了。
			return fmt.Errorf("%w: %q: %v", ErrWidgetSchemaBadPattern, p, err)
		}
		if !matched {
			return &WidgetSchemaError{Path: path, Message: fmt.Sprintf("does not match pattern %s", p)}
		}
	}
	return nil
}

// validateNumberNode 数值约束：最小/最大（含开区间）。
func validateNumberNode(node map[string]any, value float64, path string) error {
	if n, ok := numberKeyword(node, "minimum"); ok && value < n {
		return &WidgetSchemaError{Path: path, Message: fmt.Sprintf("%s is below minimum %s", formatNumber(value), formatNumber(n))}
	}
	if n, ok := numberKeyword(node, "maximum"); ok && value > n {
		return &WidgetSchemaError{Path: path, Message: fmt.Sprintf("%s exceeds maximum %s", formatNumber(value), formatNumber(n))}
	}
	if n, ok := numberKeyword(node, "exclusiveMinimum"); ok && value <= n {
		return &WidgetSchemaError{Path: path, Message: fmt.Sprintf("%s must be greater than %s", formatNumber(value), formatNumber(n))}
	}
	if n, ok := numberKeyword(node, "exclusiveMaximum"); ok && value >= n {
		return &WidgetSchemaError{Path: path, Message: fmt.Sprintf("%s must be less than %s", formatNumber(value), formatNumber(n))}
	}
	return nil
}

// validateArrayNode 数组约束：元素 schema 与长度。
func validateArrayNode(node map[string]any, value []any, path string) error {
	if n, ok := numberKeyword(node, "minItems"); ok && float64(len(value)) < n {
		return &WidgetSchemaError{Path: path, Message: fmt.Sprintf("%d item(s) is below minItems %s", len(value), formatNumber(n))}
	}
	if n, ok := numberKeyword(node, "maxItems"); ok && float64(len(value)) > n {
		return &WidgetSchemaError{Path: path, Message: fmt.Sprintf("%d item(s) exceeds maxItems %s", len(value), formatNumber(n))}
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
				return &WidgetSchemaError{Path: path + "." + name, Message: "is required"}
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
				return &WidgetSchemaError{Path: path + "." + n, Message: "is not an allowed property"}
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
	return asNumber(v)
}

// asNumber 把 JSON 数字的各种 Go 表示归一化成 float64。
func asNumber(v any) (float64, bool) {
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
	if _, ok := asNumber(v); ok {
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
	if af, ok := asNumber(a); ok {
		if bf, ok := asNumber(b); ok {
			return af == bf
		}
		return false
	}
	if ab, ok := a.(bool); ok {
		if bb, ok := b.(bool); ok {
			return ab == bb
		}
		return false
	}
	return reflect.DeepEqual(a, b)
}
