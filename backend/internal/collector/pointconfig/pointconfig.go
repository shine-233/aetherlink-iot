// 文件用途：内置采集器点表配置的解析与校验（ROADMAP C6 收尾）——独立子包，零内部依赖
//（仅 internal/snmp、internal/opcua 协议库），供 collector 轮询器与 service 保存校验共用，
// 避免 collector→mqttadapter→uplink→service 的导入环。
// 核心逻辑：SnmpConfig/OpcuaConfig 点表 JSON 结构、字段校验（fail-closed：空目标/空点位拒绝）。
// 关键注意事项：JSON 键是前后端契约（device_configs.protocol_config，前端动态表单 dataKey
// 与此一一对应，见 internal/service/collector_form.go），任何一侧改名需两侧同批提交。
// v3 段（TB-22）：v3_user/auth_proto/priv_proto/auth_passphrase/priv_passphrase 为可选
// SNMPv3 段，v3_user 非空即走 v3 客户端路径（authNoPriv；priv 加密未实现，点表校验拒绝）。
package pointconfig

import (
	"encoding/json"
	"fmt"
	"strings"

	"aetherlink-iot/backend/internal/opcua"
	"aetherlink-iot/backend/internal/snmp"
)

// Point 采集点表行：遥测键 + 协议寻址（SNMP 用 OID，OPC UA 用 Node）。
type Point struct {
	Key  string `json:"key"`
	OID  string `json:"oid,omitempty"`
	Node string `json:"node,omitempty"`
}

// SnmpConfig SNMP protocol_config JSON 结构。
type SnmpConfig struct {
	Target    string  `json:"target"` // host:port（UDP）
	Community string  `json:"community"`
	TimeoutMs int     `json:"timeout_ms,omitempty"` // 单次 Get 超时；缺省用 Runner 预算
	Points    []Point `json:"points"`
	// SNMPv3 可选段（TB-22）：V3User 非空即走 v3 客户端路径（v2c community 该路径下忽略）。
	// 口令与 v2c community 同为 protocol_config 明文存储（仅限受信内网，明文门禁同口径）。
	V3User           string `json:"v3_user,omitempty"`
	V3AuthProto      string `json:"auth_proto,omitempty"`      // md5 / sha（RFC 3414 HMAC-96）
	V3PrivProto      string `json:"priv_proto,omitempty"`      // 仅 none（authPriv 加密未实现）
	V3AuthPassphrase string `json:"auth_passphrase,omitempty"` // ≥8 字符
	V3PrivPassphrase string `json:"priv_passphrase,omitempty"` // v3 段启用且 priv 关闭时不得填写
}

// V3Enabled 判断点表是否走 SNMPv3 路径（v3_user 非空即 v3）。
func (c *SnmpConfig) V3Enabled() bool { return c.V3User != "" }

// ParseSnmpConfig 解析并校验 SNMP 点表；空目标/community/点位均拒绝（fail-closed）。
// v2c 路径 community 必填；v3 路径（v3_user 非空）community 可缺省且被忽略。
func ParseSnmpConfig(raw string) (*SnmpConfig, error) {
	if raw == "" {
		return nil, fmt.Errorf("snmp: protocol_config 为空")
	}
	var cfg SnmpConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("snmp: protocol_config 非法 JSON: %w", err)
	}
	if cfg.Target == "" {
		return nil, fmt.Errorf("snmp: target 必填（host:port）")
	}
	if cfg.Community == "" && !cfg.V3Enabled() {
		return nil, fmt.Errorf("snmp: community 必填")
	}
	if len(cfg.Points) == 0 {
		return nil, fmt.Errorf("snmp: points 至少一条")
	}
	for i, p := range cfg.Points {
		if p.Key == "" || p.OID == "" {
			return nil, fmt.Errorf("snmp: points[%d] key/oid 必填", i)
		}
	}
	if err := cfg.validateV3(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// minV3PassphraseLen 认证口令下限（RFC 3414 §11.2 建议 ≥8 字符，取强制口径）。
const minV3PassphraseLen = 8

// validateV3 校验 SNMPv3 可选段：字段间约束互斥检查 + 协议白名单（fail-closed）。
func (c *SnmpConfig) validateV3() error {
	if !c.V3Enabled() {
		if c.V3AuthProto != "" || c.V3PrivProto != "" || c.V3AuthPassphrase != "" || c.V3PrivPassphrase != "" {
			return fmt.Errorf("snmp: 指定了 v3 认证字段但缺少 v3_user")
		}
		return nil
	}
	if c.V3AuthPassphrase == "" {
		return fmt.Errorf("snmp: v3 模式 auth_passphrase 必填")
	}
	if len(c.V3AuthPassphrase) < minV3PassphraseLen {
		return fmt.Errorf("snmp: v3 auth_passphrase 至少 %d 字符（RFC 3414）", minV3PassphraseLen)
	}
	if _, err := snmp.AuthProtocolByName(c.V3AuthProto); err != nil {
		return fmt.Errorf("snmp: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(c.V3PrivProto)) {
	case "", "none":
		if c.V3PrivPassphrase != "" {
			return fmt.Errorf("snmp: priv_proto 为 none 时不得填写 priv_passphrase")
		}
	default:
		return fmt.Errorf("snmp: priv_proto %q 暂不支持（authPriv 加密未实现，仅支持 none）", c.V3PrivProto)
	}
	return nil
}

// ValidateSnmpConfig 校验 SNMP 点表 JSON（设备配置保存链路入口）。
func ValidateSnmpConfig(raw string) error {
	_, err := ParseSnmpConfig(raw)
	return err
}

// OpcuaConfig OPC UA protocol_config JSON 结构（连接段复用 opcua.Config 校验）。
type OpcuaConfig struct {
	opcua.Config
	Points []Point `json:"points"`
}

// ParseOpcuaConfig 解析并校验 OPC UA 点表；连接段经 opcua.Validate
// （endpoint 前缀/SecurityMode 白名单）。
func ParseOpcuaConfig(raw string) (*OpcuaConfig, error) {
	if raw == "" {
		return nil, fmt.Errorf("opcua: protocol_config 为空")
	}
	var cfg OpcuaConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("opcua: protocol_config 非法 JSON: %w", err)
	}
	if len(cfg.Points) == 0 {
		return nil, fmt.Errorf("opcua: points 至少一条")
	}
	for i, p := range cfg.Points {
		if p.Key == "" || p.Node == "" {
			return nil, fmt.Errorf("opcua: points[%d] key/node 必填", i)
		}
	}
	if err := opcua.Validate(cfg.Config); err != nil {
		return nil, err
	}
	cfg.Config = opcua.Normalize(cfg.Config)
	return &cfg, nil
}

// ValidateOpcuaConfig 校验 OPC UA 点表 JSON（设备配置保存链路入口）。
func ValidateOpcuaConfig(raw string) error {
	_, err := ParseOpcuaConfig(raw)
	return err
}
