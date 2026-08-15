// 文件用途：实现 payload schema registry 的静态校验引擎。
// 核心逻辑：把一组字段约束(类型/必填/范围/枚举/正则)应用到一份样本 payload,
//
//	产出结构化诊断。它是纯函数式静态推演,不落库、不连 broker、不改协议契约。
//
// 关键注意事项：这是 payload schema 能力中"可离线验证"的一半;broker 侧对上行 payload 的
//
//	真实拦截属于外部 MQTT 契约的破坏性变更,需要运行时(broker+PG+设备)验证,不在此实现。
//
// 重构建议：若引入持久化 registry,应让 CRUD 层复用本引擎,保持"校验逻辑单一来源"。
package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

// PayloadSchema 是 payload schema 能力的服务入口(无状态)。
type PayloadSchema struct{}

// ValidatePayload 针对提交的字段约束和样本 payload 做静态校验。
// 它不保存 schema、不连接 broker、不下发任何消息,只回报"这份 payload 是否满足声明的结构"。
func (*PayloadSchema) ValidatePayload(req *model.ValidatePayloadReq, claims *utils.UserClaims) (*model.ValidatePayloadResult, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to validate payload schema")
	}

	result := &model.ValidatePayloadResult{
		Supported:    true,
		Valid:        true,
		Summary:      "payload schema 校验只静态比对样本 payload 与声明的字段约束,不保存 schema、不连接 broker、不下发消息。",
		FieldCount:   len(req.Fields),
		Errors:       []string{},
		Warnings:     []string{},
		UnknownKeys:  []string{},
		IsSimulation: true,
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(req.SamplePayload), &decoded); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, "sample payload is not a valid JSON object")
		result.Diagnostics = buildPayloadSchemaDiagnostics(result.Errors, result.Warnings)
		result.NextSteps = buildPayloadSchemaNextSteps(result)
		return result, nil
	}

	seen := map[string]struct{}{}
	for _, field := range req.Fields {
		seen[field.Name] = struct{}{}
		result.CheckedFields++
		value, present := decoded[field.Name]

		if !present {
			if field.Required {
				msg := fmt.Sprintf("required field %q is missing", field.Name)
				result.Errors = append(result.Errors, msg)
				result.Valid = false
			}
			continue
		}

		if fieldErr := validatePayloadFieldValue(field, value); fieldErr != "" {
			result.Errors = append(result.Errors, fieldErr)
			result.Valid = false
		}
	}

	for key := range decoded {
		if _, ok := seen[key]; !ok {
			result.UnknownKeys = append(result.UnknownKeys, key)
		}
	}
	sort.Strings(result.UnknownKeys)

	if len(result.UnknownKeys) > 0 {
		if req.Strict {
			for _, key := range result.UnknownKeys {
				result.Errors = append(result.Errors, fmt.Sprintf("strict mode: unknown key %q is not declared in the schema", key))
			}
			result.Valid = false
		} else {
			for _, key := range result.UnknownKeys {
				result.Warnings = append(result.Warnings, fmt.Sprintf("payload carries undeclared key %q (allowed in non-strict mode)", key))
			}
		}
	}

	result.Diagnostics = buildPayloadSchemaDiagnostics(result.Errors, result.Warnings)
	result.NextSteps = buildPayloadSchemaNextSteps(result)
	return result, nil
}

// validatePayloadFieldValue 校验单个字段值是否满足其类型与约束,返回错误消息(空串表示通过)。
func validatePayloadFieldValue(field model.PayloadSchemaField, value any) string {
	switch field.Type {
	case model.PayloadSchemaFieldTypeNumber:
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
	case model.PayloadSchemaFieldTypeString:
		str, ok := value.(string)
		if !ok {
			return fmt.Sprintf("field %q must be a string", field.Name)
		}
		if len(field.Enum) > 0 && !contains(field.Enum, str) {
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
	case model.PayloadSchemaFieldTypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Sprintf("field %q must be a boolean", field.Name)
		}
	case model.PayloadSchemaFieldTypeObject:
		if _, ok := value.(map[string]any); !ok {
			return fmt.Sprintf("field %q must be an object", field.Name)
		}
	case model.PayloadSchemaFieldTypeArray:
		if _, ok := value.([]any); !ok {
			return fmt.Sprintf("field %q must be an array", field.Name)
		}
	}
	return ""
}

func buildPayloadSchemaDiagnostics(errors []string, warnings []string) []model.PayloadSchemaValidationDiagnostic {
	diagnostics := make([]model.PayloadSchemaValidationDiagnostic, 0, len(errors)+len(warnings)+1)
	for _, errText := range errors {
		diagnostics = append(diagnostics, model.PayloadSchemaValidationDiagnostic{
			Severity: "error",
			Scope:    "field",
			Message:  errText,
		})
	}
	for _, warning := range warnings {
		diagnostics = append(diagnostics, model.PayloadSchemaValidationDiagnostic{
			Severity: "warning",
			Scope:    "payload",
			Message:  warning,
		})
	}
	if len(errors) == 0 {
		diagnostics = append(diagnostics, model.PayloadSchemaValidationDiagnostic{
			Severity: "success",
			Scope:    "payload",
			Message:  "sample payload satisfies the declared schema fields",
		})
	}
	return diagnostics
}

func buildPayloadSchemaNextSteps(result *model.ValidatePayloadResult) []string {
	if !result.Valid {
		return []string{
			"fix the reported field errors and re-run the validation",
			"broker 侧真实拦截需在部署环境(broker+PG+设备)验证,本校验只做静态比对",
		}
	}
	if len(result.Warnings) > 0 {
		return []string{
			"review undeclared keys before enabling strict validation at the broker",
			"如需 broker 硬性拒收未声明字段,请在部署环境启用严格模式并做运行时验证",
		}
	}
	return []string{
		"schema matches the sample payload; wire it into the broker validation config when deploying",
		"broker 侧真实拦截属于外部 MQTT 契约变更,部署时需运行时验证",
	}
}
