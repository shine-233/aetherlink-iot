// 文件用途：维护 plugin\aetherlink\topicmap_service_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"testing"

	"go.uber.org/zap"
)

func init() {
	Log = zap.NewNop()
}

func TestTopicMapServiceResolvesUpTargetAndRejectsInvalidSourceMapping(t *testing.T) {
	mappings := []DeviceTopicMapping{
		{
			SourceTopic: "devices/#",
			TargetTopic: "should/not/match",
		},
		{
			SourceTopic: "raw/{device_number}/telemetry/+",
			TargetTopic: "devices/telemetry",
		},
	}

	target, ok := resolveUpTargetFromMappings(mappings, "raw/dev-001/telemetry/temp")
	if !ok {
		t.Fatal("expected up mapping to match telemetry source topic")
	}
	if target != "devices/telemetry" {
		t.Fatalf("target = %q", target)
	}

	if _, ok := resolveUpTargetFromMappings(mappings, "raw/dev-001/event/temp"); ok {
		t.Fatal("unexpected up mapping match for unrelated source topic")
	}
}

func TestTopicMapServiceAllowsDownSubscribeOnlyForConfiguredSourceTopic(t *testing.T) {
	mappings := []DeviceTopicMapping{
		{
			SourceTopic: "devices/{device_number}/command/+",
			TargetTopic: "platform/command/{device_number}/+",
		},
	}

	if !allowDownSubscribeFromMappings(mappings, "devices/dev-001/command/reboot") {
		t.Fatal("expected down subscribe to be allowed by source mapping")
	}
	if allowDownSubscribeFromMappings(mappings, "platform/command/dev-001/reboot") {
		t.Fatal("device down subscribe must match source_topic, not normalized target_topic")
	}
	if allowDownSubscribeFromMappings(mappings, "devices/dev-001/attributes/mode") {
		t.Fatal("unexpected down subscribe allowance for non-matching topic")
	}
}

func TestTopicMapServiceFiltersDownPayloadByDataIdentifier(t *testing.T) {
	method := "set_mode"
	mappings := []DeviceTopicMapping{
		{
			SourceTopic:    "raw/{device_number}/set",
			TargetTopic:    "devices/command/{device_number}/+",
			DataIdentifier: &method,
		},
	}
	payload := []byte(`{"method":"set_mode","params":{"mode":"eco"}}`)

	source, out, ok := resolveDownSourceFromMappings(mappings, "devices/command/dev-001/set_mode", "dev-001", payload)
	if !ok {
		t.Fatal("expected down source to match data identifier")
	}
	if source != "raw/dev-001/set" {
		t.Fatalf("source = %q", source)
	}
	if string(out) != `{"mode":"eco"}` {
		t.Fatalf("filtered payload = %s", string(out))
	}
}

func TestTopicMapServiceFallsBackToUnfilteredDownPayload(t *testing.T) {
	payload := []byte(`{"method":"unknown","params":{"value":1}}`)
	mappings := []DeviceTopicMapping{
		{
			SourceTopic: "raw/{device_number}/fallback",
			TargetTopic: "devices/command/{device_number}/+",
		},
	}

	source, out, ok := resolveDownSourceFromMappings(mappings, "devices/command/dev-002/unknown", "dev-002", payload)
	if !ok {
		t.Fatal("expected down source fallback mapping")
	}
	if source != "raw/dev-002/fallback" {
		t.Fatalf("source = %q", source)
	}
	if string(out) != string(payload) {
		t.Fatalf("fallback payload = %s", string(out))
	}
}
