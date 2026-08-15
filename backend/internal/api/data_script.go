// 文件用途：提供数据脚本相关 HTTP 接口处理器，负责把 Gin 请求转换为数据脚本服务层可消费的入参，
// 并把 service 返回结果写回统一响应上下文。
// 核心调用链：路由进入本文件 handler 后，先经 BindAndValidate 完成 body/query/path 绑定与结构校验，
// 再从 c.MustGet("claims") 读取认证中间件写入的用户身份，随后调用 service.GroupApp.DataScript
// 对应的 CRUD、调试和启停能力；若 service 返回错误则通过 c.Error 交给统一错误处理，成功时通过 c.Set("data", ...)
// 交给统一响应封装器输出。
// 参数绑定边界：POST/PUT 默认以 JSON body 绑定，GET 会按 query/form 绑定；DeleteDataScript 的 id 来自路径参数，
// 其余 handler 的业务字段来自请求体或查询串。本层只做通用绑定校验和少量显式前置判断，不重复实现 service 内部的权限、
// 租户归属、唯一启用约束、缓存失效或脚本执行逻辑。
// 关键注意事项：本文件假设鉴权中间件已写入 claims，若路由绕过鉴权则 c.MustGet("claims") 会直接 panic；
// 因此这些 handler 必须挂在受保护路由组下。UpdateDataScript 额外要求 description 与 name 不能同时为空，
// 其余更细的业务边界例如设备配置读写权限、跨租户更新禁止、同脚本类型仅允许一个启用实例、调试执行失败映射等，
// 都在 service 层兜底。
// 静态审查建议：若后续同类接口继续增加，可优先抽取 claims 获取与 id/path 解析辅助函数，减少重复的 MustGet/Param 模式；
// 还可以把各 handler 的调用链和边界约定同步到接口文档，避免前后端对必填字段、空值语义和启停副作用理解不一致。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DataScriptApi struct{}

// CreateDataScript 创建数据处理脚本。
// 调用链：BindAndValidate 绑定并校验 CreateDataScriptReq -> 从 Gin 上下文读取 claims -> 调用
// service.GroupApp.DataScript.CreateDataScript -> 成功后把新建脚本实体写入响应 data。
// 参数绑定：请求体提供 name、device_config_id、content、script_type、last_analog_input、
// description、remark；字段格式和必填约束由 req 标签与 BindAndValidate 统一处理。
// 业务边界：handler 不负责检查设备配置写权限、默认启用状态或建库失败映射，这些都由 service 层完成；
// 若 claims 缺失或服务层报错，本层直接中断并交给统一错误处理。
// @Router   /api/v1/data_script [post]
func (*DataScriptApi) CreateDataScript(c *gin.Context) {
	var req model.CreateDataScriptReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DataScript.CreateDataScript(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// UpdateDataScript 更新数据处理脚本。
// 调用链：BindAndValidate 绑定 UpdateDataScriptReq -> 本层补充校验 description 与 name
// 不能同时为空 -> 读取 claims -> 调用 service.GroupApp.DataScript.UpdateDataScript ->
// 成功后返回空 data，由统一响应层输出成功状态。
// 参数绑定：id、name、device_config_id、script_type 等主体字段来自 PUT 请求体；updated_at
// 也会透传给 service/DAO 参与更新。
// 业务边界：跨租户设备配置变更、脚本写权限校验、数据库更新和缓存删除都在 service 层处理；
// 本层只补一条显式“至少保留名称或描述之一”的前置约束，避免把明显空更新继续下沉。
// @Router   /api/v1/data_script [put]
func (*DataScriptApi) UpdateDataScript(c *gin.Context) {
	var req model.UpdateDataScriptReq
	if !BindAndValidate(c, &req) {
		return
	}

	if req.Description == nil && req.Name == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "description and name can not be empty at the same time"))
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DataScript.UpdateDataScript(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// DeleteDataScript 删除数据处理脚本。
// 调用链：从路径参数读取 id -> 从上下文读取 claims -> 调用
// service.GroupApp.DataScript.DeleteDataScript -> 成功后返回空 data。
// 参数绑定：这里只消费 /api/v1/data_script/{id} 中的路径参数，不经过 BindAndValidate，
// 因此 id 的非空、可访问性和是否存在主要依赖 service 层继续校验。
// 业务边界：若脚本已启用，service 删除后会尝试清理缓存；handler 本身不感知缓存、副作用或资源是否存在。
// @Router   /api/v1/data_script/{id} [delete]
func (*DataScriptApi) DeleteDataScript(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DataScript.DeleteDataScript(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleDataScriptListByPage 分页查询数据处理脚本列表。
// 调用链：BindAndValidate 绑定 GetDataScriptListByPageReq（含分页与筛选条件）-> 读取 claims ->
// 调用 service.GroupApp.DataScript.GetDataScriptListByPage -> 将 total/list 结果对象写入响应 data。
// 参数绑定：page/page_size 等分页字段与 device_config_id、script_type 来自 GET query/form；
// 其中 device_config_id 在结构标签和 service 层都被视为必填。
// 业务边界：本层不负责设备配置读权限判定、空列表语义或数据库分页细节；service 会再次校验
// DeviceConfigId 非空并检查可读权限，从而兜住 query 绑定绕过或手工构造请求的场景。
// @Router   /api/v1/data_script [get]
func (*DataScriptApi) HandleDataScriptListByPage(c *gin.Context) {
	var req model.GetDataScriptListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data_scriptList, err := service.GroupApp.DataScript.GetDataScriptListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data_scriptList)
}

// QuizDataScript 调试执行数据脚本。
// 调用链：BindAndValidate 绑定 QuizDataScriptReq -> 读取 claims -> 调用
// service.GroupApp.DataScript.QuizDataScript -> service 内部按 last_analog_input 是否以 0x
// 开头决定走十六进制解码还是原始字节串执行 -> 将脚本输出字符串写入响应 data。
// 参数绑定：content、last_analog_input、topic 来自请求体，其中 content 必填且长度受限；
// 本层不解析 payload 编码，只负责把原始文本交给 service。
// 业务边界：只有已鉴权用户可调试执行；hex 解码失败、脚本运行异常和错误码映射都由 service 负责，
// handler 不直接接触脚本引擎，也不做持久化副作用。
// @Router   /api/v1/data_script/quiz [post]
func (*DataScriptApi) QuizDataScript(c *gin.Context) {
	var req model.QuizDataScriptReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DataScript.QuizDataScript(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// EnableDataScript 启用或停用数据脚本。
// 调用链：BindAndValidate 绑定 EnableDataScriptReq -> 读取 claims -> 调用
// service.GroupApp.DataScript.EnableDataScript -> service 内部完成写权限校验、启用唯一性检查、
// 状态落库，以及停用时的缓存清理 -> 成功后返回空 data。
// 参数绑定：id 与 enable_flag 来自 PUT 请求体，其中 enable_flag 只允许 Y/N，分别对应启用和停用。
// 业务边界：本层不判断同脚本类型是否已存在启用实例，也不处理缓存键细节；这些副作用与约束全部在
// service 层统一维护，因此 handler 只承接状态切换入口。
// @Router   /api/v1/data_script/enable [put]
func (*DataScriptApi) EnableDataScript(c *gin.Context) {
	var req model.EnableDataScriptReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DataScript.EnableDataScript(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}
