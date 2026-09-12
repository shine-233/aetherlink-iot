// 文件用途：规则链版本 HTTP 请求结构（ROADMAP P1.2）。
// 关键注意事项：chain_id 优先取路径参数；发布/回滚为不带路径参数的动作端点，
// 故 chain_id 放在请求体里并强制必填——缺省会导致跨链操作。
package model

// CreateRuleChainVersionReq 新建草稿版本请求。chain_id 来自路径 :id。
type CreateRuleChainVersionReq struct {
	GraphHash string `json:"graph_hash" form:"graph_hash" binding:"required"`
}

// RuleChainVersionActionReq 发布/回滚动作请求。
type RuleChainVersionActionReq struct {
	ChainID string `json:"chain_id" form:"chain_id" binding:"required"`
	Version int    `json:"version" form:"version" binding:"required"`
}
