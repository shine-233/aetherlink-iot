// 文件用途：模板市场运营化 HTTP 入口（PHASE-D-D10）。
// 核心逻辑：行业分类目录 + 按行业打包导出（base64 载荷，前端解码后下载）。
package api

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// HandleMarketCatalog 行业分类目录（浏览页 tab 数据源）。
// GET /api/v1/device/template/market/catalog
func (*DeviceApi) HandleMarketCatalog(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	rows, err := service.GroupApp.DeviceTemplate.MarketCatalog(userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", rows)
}

// HandleExportMarketBundle 按行业打包导出（type_key 为空=全量）。
// GET /api/v1/device/template/market/bundle?type_key=xxx
// 返回标准 JSON 信封：file_name/content_base64/count——前端解码为文件下载；
// curl 侧可 jq -r .data.content_base64 | base64 -d > bundle.json 后逐条回放 import。
func (*DeviceApi) HandleExportMarketBundle(c *gin.Context) {
	typeKey := c.Query("type_key")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	bundle, err := service.GroupApp.DeviceTemplate.ExportMarketBundle(typeKey, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	payload, jsonErr := json.Marshal(bundle)
	if jsonErr != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "marshal bundle failed"))
		return
	}
	safe := strings.ReplaceAll(strings.TrimSpace(typeKey), "/", "_")
	if safe == "" {
		safe = "all"
	}
	c.Set("data", gin.H{
		"file_name":      "aetherlink-market-bundle-" + safe + ".json",
		"content_base64": base64.StdEncoding.EncodeToString(payload),
		"count":          bundle.Count,
	})
}

// HandleImportMarketBundle 打包载荷导入/预览（ROADMAP P1.6）。
// @Summary  模板市场打包载荷导入/预览
// @Tags     DeviceTemplateMarket
// @Router   /api/v1/device/template/market/bundle/import [post]
// preview=true 只读预览；预览含覆盖项时导入需 confirm_overwrite=true 显式确认。
// 与 HandleExportMarketBundle 同一载荷契约（导出的 bundle 字段原样回传即可），
// 形成出包即签名、入包必验签的闭环。
func (*DeviceApi) HandleImportMarketBundle(c *gin.Context) {
	var req model.ImportMarketBundleReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	rsp, err := service.GroupApp.DeviceTemplate.ImportMarketBundle(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", rsp)
}

// HandleUpgradeDeviceTemplate 模板升级：新版本载荷导入并记录可回滚历史（P1.6）。
// @Summary  模板升级
// @Tags     DeviceTemplateMarket
// @Router   /api/v1/device/template/upgrade [post]
func (*DeviceApi) HandleUpgradeDeviceTemplate(c *gin.Context) {
	var req model.UpgradeDeviceTemplateReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	rsp, err := service.GroupApp.DeviceTemplate.UpgradeDeviceTemplate(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", rsp)
}

// HandleRollbackTemplateUpgrade 按历史记录回滚到旧版本（重放旧载荷，幂等）。
// @Summary  模板升级回滚
// @Tags     DeviceTemplateMarket
// @Router   /api/v1/device/template/upgrade/{history_id}/rollback [post]
func (*DeviceApi) HandleRollbackTemplateUpgrade(c *gin.Context) {
	historyID := c.Param("history_id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	template, err := service.GroupApp.DeviceTemplate.RollbackTemplateUpgrade(historyID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", template)
}

// HandleListTemplateUpgradeHistory 某模板的升级历史（回滚点列表）。
// @Summary  模板升级历史
// @Tags     DeviceTemplateMarket
// @Router   /api/v1/device/template/upgrade/history [get]
func (*DeviceApi) HandleListTemplateUpgradeHistory(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	rows, err := dal.ListTemplateUpgradeHistoryInTenant(userClaims.TenantID, strings.TrimSpace(c.Query("template_name")), 0)
	if err != nil {
		c.Error(errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()}))
		return
	}
	c.Set("data", rows)
}
