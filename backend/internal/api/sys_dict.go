// 文件用途：提供系统字典、字典多语言和枚举下拉相关的 HTTP handler，
// 将 Gin 上下文中的 JSON、Query、Path、Header 信息整理为 service 层可消费的入参。
// 核心职责：通过 handler_adapter 泛型适配器收敛“绑定 -> 校验 -> 取 claims -> 调 service -> 响应”样板，
// 响应信封仍由 middleware/response 统一渲染，迁移前后 JSON 输出逐字节一致（见 handler_adapter_golden_7_test.go）。
// claims 约定：依赖鉴权中间件预先通过 c.Set("claims") 注入 *utils.UserClaims，
// 适配器经 RequireClaims 读取；路由漏配鉴权时不 panic，返回标准 CodeUnauthorized（有意行为差异，见 handler_adapter.go 头部）。
// 调用链概览：handler -> service.GroupApp.Dict -> dal.*；写操作最终落到 sys_dict/sys_dict_language 相关 DAL。
// 静态审查建议：关注路径 ID 是否需要统一格式校验、claims 读取的前置条件是否稳定、
// Accept-Language 与 FormatLangCode 的语言约定是否和前端保持一致。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type DictApi struct{}

// CreateDictColumn 创建字典列。
// 参数绑定：Handle 经 BindAndValidate 绑定 JSON body 到 model.CreateDictReq，
// 要求 dict_code、dict_value 必填，remark 为可选字段。
// claims：适配器经 RequireClaims 读取 *utils.UserClaims，service 层据此限制仅系统管理员可创建。
// 调用链：DictApi.CreateDictColumn -> service.GroupApp.Dict.CreateDictColumn -> dal.CreateDict。
// 静态审查建议：确认 claims 注入中间件覆盖全部路由，并关注 dict_code 重复写入时的错误语义是否足够清晰。
// @Router   /api/v1/dict/column [post]
func (*DictApi) CreateDictColumn(c *gin.Context) {
	Handle(c, func(req *model.CreateDictReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Dict.CreateDictColumn(req, userClaims)
	})
}

// CreateDictLanguage 创建字典多语言。
// 参数绑定：Handle 经 BindAndValidate 绑定 JSON body 到 model.CreateDictLanguageReq，
// 要求 dict_id、language_code、translation 必填。
// claims：适配器经 RequireClaims 读取 *utils.UserClaims，service 层会先校验管理员权限，再校验 dict_id 是否存在。
// 调用链：DictApi.CreateDictLanguage -> service.GroupApp.Dict.CreateDictLanguage -> dal.GetDictById + dal.CreateDictLanguage。
// 静态审查建议：关注 language_code 是否需要统一大小写/区域格式，以及翻译重复插入时的唯一性约束。
// @Router   /api/v1/dict/language [post]
func (*DictApi) CreateDictLanguage(c *gin.Context) {
	Handle(c, func(req *model.CreateDictLanguageReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Dict.CreateDictLanguage(req, userClaims)
	})
}

// DeleteDictColumn 删除字典列。
// 参数绑定：HandlePathAction 读取路径参数 id，不经过 BindAndValidate；成功时 data 为 nil（响应省略 data 字段）。
// claims：适配器经 RequireClaims 读取 *utils.UserClaims，service 层据此限制仅系统管理员可删除。
// 调用链：DictApi.DeleteDictColumn -> service.GroupApp.Dict.DeleteDict -> dal.DeleteDictById。
// 静态审查建议：可评估是否在 API 边界统一校验 id 的 UUID/长度格式，并补充被引用字典删除的风险说明。
// @Router   /api/v1/dict/column/{id} [delete]
func (*DictApi) DeleteDictColumn(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		return service.GroupApp.Dict.DeleteDict(id, userClaims)
	})
}

// DeleteDictLanguage 删除字典多语言。
// 参数绑定：HandlePathAction 读取路径参数 id，不经过 BindAndValidate；成功时 data 为 nil（响应省略 data 字段）。
// claims：适配器经 RequireClaims 读取 *utils.UserClaims，service 层据此限制仅系统管理员可删除。
// 调用链：DictApi.DeleteDictLanguage -> service.GroupApp.Dict.DeleteDictLanguage -> dal.DeleteDictLanguageById。
// 静态审查建议：关注路径 id 的格式校验是否缺失，以及删除后是否需要额外审计日志或引用追踪。
// @Router   /api/v1/dict/language/{id} [delete]
func (*DictApi) DeleteDictLanguage(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		return service.GroupApp.Dict.DeleteDictLanguage(id, userClaims)
	})
}

