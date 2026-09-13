// 文件用途：P1.5 边缘节点注册表的数据模型与 HTTP 请求体。
// 核心逻辑：一张 edge_nodes 表承载"谁注册过、什么版本、会什么能力、最后一次心跳"。
// 关键注意事项：
//   - ID 是**节点自报身份**（边缘代理生成的稳定标识），不是平台签发的 uuid——
//     注册的语义是"把一个已存在的边缘身份登记进来"，重新注册不得换 ID。
//   - capabilities 以 JSON 数组存文本列：能力集合很小且只在应用层比对，
//     规范化（去空白/去重/排序）在 service 层完成，数据库不承担语义。
//   - 跨租户抢注是安全事件：判定在 service 层（EvaluateEdgeNodeRegistration），
//     DAL 必须提供**不带租户过滤**的按 ID 读取，才能发现"这个 ID 已属于别人"。
package model

import "time"

const TableNameEdgeNode = "edge_nodes"

// EdgeNode 边缘节点注册记录。
type EdgeNode struct {
	ID           string     `gorm:"column:id;primaryKey" json:"id"` // 节点自报身份（非平台 uuid）
	TenantID     string     `gorm:"column:tenant_id;not null;index" json:"tenant_id"`
	Version      string     `gorm:"column:version;not null" json:"version"`
	Capabilities string     `gorm:"column:capabilities;type:text;not null;default:[]" json:"-"` // JSON 字符串数组
	Status       string     `gorm:"column:status;not null;default:active" json:"status"`
	LastSeenAt   *time.Time `gorm:"column:last_seen_at" json:"last_seen_at"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*EdgeNode) TableName() string { return TableNameEdgeNode }

// 边缘节点状态。当前只有 active；revoked 预留给"吊销后拒绝心跳"，迁移先行、逻辑后补。
const (
	EdgeNodeStatusActive  = "active"
	EdgeNodeStatusRevoked = "revoked"
)

// ---- HTTP 请求结构体 ----

// RegisterEdgeNodeReq 节点注册/重注册（幂等）。
type RegisterEdgeNodeReq struct {
	NodeID       string   `json:"node_id" validate:"required,max=64"`
	Version      string   `json:"version" validate:"required,max=32"`
	Capabilities []string `json:"capabilities" validate:"omitempty,max=32,dive,max=64"`
}

// EdgeNodeHeartbeatReq 心跳：可选携带版本/能力变化（走同一条注册判定路径）。
type EdgeNodeHeartbeatReq struct {
	Version      string   `json:"version" validate:"omitempty,max=32"`
	Capabilities []string `json:"capabilities" validate:"omitempty,max=32,dive,max=64"`
}

// EdgeNodeRegistrationRsp 注册/心跳响应：节点行 + 判定结果 + 健康分类。
type EdgeNodeRegistrationRsp struct {
	Node    *EdgeNode `json:"node"`
	Outcome string    `json:"outcome"` // updated / unchanged（rejected 以错误返回）
	Reason  string    `json:"reason,omitempty"`
	Health  string    `json:"health"` // online / degraded / offline / unknown
}

// EdgeNodeListEntry 列表条目：capabilities 展开为数组，附健康分类。
type EdgeNodeListEntry struct {
	ID           string     `json:"id"`
	TenantID     string     `json:"tenant_id"`
	Version      string     `json:"version"`
	Capabilities []string   `json:"capabilities"`
	Status       string     `json:"status"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	Health       string     `json:"health"`
}

// EdgeNodeReconcileResource 节点上报的一个资源及其本地修订号（nil=从未拿到）。
type EdgeNodeReconcileResource struct {
	ResourceType string `json:"resource_type" validate:"required,oneof=dashboard rule_chain"`
	ResourceID   string `json:"resource_id" validate:"required,max=36"`
	Revision     *int64 `json:"revision"`
}

// EdgeNodeReconcileReq 重连后按版本同步的编排请求：
// 节点带着自己手里的资源修订号来问云端"我要补哪些"。
type EdgeNodeReconcileReq struct {
	GatewayDeviceID string                      `json:"gateway_device_id" validate:"required,max=36"`
	Resources       []EdgeNodeReconcileResource `json:"resources" validate:"omitempty,max=200,dive"`
}

// EdgeNodeReconcileRsp 编排结果：完整计划 + 实际下发的任务 + 是否存在人工阻断。
type EdgeNodeReconcileRsp struct {
	Health   string                    `json:"health"`
	Version  string                    `json:"version"` // 版本兼容原因（兼容时为空）
	Items    []EdgeReconcilePlanItem   `json:"items"`   // 逐资源计划
	Dispatch []*EdgeSyncTaskDispatched `json:"dispatch,omitempty"`
	Blocked  bool                      `json:"blocked"` // 存在 needs_attention 时为 true，且不自动下发
	Synced   int                       `json:"synced"`  // 实际发起下发的资源数
}

// EdgeReconcilePlanItem 逐资源计划项（service.EdgeReconcileItem 的传输投影——
// model 不能反向依赖 service，故在此另立结构，service 层负责转换）。
type EdgeReconcilePlanItem struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Action       string `json:"action"` // sync / skip / needs_attention
	Reason       string `json:"reason"`
}

// EdgeSyncTaskDispatched 一次实际下发的结果（复用 CreateEdgeSync 的冲突闸门）。
type EdgeSyncTaskDispatched struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	TaskID       string `json:"task_id,omitempty"`
	Error        string `json:"error,omitempty"`
}
