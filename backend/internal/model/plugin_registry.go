package model

import "time"

// PHASE-D-D9 BEGIN 插件注册表（74.sql 对应）
//
// PluginRegistry 平台侧 gRPC 插件网关的插件登记：
// 插件以接入凭证（token，仅存 sha256 摘要）向网关注册，上报 version/transport，
// 网关维护 status（online/offline/disabled）与 last_heartbeat。
type PluginRegistry struct {
	ID            string     `gorm:"column:id;primaryKey" json:"id"`
	Name          string     `gorm:"column:name;not null;comment:插件唯一名" json:"name"`
	Version       string     `gorm:"column:version;comment:插件版本" json:"version"`
	Transport     string     `gorm:"column:transport;not null;default:grpc;comment:传输契约 grpc" json:"transport"`
	TokenHash     string     `gorm:"column:token_hash;not null;comment:接入凭证 sha256 摘要" json:"-"`
	Status        string     `gorm:"column:status;not null;default:disabled;comment:online/offline/disabled" json:"status"`
	LastHeartbeat *time.Time `gorm:"column:last_heartbeat;comment:最近心跳" json:"last_heartbeat"`
	Description   *string    `gorm:"column:description;comment:描述" json:"description"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName 表名。
func (*PluginRegistry) TableName() string { return TableNamePluginRegistry }

// TableNamePluginRegistry 表名常量（74.sql）。
const TableNamePluginRegistry = "plugin_registries"

// 插件状态常量。
const (
	PluginStatusOnline   = "online"
	PluginStatusOffline  = "offline"
	PluginStatusDisabled = "disabled"
)

// PHASE-D-D9 END
