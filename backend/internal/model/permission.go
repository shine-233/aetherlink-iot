package model

import (
	"encoding/json"
	"time"
)

const (
	TableNameSysPermission     = "sys_permissions"
	TableNameSysRolePermission = "sys_role_permissions"
)

// SysPermission 权限点字典模型
type SysPermission struct {
	Code        string          `gorm:"column:code;primaryKey" json:"code"`
	Name        string          `gorm:"column:name;not null" json:"name"`
	Module      string          `gorm:"column:module;not null" json:"module"`
	Description *string         `gorm:"column:description" json:"description"`
	ApiPatterns json.RawMessage `gorm:"column:api_patterns;not null" json:"api_patterns"`
	CreatedAt   time.Time       `gorm:"column:created_at;not null" json:"created_at"`
}

func (*SysPermission) TableName() string {
	return TableNameSysPermission
}

// SysRolePermission 角色与权限点关联表
type SysRolePermission struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	RoleID         string    `gorm:"column:role_id;not null" json:"role_id"`
	PermissionCode string    `gorm:"column:permission_code;not null" json:"permission_code"`
	TenantID       string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	CreatedAt      time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*SysRolePermission) TableName() string {
	return TableNameSysRolePermission
}

// AssignRolePermissionsReq 分配权限入参
type AssignRolePermissionsReq struct {
	PermissionCodes []string `json:"permission_codes"`
}

// AssignRoleUsersReq 角色分配用户入参
type AssignRoleUsersReq struct {
	UserIDs []string `json:"user_ids"`
}

// RolePermissionsResp 角色权限列表出参
type RolePermissionsResp struct {
	RoleID      string          `json:"role_id"`
	RoleName    string          `json:"role_name"`
	TenantID    string          `json:"tenant_id"`
	Codes       []string        `json:"codes"`
	Permissions []SysPermission `json:"permissions"`
}

// RoleUsersResp 角色关联用户出参
type RoleUsersResp struct {
	RoleID   string             `json:"role_id"`
	RoleName string             `json:"role_name"`
	Users    []RoleUserSummary `json:"users"`
}

// RoleUserSummary 用户摘要
type RoleUserSummary struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	Name        *string `json:"name"`
	PhoneNumber string  `json:"phone_number"`
	Authority   *string `json:"authority"`
}
