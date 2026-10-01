package api

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

// TestDeviceModelKindFromURI 覆盖 URI 片段到物模型类别的分发，以及 allowed 子集外的拒绝。
func TestDeviceModelKindFromURI(t *testing.T) {
	all := []string{model.DEVICE_MODEL_TELEMETRY, model.DEVICE_MODEL_ATTRIBUTES, model.DEVICE_MODEL_EVENTS, model.DEVICE_MODEL_COMMANDS}
	cases := []struct {
		uri     string
		allowed []string
		want    string
		wantErr bool
	}{
		{"/api/v1/device/model/telemetry?page=1", all, model.DEVICE_MODEL_TELEMETRY, false},
		{"/api/v1/device/model/attributes/abc", all, model.DEVICE_MODEL_ATTRIBUTES, false},
		{"/api/v1/device/model/events", all, model.DEVICE_MODEL_EVENTS, false},
		{"/api/v1/device/model/commands", all, model.DEVICE_MODEL_COMMANDS, false},
		{"/api/v1/device/model/unknown", all, "", true},
		{"/api/v1/device/model/events", all[:2], "", true},
		{"/api/v1/device/model/telemetry", all[2:], "", true},
		// 多片段同时出现时按 allowed 顺序取第一个命中（与原 if/else 链一致）。
		{"/api/v1/device/model/commands?from=telemetry", all, model.DEVICE_MODEL_TELEMETRY, false},
	}
	for _, tc := range cases {
		got, err := deviceModelKindFromURI(tc.uri, tc.allowed...)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("uri=%q allowed=%v: got (%q, %v), want %q wantErr=%v", tc.uri, tc.allowed, got, err, tc.want, tc.wantErr)
		}
	}
}
