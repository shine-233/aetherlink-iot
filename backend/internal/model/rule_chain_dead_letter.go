package model

import "time"

const TableNameRuleChainDeadLetter = "rule_chain_dead_letters"

// RuleChainDeadLetter 规则链节点终局失败死信记录（P1.2 护城河）。
// 遵循审计最小化约定，不落原始业务载荷，仅存租户、链、节点、设备、尝试次数与错误摘要。
type RuleChainDeadLetter struct {
	ID        string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID  string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ChainID   string    `gorm:"column:chain_id;not null" json:"chain_id"`
	ExecID    string    `gorm:"column:exec_id;not null" json:"exec_id"`
	NodeID    string    `gorm:"column:node_id;not null" json:"node_id"`
	NodeType  string    `gorm:"column:node_type;not null" json:"node_type"`
	DeviceID  *string   `gorm:"column:device_id" json:"device_id"`
	Error     *string   `gorm:"column:error" json:"error"`
	Attempts  int       `gorm:"column:attempts;not null;default:1" json:"attempts"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*RuleChainDeadLetter) TableName() string {
	return TableNameRuleChainDeadLetter
}
