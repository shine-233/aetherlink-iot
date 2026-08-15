// notification_group.go 提供通知组相关的 HTTP 入口。
// 核心链路：
// 1. 绑定通知组增删改查与分页查询请求。
// 2. 注入 claims，交给 NotificationGroup service 处理租户与业务规则。
// 3. 通过输出 schema 把内部实体序列化成对外响应结构。
// 静态审查建议：
// 1. 当前文件对详情、更新、列表都做了 SerializeData 输出整形，后续新增字段时要同步检查 schema，而不是只改 service 实体。
// 2. 通知组会影响告警、消息推送和通知策略编排，修改 ID、成员或输出结构时要同步前端通知组管理页。
// 3. if/else 包裹式写法已经偏多，后续可逐步统一成“先判错再返回”的薄 handler 风格。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type NotificationGroupApi struct{}

// CreateNotificationGroup 创建消息通知组。
// 创建成功后会按输出 schema 返回标准化通知组结构，而不是直接暴露内部模型。
// @Router   /api/v1/notification_group [post]
func (*NotificationGroupApi) CreateNotificationGroup(c *gin.Context) {
	var req model.CreateNotificationGroupReq

	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	notificationGroup, err := service.GroupApp.NotificationGroup.CreateNotificationGroup(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	notificationGroupOs, err := utils.SerializeData(*notificationGroup, ReadNotificationGroupOutSchema{})
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", notificationGroupOs)
}

// HandleNotificationGroupById 获取通知组详情。
// 这里先取业务实体，再通过 ReadNotificationGroupOutSchema 做输出裁剪。
// @Router   /api/v1/notification_group/{id} [get]
func (*NotificationGroupApi) HandleNotificationGroupById(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if ntfgroup, err := service.GroupApp.NotificationGroup.GetNotificationGroupById(id, userClaims); err != nil {
		c.Error(err)
		return
	} else {
		notificationGroupOs, err := utils.SerializeData(*ntfgroup, ReadNotificationGroupOutSchema{})
		if err != nil {
			c.Error(err)
			return
		}
		c.Set("data", notificationGroupOs)
	}
}

// UpdateNotificationGroup 更新通知组。
// 更新成功后的返回值也会经过输出 schema，保持与详情接口的对外响应风格一致。
// @Router   /api/v1/notification_group/{id} [put]
func (*NotificationGroupApi) UpdateNotificationGroup(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateNotificationGroupReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if updated, err := service.GroupApp.NotificationGroup.UpdateNotificationGroup(id, &req, userClaims); err != nil {
		c.Error(err)
		return
	} else {
		updateoutput, err := utils.SerializeData(updated, UpdateNotificationGroupOutSchema{})
		if err != nil {
			c.Error(err)
			return
		}
		c.Set("data", updateoutput)
	}
}

// DeleteNotificationGroup 删除通知组。
// 删除后不返回实体，只回传空 data。
// @Router   /api/v1/notification_group/{id} [delete]
func (*NotificationGroupApi) DeleteNotificationGroup(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.NotificationGroup.DeleteNotificationGroup(id, userClaims); err != nil {
		c.Error(err)
		return
	} else {
		c.Set("data", nil)
	}
}

// HandleNotificationGroupListByPage 获取通知组分页列表。
// 该接口会把 service 返回的分页结构按列表输出 schema 再次整理，供前端表格直接消费。
// @Router   /api/v1/notification_group/list [get]
func (*NotificationGroupApi) HandleNotificationGroupListByPage(c *gin.Context) {
	var req model.GetNotificationGroupListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	notificationList, err := service.GroupApp.NotificationGroup.GetNotificationGroupListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	ntfoutput, err := utils.SerializeData(notificationList, GetNotificationGroupListByPageOutSchema{})
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", ntfoutput)
}