// HandleDict 枚举查询接口。
// 参数绑定：HandlePublicQuery 强制按 query 绑定到 model.DictListReq（GET 下与 BindAndValidate 行为一致），
// 其中 dict_code 必填；同时读取 Accept-Language 请求头参与语言选择。
// claims：当前 handler 不要求 claims（HandlePublicQuery），接口可在无显式用户上下文下查询字典枚举。
// 调用链：DictApi.HandleDict -> service.GroupApp.Dict.GetDict -> dal.GetDictListByCode + dal.GetDictLanguageByDictIdListAndLanguageCode。
// 静态审查建议：关注 Accept-Language 缺省值、非法值回退策略，以及字典列表为空时的返回契约是否稳定。
// @Router   /api/v1/dict/enum [get]
func (*DictApi) HandleDict(c *gin.Context) {
	HandlePublicQuery(c, func(req *model.DictListReq) (interface{}, error) {
		return service.GroupApp.Dict.GetDict(req, c.GetHeader("Accept-Language"))
	})
}

// HandleProtocolAndService 协议服务下拉菜单查询接口。
// 参数绑定：HandlePublicQuery 强制按 query 绑定到 model.ProtocolMenuReq（GET 下与 BindAndValidate 行为一致），
// language_code 可选，service 层在缺省时会回退到 zh。
// claims：当前 handler 不要求 claims（HandlePublicQuery），返回值依赖字典表中的协议和服务配置。
// 调用链：DictApi.HandleProtocolAndService -> service.GroupApp.Dict.GetProtocolMenu ->
// dal.GetDictLanguageByDictCodeAndLanguageCode（分别查询直连协议和网关协议）。
// 静态审查建议：建议关注返回中 device_type 的硬编码分支是否需要枚举常量化，并补充 Swagger 路由注释的一致性。
// @Router   /api/v1/dict/protocol/service [get]
func (*DictApi) HandleProtocolAndService(c *gin.Context) {
	HandlePublicQuery(c, func(req *model.ProtocolMenuReq) (interface{}, error) {
		return service.GroupApp.Dict.GetProtocolMenu(req)
	})
}

// HandleDictLanguage 字典多语言查询。
// 参数绑定：路径参数 id，不经过 BindAndValidate。
// claims：当前 handler 不要求 claims（HandlePublicNoBody），是否允许公开查询由路由和中间件策略决定。
// 调用链：DictApi.HandleDictLanguage -> service.GroupApp.Dict.GetDictLanguageListById -> dal.GetDictLanguageListByDictId。
// 静态审查建议：关注路径 id 的格式约束、空结果返回约定，以及是否需要避免泄露不存在字典的探测信息。
// @Router   /api/v1/dict/language/{id} [get]
func (*DictApi) HandleDictLanguage(c *gin.Context) {
	HandlePublicNoBody(c, func() (interface{}, error) {
		return service.GroupApp.Dict.GetDictLanguageListById(c.Param("id"))
	})
}

// HandleDictLisyByPage 字典列表分页查询。
// 参数绑定：Handle 经 BindAndValidate 绑定 Query/Form 到 model.GetDictLisyByPageReq（GET 走 query），
// 继承分页参数并支持可选的 dict_code 过滤。
// claims：适配器经 RequireClaims 读取 *utils.UserClaims，service/DAL 可据此做租户或角色范围过滤。
// 调用链：DictApi.HandleDictLisyByPage -> service.GroupApp.Dict.GetDictListByPage -> dal.GetDictListByPage。
// 静态审查建议：关注分页参数上限、claims 对查询范围的影响是否有单测覆盖，以及调试日志是否需要降级或脱敏。
// @Router   /api/v1/dict [get]
func (*DictApi) HandleDictLisyByPage(c *gin.Context) {
	Handle(c, func(req *model.GetDictLisyByPageReq, userClaims *utils.UserClaims) (interface{}, error) {
		logrus.Info("dictionary list request received")
		return service.GroupApp.Dict.GetDictListByPage(req, userClaims)
	})
}
