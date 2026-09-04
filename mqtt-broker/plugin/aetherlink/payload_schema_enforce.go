// 文件用途：broker 侧 payload schema 强制校验的“纯决策”函数。
// 核心逻辑：给定一份已解码的上行 payload 与一组字段约束(类型/必填/范围/枚举/正则/严格模式),
//
//	推演出 accept / reject / warn 决策及诊断信息。它是无副作用的纯函数,
//	不连 broker 会话、不查 registry、不改写 MQTT 消息、不落库。
//
// 关键注意事项：这是 payload-schema broker 强制能力中“可离线验证”的一半。
//
//	真正把它接入 OnMsgArrivedWrapper 需要:①运行时的 schema-registry 查询(设备->schema 绑定),
//	②对上行消息的真实拦截(拒收=断开/丢弃),这属于外部 MQTT 契约的破坏性变更,
//	必须在部署环境(broker+PG+真实设备)做运行时验证,不在此文件实现。
//
// 单一来源：字段约束语义刻意与 backend service.validatePayloadFieldValue 保持一致,
//
//	若后端规则调整,应同步本文件的推演逻辑,避免两侧漂移。
package aetherlink

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// PayloadSchemaFieldType 是 broker 侧字段类型的本地镜像(不 import backend module)。
type PayloadSchemaFieldType string

const (
	PayloadSchemaFieldTypeNumber  PayloadSchemaFieldType = "number"
	PayloadSchemaFieldTypeString  PayloadSchemaFieldType = "string"
	PayloadSchemaFieldTypeBoolean PayloadSchemaFieldType = "boolean"
	PayloadSchemaFieldTypeObject  PayloadSchemaFieldType = "object"
	PayloadSchemaFieldTypeArray   PayloadSchemaFieldType = "array"
)

// PayloadSchemaFieldConstraint 描述单个字段的约束,语义对齐 backend 的 PayloadSchemaField。
type PayloadSchemaFieldConstraint struct {
	Name     string
	Type     PayloadSchemaFieldType
	Required bool
	Min      *float64
	Max      *float64
	Enum     []string
	Pattern  *string
}

// PayloadSchemaEnforcement 是一份 schema 的强制配置:字段约束 + 是否严格拒收未声明字段。
type PayloadSchemaEnforcement struct {
	Strict bool
	Fields []PayloadSchemaFieldConstraint
}

// validatePayloadSchemaEnforcement mirrors the backend registry's structural
// checks before a DB-resolved schema is marked as bound.  A malformed or
// incompatible persisted schema must fail closed at the broker boundary,
// rather than being interpreted differently from the backend runtime path.
func validatePayloadSchemaEnforcement(enf PayloadSchemaEnforcement) error {
	if len(enf.Fields) == 0 {
		return fmt.Errorf("payload schema must declare at least one field")
	}
	seen := make(map[string]struct{}, len(enf.Fields))
	for _, field := range enf.Fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return fmt.Errorf("payload schema field name is required")
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("payload schema field names must be unique")
		}
		seen[name] = struct{}{}

		switch field.Type {
		case PayloadSchemaFieldTypeNumber:
			if field.Min != nil && field.Max != nil && *field.Min > *field.Max {
				return fmt.Errorf("payload schema field min cannot exceed max")
			}
			if len(field.Enum) > 0 || field.Pattern != nil {
				return fmt.Errorf("number fields cannot declare enum or pattern constraints")
			}
		case PayloadSchemaFieldTypeString:
			if field.Min != nil || field.Max != nil {
				return fmt.Errorf("string fields cannot declare min or max constraints")
			}
			if field.Pattern != nil {
				if _, err := regexp.Compile(*field.Pattern); err != nil {
					return fmt.Errorf("payload schema field pattern is invalid")
				}
			}
		case PayloadSchemaFieldTypeBoolean, PayloadSchemaFieldTypeObject, PayloadSchemaFieldTypeArray:
			if field.Min != nil || field.Max != nil || len(field.Enum) > 0 || field.Pattern != nil {
				return fmt.Errorf("payload schema field type does not support scalar constraints")
			}
		default:
			return fmt.Errorf("unsupported payload schema field type")
		}
	}
	return nil
}

// PayloadSchemaDecisionOutcome 是决策结果枚举。
type PayloadSchemaDecisionOutcome string

const (
	// PayloadSchemaAccept 表示 payload 满足约束,应放行。
	PayloadSchemaAccept PayloadSchemaDecisionOutcome = "accept"
	// PayloadSchemaWarn 表示 payload 可放行但存在非致命问题(非严格模式下的未声明字段)。
	PayloadSchemaWarn PayloadSchemaDecisionOutcome = "warn"
	// PayloadSchemaReject 表示 payload 违反约束,broker 侧应拒收(运行时接入时)。
	PayloadSchemaReject PayloadSchemaDecisionOutcome = "reject"
)

