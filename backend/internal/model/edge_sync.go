// 文件用途：边缘计算 2.0（ROADMAP D6）数据模型与 HTTP 请求体。
// 核心逻辑：边缘同步任务（看板/规则链快照下发 + OTA 经边分发路由记录）的存储结构。
// 关键注意事项：
//   - 状态机：pending（已创建待投递）→ synced（broker ACK）| failed（投递失败，error 落库，可 retry）。
//   - payload 为下发给边缘网关的完整快照 JSON，重试不重拍快照（保证幂等与可审计）。
package model

import "time"

const TableNameEdgeSyncTask = "edge_sync_tasks"

// 边缘同步资源类型。
const (
	EdgeResourceDashboard  = "dashboard"
	EdgeResourceRuleChain  = "rule_chain"
	EdgeResourceOtaPackage = "ota"
)

// EdgeSyncTask 边缘同步任务。
type EdgeSyncTask struct {
	ID                  string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID            string     `gorm:"column:tenant_id;not null;index" json:"tenant_id"`
	GatewayDeviceID     string     `gorm:"column:gateway_device_id;not null;index" json:"gateway_device_id"`
	GatewayDeviceNumber string     `gorm:"column:gateway_device_number;not null" json:"gateway_device_number"`
	ResourceType        string     `gorm:"column:resource_type;not null;index" json:"resource_type"` // dashboard/rule_chain/ota
	ResourceID          string     `gorm:"column:resource_id;not null" json:"resource_id"`
	Payload             string     `gorm:"column:payload;type:text;not null" json:"payload"` // 下发快照 JSON
	Status              string     `gorm:"column:status;not null;default:pending;index" json:"status"`
	Error               *string    `gorm:"column:error" json:"error"`
	Attempts            int        `gorm:"column:attempts;not null;default:0" json:"attempts"`
	SyncedAt            *time.Time `gorm:"column:synced_at" json:"synced_at"`
	CreatedAt           time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*EdgeSyncTask) TableName() string { return TableNameEdgeSyncTask }

// ---- HTTP 请求结构体 ----

// CreateEdgeSyncReq 创建边缘同步任务（看板/规则链快照下发）。
type CreateEdgeSyncReq struct {
	GatewayDeviceID string `json:"gateway_device_id" validate:"required,max=36"`
	ResourceType    string `json:"resource_type" validate:"required,oneof=dashboard rule_chain"`
	ResourceID      string `json:"resource_id" validate:"required,max=36"`
}

// RetryEdgeSyncReq 重试投递（路径参数 :id）。
type RetryEdgeSyncReq struct {
	ID string `json:"id" validate:"required"`
}

// EdgeOtaDistributeReq OTA 经边分发：指定边缘网关 + 目标设备集合 + 升级包。
type EdgeOtaDistributeReq struct {
	GatewayDeviceID string   `json:"gateway_device_id" validate:"required,max=36"`
	PackageID       string   `json:"package_id" validate:"required,max=36"`
	DeviceIDs       []string `json:"device_ids" validate:"required,min=1,dive,max=36"`
}
