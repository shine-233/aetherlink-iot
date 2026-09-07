package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// DeviceCertificate 接入安全 X.509 路由组（ROADMAP D5）。
type DeviceCertificate struct{}

func (*DeviceCertificate) InitDeviceCertificate(Router *gin.RouterGroup) {
	g := Router.Group("device-certificates")
	{
		g.POST("issue", api.Controllers.DeviceCertificateApi.Issue)
		g.POST("verify", api.Controllers.DeviceCertificateApi.Verify)
		g.GET("", api.Controllers.DeviceCertificateApi.List)
		g.GET(":id", api.Controllers.DeviceCertificateApi.Get)
		g.POST(":id/revoke", api.Controllers.DeviceCertificateApi.Revoke)
		g.POST(":id/renew", api.Controllers.DeviceCertificateApi.Renew)
	}
}