// PayloadSchemaDecision 是纯决策的结构化产物。
type PayloadSchemaDecision struct {
	Outcome     PayloadSchemaDecisionOutcome
	Checkedn    int
	Errors      []string
	Warnings    []string
	UnknownKeys []string
	// Reason 是对最终 outcome 的一句话概述,便于日志与诊断。
	Reason string
}

// DecidePayloadSchemaEnforcement 对已解码的 payload 应用一份 schema 强制配置,给出纯决策。
// 它不修改入参、无副作用;调用方(未来接入 OnMsgArrivedWrapper 时)负责把 reject 翻译成真实拦截。
func DecidePayloadSchemaEnforcement(enf PayloadSchemaEnforcement, rawPayload []byte) PayloadSchemaDecision {
	decision := PayloadSchemaDecision{
		Outcome:     PayloadSchemaAccept,
		Errors:      []string{},
		Warnings:    []string{},
		UnknownKeys: []string{},
	}

	var decoded map[string]any
	if err := json.Unmarshal(rawPayload, &decoded); err != nil || decoded == nil {
		decision.Outcome = PayloadSchemaReject
		decision.Errors = append(decision.Errors, "payload is not a valid JSON object")
		decision.Reason = "payload is not a valid JSON object"
		return decision
	}

	seen := map[string]struct{}{}
	for _, field := range enf.Fields {
		seen[field.Name] = struct{}{}
		decision.Checkedn++
		value, present := decoded[field.Name]

		if !present {
			if field.Required {
				decision.Errors = append(decision.Errors, fmt.Sprintf("required field %q is missing", field.Name))
			}
			continue
		}

		if fieldErr := decidePayloadFieldValue(field, value); fieldErr != "" {
			decision.Errors = append(decision.Errors, fieldErr)
		}
	}

	for key := range decoded {
		if _, ok := seen[key]; !ok {
			decision.UnknownKeys = append(decision.UnknownKeys, key)
		}
	}
	sort.Strings(decision.UnknownKeys)

	if len(decision.UnknownKeys) > 0 {
		if enf.Strict {
			for _, key := range decision.UnknownKeys {
				decision.Errors = append(decision.Errors, fmt.Sprintf("strict mode: unknown key %q is not declared in the schema", key))
			}
		} else {
			for _, key := range decision.UnknownKeys {
				decision.Warnings = append(decision.Warnings, fmt.Sprintf("payload carries undeclared key %q (allowed in non-strict mode)", key))
			}
		}
	}

	switch {
	case len(decision.Errors) > 0:
		decision.Outcome = PayloadSchemaReject
		decision.Reason = fmt.Sprintf("payload violates %d schema constraint(s)", len(decision.Errors))
	case len(decision.Warnings) > 0:
		decision.Outcome = PayloadSchemaWarn
		decision.Reason = fmt.Sprintf("payload accepted with %d warning(s)", len(decision.Warnings))
	default:
		decision.Outcome = PayloadSchemaAccept
		decision.Reason = "payload satisfies the declared schema fields"
	}

	return decision
}

// decidePayloadFieldValue 校验单个字段值是否满足其类型与约束,返回错误消息(空串=通过)。
// 语义与 backend service.validatePayloadFieldValue 一一对应。
func decidePayloadFieldValue(field PayloadSchemaFieldConstraint, value any) string {
	switch field.Type {
	case PayloadSchemaFieldTypeNumber:
		num, ok := value.(float64)
		if !ok {
			return fmt.Sprintf("field %q must be a number", field.Name)
		}
		if field.Min != nil && num < *field.Min {
			return fmt.Sprintf("field %q value %.6g is below min %.6g", field.Name, num, *field.Min)
		}
		if field.Max != nil && num > *field.Max {
			return fmt.Sprintf("field %q value %.6g is above max %.6g", field.Name, num, *field.Max)
		}
	case PayloadSchemaFieldTypeString:
		str, ok := value.(string)
		if !ok {
			return fmt.Sprintf("field %q must be a string", field.Name)
		}
		if len(field.Enum) > 0 && !payloadSchemaContains(field.Enum, str) {
			return fmt.Sprintf("field %q value %q is not in the allowed enum", field.Name, str)
		}
		if field.Pattern != nil && *field.Pattern != "" {
			re, err := regexp.Compile(*field.Pattern)
			if err != nil {
				return fmt.Sprintf("field %q has an invalid pattern: %s", field.Name, err.Error())
			}
			if !re.MatchString(str) {
				return fmt.Sprintf("field %q value %q does not match pattern %q", field.Name, str, *field.Pattern)
			}
		}
	case PayloadSchemaFieldTypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Sprintf("field %q must be a boolean", field.Name)
		}
	case PayloadSchemaFieldTypeObject:
		if _, ok := value.(map[string]any); !ok {
			return fmt.Sprintf("field %q must be an object", field.Name)
		}
	case PayloadSchemaFieldTypeArray:
		if _, ok := value.([]any); !ok {
			return fmt.Sprintf("field %q must be an array", field.Name)
		}
	default:
		return fmt.Sprintf("field %q has unsupported type %q", field.Name, field.Type)
	}
	return ""
}

func payloadSchemaContains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
