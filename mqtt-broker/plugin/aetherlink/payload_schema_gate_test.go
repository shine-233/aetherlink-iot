// 文件用途：payload_schema_gate.go 门控接线层的单元测试。
// 覆盖:默认关闭(nil resolver)零拦截、设备无绑定放行、reject 拦截、warn/accept 放行,
// 以及 SetPayloadSchemaResolver 的注入/清空。
// 注意:这些用例只验证门控开关与 reject→拦截翻译的纯逻辑;真实 broker 会话拦截需运行时验证,不在此覆盖。
package aetherlink

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// resetPayloadSchemaResolver 在每个用例结束后恢复“强制关闭”默认,避免污染其它测试。
func resetPayloadSchemaResolver(t *testing.T) {
	t.Cleanup(func() { SetPayloadSchemaResolver(nil) })
}

func TestEnforcePayloadSchemaOnUplink_DisabledByDefault(t *testing.T) {
	resetPayloadSchemaResolver(t)
	// 未注入 resolver ⇒ 强制关闭 ⇒ 即便 payload 非法也不拦截(行为不变)。
	if payloadSchemaEnforcementEnabled() {
		t.Fatal("enforcement should be disabled by default")
	}
	if enforcePayloadSchemaOnUplink("dev-1", "cfg-1", []byte(`not json`)) {
		t.Fatal("disabled enforcement must never reject")
	}
}

func TestEnforcePayloadSchemaOnUplink_UnboundDevicePasses(t *testing.T) {
	resetPayloadSchemaResolver(t)
	// resolver 返回 bound=false ⇒ 该设备无绑定 schema ⇒ 放行不校验。
	SetPayloadSchemaResolver(func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool) {
		return PayloadSchemaEnforcement{}, false
	})
	if !payloadSchemaEnforcementEnabled() {
		t.Fatal("enforcement should report enabled after injecting a resolver")
	}
	if enforcePayloadSchemaOnUplink("dev-1", "cfg-1", []byte(`not json`)) {
		t.Fatal("unbound device must pass even with invalid payload")
	}
}

func TestEnforcePayloadSchemaOnUplink_ResolverErrorIsBlocked(t *testing.T) {
	resetPayloadSchemaResolver(t)
	setPayloadSchemaResolverWithError(func(deviceID, deviceConfigID string) payloadSchemaResolution {
		return payloadSchemaResolution{err: errors.New("database unavailable")}
	})

	if !enforcePayloadSchemaOnUplink("dev-1", "cfg-1", []byte(`{"temp":42}`)) {
		t.Fatal("enabled enforcement must reject when its production dependency is unavailable")
	}
}

func TestEnforcePayloadSchemaOnUplink_RejectIsBlocked(t *testing.T) {
	resetPayloadSchemaResolver(t)
	SetPayloadSchemaResolver(func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool) {
		return PayloadSchemaEnforcement{
			Fields: []PayloadSchemaFieldConstraint{
				{Name: "temp", Type: PayloadSchemaFieldTypeNumber, Required: true, Min: f64(0), Max: f64(100)},
			},
		}, true
	})
	if !enforcePayloadSchemaOnUplink("dev-1", "cfg-1", []byte(`{"temp":150}`)) {
		t.Fatal("out-of-range payload should be rejected when enforcement is bound")
	}
}

func TestEnforcePayloadSchemaOnUplink_WarnPasses(t *testing.T) {
	resetPayloadSchemaResolver(t)
	SetPayloadSchemaResolver(func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool) {
		return PayloadSchemaEnforcement{
			Strict: false,
			Fields: []PayloadSchemaFieldConstraint{
				{Name: "temp", Type: PayloadSchemaFieldTypeNumber},
			},
		}, true
	})
	// 非 strict 未声明键 ⇒ warn ⇒ 放行(不拦截)。
	if enforcePayloadSchemaOnUplink("dev-1", "cfg-1", []byte(`{"temp":20,"extra":1}`)) {
		t.Fatal("warn outcome must not reject the message")
	}
}

