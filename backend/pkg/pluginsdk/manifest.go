// Manifest 与其校验（ROADMAP P2.1 交付物：插件 manifest、配置 Schema、点表、凭证映射）。
//
// 关键注意事项：
//   - 凭证映射**只声明字段，不承载值**：secret 字段的值走平台凭证通道，
//     manifest 是可分发文档，把凭证写进去等于把密钥抄进说明书。
//   - 点表 name 必须唯一：重名点表会让上行数据归属取决于处理顺序。
package pluginsdk

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PointKind 点表条目的种类。
type PointKind string

const (
	PointTelemetry PointKind = "telemetry"
	PointAttribute PointKind = "attribute"
	PointCommand   PointKind = "command"
)

// PointDefinition 点表：插件上报/可写的数据点声明。
type PointDefinition struct {
	Name     string    `json:"name"`
	Kind     PointKind `json:"kind"`
	Unit     string    `json:"unit,omitempty"`
	Writable bool      `json:"writable,omitempty"`
}

// CredentialField 凭证映射声明：只声明接入需要的字段及其敏感性。
type CredentialField struct {
	Name     string `json:"name"`
	Required bool   `json:"required,omitempty"`
	Secret   bool   `json:"secret,omitempty"`
}

// Manifest 插件自描述清单。注册时由平台校验并持久化。
type Manifest struct {
	Name             string            `json:"name"`
	Version          string            `json:"version"`
	Title            string            `json:"title,omitempty"`
	Description      string            `json:"description,omitempty"`
	Transport        string            `json:"transport,omitempty"`
	MinHostVersion   string            `json:"min_host_version,omitempty"`
	ConfigSchema     map[string]any    `json:"config_schema,omitempty"`
	PointTable       []PointDefinition `json:"point_table,omitempty"`
	CredentialFields []CredentialField `json:"credential_fields,omitempty"`

	// 以下两项由 SignManifest（signing.go）填充，均 omitempty：老 manifest 解析不受影响。
	// 注册侧语义：未签名允许（D9 兼容过渡），但带了签名就必须验过——坏的签名比没有更危险。
	Signature string `json:"signature,omitempty"` // Ed25519(规范 JSON，不含本字段与 SignedBy)，base64
	SignedBy  string `json:"signed_by,omitempty"` // 厂商密钥 ID
}

// ParseManifest 解析并校验 manifest JSON。任何失败都拒绝注册。
func ParseManifest(raw []byte) (*Manifest, error) {
	var manifest Manifest
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	if err := dec.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("pluginsdk: manifest is not valid JSON: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// Validate manifest 自身一致性。
func (m *Manifest) Validate() error {
	if m == nil {
		return fmt.Errorf("pluginsdk: manifest is nil")
	}
	name := strings.TrimSpace(m.Name)
	if name == "" || len(name) > 128 {
		return fmt.Errorf("pluginsdk: manifest name is required (<=128 chars)")
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return fmt.Errorf("pluginsdk: manifest name %q contains unsupported characters", name)
		}
	}
	if !isDottedNumeric(m.Version) {
		return fmt.Errorf("pluginsdk: manifest version %q must be dotted numeric (e.g. 1.2.0)", m.Version)
	}
	if m.Transport != "" && m.Transport != "grpc" {
		return fmt.Errorf("pluginsdk: manifest transport %q is unsupported (only grpc)", m.Transport)
	}
	if m.MinHostVersion != "" && !isDottedNumeric(m.MinHostVersion) {
		return fmt.Errorf("pluginsdk: manifest min_host_version %q must be dotted numeric", m.MinHostVersion)
	}
	if err := CheckHostCompatibility(m.MinHostVersion); err != nil {
		return err
	}
	if m.ConfigSchema != nil {
		if err := CheckConfigSchema(m.ConfigSchema); err != nil {
			return fmt.Errorf("pluginsdk: manifest config_schema is invalid: %w", err)
		}
	}
	seen := make(map[string]bool, len(m.PointTable))
	for i, point := range m.PointTable {
		if strings.TrimSpace(point.Name) == "" {
			return fmt.Errorf("pluginsdk: point_table[%d] has empty name", i)
		}
		switch point.Kind {
		case PointTelemetry, PointAttribute, PointCommand:
		default:
			return fmt.Errorf("pluginsdk: point_table[%d] (%s) has unsupported kind %q", i, point.Name, point.Kind)
		}
		if seen[point.Name] {
			return fmt.Errorf("pluginsdk: duplicate point name %q in point_table", point.Name)
		}
		seen[point.Name] = true
	}
	credSeen := make(map[string]bool, len(m.CredentialFields))
	for i, field := range m.CredentialFields {
		if strings.TrimSpace(field.Name) == "" {
			return fmt.Errorf("pluginsdk: credential_fields[%d] has empty name", i)
		}
		if credSeen[field.Name] {
			return fmt.Errorf("pluginsdk: duplicate credential field %q", field.Name)
		}
		credSeen[field.Name] = true
	}
	return nil
}

// ValidateConfig 用 manifest.config_schema 校验一份设备接入配置。
// 未声明 config_schema 的插件跳过 schema 校验（由插件自身 ValidateConfig 承担）。
func (m *Manifest) ValidateConfig(config map[string]any) error {
	if m == nil || m.ConfigSchema == nil {
		return nil
	}
	return ValidateConfigValue(m.ConfigSchema, config)
}

// isDottedNumeric 版本必须是点分数字。
func isDottedNumeric(version string) bool {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return false
	}
	_, ok := parseVersionSegments(trimmed)
	return ok
}
