package uplink

import (
	"testing"

	"aetherlink-iot/backend/internal/storage"
)

func TestConvertTelemetryMapToPointsSortsKeysForStableFanOut(t *testing.T) {
	points, triggerParam, triggerValues := convertTelemetryMapToPoints(map[string]interface{}{
		"temperature": 23,
		"battery":     91,
		"humidity":    44,
	})

	wantKeys := []string{"battery", "humidity", "temperature"}
	if len(points) != len(wantKeys) || len(triggerParam) != len(wantKeys) {
		t.Fatalf("unexpected lengths points=%d triggerParam=%d", len(points), len(triggerParam))
	}
	for idx, want := range wantKeys {
		if points[idx].Key != want {
			t.Fatalf("point[%d].Key = %q, want %q", idx, points[idx].Key, want)
		}
		if triggerParam[idx] != want {
			t.Fatalf("triggerParam[%d] = %q, want %q", idx, triggerParam[idx], want)
		}
		if triggerValues[want] == nil {
			t.Fatalf("triggerValues missing key %q", want)
		}
	}
}

func TestConvertTelemetryMapToPointsKeepsEmptyPayloadSideEffectFree(t *testing.T) {
	points, triggerParam, triggerValues := convertTelemetryMapToPoints(map[string]interface{}{})

	if len(points) != 0 || len(triggerParam) != 0 || len(triggerValues) != 0 {
		t.Fatalf("expected empty telemetry conversion, got points=%#v triggerParam=%#v triggerValues=%#v", points, triggerParam, triggerValues)
	}
}

func TestNormalizeLegacyRDITelemetryAliasesAddsCanonicalKeys(t *testing.T) {
	data := normalizeLegacyRDITelemetryAliases(map[string]interface{}{
		"T1":               21.5,
		"T2":               18.75,
		"NC_INPUT_1_LEVEL": 1,
		"NC_INPUT_2_Level": 0,
		"NO_Level":         true,
	})

	want := map[string]interface{}{
		"temperature_1":      21.5,
		"temperature_2":      18.75,
		"switch_1":           1,
		"switch_2":           0,
		"dry_contact_output": true,
	}
	for key, expected := range want {
		if data[key] != expected {
			t.Fatalf("alias %q = %#v, want %#v", key, data[key], expected)
		}
	}
}

func TestNormalizeLegacyRDITelemetryAliasesKeepsCanonicalValues(t *testing.T) {
	data := normalizeLegacyRDITelemetryAliases(map[string]interface{}{
		"T1":            21.5,
		"temperature_1": 22.75,
	})

	if data["temperature_1"] != 22.75 {
		t.Fatalf("canonical temperature_1 was overwritten: %#v", data["temperature_1"])
	}
}

func TestTelemetrySideEffectsEnabledOnlyWhenPointsExist(t *testing.T) {
	if telemetrySideEffectsEnabled(nil) {
		t.Fatalf("nil telemetry points should not enable side effects")
	}
	if telemetrySideEffectsEnabled([]storage.TelemetryDataPoint{}) {
		t.Fatalf("empty telemetry points should not enable side effects")
	}
	if !telemetrySideEffectsEnabled([]storage.TelemetryDataPoint{{Key: "temperature", Value: 23}}) {
		t.Fatalf("non-empty telemetry points should enable side effects")
	}
}
