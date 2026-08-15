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

func NewTopicMapService() *TopicMapService {
	return &TopicMapService{}
}

func (s *TopicMapService) ResolveUpTarget(ctx context.Context, deviceConfigID string, incomingSource string) (string, bool) {
	mappings, err := GetMappingsWithCache(ctx, deviceConfigID, DirectionUp)
	if err != nil || len(mappings) == 0 {
		return "", false
	}
	return resolveUpTargetFromMappings(mappings, incomingSource)
}

func resolveUpTargetFromMappings(mappings []DeviceTopicMapping, incomingSource string) (string, bool) {
	for _, mapping := range mappings {
		rx, ok := compileSourcePattern(mapping.SourceTopic)
		if !ok {
			continue
		}
		if rx.MatchString(incomingSource) {
			return applyTarget(mapping.TargetTopic, incomingSource), true
		}
	}
	return "", false
}

func (s *TopicMapService) AllowDownSubscribe(ctx context.Context, deviceConfigID string, subscribeTopic string) bool {
	mappings, err := GetMappingsWithCache(ctx, deviceConfigID, DirectionDown)
	if err != nil || len(mappings) == 0 {
		return false
	}
	return allowDownSubscribeFromMappings(mappings, subscribeTopic)
}

func allowDownSubscribeFromMappings(mappings []DeviceTopicMapping, subscribeTopic string) bool {
	for _, mapping := range mappings {
		rx, ok := compileSourcePattern(mapping.SourceTopic)
		if !ok {
			continue
		}
		if rx.MatchString(subscribeTopic) {
			return true
		}
	}
	return false
}

func (s *TopicMapService) ResolveDownSource(ctx context.Context, deviceConfigID string, normalizedTarget string, deviceNumber string, payload []byte) (string, []byte, bool) {
	mappings, err := GetMappingsWithCache(ctx, deviceConfigID, DirectionDown)
	if err != nil || len(mappings) == 0 {
		return "", nil, false
	}
	return resolveDownSourceFromMappings(mappings, normalizedTarget, deviceNumber, payload)
}

func resolveDownSourceFromMappings(mappings []DeviceTopicMapping, normalizedTarget string, deviceNumber string, payload []byte) (string, []byte, bool) {
	fallbackSource := ""
	fallbackPayload := payload

	for _, mapping := range mappings {
		rx, ok := compileTargetPattern(mapping.TargetTopic)
		if !ok {
			Log.Debug("compile normalized target pattern failed", zap.String("target_topic", mapping.TargetTopic))
			continue
		}
		if !rx.MatchString(normalizedTarget) {
			continue
		}

		vars := map[string]string{
			"device_number": deviceNumber,
		}
		source := renderTopicFromTemplate(mapping.SourceTopic, vars)
		Log.Debug("rendered original source topic", zap.String("rendered_source", source))
		if strings.Contains(source, "+") || strings.Contains(source, "#") {
			Log.Debug("rendered source topic still contains wildcard", zap.String("rendered_source", source))
			continue
		}

		if mapping.DataIdentifier != nil && strings.TrimSpace(*mapping.DataIdentifier) != "" {
			var cmd struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(payload, &cmd); err != nil {
				Log.Warn("payload parse failed, skip data identifier match", zap.Error(err))
				continue
			}
			if cmd.Method != strings.TrimSpace(*mapping.DataIdentifier) {
				continue
			}
			out := cmd.Params
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
		return fallbackSource, fallbackPayload, true
	}
	return "", nil, false
}
