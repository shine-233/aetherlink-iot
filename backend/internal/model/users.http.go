// 文件用途：定义 users 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

import (
	"encoding/json"
	"time"
)

type CreateUserReq struct {
	AdditionalInfo  *json.RawMessage      `json:"additional_info" validate:"omitempty,max=10000"` // 附加信息
	Email           string                `json:"email"  validate:"required,email"`               // 邮箱
	Password        string                `json:"password" validate:"required,min=8,max=20"`      // 密码
	Name            *string               `json:"name" validate:"omitempty,min=2,max=50"`         // 姓名
	PhoneNumber     string                `json:"phone_number" validate:"required,max=50"`        // 手机号
	RoleIDs         []string              `json:"userRoles" validate:"omitempty"`                 // 角色ID
	Remark          *string               `json:"remark" validate:"omitempty,max=255"`            // 备注
	Organization    *string               `json:"organization" validate:"omitempty,max=200"`      // 用户所属组织机构名称
	Timezone        *string               `json:"timezone" validate:"omitempty,max=50"`           // 所在时区
	DefaultLanguage *string               `json:"default_language" validate:"omitempty,max=10"`   // 默认语言
	Address         *CreateUserAddressReq `json:"address" validate:"omitempty"`                   // 地址信息
}

type LoginReq struct {
	Email    string `json:"email" validate:"required" example:"user@example.com"`           // 登录账号(输入邮箱或者手机号)
	Password string `json:"password" validate:"required,min=6,max=512" example:"Aa123456!"` // 密码
	Salt     string `json:"salt" validate:"omitempty,max=512"`                              // 随机盐(如果在超管设置了前端RSA加密则需要上送)
}

type LoginRsp struct {
	Token     *string `gorm:"column:token" json:"token"` // 登录凭证
	ExpiresIn int64   `json:"expires_in"`                // 过期时间(单位:秒)
}

type UserListReq struct {
	PageReq
	Email        *string `json:"email" form:"email" validate:"omitempty"`                       // 邮箱
	PhoneNumber  *string `json:"phone_number" form:"phone_number" validate:"omitempty,max=50"`  // 手机号
	Name         *string `json:"name" form:"name" validate:"omitempty,max=50"`                  // 姓名
	Status       *string `json:"status" form:"status" validate:"omitempty,oneof=N F"`           // 用户状态 F-冻结 N-正常
	Organization *string `json:"organization" form:"organization" validate:"omitempty,max=200"` // 组织机构名称
	// 地址相关查询字段
	Country  *string `json:"country" form:"country" validate:"omitempty,max=50"`   // 国家
	Province *string `json:"province" form:"province" validate:"omitempty,max=50"` // 省份
	City     *string `json:"city" form:"city" validate:"omitempty,max=50"`         // 城市
}

type UserSelectorReq struct {
	PageReq
	Name *string `json:"name" form:"name" validate:"omitempty,max=50"` // 用户名称（模糊匹配）
}

type UserSelectorItem struct {
	UserID   string `json:"user_id"`   // 用户ID
	Name     string `json:"name"`      // 用户姓名
	Email    string `json:"email"`     // 用户邮箱
	UserType string `json:"user_type"` // 用户类型：TENANT_ADMIN 或 TENANT_USER
}

type UpdateUserReq struct {
	ID              string                `json:"id" validate:"required,uuid"`                    // 主键ID
	AdditionalInfo  *string               `json:"additional_info" validate:"omitempty,max=10000"` // 附加信息
	Email           *string               `json:"email"  validate:"omitempty,email"`              // 邮箱
	Name            *string               `json:"name" validate:"omitempty,min=2,max=50"`         // 姓名
	PhoneNumber     *string               `json:"phone_number" validate:"omitempty,max=50"`       // 手机号
	Remark          *string               `json:"remark" validate:"omitempty,max=255"`            // 备注
	Status          *string               `json:"status" validate:"omitempty,oneof=N F"`          // 用户状态 F-冻结 N-正常
	Password        *string               `json:"password" validate:"omitempty,min=8,max=20"`     // 密码
	UpdatedAt       *time.Time            `json:"updated_at" validate:"omitempty"`                // 更新时间
	RoleIDs         []string              `json:"userRoles" validate:"omitempty"`                 // 角色ID
	Organization    *string               `json:"organization" validate:"omitempty,max=200"`      // 用户所属组织机构名称
	Timezone        *string               `json:"timezone" validate:"omitempty,max=50"`           // 所在时区
	DefaultLanguage *string               `json:"default_language" validate:"omitempty,max=10"`   // 默认语言
	Address         *UpdateUserAddressReq `json:"address" validate:"omitempty"`                   // 地址信息
}

type UpdateUserInfoReq struct {
	ID        string     `json:"id" validate:"required"`                 // 主键ID
	Name      *string    `json:"name" validate:"omitempty,min=2,max=50"` // 姓名
	Remark    *string    `json:"remark" validate:"omitempty,max=255"`    // 备注
	Password  *string    `json:"password" validate:"omitempty,max=512"`  // 密码
	UpdatedAt *time.Time `json:"updated_at" validate:"omitempty"`        // 更新时间
	Salt      string     `json:"salt"`
}

type TransformUserReq struct {
	BecomeUserID string `json:"become_user_id" validate:"required,uuid"` // 用户ID
}

type ResetPasswordReq struct {
	Email      string `json:"email" validate:"required,email"`           // 邮箱
	VerifyCode string `json:"verify_code" validate:"omitempty"`          // 验证码；兼容旧的同页重置流程
	ResetToken string `json:"reset_token" validate:"omitempty,max=256"`  // 邮件重置链接 token；覆盖用户手册中的链接式重置流程
	Password   string `json:"password" validate:"required,min=8,max=20"` // 新密码
}

