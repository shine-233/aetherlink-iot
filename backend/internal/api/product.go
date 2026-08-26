// 文件用途：产品选择列表 API——预注册建档等页面的产品下拉数据源。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type ProductApi struct{}

// HandleProductSelectListByPage 分页查询当前租户可选产品
// @Router   /api/v1/product [get]
func (*ProductApi) HandleProductSelectListByPage(c *gin.Context) {
	var req model.GetProductSelectListReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Device.GetProductSelectListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
