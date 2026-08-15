// 文件用途：提供告警邮件模板列表、创建、修改、删除、默认切换和预览 HTTP 入口。
// 核心逻辑：handler 只绑定参数并透传 claims，系统/租户作用域权限由 service 统一判定。
// 关键注意事项：预览只渲染白名单变量，不发送邮件；模板接口不接收 tenant_id，防止客户端越权指定作用域。
package api

import (
	"strings"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

func (*NotificationServicesConfigApi) ListEmailTemplates(c *gin.Context) {
	var req model.PageReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.NotificationServicesConfig.ListEmailTemplates(req.Page, req.PageSize, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*NotificationServicesConfigApi) CreateEmailTemplate(c *gin.Context) {
	var req model.EmailTemplateUpsertReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.NotificationServicesConfig.CreateEmailTemplate(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*NotificationServicesConfigApi) UpdateEmailTemplate(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "email template id is required"))
		return
	}
	var req model.EmailTemplateUpsertReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.NotificationServicesConfig.UpdateEmailTemplate(id, &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*NotificationServicesConfigApi) DeleteEmailTemplate(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "email template id is required"))
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.NotificationServicesConfig.DeleteEmailTemplate(id, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func (*NotificationServicesConfigApi) SetDefaultEmailTemplate(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "email template id is required"))
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.NotificationServicesConfig.SetDefaultEmailTemplate(id, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func (*NotificationServicesConfigApi) PreviewEmailTemplate(c *gin.Context) {
	var req model.EmailTemplatePreviewReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.NotificationServicesConfig.PreviewEmailTemplate(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
