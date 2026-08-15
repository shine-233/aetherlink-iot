// sys_function.go 提供系统功能开关相关的 HTTP 入口。
// 主要负责两类动作：
// 1. 按语言读取系统功能配置列表，供系统设置页或其他依赖功能开关的界面回显当前状态。
// 2. 仅允许系统管理员切换单个功能项，再把真正的配置更新、缓存刷新和级联影响处理下沉到 SysFunction service。
// 这一层的职责边界比较明确：读取请求头、claims 和路径参数，完成最小权限收口后调用 service，不直接承载功能语义。
// 静态审查建议：
// 1. 读取接口与更新接口的权限边界不同，后续调整时要同步检查前端系统设置页的可见性与可编辑性。
// 2. 文件中仍保留 `Fcuntion` / `Funcion` 历史命名，后续若清理拼写，应视为接口、路由、文档与前端调用双侧同步改动。
// 3. 更新接口目前用参数错误码表达“非系统管理员”，若后续前后端要严格区分参数错误与权限失败，建议改成更明确的鉴权类错误码。
// 4. 功能开关会影响其他页面和密码加密等系统级行为，修改 service 语义时要同步检查设置页提示文案和依赖链路。
package api

import (
	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type SysFunctionApi struct{}

// HandleSysFcuntion 返回系统功能开关列表。
// 绑定/claims：不绑定请求体，也不强依赖 claims；当前只读取 Accept-Language，
// 由 service 决定返回的功能项说明、分组名称或展示文案使用哪套语言。
// 边界说明：该接口只负责把语言上下文透传给 service，不在 handler 中拼装功能开关的业务含义。
// 静态审查建议：局部变量 `date` 实际承载的是功能配置数据而不是日期，后续可改名为 `data` 或 `res`，降低阅读误导。
// /api/v1/sys_function GET
func (*SysFunctionApi) HandleSysFcuntion(c *gin.Context) {
	lang := c.GetHeader("Accept-Language")
	date, err := service.GroupApp.SysFunction.GetSysFuncion(lang)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", date)
}

// UpdateSysFcuntion 更新单个系统功能开关。
// 绑定/claims：不绑定请求体，核心输入来自路径参数 `id` 和登录 claims；
// 当前显式限制为 SYS_ADMIN，避免普通租户管理员直接改动平台级功能项。
// 边界说明：handler 只做最小权限门槛判断，不负责校验该功能项是否存在、切换后会触发哪些系统级副作用，这些都交给 service 层。
// 静态审查建议：
// 1. 路由注释写的是 `{function_id}`，实际读取的是 `c.Param("id")`，后续应确认路由注册名称与文档保持一致。
// 2. `id` 为空或格式非法时目前依赖 service 再兜底，若后续功能项标识规则稳定，可以在 handler 层补更轻量的格式校验。
// /api/v1/sys_function/{function_id} PUT
func (*SysFunctionApi) UpdateSysFcuntion(c *gin.Context) {
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	if userClaims.Authority != dal.SYS_ADMIN {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"authority": "authority is not sys admin",
		}))
		return
	}
	id := c.Param("id")
	err := service.GroupApp.SysFunction.UpdateSysFuncion(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}
