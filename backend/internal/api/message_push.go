// message_push.go 负责消息推送相关的 HTTP 接口。
// 这里承接的是“创建推送记录/任务”“推送管理登出”“读取个人或租户推送配置”“保存推送配置”四类入口，
// API 层的职责是完成 DTO 绑定、claims 提取、当前用户身份透传，并把实际的渠道配置、第三方状态维护和业务校验交给 service 层。
// 由于消息推送经常涉及个人身份、通知渠道令牌和租户配置，这一层虽然很薄，但必须把“谁在操作谁的配置”交代清楚。
// 静态审查建议：
// 1. 几个 handler 都是相似的“BindAndValidate -> 取 claims -> 调 service -> c.Set”模板，后续如果推送接口继续增加，
//    可以考虑抽统一辅助函数减少样板代码和遗漏风险。
// 2. MessagePushMangeLogout 存在历史拼写问题，后续若统一命名，需要同时核对前端调用、路由注册和接口文档。
// 3. 当前 handler 默认使用 c.MustGet("claims")，意味着路由层必须保证鉴权中间件先行；若后续有匿名推送场景，需要重新梳理中间件边界。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type MessagePushApi struct {
}

// CreateMessagePush 创建一条消息推送记录或触发一次推送创建流程。
// 绑定/claims：先把请求体绑定到 CreateMessagePushReq，再从 claims 中提取当前登录用户 ID，确保创建动作与操作者身份绑定。
// 边界说明：API 层不决定具体走哪种推送渠道，也不处理令牌刷新、渠道连通性或消息模板渲染，这些都交给 service 层。
// 静态审查建议：当前 handler 只把 userClaims.ID 透传下去，若后续需要区分租户管理员代操作、系统管理员越权操作等场景，建议把更多 claims 上下文显式传递。
// 路由：/api/v1/message_push [post]
func (*MessagePushApi) CreateMessagePush(c *gin.Context) {
	var req model.CreateMessagePushReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.MessagePush.CreateMessagePush(&req, userClaims.ID)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// MessagePushMangeLogout 处理消息推送管理侧的登出或解绑动作。
// 绑定/claims：绑定 MessagePushMangeLogoutReq，并使用 claims 中的用户 ID 限定“当前是谁在发起登出”。
// 边界说明：这里不直接操作第三方 SDK 或本地会话存储，真正的解绑、令牌清理和状态回写都在 service 层。
// 静态审查建议：如果后续要支持按渠道分别登出，建议把请求 DTO 与返回结构做得更显式，避免前端只能靠错误文案区分失败原因。
// 路由：/api/v1/message_push/logout [post]
func (*MessagePushApi) MessagePushMangeLogout(c *gin.Context) {
	var req model.MessagePushMangeLogoutReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.MessagePush.MessagePushMangeLogout(&req, userClaims.ID)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// GetMessagePushConfig 读取当前登录上下文可见的消息推送配置。
// 绑定/claims：不绑定请求体，直接从 claims 提取当前用户与权限上下文，再交由 service 判断应返回个人配置、租户配置或合并视图。
// 边界说明：API 层不缓存配置，也不解释字段含义；权限边界、脱敏策略和默认值补齐由 service 层决定。
// 静态审查建议：若后续配置结构继续膨胀，建议在 service 与接口文档中显式区分“只读回显字段”和“可提交字段”，降低前后端误用概率。
// 路由：/api/v1/message_push/config [get]
func (*MessagePushApi) GetMessagePushConfig(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	res, err := service.GroupApp.MessagePush.GetMessagePushConfig(userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", res)
}

// SetMessagePushConfig 保存当前登录上下文下的消息推送配置。
// 绑定/claims：先绑定 MessagePushConfigReq，再把完整 claims 透传给 service，
// 让下层根据用户身份、租户边界和权限模型决定这次提交究竟能修改哪些配置。
// 边界说明：API 层不做字段级业务校验，也不处理配置落库后的副作用，例如缓存刷新、渠道重连或默认模板补全。
// 静态审查建议：当前成功响应统一返回 nil，若后续前端需要立即拿到归一化后的最新配置，可考虑改成“写后回读”返回模式，减少前端二次请求。
// 路由：/api/v1/message_push/config [post]
func (*MessagePushApi) SetMessagePushConfig(c *gin.Context) {
	var req model.MessagePushConfigReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.MessagePush.SetMessagePushConfig(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}
