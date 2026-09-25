package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Customer struct{}

func (*Customer) Init(Router *gin.RouterGroup) {
	// 单体与列表路由
	customerGroup := Router.Group("customer")
	{
		customerGroup.POST("", api.Controllers.CustomerApi.SaveCustomer)
		customerGroup.GET("/:id", api.Controllers.CustomerApi.GetCustomer)
		customerGroup.DELETE("/:id", api.Controllers.CustomerApi.DeleteCustomer)
		customerGroup.POST("/:id/devices", api.Controllers.CustomerApi.AssignDevices)
		customerGroup.DELETE("/:id/device/:device_id", api.Controllers.CustomerApi.UnassignDevice)
		customerGroup.GET("/:id/devices", api.Controllers.CustomerApi.ListCustomerDevices)
	}

	customersGroup := Router.Group("customers")
	{
		customersGroup.GET("", api.Controllers.CustomerApi.ListCustomers)
	}
}
