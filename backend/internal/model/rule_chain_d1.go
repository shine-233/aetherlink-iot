package model

import "time"

// PHASE-D-D1 BEGIN 规则链 2.0 落库模型（67.sql 对应）
//
// RuleChainNodeTrace 节点级调试 trace：由引擎在热路径异步写入（见
// service/rule_chain_trace.go 的旁路开关），单条执行不阻塞消息管道。
type RuleChainNodeTrace struct {
	ID        string    `gorm:"column:id;primaryKey" json:"id"`
	ExecID    string    `gorm:"column:exec_id;not null;comment:链执行批次id" json:"exec_id"`
	ChainID   string    `gorm:"column:chain_id;not null;comment:规则链id" json:"chain_id"`
	NodeID    string    `gorm:"column:node_id;not null;comment:节点id" json:"node_id"`
	NodeType  string    `gorm:"column:node_type;not null;comment:节点类型" json:"node_type"`
	Pass      bool      `gorm:"column:pass;not null;comment:分支是否通过" json:"pass"`
	ErrorMsg  *string   `gorm:"column:error_msg;comment:错误摘要(受控,截断)" json:"error_msg"`
	ElapsedMs int64     `gorm:"column:elapsed_ms;not null;comment:耗时毫秒" json:"elapsed_ms"`
	TenantID  string    `gorm:"column:tenant_id;not null;comment:租户id" json:"tenant_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

// TableName 表名。
func (*RuleChainNodeTrace) TableName() string { return TableNameRuleChainNodeTrace }

// TableNameRuleChainNodeTrace 表名常量（67.sql）。
const TableNameRuleChainNodeTrace = "rule_chain_node_traces"

// RuleChainCheckpoint flow.checkpoint 落库记录：消息在节点边界的持久化锚点，
// 用于断点续跑与下游幂等去重。
type RuleChainCheckpoint struct {
	ID        string    `gorm:"column:id;primaryKey" json:"id"`
	ExecID    string    `gorm:"column:exec_id;not null;comment:链执行批次id" json:"exec_id"`
	ChainID   string    `gorm:"column:chain_id;not null;comment:规则链id" json:"chain_id"`
	NodeID    string    `gorm:"column:node_id;not null;comment:检查点节点id" json:"node_id"`
	TenantID  string    `gorm:"column:tenant_id;not null;comment:租户id" json:"tenant_id"`
	DeviceID  string    `gorm:"column:device_id;comment:originator设备id" json:"device_id"`
	Payload   []byte    `gorm:"column:payload;type:jsonb;not null;comment:检查点载荷" json:"payload"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

// TableName 表名。
func (*RuleChainCheckpoint) TableName() string { return TableNameRuleChainCheckpoint }

// TableNameRuleChainCheckpoint 表名常量（67.sql）。
const TableNameRuleChainCheckpoint = "rule_chain_checkpoints"

// PHASE-D-D1 END
