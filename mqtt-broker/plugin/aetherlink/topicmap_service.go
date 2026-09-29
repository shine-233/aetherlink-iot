// 文件用途：维护 plugin\aetherlink\topicmap_service.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"context"
	"encoding/json"
	"strings"

	"go.uber.org/zap"
)

type TopicMapService struct{}

var resolveUpTarget = func(ctx context.Context, deviceConfigID string, incomingSource string) (string, bool) {
	return NewTopicMapService().ResolveUpTarget(ctx, deviceConfigID, incomingSource)
}

func NewTopicMapService() *TopicMapService {
	return &TopicMapService{}
}

func (s *TopicMapService) ResolveUpTarget(ctx context.Context, deviceConfigID string, incomingSource string) (string, bool) {
	mappings, err := getCompiledMappings(ctx, deviceConfigID, DirectionUp)
	if err != nil || len(mappings) == 0 {
		return "", false
	}
	return resolveUpTargetCompiled(mappings, incomingSource)
}

func resolveUpTargetFromMappings(mappings []DeviceTopicMapping, incomingSource string) (string, bool) {
	return resolveUpTargetCompiled(compileTopicMappings(mappings), incomingSource)
}

func resolveUpTargetCompiled(mappings []compiledTopicMapping, incomingSource string) (string, bool) {
	for i := range mappings {
		mapping := &mappings[i]
		if mapping.sourceRx == nil {
			continue
		}
		if mapping.sourceRx.MatchString(incomingSource) {
			return applyTarget(mapping.TargetTopic, incomingSource), true
		}
	}
	return "", false
}

func (s *TopicMapService) AllowDownSubscribe(ctx context.Context, deviceConfigID string, subscribeTopic string) bool {
	mappings, err := getCompiledMappings(ctx, deviceConfigID, DirectionDown)
	if err != nil || len(mappings) == 0 {
		return false
	}
	return allowDownSubscribeCompiled(mappings, subscribeTopic)
}

func allowDownSubscribeFromMappings(mappings []DeviceTopicMapping, subscribeTopic string) bool {
	return allowDownSubscribeCompiled(compileTopicMappings(mappings), subscribeTopic)
}

func allowDownSubscribeCompiled(mappings []compiledTopicMapping, subscribeTopic string) bool {
	for i := range mappings {
		if rx := mappings[i].sourceRx; rx != nil && rx.MatchString(subscribeTopic) {
			return true
		}
	}
	return false
}

func (s *TopicMapService) ResolveDownSource(ctx context.Context, deviceConfigID string, normalizedTarget string, deviceNumber string, payload []byte) (string, []byte, bool) {
	mappings, err := getCompiledMappings(ctx, deviceConfigID, DirectionDown)
	if err != nil || len(mappings) == 0 {
		return "", nil, false
	}
	return resolveDownSourceCompiled(mappings, normalizedTarget, deviceNumber, payload)
}

func resolveDownSourceFromMappings(mappings []DeviceTopicMapping, normalizedTarget string, deviceNumber string, payload []byte) (string, []byte, bool) {
	return resolveDownSourceCompiled(compileTopicMappings(mappings), normalizedTarget, deviceNumber, payload)
}

func resolveDownSourceCompiled(mappings []compiledTopicMapping, normalizedTarget string, deviceNumber string, payload []byte) (string, []byte, bool) {
	fallbackSource := ""
	// 命令载荷至多解析一次（原实现每条带 data_identifier 的映射都会重新 Unmarshal）。
	var cmd struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	cmdParsed, cmdErr := false, error(nil)

	for i := range mappings {
		mapping := &mappings[i]
		if mapping.targetRx == nil {
			Log.Debug("compile normalized target pattern failed", zap.String("target_topic", mapping.TargetTopic))
			continue
		}
		if !mapping.targetRx.MatchString(normalizedTarget) {
			continue
		}

		source := renderTopicFromTemplate(mapping.SourceTopic, map[string]string{
			"device_number": deviceNumber,
		})
		Log.Debug("rendered original source topic", zap.String("rendered_source", source))
		if strings.Contains(source, "+") || strings.Contains(source, "#") {
			Log.Debug("rendered source topic still contains wildcard", zap.String("rendered_source", source))
			continue
		}

		if mapping.DataIdentifier != nil && strings.TrimSpace(*mapping.DataIdentifier) != "" {
			if !cmdParsed {
				cmdParsed = true
				cmdErr = json.Unmarshal(payload, &cmd)
			}
			if cmdErr != nil {
				Log.Warn("payload parse failed, skip data identifier match", zap.Error(cmdErr))
				continue
			}
			if cmd.Method != strings.TrimSpace(*mapping.DataIdentifier) {
				continue
			}
			out := append([]byte(nil), cmd.Params...)
			if len(out) == 0 {
				out = []byte("{}")
			}
			return source, out, true
		}

		if fallbackSource == "" {
			fallbackSource = source
		}
	}

	if fallbackSource != "" {
		return fallbackSource, append([]byte(nil), payload...), true
	}
	return "", nil, false
}
