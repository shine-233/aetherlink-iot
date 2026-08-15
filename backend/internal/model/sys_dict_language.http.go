// 文件用途：定义 sys dict language 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

type CreateDictLanguageReq struct {
	DictId       string `json:"dict_id" validate:"required,max=36"`
	LanguageCode string `json:"language_code"  validate:"required,max=36"`
	Translation  string `json:"translation" validate:"required,max=255"`
}
