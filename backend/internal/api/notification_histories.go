// notification_histories.go 提供通知历史查询相关的 HTTP 入口。
// 核心链路：
// 1. 绑定分页、筛选等查询参数，确保列表请求在进入 service 前完成基础校验。
// 2. 从 gin 上下文读取 claims，把租户、用户和可见范围边界透传给通知历史 service。
// 3. 将 service 返回的数据按出参 schema 序列化，避免直接把内部结构裸露给前端。
// 静态审查建议：
// 1. 通知历史常被用于审计和问题追踪，后续若追加敏感字段，必须同步复核脱敏与最小可见范围。
// 2. 当前 handler 同时承担查询和出参序列化，若同类接口继续增加，可抽公共列表响应模板减少重复代码。
// 3. `c.MustGet("claims")` 依赖鉴权中间件先行注入，路由改造时要持续校验中间件链顺序，避免运行时 panic。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type NotificationHistoryApi struct{}

// HandleNotificationHistoryListByPage 获取通知历史分页列表。
// 该接口只负责查询参数绑定、claims 注入和出参结构收口，
// 真正的数据权限、筛选组合和排序规则由 NotificationHisory service 决定。
// @Router   /api/v1/notification_history/list [get]
func (*NotificationHistoryApi) HandleNotificationHistoryListByPage(c *gin.Context) {
	var req model.GetNotificationHistoryListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}

	// claims 决定当前用户能看到哪些通知历史记录，API 层不自行推导租户范围。
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	notificationList, err := service.GroupApp.NotificationHisory.GetNotificationHistoryListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	// 统一用出参 schema 做一次收口，避免把 service 内部字段或后续新增敏感字段直接返回给前端。
	ntfoutput, err := utils.SerializeData(notificationList, GetNotificationHistoryListByPageOutSchema{})
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", ntfoutput)
}
