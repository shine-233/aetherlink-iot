package pluginsdk

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestCheckHostCompatibility 宿主版本兼容判定表。
func TestCheckHostCompatibility(t *testing.T) {
	cases := []struct {
		minHostVersion string
		wantErr        bool
	}{
		{"", false},
		{"1.0.0", false},
		{"0.9.0", false},
		{"1.0.1", true},
		{"2.0.0", true},
		{"not-a-version", true},
	}
	for _, tc := range cases {
		err := CheckHostCompatibility(tc.minHostVersion)
		if (err != nil) != tc.wantErr {
			t.Fatalf("CheckHostCompatibility(%q) = %v, wantErr=%v", tc.minHostVersion, err, tc.wantErr)
		}
	}
}

// TestManifestValidateRoundtrip 合法 manifest 必须通过；关键字段缺失/非法必须拒绝。
func TestManifestValidate(t *testing.T) {
	valid := &Manifest{
		Name:      "modbus-tcp",
		Version:   "1.2.0",
		Transport: "grpc",
		PointTable: []PointDefinition{
			{Name: "temperature", Kind: PointTelemetry, Unit: "celsius"},
			{Name: "setpoint", Kind: PointCommand, Writable: true},
		},
		CredentialFields: []CredentialField{
			{Name: "host", Required: true},
			{Name: "token", Required: true, Secret: true},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	broken := []*Manifest{
		{Name: "", Version: "1.0.0"},
		{Name: "bad name!", Version: "1.0.0"},
		{Name: "x", Version: "v1"},
		{Name: "x", Version: "1.0.0", Transport: "serial"},
		{Name: "x", Version: "1.0.0", MinHostVersion: "9.9.9"},
		{Name: "x", Version: "1.0.0", PointTable: []PointDefinition{
			{Name: "a", Kind: PointTelemetry}, {Name: "a", Kind: PointCommand},
		}},
		{Name: "x", Version: "1.0.0", PointTable: []PointDefinition{{Name: "a", Kind: "magic"}}},
		{Name: "x", Version: "1.0.0", CredentialFields: []CredentialField{
			{Name: "host"}, {Name: "host"},
		}},
	}
	for i, m := range broken {
		if err := m.Validate(); err == nil {
			t.Fatalf("broken manifest #%d accepted: %+v", i, m)
		}
	}
}

// TestManifestValidateConfigSchema schema 校验贯通 manifest。
func TestManifestValidateConfigSchema(t *testing.T) {
	m := &Manifest{
		Name:    "x",
		Version: "1.0.0",
		ConfigSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535},
			},
			"required":             []any{"port"},
			"additionalProperties": false,
		},
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("manifest with schema rejected: %v", err)
	}
	if err := m.ValidateConfig(map[string]any{"port": 502}); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if err := m.ValidateConfig(map[string]any{"port": 70000}); err == nil {
		t.Fatal("out-of-range config accepted")
	}
	if err := m.ValidateConfig(map[string]any{}); err == nil {
		t.Fatal("missing required field accepted")
	}

	// 未知关键字必须在编译期拒绝——静默忽略的约束等于没有约束。
	bad := &Manifest{Name: "x", Version: "1.0.0", ConfigSchema: map[string]any{
		"type": "object", "oneOf": []any{},
	}}
	if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "unsupported keyword") {
		t.Fatalf("unsupported keyword not rejected: %v", err)
	}
}

// TestParseManifest JSON 解析路径：坏 JSON 拒绝，未知 manifest 级字段保留但校验兜底。
func TestParseManifest(t *testing.T) {
	if _, err := ParseManifest([]byte("{not json")); err == nil {
		t.Fatal("invalid JSON accepted")
	}
	m, err := ParseManifest([]byte(`{"name":"bacnet-ip","version":"0.1.0"}`))
	if err != nil {
		t.Fatalf("minimal manifest rejected: %v", err)
	}
	if m.Name != "bacnet-ip" || m.Version != "0.1.0" {
		t.Fatalf("manifest fields lost: %+v", m)
	}
}

// TestSchemaIntegerRejectsFloat integer 类型必须拒绝非整数数值。
func TestSchemaIntegerRejectsFloat(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"v": map[string]any{"type": "integer"},
		},
	}
	if err := ValidateConfigValue(schema, map[string]any{"v": 2.0}); err != nil {
		t.Fatalf("integral float rejected: %v", err)
	}
	if err := ValidateConfigValue(schema, map[string]any{"v": 2.5}); err == nil {
		t.Fatal("fractional value accepted as integer")
	}
}

// 插件契约的编译期锚点：一个仅返回错误的 stub 也必须满足完整接口，
// 保证未来增加方法时是显式破坏性变更。
type stubAdapter struct{}

func (stubAdapter) Name() string                                        { return "stub" }
func (stubAdapter) ValidateConfig(context.Context, AdapterConfig) error { return nil }
func (stubAdapter) Connect(context.Context, AdapterConfig) error        { return nil }
func (stubAdapter) Discover(context.Context) ([]Device, error)          { return nil, nil }
func (stubAdapter) ReadTelemetry(context.Context, string, []string) (Telemetry, error) {
	return Telemetry{}, nil
}
func (stubAdapter) WriteCommand(context.Context, Command) error { return nil }
func (stubAdapter) Health(context.Context) (Health, error) {
	return Health{Status: HealthDown, Message: "stub"}, nil
}
func (stubAdapter) Close(context.Context) error { return errors.New("close") }

var _ ProtocolAdapter = stubAdapter{}
