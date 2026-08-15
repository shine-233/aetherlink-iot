// device_topic_mapping_dry_run.go contains the read-only topic mapping preview
// used by the public dry-run endpoint. CRUD and cache invalidation remain in
// device_topic_mapping.go.
package service

import (
	"context"
	"fmt"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type normalizedDryRunDeviceTopicMappingInput struct {
	Direction      string
	SourceTopic    string
	TargetTopic    string
	TestTopic      string
	DataIdentifier *string
}

func (*DeviceTopicMapping) DryRunDeviceTopicMapping(req *model.DryRunDeviceTopicMappingReq, claims *utils.UserClaims) (model.DryRunDeviceTopicMappingResp, error) {
	ctx := context.Background()
	normalized := normalizeDryRunDeviceTopicMappingInput(req)
	resp := model.DryRunDeviceTopicMappingResp{
		Direction:      normalized.Direction,
		SourceTopic:    normalized.SourceTopic,
		TargetTopic:    normalized.TargetTopic,
		TestTopic:      normalized.TestTopic,
		SampleTopic:    normalized.TestTopic,
		ResolvedTopic:  normalized.TargetTopic,
		DataIdentifier: normalized.DataIdentifier,
		Diagnostics:    []model.DeviceTopicMappingDryRunDiagnostic{},
		NextSteps:      []string{},
	}

	if _, err := loadOwnedDeviceConfig(ctx, req.DeviceConfigID, claims, "device config not found", "device config not owned by current tenant", func(err error) error {
		logrus.Error(err)
		return wrapTopicMappingDBError(err)
	}); err != nil {
		return resp, err
	}

	captures, diagnostics := previewDeviceTopicMapping(resp.SourceTopic, resp.TestTopic)
	resp.Diagnostics = append(resp.Diagnostics, diagnostics...)
	resp.Matched = topicMappingDryRunHasNoErrors(resp.Diagnostics)
	resp.ResolvedTopic = resolveTopicMappingTarget(resp.TargetTopic, captures)
	resp.Diagnostics = append(resp.Diagnostics, diagnoseResolvedTopic(resp.TargetTopic, resp.ResolvedTopic, captures, resp.DataIdentifier)...)
	resp.Matched = topicMappingDryRunHasNoErrors(resp.Diagnostics)
	resp.NextSteps = topicMappingDryRunNextSteps(resp.Matched, resp.Direction, resp.DataIdentifier)
	return resp, nil
}

func normalizeDryRunDeviceTopicMappingInput(req *model.DryRunDeviceTopicMappingReq) normalizedDryRunDeviceTopicMappingInput {
	return normalizedDryRunDeviceTopicMappingInput{
		Direction:      strings.ToLower(strings.TrimSpace(req.Direction)),
		SourceTopic:    normalizeDryRunTopic(req.SourceTopic),
		TargetTopic:    normalizeDryRunTopic(req.TargetTopic),
		TestTopic:      normalizeDryRunTopic(firstNonBlank(req.TestTopic, req.SampleTopic)),
		DataIdentifier: normalizeOptionalTopicMappingDataIdentifier(req.DataIdentifier),
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeDryRunTopic(value string) string {
	return strings.Trim(strings.TrimSpace(value), "/")
}

func previewDeviceTopicMapping(pattern string, testTopic string) (map[string]string, []model.DeviceTopicMappingDryRunDiagnostic) {
	captures := map[string]string{}
	diagnostics := []model.DeviceTopicMappingDryRunDiagnostic{}
	if pattern == "" || testTopic == "" {
		return captures, append(diagnostics, newTopicMappingDryRunDiagnostic("error", "source_topic", "source_topic and test_topic cannot be blank"))
	}
	if strings.Contains(pattern, "#") {
		diagnostics = append(diagnostics, newTopicMappingDryRunDiagnostic("error", "source_topic", "multi-level wildcard # is not supported by this mapping preview"))
	}

	patternParts := strings.Split(pattern, "/")
	testTopicParts := strings.Split(testTopic, "/")
	if len(patternParts) != len(testTopicParts) {
		diagnostics = append(diagnostics, newTopicMappingDryRunDiagnostic(
			"error",
			"test_topic",
			fmt.Sprintf("segment count mismatch: source_topic has %d segments, test_topic has %d", len(patternParts), len(testTopicParts)),
		))
		return captures, diagnostics
	}

	wildcardIndex := 1
	for index, patternPart := range patternParts {
		testTopicPart := testTopicParts[index]
		switch {
		case patternPart == "+":
			captures[fmt.Sprintf("wildcard_%d", wildcardIndex)] = testTopicPart
			wildcardIndex++
		case isTopicMappingVariable(patternPart):
			captures[strings.TrimSuffix(strings.TrimPrefix(patternPart, "{"), "}")] = testTopicPart
		case patternPart != testTopicPart:
			diagnostics = append(diagnostics, newTopicMappingDryRunDiagnostic(
				"error",
				"test_topic",
				fmt.Sprintf("segment %d does not match: expected %q, got %q", index+1, patternPart, testTopicPart),
			))
		}
	}

	if len(diagnostics) == 0 {
		diagnostics = append(diagnostics, newTopicMappingDryRunDiagnostic("success", "source_topic", "test_topic matches the device topic pattern"))
	}
	return captures, diagnostics
}

func isTopicMappingVariable(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") && len(segment) > 2
}

func resolveTopicMappingTarget(target string, captures map[string]string) string {
	resolved := target
	for key, value := range captures {
		resolved = strings.ReplaceAll(resolved, "{"+key+"}", value)
	}
	return resolved
}

func diagnoseResolvedTopic(target string, resolved string, captures map[string]string, dataIdentifier *string) []model.DeviceTopicMappingDryRunDiagnostic {
	diagnostics := []model.DeviceTopicMappingDryRunDiagnostic{}
	for _, segment := range strings.Split(target, "/") {
		if !isTopicMappingVariable(segment) {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
		if _, ok := captures[name]; !ok {
			diagnostics = append(diagnostics, newTopicMappingDryRunDiagnostic("warning", "target_topic", fmt.Sprintf("target variable %q is not captured from source_topic", name)))
		}
	}
	if target != resolved {
		diagnostics = append(diagnostics, newTopicMappingDryRunDiagnostic("info", "target_topic", fmt.Sprintf("target topic resolves to %q", resolved)))
	}
	if dataIdentifier != nil {
		diagnostics = append(diagnostics, newTopicMappingDryRunDiagnostic("info", "data_identifier", fmt.Sprintf("command identifier %q will be included in the preview result", *dataIdentifier)))
	}
	return diagnostics
}

func newTopicMappingDryRunDiagnostic(severity string, scope string, message string) model.DeviceTopicMappingDryRunDiagnostic {
	return model.DeviceTopicMappingDryRunDiagnostic{
		Severity: severity,
		Scope:    scope,
		Message:  message,
	}
}

func topicMappingDryRunHasNoErrors(diagnostics []model.DeviceTopicMappingDryRunDiagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return false
		}
	}
	return true
}

func topicMappingDryRunNextSteps(matched bool, direction string, dataIdentifier *string) []string {
	if !matched {
		return []string{
			"调整设备主题模式或测试主题，直到所有片段匹配。",
			"单个动态片段请使用 + 或 {variable_name}；此映射中避免使用 #。",
		}
	}

	steps := []string{
		"确认解析出的主题就是预期系统路由后，再保存此映射。",
	}
	if direction == "down" {
		steps = append(steps, "发送云端命令前，请确认设备已订阅设备主题。")
		if dataIdentifier != nil {
			steps = append(steps, "请保持命令标识符与 Web 控制台或 API 使用的命令方法一致。")
		}
	} else {
		steps = append(steps, "在测试主题上发布一条真实 MQTT 消息，验证载荷解析和存储。")
	}
	return steps
}
