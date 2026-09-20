package dal

import (
	"context"
	"testing"
)

func TestResolveDeviceTelemetryUnitEmptyInput(t *testing.T) {
	ctx := context.Background()

	// 空 deviceID 或空 key 必须安全返回 ("", nil)
	unit, err := ResolveDeviceTelemetryUnit(ctx, "", "temperature")
	if err != nil || unit != "" {
		t.Fatalf("expected empty unit and nil error, got unit=%q err=%v", unit, err)
	}

	unit, err = ResolveDeviceTelemetryUnit(ctx, "dev-123", "")
	if err != nil || unit != "" {
		t.Fatalf("expected empty unit and nil error, got unit=%q err=%v", unit, err)
	}

	unit, err = ResolveDeviceTelemetryUnit(ctx, "   ", "   ")
	if err != nil || unit != "" {
		t.Fatalf("expected empty unit and nil error, got unit=%q err=%v", unit, err)
	}
}
