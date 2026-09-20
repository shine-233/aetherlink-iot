// 文件用途：P1.x native-board-provider 项目增删改的数据模型（看板项目分组）。
// 核心逻辑：board_projects 是租户内的看板分组实体；board_project_members 记录
// 看板归属（一板一项目，成员表唯一约束收口）。
// 关键注意事项：
//   - **刻意不动 boards 表**：boards 是 gorm gen 生成模型，加列需重生成产物；
//     分组语义放独立关联表，迁移与应用层都可独立演进。
//   - 一块看板同时只属于一个项目（member 表 board_id 唯一索引）——
//     多对多会让 listDashboards 的分页与"从项目移除"语义都变模糊。
//   - NATIVE_BOARD_PROJECT_ID（内置项目）不落库：project_id 为空的看板即属于它，
//     与前端既有"一个内置项目"的语义无缝兼容。
package model

import "time"

const (
	TableNameBoardProject = "board_projects"
	// TableNameBoardProjectMember 看板-项目关联表。
	TableNameBoardProjectMember = "board_project_members"
)

// BoardProject 看板项目分组。
type BoardProject struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID    string    `gorm:"column:tenant_id;not null;index" json:"tenant_id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description *string   `gorm:"column:description" json:"description"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*BoardProject) TableName() string { return TableNameBoardProject }

// BoardProjectMember 看板归属记录。
type BoardProjectMember struct {
	ProjectID string    `gorm:"column:project_id;primaryKey" json:"project_id"`
	BoardID   string    `gorm:"column:board_id;primaryKey" json:"board_id"`
	TenantID  string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*BoardProjectMember) TableName() string { return TableNameBoardProjectMember }

// ---- HTTP 请求结构体 ----

// CreateBoardProjectReq 创建看板项目。
type CreateBoardProjectReq struct {
	Name        string  `json:"name" validate:"required,max=255"`
	Description *string `json:"description" validate:"omitempty,max=500"`
}

// UpdateBoardProjectReq 更新看板项目（PUT 语义：整体替换可变字段）。
type UpdateBoardProjectReq struct {
	Name        string  `json:"name" validate:"required,max=255"`
	Description *string `json:"description" validate:"omitempty,max=500"`
}

// AddBoardProjectMemberReq 把看板加入项目（路径参数携带 IDs；请求体留空）。
type AddBoardProjectMemberReq struct{}

// BoardProjectListReq 列表查询（支持按看板 ID 反查其所属项目）。
type BoardProjectListReq struct {
	BoardID string `json:"board_id" form:"board_id" validate:"omitempty,max=36"`
}