type ResetPasswordLinkReq struct {
	Email      string `json:"email" validate:"required,email"` // 邮箱
	VerifyCode string `json:"verify_code" validate:"required"` // 验证码
}

type ResetPasswordLinkRsp struct {
	ExpiresIn int64 `json:"expires_in"` // 重置链接有效期，单位秒
}

type ChangeEmailReq struct {
	NewEmail   string `json:"new_email" validate:"required,email"` // 新邮箱
	VerifyCode string `json:"verify_code" validate:"required"`     // 邮箱验证码；兼容旧站当前邮箱验证码，并保留新邮箱验证码兜底
}

type WarningEmailReq struct {
	Emails []string `json:"emails" validate:"omitempty,dive,omitempty,email"` // 告警接收邮箱，沿用既有 /user/warning-email 路径
}

type PreferLanguageReq struct {
	PreferLang      string `json:"prefer_lang" validate:"omitempty,max=10"`      // 沿用既有 /user/prefer-lang 路径
	DefaultLanguage string `json:"default_language" validate:"omitempty,max=10"` // 当前用户默认语言
}

type EmailRegisterReq struct {
	Email           string  `json:"email" validate:"required,email"` // 邮箱
	VerifyCode      string  `json:"verify_code" validate:"required"` // 验证码
	Password        string  `json:"password" validate:"required"`    // 新密码
	ConfirmPassword *string `json:"confirm_password" validate:"omitempty"`
	PhoneNumber     string  `json:"phone_number" validate:"omitempty,max=50"` // 可选手机号码；RDI 手册注册仅要求邮箱、密码和验证码
	PhonePrefix     string  `json:"phone_prefix" validate:"omitempty,max=10"` // 可选手机前缀
	Salt            *string `json:"salt" validate:"omitempty"`                // 随机盐
}

// SuperAdminInitReq 首次安装超管初始化请求
type SuperAdminInitReq struct {
	Email            string `json:"email" validate:"required,email"`                   // 超管邮箱
	Password         string `json:"password" validate:"required,min=8,max=20"`         // 超管密码
	MarketRegistered bool   `json:"market_registered,omitempty"`                       // 是否由市场回流确认已注册
	MarketEmail      string `json:"market_email,omitempty" validate:"omitempty,email"` // 市场回流邮箱（需与 email 一致）
	MarketSource     string `json:"market_source,omitempty"`                           // 市场来源标识（可选）
}

// MarketRegisterReq 沿用既有接口命名（/tenant/market-register）
type MarketRegisterReq = SuperAdminInitReq

// TenantSetupStateRsp 首次安装/注册态
type TenantSetupStateRsp struct {
	HasAdmin          bool   `json:"has_admin"`                     // 是否已有超管
	HasTenantAdmin    bool   `json:"has_tenant_admin"`              // 是否已有有效租户管理员
	HasTenant         bool   `json:"has_tenant"`                    // 是否已有可用租户上下文
	Entry             string `json:"entry"`                         // login | register
	NextStep          string `json:"next_step"`                     // create_super_admin | create_tenant_admin | login
	MarketBaseURL     string `json:"market_base_url,omitempty"`     // 市场基础地址
	MarketRegisterURL string `json:"market_register_url,omitempty"` // 市场注册页地址
}

type CreateUserAddressReq struct {
	Country         *string `json:"country" validate:"omitempty,max=50"`           // 国家
	Province        *string `json:"province" validate:"omitempty,max=50"`          // 省份
	City            *string `json:"city" validate:"omitempty,max=50"`              // 城市
	District        *string `json:"district" validate:"omitempty,max=50"`          // 区县
	Street          *string `json:"street" validate:"omitempty,max=100"`           // 街道/乡镇
	DetailedAddress *string `json:"detailed_address" validate:"omitempty,max=200"` // 详细地址
	PostalCode      *string `json:"postal_code" validate:"omitempty,max=10"`       // 邮政编码
	AddressLabel    *string `json:"address_label" validate:"omitempty,max=50"`     // 地址标签
	Longitude       *string `json:"longitude" validate:"omitempty,max=20"`         // 经度
	Latitude        *string `json:"latitude" validate:"omitempty,max=20"`          // 纬度
	AdditionalInfo  *string `json:"additional_info" validate:"omitempty,max=500"`  // 附加信息
}

type UpdateUserAddressReq struct {
	Country         *string `json:"country" validate:"omitempty,max=50"`           // 国家
	Province        *string `json:"province" validate:"omitempty,max=50"`          // 省份
	City            *string `json:"city" validate:"omitempty,max=50"`              // 城市
	District        *string `json:"district" validate:"omitempty,max=50"`          // 区县
	Street          *string `json:"street" validate:"omitempty,max=100"`           // 街道/乡镇
	DetailedAddress *string `json:"detailed_address" validate:"omitempty,max=200"` // 详细地址
	PostalCode      *string `json:"postal_code" validate:"omitempty,max=10"`       // 邮政编码
	AddressLabel    *string `json:"address_label" validate:"omitempty,max=50"`     // 地址标签
	Longitude       *string `json:"longitude" validate:"omitempty,max=20"`         // 经度
	Latitude        *string `json:"latitude" validate:"omitempty,max=20"`          // 纬度
	AdditionalInfo  *string `json:"additional_info" validate:"omitempty,max=500"`  // 附加信息
}
