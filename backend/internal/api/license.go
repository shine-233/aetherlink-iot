// 文件用途：P3 商业许可证状态的 HTTP 入口。
// 边界说明：只读状态视图；验证与执法在 service 层（启动门控 / 设备配额）。
package api

import (
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type LicenseApi struct{}

// Status 许可证状态（SYS_ADMIN）。不返回材料本身，只返回验证结论与摘要。
// @Summary  商业许可证状态
// @Tags     License
// @Router   /api/v1/license/status [get]
func (*LicenseApi) Status(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	status, err := service.GroupApp.License.GetStatus(claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", status)
}
