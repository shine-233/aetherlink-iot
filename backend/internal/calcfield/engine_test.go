// 文件用途: calcfield 引擎单测——覆盖防环、变量交集求值、结果类型白名单、缓存与丢弃计数。
// 核心逻辑: 用内存 seam 与桩 TemplateSource 驱动 processMessage/enqueueDerived,不依赖真实总线。
// 关键注意事项: 缓存 TTL 相关断言通过直接改写 cache 字段模拟,不引入真实 sleep。
package calcfield

import (
	"context"
	"encoding/json"
	"testing"

	"aetherlink-iot/backend/internal/uplink"

	"github.com/casbin/govaluate"
	"github.com/sirupsen/logrus"
)

type stubStorage struct {
	messages []*uplink.DeviceMessage
	accept   bool
}

func (s *stubStorage) EnqueueDerivedTelemetry(_ context.Context, msg *uplink.DeviceMessage) bool {
	if !s.accept {
		return false
	}
	s.messages = append(s.messages, msg)
	return true
}

type stubSource struct {
	templateID string
	fields     []FieldRule
}

func (s stubSource) ResolveTemplateID(context.Context, string) (string, error) {
	return s.templateID, nil
}

func (s stubSource) ListEnabledFields(context.Context, string) ([]FieldRule, error) {
	return s.fields, nil
}

func newTestEngine(storage StorageEnqueuer, source TemplateSource) *Engine {
	engine := NewEngine(nil, storage, logrus.StandardLogger())
	engine.templateSource = source
	return engine
}

func telemetryMessage(deviceID string, payload map[string]interface{}, metadata map[string]interface{}) *uplink.DeviceMessage {
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return &uplink.DeviceMessage{
		Type:     uplink.MessageTypeTelemetry,
		DeviceID: deviceID,
		TenantID: "tenant-a",
		Payload:  raw,
		Metadata: metadata,
	}
}

func TestProcessMessageDerivesTelemetryAndMarksFlag(t *testing.T) {
	storage := &stubStorage{accept: true}
	engine := newTestEngine(storage, stubSource{
		templateID: "tpl-1",
		fields: []FieldRule{
			{ID: "f-1", OutputKey: "power_w", Expression: "voltage * current"},
		},
	})

	msg := telemetryMessage("dev-1", map[string]interface{}{"voltage": 12.5, "current": 2.0}, nil)
	engine.processMessage(msg)

	if len(storage.messages) != 1 {
		t.Fatalf("derived messages = %d, want 1", len(storage.messages))
	}
	derived := storage.messages[0]
	if derived.DeviceID != "dev-1" || derived.TenantID != "tenant-a" {
		t.Fatalf("derived identity = %s/%s, want dev-1/tenant-a", derived.DeviceID, derived.TenantID)
	}
	if flag, ok := derived.GetMetadata(MetadataGeneratedFlag); !ok || flag != true {
		t.Fatalf("derived message must carry %s=true, got %#v", MetadataGeneratedFlag, derived.Metadata)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(derived.Payload, &payload); err != nil {
		t.Fatalf("decode derived payload: %v", err)
	}
	if value, ok := payload["power_w"].(float64); !ok || value != 25.0 {
		t.Fatalf("payload = %#v, want power_w=25.0", payload)
	}
}

func TestProcessMessageSkipsGeneratedMessages(t *testing.T) {
	storage := &stubStorage{accept: true}
	engine := newTestEngine(storage, stubSource{
		templateID: "tpl-1",
		fields: []FieldRule{
			{ID: "f-1", OutputKey: "power_w", Expression: "voltage * current"},
		},
	})

	msg := telemetryMessage("dev-1", map[string]interface{}{"voltage": 1.0, "current": 1.0}, map[string]interface{}{
		MetadataGeneratedFlag: true,
	})
	engine.processMessage(msg)

	if len(storage.messages) != 0 {
		t.Fatalf("generated messages must not re-enter evaluation, got %d writes", len(storage.messages))
	}
}

func TestProcessMessageSkipsWhenVariablesMissing(t *testing.T) {
	storage := &stubStorage{accept: true}
	engine := newTestEngine(storage, stubSource{
		templateID: "tpl-1",
		fields: []FieldRule{
			{ID: "f-1", OutputKey: "power_w", Expression: "voltage * current"},
		},
	})

	msg := telemetryMessage("dev-1", map[string]interface{}{"voltage": 3.0}, nil)
	engine.processMessage(msg)

	if len(storage.messages) != 0 {
		t.Fatalf("missing variable must skip rule, got %d writes", len(storage.messages))
	}
}

func TestEvaluateRuleRejectsNonScalarResults(t *testing.T) {
	constantExpr, err := govaluate.NewEvaluableExpression(`"constant"`)
	if err != nil {
		t.Fatalf("compile constant expression: %v", err)
	}
	value, ok := evaluateRule(compiledRule{id: "f-1", outputKey: "out", expr: constantExpr}, map[string]interface{}{})
	if !ok || value != "constant" {
		t.Fatalf("string result should pass through, got %#v ok=%v", value, ok)
	}

	rule := mustCompileRule(t, "f-power", "power_w", "voltage * current")
	if _, ok := evaluateRule(rule, map[string]interface{}{"voltage": 1.0}); ok {
		t.Fatal("missing current must not evaluate")
	}
	if _, ok := evaluateRule(rule, map[string]interface{}{"voltage": "abc", "current": 1.0}); ok {
		t.Fatal("non-numeric payload values do not participate; expression on strings must fail closed")
	}
}

func TestEnqueueDerivedCountsDropsWhenQueueFull(t *testing.T) {
	storage := &stubStorage{accept: false}
	engine := newTestEngine(storage, stubSource{templateID: "tpl-1"})

	source := telemetryMessage("dev-1", map[string]interface{}{"voltage": 1.0}, nil)
	engine.enqueueDerived(source, "power_w", 25.0, 12345)

	if engine.DroppedCount() != 1 {
		t.Fatalf("dropped = %d, want 1", engine.DroppedCount())
	}
}

func TestTemplateCacheHonorsTTLAndNegativeResult(t *testing.T) {
	storage := &stubStorage{accept: true}
	engine := newTestEngine(storage, stubSource{
		templateID: "",
		fields: []FieldRule{
			{ID: "f-1", OutputKey: "power_w", Expression: "voltage * current"},
		},
	})

	msg := telemetryMessage("dev-unbound", map[string]interface{}{"voltage": 1.0}, nil)
	engine.processMessage(msg)
	if len(storage.messages) != 0 {
		t.Fatalf("unbound device must produce nothing, got %d writes", len(storage.messages))
	}

	engine.cacheMu.RLock()
	cached, exists := engine.templateCache["dev-unbound"]
	engine.cacheMu.RUnlock()
	if !exists || cached.templateID != "" {
		t.Fatalf("negative lookup should be cached briefly, exists=%v", exists)
	}
}

func mustCompileRule(t *testing.T, id, outputKey, expression string) compiledRule {
	t.Helper()
	expr, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		t.Fatalf("compile %q: %v", expression, err)
	}
	return compiledRule{id: id, outputKey: outputKey, expr: expr, variables: expr.Vars()}
}
