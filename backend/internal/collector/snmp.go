// 文件用途：SNMP 轮询采集器（ROADMAP C6 SNMP 行——库级 internal/snmp 的管理侧接入）。
// 核心逻辑：解析 protocol_config 点表（target/community/points），一次 Get 批量携带全部 OID，
//
//	响应按 OID 回填遥测键（INTEGER/Counter/TimeTicks → float64，OCTET STRING → 原文）。
//
// 关键注意事项：
//  - v2c community 明文传输，仅限受信内网（与平台 MQTT 明文门禁口径一致）；
//  - 每次采集独立 UDP 事务（连接即用即弃），代理不维持 SNMP 会话状态；
//  - 响应缺失的 OID 跳过不报错（代理常见部分应答），全缺失视为空轮次不发布。
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
}

// parseSnmpConfig 解析并校验点表；空目标/community/点位均拒绝（fail-closed）。
func parseSnmpConfig(raw string) (*SnmpConfig, error) {
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

// SnmpPoller 无状态 SNMP 采集实现。
type SnmpPoller struct{}

// Protocol 返回 source_protocol 遥测标记。
func (SnmpPoller) Protocol() string { return "snmp" }

// ConfigType 返回 device_configs.protocol_type 过滤值。
func (SnmpPoller) ConfigType() string { return "SNMP" }

// Poll 单目标采集：批量 Get → OID 回填遥测键。
func (SnmpPoller) Poll(ctx context.Context, t deviceTarget) (map[string]interface{}, error) {
	cfg, err := parseSnmpConfig(t.ConfigJSON)
	if err != nil {
		return nil, err
	}
	// 采集预算：protocol_config 显式 timeout_ms 优先，否则用 ctx 剩余预算（下限 200ms 防零超时）。
	budget := snmpPollBudget(cfg, ctx)

	oids := make([]string, 0, len(cfg.Points))
	oidKey := make(map[string]string, len(cfg.Points))
	for _, p := range cfg.Points {
		oids = append(oids, p.OID)
		oidKey[p.OID] = p.Key
	}

	resp, err := snmp.Get(cfg.Target, cfg.Community, oids, budget)
	if err != nil {
		return nil, fmt.Errorf("snmp: Get %s 失败: %w", cfg.Target, err)
	}
	if resp.ErrorStatus != 0 {
		return nil, fmt.Errorf("snmp: 代理返回错误 status=%d index=%d", resp.ErrorStatus, resp.ErrorIndex)
	}

	values := make(map[string]interface{}, len(cfg.Points))
	for oid, v := range resp.Varbinds {
		key, ok := oidKey[oid]
		if !ok {
			continue // 代理未按请求回填的绑定忽略
		}
		if val, ok := snmpValue(v); ok {
			values[key] = val
		}
	}
	return values, nil
}

// snmpPollBudget 解析采集超时预算。
func snmpPollBudget(cfg *SnmpConfig, ctx context.Context) time.Duration {
	if cfg.TimeoutMs > 0 {
		return time.Duration(cfg.TimeoutMs) * time.Millisecond
	}
	if dl, ok := ctx.Deadline(); ok {
		if d := time.Until(dl); d > 0 {
			return d
		}
	}
	return 200 * time.Millisecond
}

// snmpValue 把响应值转为遥测值：数值类 → float64，OCTET STRING → 原文；NULL 丢弃。
func snmpValue(v snmp.Value) (interface{}, bool) {
	if v.Type == 0x05 { // NULL
		return nil, false
	}
	if s, ok := v.AsString(); ok {
		return s, true
	}
	if n, err := v.AsInt(); err == nil {
		return float64(n), true
	}
	return nil, false
}
