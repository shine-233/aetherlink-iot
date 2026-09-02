// 文件用途：定义 logo 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

type UpdateLogoReq struct {
	Id             string  `json:"id" validate:"required,max=36"`                // Id
	SystemName     string  `json:"system_name" validate:"omitempty,max=99"`      // 系统名称
	LogoCache      *string `json:"logo_cache" validate:"omitempty,max=255"`      // 缓冲logo
	LogoBackground *string `json:"logo_background" validate:"omitempty,max=255"` // 站标Logo
	LogoLoading    *string `json:"logo_loading" validate:"omitempty,max=255"`    // 加载页面Logo
	HomeBackground *string `json:"home_background" validate:"omitempty,max=255"` // 首页背景
	ThemeColor     *string `json:"theme_color" validate:"omitempty,max=32"`      // 主题色（C5 白标，#RRGGBB）
	Favicon        *string `json:"favicon" validate:"omitempty,max=255"`         // 页签favicon URL（C5 白标）
	Remark         *string `json:"remark" validate:"omitempty,max=255"`
}
