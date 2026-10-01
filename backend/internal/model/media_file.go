// 文件用途：定义媒体库（media_files，ROADMAP TB-41）的持久化模型与 HTTP 入参/出参契约。
// 核心逻辑：媒体登记实体（原始文件名 + 对外路径 + 大小 + MIME + 引用计数）与
//
//	列表查询、删除（含引用方返回）的请求响应结构。
//
// 关键注意事项：file_path 是对外访问路径（如 ./files/board/2026-09-25/xxx.png），
//
//	UNIQUE(tenant_id, file_path) 保证同租户同路径只登记一次；磁盘真实路径与
//	该字段的映射（含 OTA 前缀特例）由 service 层统一解析，禁止客户端直接传 file_path。
//
// 重构建议：若后续看板/SCADA 引用关系需要精确到部件级，新增 media_file_refs 引用明细表，
//
//	不要在本表堆 JSONB 引用列表。
package model

import "time"

const TableNameMediaFile = "media_files"

// MediaFile 对应数据库表 media_files（媒体登记行）。
// UNIQUE(tenant_id, file_path) 与 129.sql 的 uk_media_files_tenant_path 同源，登记幂等的最终保障。
type MediaFile struct {
	ID       string `gorm:"column:id;primaryKey" json:"id"`
	TenantID string `gorm:"column:tenant_id;not null;uniqueIndex:uk_media_files_tenant_path,priority:1" json:"tenant_id"`
	FileName string `gorm:"column:file_name;not null" json:"file_name"`
	FilePath string `gorm:"column:file_path;not null;uniqueIndex:uk_media_files_tenant_path,priority:2" json:"file_path"`
	FileSize int64  `gorm:"column:file_size;not null;default:0" json:"file_size"`
	Mime     string `gorm:"column:mime;not null;default:'application/octet-stream'" json:"mime"`
	// ReferencedCount 最近一次引用扫描的引用方数量（详情/删除时读时统计后回写）。
	ReferencedCount int        `gorm:"column:referenced_count;not null;default:0" json:"referenced_count"`
	CreatedAt       *time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 返回媒体登记表名。
func (*MediaFile) TableName() string { return TableNameMediaFile }

// GetMediaFileListReq 媒体列表查询入参：分页 + 文件名/MIME 模糊搜索。
type GetMediaFileListReq struct {
	PageReq
	Search *string `form:"search" json:"search" validate:"omitempty,max=100"`
	Mime   *string `form:"mime" json:"mime" validate:"omitempty,max=100"`
}

// GetMediaFileListRsp 媒体列表分页响应。
type GetMediaFileListRsp struct {
	Total int64        `json:"total"`
	List  []*MediaFile `json:"list"`
}

// MediaReferencer 引用方描述：kind 标识引用来源表面（board/scada_document/ota_package）。
type MediaReferencer struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MediaFileDetailRsp 媒体详情响应：登记行 + 实时引用方列表。
type MediaFileDetailRsp struct {
	File        *MediaFile        `json:"file"`
	Referencers []MediaReferencer `json:"referencers"`
}

// MediaFileDeleteRsp 删除结果：deleted=false 表示被引用拒绝，referencers 携带引用方列表。
type MediaFileDeleteRsp struct {
	Deleted         bool              `json:"deleted"`
	ReferencedCount int               `json:"referenced_count"`
	Referencers     []MediaReferencer `json:"referencers,omitempty"`
}
