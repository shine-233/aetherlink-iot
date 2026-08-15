// device_auth.go 提供设备动态认证入口。
// 核心链路：
// 1. 绑定设备动态认证请求。
// 2. 把设备配置密钥、设备编号、产品标识等参数交给 DeviceAuth service。
// 3. 返回设备接入所需的认证结果或凭证信息。
// 静态审查建议：
// 1. 该入口面向设备接入链路，后续不要把租户后台的管理逻辑混入这里。
// 2. 认证失败的错误语义会直接影响设备侧排障体验，修改 service 错误码时要同步检查这个入口的表现。
// 3. Swagger 请求片段中的字段契约应与真实请求结构保持一致，避免设备接入文档漂移。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// DeviceAuthApi 是设备动态认证的 handler 门面。
type DeviceAuthApi struct{}

// DeviceAuth 处理设备动态认证请求。
// 该接口通常服务于一型一密或设备配置密钥换取设备凭证的接入流程。
// @Summary      设备动态认证
// @Description  实现一型一密认证机制，设备通过此接口获取凭证
// @Tags         设备认证
// @Accept       json
// @Produce      json
// @Param        request body model.DeviceAuthReq true "认证请求参数"
// @Success      200 {object} model.DeviceAuthRes "成功"
// @Failure      400 {object} errcode.Error "错误响应"
// @Router       /api/v1/device/auth [post]
// @example request - "请求片段" {"template_secret":"first_device_config_secret", "device_number":"first-device-001", "device_name":"首台温湿度传感器", "product_key":"aetherlink_temp_hum"}
func (*DeviceAuthApi) DeviceAuth(c *gin.Context) {
	var req model.DeviceAuthReq
	if !BindAndValidate(c, &req) {
		return
	}

	// 调用服务层进行设备认证
	resp, err := service.GroupApp.DeviceAuth.Auth(&req)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", resp)
}