func TestEnforcePayloadSchemaOnUplink_RejectLogOmitsSensitiveValue(t *testing.T) {
	resetPayloadSchemaResolver(t)
	core, observedLogs := observer.New(zapcore.DebugLevel)
	previousLog := Log
	Log = zap.New(core)
	t.Cleanup(func() { Log = previousLog })

	const sensitiveValue = "customer-secret-token-9f3c"
	SetPayloadSchemaResolver(func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool) {
		return PayloadSchemaEnforcement{
			Fields: []PayloadSchemaFieldConstraint{
				{Name: "token", Type: PayloadSchemaFieldTypeString, Enum: []string{"allowed"}},
			},
		}, true
	})

	if !enforcePayloadSchemaOnUplink("dev-1", "cfg-1", []byte(fmt.Sprintf(`{"token":%q}`, sensitiveValue))) {
		t.Fatal("enum violation should be rejected")
	}
	entries := observedLogs.AllUntimed()
	if len(entries) != 1 {
		t.Fatalf("expected one reject log, got %d", len(entries))
	}
	logged := fmt.Sprint(entries[0].ContextMap())
	if strings.Contains(logged, sensitiveValue) {
		t.Fatalf("reject log leaked sensitive payload value: %s", logged)
	}
	if strings.Contains(logged, "not in the allowed enum") {
		t.Fatalf("reject log included the complete validation error: %s", logged)
	}
	if fields := entries[0].ContextMap(); fields["error_count"] != int64(1) {
		t.Fatalf("expected stable error_count summary, got %#v", fields["error_count"])
	}
}

func TestEnforcePayloadSchemaOnUplink_NilLogDoesNotPanic(t *testing.T) {
	resetPayloadSchemaResolver(t)
	previousLog := Log
	Log = nil
	t.Cleanup(func() { Log = previousLog })

	t.Run("reject", func(t *testing.T) {
		SetPayloadSchemaResolver(func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool) {
			return PayloadSchemaEnforcement{
				Fields: []PayloadSchemaFieldConstraint{
					{Name: "temp", Type: PayloadSchemaFieldTypeNumber, Max: f64(100)},
				},
			}, true
		})
		if !enforcePayloadSchemaOnUplink("dev-1", "cfg-1", []byte(`{"temp":150}`)) {
			t.Fatal("reject outcome changed when Log is nil")
		}
	})

	t.Run("warn", func(t *testing.T) {
		SetPayloadSchemaResolver(func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool) {
			return PayloadSchemaEnforcement{}, true
		})
		if enforcePayloadSchemaOnUplink("dev-1", "cfg-1", []byte(`{"extra":1}`)) {
			t.Fatal("warn outcome changed when Log is nil")
		}
	})
}

func TestEnforcePayloadSchemaOnUplink_AcceptPasses(t *testing.T) {
	resetPayloadSchemaResolver(t)
	SetPayloadSchemaResolver(func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool) {
		return PayloadSchemaEnforcement{
			Fields: []PayloadSchemaFieldConstraint{
				{Name: "temp", Type: PayloadSchemaFieldTypeNumber, Required: true, Min: f64(0), Max: f64(100)},
			},
		}, true
	})
	if enforcePayloadSchemaOnUplink("dev-1", "cfg-1", []byte(`{"temp":42}`)) {
		t.Fatal("valid payload must pass")
	}
}

func TestSetPayloadSchemaResolver_NilRestoresDisabled(t *testing.T) {
	resetPayloadSchemaResolver(t)
	SetPayloadSchemaResolver(func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool) {
		return PayloadSchemaEnforcement{}, true
	})
	if !payloadSchemaEnforcementEnabled() {
		t.Fatal("expected enabled after inject")
	}
	SetPayloadSchemaResolver(nil)
	if payloadSchemaEnforcementEnabled() {
		t.Fatal("expected disabled after clearing resolver")
	}
}

func TestPayloadSchemaResolverConcurrentReplacement(t *testing.T) {
	resetPayloadSchemaResolver(t)
	resolver := func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool) {
		return PayloadSchemaEnforcement{}, false
	}

	const iterations = 1000
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i < iterations; i++ {
			SetPayloadSchemaResolver(resolver)
			SetPayloadSchemaResolver(nil)
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < iterations; i++ {
			_ = payloadSchemaEnforcementEnabled()
			_ = enforcePayloadSchemaOnUplink("dev-1", "cfg-1", []byte(`{"temp":42}`))
		}
	}()
	workers.Wait()
}
