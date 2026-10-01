// 文件用途：设备生效规则链解析（TB-18，125.sql）的 HTTP 出参结构。
// 核心逻辑：描述单设备"档案绑定链优先、租户级启用链兜底"的生效链清单与绑定元数据。
// 关键注意事项：default_rule_chain_id 为档案上的原始绑定值（即使链已停用也原样回显）；
//
//	chains 是执行面真正会命中的集合（停用/不存在/跨租户的绑定不会出现，source 区分来源）。
//
// 重构建议：若后续增加按档案维度的默认队列等字段，扩展本结构而非新增端点。
package model

// EffectiveRuleChainRef 生效规则链引用。source：profile=档案绑定链（优先）；tenant=租户级启用链（兜底）。
type EffectiveRuleChainRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
}

// DeviceEffectiveRuleChainsRes GET /api/v1/rule-chains/device-effective/:deviceId 的响应体。
type DeviceEffectiveRuleChainsRes struct {
	DeviceID           string                  `json:"device_id"`
	DeviceConfigID     *string                 `json:"device_config_id"`      // 设备绑定的档案 id（未绑档案为 null）
	DefaultRuleChainID *string                 `json:"default_rule_chain_id"` // 档案上的原始绑定值（未绑定/未绑档案为 null）
	Chains             []EffectiveRuleChainRef `json:"chains"`                // 实际生效集合：档案链在前、租户链在后，按 ChainID 去重
}
