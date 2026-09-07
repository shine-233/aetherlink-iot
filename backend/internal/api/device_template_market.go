// 文件用途：模板市场运营化 HTTP 入口（PHASE-D-D10）。
// 核心逻辑：行业分类目录 + 按行业打包导出（base64 载荷，前端解码后下载）。
package api

import (
	"encoding/base64"
	"encoding/json"
	"strings"

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
