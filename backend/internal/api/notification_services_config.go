// notification_services_config.go 提供通知服务配置相关的 HTTP 入口。
// 核心链路：
// 1. 仅允许系统管理员读取或修改通知服务配置。
// 2. 对通知类型和开关状态做基础枚举校验。
// 3. 调用 service 完成邮件/短信等通知服务配置保存、查询与测试发送。
// 静态审查建议：
// 1. 该文件里多处重复 SYS_ADMIN 校验，后续可抽小 helper，但不要把权限判断下放到前端。
// 2. 当前通知类型校验只覆盖邮件和短信，若继续扩展推送或其他通道，要同步更新这里的显式枚举。
// 3. 测试邮件入口直接影响配置可用性回验，修改请求结构时要同步前端通知管理页的调试弹窗。
package api

import (
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type NotificationServicesConfigApi struct{}

// SaveNotificationServicesConfig 创建或修改通知服务配置。
// 当前采用“保存即新增/更新合一”的接口形态，管理员提交时会同时经过通知类型与开关状态校验。
// @Router   /api/v1/notification/services/config [post]
func (*NotificationServicesConfigApi) SaveNotificationServicesConfig(c *gin.Context) {
	var req model.SaveNotificationServicesConfigReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	// 这里只允许系统管理员维护全局通知服务配置。
	if userClaims.Authority != dal.SYS_ADMIN {
		c.Error(errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"authority": "authority is not sys admin",
		}))
		return
	}

	// 当前只接受邮件与短信两类通知服务配置。
	if req.NoticeType != model.NoticeType_Email && req.NoticeType != model.NoticeType_SME_CODE {
		c.Error(errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"noticeType": "noticeType is not email or sme",
		}))
		return
	}

	// 开关状态必须显式落在 OPEN/CLOSE 枚举内，避免脏值进入配置表。
	if req.Status != model.OPEN && req.Status != model.CLOSE {
		c.Error(errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"status": "status is not open or close",
		}))
		return
	}

	data, err := service.GroupApp.NotificationServicesConfig.SaveNotificationServicesConfig(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleNotificationServicesConfig 根据通知类型获取配置详情。
// 前端通知管理页会依赖这个接口回显邮件与短信服务的当前配置。
// @Router   /api/v1/notification/services/config/{type} [get]
func (*NotificationServicesConfigApi) HandleNotificationServicesConfig(c *gin.Context) {
	noticeType := c.Param("type")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	// 验证SYS_ADMIN
	if userClaims.Authority != dal.SYS_ADMIN {
		c.Error(errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"authority": "authority is not sys admin",
		}))
		return
	}
	data, err := service.GroupApp.NotificationServicesConfig.GetNotificationServicesConfig(noticeType, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// SendTestEmail 发送测试邮件。
// 该入口主要用于管理员在保存邮件服务配置后快速验证通道是否可用。
// @Router   /api/v1/notification/services/config/e-mail/test [post]
func (*NotificationServicesConfigApi) SendTestEmail(c *gin.Context) {
	var req model.SendTestEmailReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if userClaims.Authority != dal.SYS_ADMIN {
		c.Error(errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"authority": "authority is not sys admin",
		}))
		return
	}
	err := service.GroupApp.NotificationServicesConfig.SendTestEmailByAdmin(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}
