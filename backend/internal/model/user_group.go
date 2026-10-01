// 文件用途：定义用户组与组权限（TB-46 GPE v1，131.sql）的持久化模型与 HTTP 入参/出参契约。
// 核心逻辑：user_groups 用户组主表 + r_group_user 成员关联（复合主键）+ group_permissions
//
//	组-权限元素绑定，以及组 CRUD / 成员管理 / 权限绑定 / 共享可见性的请求响应结构。
//
// 关键注意事项：element_code 是带命名空间的权限元素码（v1: board:<id> / asset:<id>），
//
//	多态引用无外键，合法性（资源存在且同租户）由 service 层绑定前校验；
//	三张表与 131.sql 列定义同源，生成器重跑前请保持一致（沿 operation_logs.gen.go 先例）。
//
// 重构建议：若后续 element_code 扩展功能权限点（sys_permissions.code）或按钮元素，
//
//	把命名空间解析收敛到 dal/element code helper 一处，不要在 service 里散落字符串前缀判断。
package model

import (
	"strings"
	"time"
)

const (
	TableNameUserGroup       = "user_groups"
	TableNameUserGroupMember = "r_group_user"
	TableNameGroupPermission = "group_permissions"
)

// GroupElementKindBoard 组权限元素码命名空间：看板资源元素。
const GroupElementKindBoard = "board"

// GroupElementKindAsset 组权限元素码命名空间：资产资源元素。
const GroupElementKindAsset = "asset"

// ParseGroupElementCode 解析组权限元素码（"kind:id" 形态）。
// v1 仅支持 board:/asset: 两个资源元素命名空间；形态不符或命名空间未知返回 ok=false，
// 调用方必须 fail-closed 拒绝（不猜默认值）。
func ParseGroupElementCode(code string) (kind, id string, ok bool) {
	code = strings.TrimSpace(code)
	kindPart, idPart, found := strings.Cut(code, ":")
	if !found || strings.TrimSpace(kindPart) == "" || strings.TrimSpace(idPart) == "" {
		return "", "", false
	}
	switch kindPart {
	case GroupElementKindBoard, GroupElementKindAsset:
		return kindPart, strings.TrimSpace(idPart), true
	}
	return "", "", false
}

// UserGroup 对应数据库表 user_groups（用户组主表）。
// UNIQUE(tenant_id, name) 与 131.sql 的 uk_user_groups_tenant_name 同源。
type UserGroup struct {
	ID          string     `gorm:"column:id;primaryKey" json:"id"`
	Name        string     `gorm:"column:name;not null" json:"name"`
	TenantID    string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Description *string    `gorm:"column:description" json:"description"`
	CreatedAt   *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 返回用户组主表名。
func (*UserGroup) TableName() string { return TableNameUserGroup }

// UserGroupMember 对应数据库表 r_group_user（组成员关联，复合主键 group_id+user_id）。
// user_id 仅限 users 登录账号（FK CASCADE）；customer 客户不接入组授权（ROADMAP v1 边界）。
type UserGroupMember struct {
	GroupID   string     `gorm:"column:group_id;primaryKey" json:"group_id"`
	UserID    string     `gorm:"column:user_id;primaryKey" json:"user_id"`
	TenantID  string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	CreatedAt *time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 返回组成员关联表名。
func (*UserGroupMember) TableName() string { return TableNameUserGroupMember }

// GroupPermission 对应数据库表 group_permissions（组-权限元素绑定）。
// UNIQUE(group_id, element_code) 与 131.sql 的 uk_group_permissions_group_element 同源。
type GroupPermission struct {
	ID          string     `gorm:"column:id;primaryKey" json:"id"`
	GroupID     string     `gorm:"column:group_id;not null" json:"group_id"`
	ElementCode string     `gorm:"column:element_code;not null" json:"element_code"`
	TenantID    string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	CreatedAt   *time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 返回组权限绑定表名。
func (*GroupPermission) TableName() string { return TableNameGroupPermission }

// CreateUserGroupReq 创建用户组入参。
// tenant_id 仅 SYS_ADMIN 创建跨租户组时生效；TENANT_ADMIN 传非本租户值会被服务层拒绝。
type CreateUserGroupReq struct {
	Name        string  `json:"name" form:"name" validate:"required,max=100"`
	Description *string `json:"description" form:"description" validate:"omitempty,max=500"`
	TenantID    string  `json:"tenant_id" form:"tenant_id" validate:"omitempty,max=36"`
}

// UpdateUserGroupReq 更新用户组入参（名称/描述至少一项）。
type UpdateUserGroupReq struct {
	ID          string  `json:"id" form:"id" validate:"required,max=36"`
	Name        string  `json:"name" form:"name" validate:"omitempty,max=100"`
	Description *string `json:"description" form:"description" validate:"omitempty,max=500"`
}

// GetUserGroupListReq 用户组分页列表入参（名称模糊过滤）。
type GetUserGroupListReq struct {
	PageReq
	Name *string `json:"name" form:"name" validate:"omitempty,max=100"`
}

// AssignUserGroupMembersReq 组成员批量绑定入参（全量替换语义，传空数组=清空成员）。
type AssignUserGroupMembersReq struct {
	UserIDs []string `json:"user_ids" validate:"omitempty,dive,max=36"`
}

// AssignUserGroupPermissionsReq 组权限元素批量绑定入参（全量替换语义，传空数组=清空绑定）。
type AssignUserGroupPermissionsReq struct {
	ElementCodes []string `json:"element_codes" validate:"omitempty,dive,max=100"`
}

// UserGroupMemberSummary 组成员用户摘要（users 表内账号；customer 客户永不出现）。
type UserGroupMemberSummary struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	Name      *string `json:"name"`
	Authority *string `json:"authority"`
}

// GetUserGroupMembersResp 组成员列表响应。
type GetUserGroupMembersResp struct {
	GroupID   string                   `json:"group_id"`
	GroupName string                   `json:"group_name"`
	Users     []UserGroupMemberSummary `json:"users"`
}

// GroupElementInfo 权限元素码解析结果（kind + 资源标识 + 可读名称）。
type GroupElementInfo struct {
	Code string `json:"code"`
	Kind string `json:"kind"` // board / asset
	ID   string `json:"id"`   // 资源 ID（命名空间后缀）
	Name string `json:"name"` // 资源可读名称（绑定快照，查询时刷新）
}

// GetUserGroupPermissionsResp 组权限元素列表响应。
type GetUserGroupPermissionsResp struct {
	GroupID   string             `json:"group_id"`
	GroupName string             `json:"group_name"`
	Elements  []GroupElementInfo `json:"elements"`
}
