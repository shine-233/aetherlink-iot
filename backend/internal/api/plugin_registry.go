// 文件用途：插件注册表管理 HTTP 入口（PHASE-D-D9）。
// 核心逻辑：CRUD/启停/凭证轮换/下行命令下发；claims 提取与错误出口。
// 关键注意事项：token 明文只在创建/轮换响应中出现一次，任何列表接口不回显。
package api

import (
	"encoding/json"
	"io"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// PluginRegistryApi 插件管理 API 聚合（PHASE-D-D9）。
type PluginRegistryApi struct{}

// HandleCreatePlugin 注册插件（返回一次性接入凭证）。
// POST /api/v1/plugins
func (*PluginRegistryApi) HandleCreatePlugin(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 16*1024))
	if err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "invalid body"))
		return
	}
	req := &service.CreatePluginReq{}
	if err := json.Unmarshal(raw, req); err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "invalid json"))
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, svcErr := service.GroupApp.PluginRegistry.Create(req, claims)
	if svcErr != nil {
		c.Error(svcErr)
		return
	}
	c.Set("data", resp)
}

// HandleListPlugins 登记列表。
// GET /api/v1/plugins
func (*PluginRegistryApi) HandleListPlugins(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	rows, svcErr := service.GroupApp.PluginRegistry.List(claims)
	if svcErr != nil {
		c.Error(svcErr)
		return
	}
	c.Set("data", rows)
}

// HandleDeletePlugin 删除登记。
// DELETE /api/v1/plugins/:id
func (*PluginRegistryApi) HandleDeletePlugin(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.PluginRegistry.Delete(id, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}

// handlePluginEnabled 通用启停。
func handlePluginEnabled(c *gin.Context, enabled bool) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	row, err := service.GroupApp.PluginRegistry.SetEnabled(id, enabled, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", row)
}

// HandleEnablePlugin 启用插件（等待接入）。
// PUT /api/v1/plugins/:id/enable
func (p *PluginRegistryApi) HandleEnablePlugin(c *gin.Context) {
	handlePluginEnabled(c, true)
}

// HandleDisablePlugin 禁用插件。
// PUT /api/v1/plugins/:id/disable
func (p *PluginRegistryApi) HandleDisablePlugin(c *gin.Context) {
	handlePluginEnabled(c, false)
}

// HandleRotatePluginToken 凭证轮换（一次性新 token）。
// PUT /api/v1/plugins/:id/token
func (*PluginRegistryApi) HandleRotatePluginToken(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.PluginRegistry.RotateToken(id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandlePluginDownlink 平台→插件下行命令。
// POST /api/v1/plugins/:id/downlink
func (*PluginRegistryApi) HandlePluginDownlink(c *gin.Context) {
	id := c.Param("id")
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 64*1024))
	if err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "invalid body"))
		return
	}
	req := struct {
		DeviceNumber string                 `json:"device_number"`
		Identify     string                 `json:"identify"`
		Params       map[string]interface{} `json:"params"`
	}{}
	if err := json.Unmarshal(raw, &req); err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "invalid json"))
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.PluginRegistry.SendDownlink(id, req.DeviceNumber, req.Identify, req.Params, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}
