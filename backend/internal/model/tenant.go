// 文件用途：租户客户层级（ROADMAP C2）的请求/响应结构。
// 核心逻辑：CreateTenantReq 允许指定父租户挂入层级；树节点为父子嵌套结构，
//           详情响应附带父租户名称与直接子租户数，供管理端组织架构展示。
package model

// CreateTenantReq 创建子租户请求。parent_tenant_id 为空时创建根租户（仅 SYS_ADMIN）。
type CreateTenantReq struct {
	Name           string  `json:"name" binding:"required"`
	Code           *string `json:"code"`
	ParentTenantID *string `json:"parent_tenant_id"`
	Remark         *string `json:"remark"`
}

// UpdateTenantReq 更新租户请求（仅更新传入字段）。
type UpdateTenantReq struct {
	Name   *string `json:"name"`
	Code   *string `json:"code"`
	Status *string `json:"status"` // N-正常 F-冻结
	Remark *string `json:"remark"`
}

// TenantTreeNode 租户树节点（以当前管理员可管辖子树为根）。
type TenantTreeNode struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Code       string            `json:"code"`
	Status     string            `json:"status"`
	ChildCount int               `json:"child_count"`
	Children   []*TenantTreeNode `json:"children"`
}

// TenantDetailResp 租户详情响应。
type TenantDetailResp struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Code           string  `json:"code"`
	Status         string  `json:"status"`
	Remark         string  `json:"remark"`
	ParentTenantID *string `json:"parent_tenant_id"`
	ParentName     string  `json:"parent_name"`
	ChildCount     int64   `json:"child_count"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}