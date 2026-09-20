// 文件用途：预注册凭证一次性下载的 HTTP 入口（ROADMAP P0.5）。
// 核心逻辑：签发下载许可、消费许可并下发明文。
// 关键注意事项：
//  1. 下载端点是**唯一**会返回 devices.voucher 明文的路径，且只允许成功一次。
//     任何"顺手再开一个导出"的改动都会让这个门禁失守，不要在此文件里加别的出口。
//  2. 租户一律由 claims 推导，不收请求体里的 tenant_id。
//  3. 失败不回显任何凭证内容；错误文案也不得暗示批次是否存在于其它租户。
package api

import (
	"aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// GrantPreRegisterCredentials 为批次签发一次性凭证下载许可。
// @Router   /api/v1/device/preRegister/credentials/grants [post]
func (*DeviceApi) GrantPreRegisterCredentials(c *gin.Context) {
	var req model.CredentialGrantReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GrantPreRegisterCredentials(c.Request.Context(), userClaims, req.BatchNumber)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DownloadPreRegisterCredentials 消费许可并下发明文；消费成功后该许可不可再用。
// @Router   /api/v1/device/preRegister/credentials/grants/:id/download [get]
func (*DeviceApi) DownloadPreRegisterCredentials(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.DownloadPreRegisterCredentials(c.Request.Context(), userClaims, c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
