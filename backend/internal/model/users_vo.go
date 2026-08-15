// 文件用途：定义 users vo 相关视图对象结构，隔离数据库模型与前端展示/聚合响应字段。
// 核心逻辑：按页面或接口需要组合基础模型字段，减少上层直接暴露持久化结构的耦合。
// 关键注意事项：VO 字段应保持只表达响应形状，避免混入查询、副作用或权限判断。
// 重构建议：当多个接口复用相同展示字段时，可抽出共享 VO 或转换 helper，避免重复维护字段映射。

package model

type UsersRes struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	PhoneNum       string `json:"phone_num"`
	Email          string `json:"email"`
	Authority      string `json:"authority"`
	TenantID       string `json:"tenant_id"`
	Remark         string `json:"remark"`
	CreateTime     string `json:"create_time"`
	AdditionalInfo string `json:"additional_info"`
	AvatarURL      string `json:"avatar_url"`
}

type UsersUpdateReq struct {
	Name            string                `json:"name"`
	Email           *string               `json:"email" validate:"omitempty,email"`
	AdditionalInfo  *string               `json:"additional_info"`
	PhoneNumber     *string               `json:"phone_number"`
	PhonePrefix     *string               `json:"phone_prefix"`
	Organization    *string               `json:"organization" validate:"omitempty,max=200"`
	Timezone        *string               `json:"timezone" validate:"omitempty,max=50"`
	DefaultLanguage *string               `json:"default_language" validate:"omitempty,max=10"`
	Address         *UpdateUserAddressReq `json:"address" validate:"omitempty"`
	AvatarURL       *string               `json:"avatar_url" validate:"omitempty,max=500"`
}

type UsersUpdatePasswordReq struct {
	OldPassword string `json:"old_password" gorm:"old_password" validate:"required"`
	Password    string `json:"password"  gorm:"password" validate:"required"`
	Salt        string `json:"salt" gorm:"salt"`
}
