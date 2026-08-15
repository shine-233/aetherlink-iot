// internal/model/open_api_keys.http.go
// 文件用途：定义 open api keys 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

// OpenAPIKeyListReq 查询API密钥列表请求
type OpenAPIKeyListReq struct {
	PageReq        // 继承基础分页请求
	Status  *int16 `json:"status" form:"status" validate:"omitempty,oneof=0 1"` // 状态: 0-禁用 1-启用
}

// CreateOpenAPIKeyReq 创建API密钥请求
type CreateOpenAPIKeyReq struct {
	TenantID string `json:"tenant_id" validate:"required,max=36"` // 租户ID
	Name     string `json:"name" validate:"omitempty,max=200"`    // 名称
}

// UpdateOpenAPIKeyReq 更新API密钥请求
type UpdateOpenAPIKeyReq struct {
	ID     string  `json:"id" validate:"required,max=36"`         // 主键ID
	Status *int16  `json:"status" validate:"omitempty,oneof=0 1"` // 状态: 0-禁用 1-启用
	Name   *string `json:"name" validate:"omitempty,max=200"`     // 名称
}

// OpenAPIKeyListRsp API密钥列表响应
type OpenAPIKeyListRsp struct {
	OpenAPIKey         // 嵌入OpenAPIKey结构体
	UserID     *string `json:"user_id"`   // 创建者用户ID
	Email      *string `json:"email"`     // 创建者邮箱
	UserName   *string `json:"user_name"` // 创建者用户名
}
