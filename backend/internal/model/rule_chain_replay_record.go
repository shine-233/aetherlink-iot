package model

import "time"

// RuleChainReplayRecordRow 持久化的规则链回放输入快照（P1.2）。
// 放在非 .gen.go 文件中：按项目约定不手改生成产物，新表由 DAL 以
// 字符串列名读写（同 P0.2 device_shadow、P0.4 定时器与执行窗口的做法）。
type RuleChainReplayRecordRow struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	ChainID     string    `json:"chain_id"`
	ExecutionID string    `json:"execution_id"`
	NodeID      string    `json:"node_id"`
	NodeType    string    `json:"node_type"`
	Payload     string    `json:"payload"`
	Metadata    string    `json:"metadata"`
	Pass        bool      `json:"pass"`
	Error       string    `json:"error"`
	RecordedAt  time.Time `json:"recorded_at"`
}
