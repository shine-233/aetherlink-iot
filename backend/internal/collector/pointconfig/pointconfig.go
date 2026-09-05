// 文件用途：内置采集器点表配置的解析与校验（ROADMAP C6 收尾）——独立子包，零内部依赖
//（仅 internal/snmp、internal/opcua 协议库），供 collector 轮询器与 service 保存校验共用，
// 避免 collector→mqttadapter→uplink→service 的导入环。
// 核心逻辑：SnmpConfig/OpcuaConfig 点表 JSON 结构、字段校验（fail-closed：空目标/空点位拒绝）。
// 关键注意事项：JSON 键是前后端契约（device_configs.protocol_config，前端动态表单 dataKey
// 与此一一对应，见 internal/service/collector_form.go），任何一侧改名需两侧同批提交。
package pointconfig

import (
	"encoding/json"
	"fmt"
	"strings"

	"aetherlink-iot/backend/internal/opcua"
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
}

// ParseSnmpConfig 解析并校验 SNMP 点表；空目标/community/点位均拒绝（fail-closed）。
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
	if cfg.Community == "" {
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
	return &cfg, nil
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

// NormalizeProtocolType 归一化协议类型（大小写不敏感， trim 空白）；非内置协议返回原值。
// 内置协议统一为大写（与 device_configs.protocol_type 口径一致）。
func NormalizeProtocolType(protocolType string) string {
	normalized := strings.ToUpper(strings.TrimSpace(protocolType))
	switch normalized {
	case "SNMP", "OPCUA":
		return normalized
	default:
		return protocolType
	}
}
