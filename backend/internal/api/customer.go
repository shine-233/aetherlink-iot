package api

import (
	"errors"
	"strconv"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type CustomerApi struct{}

// SaveCustomer 创建或更新客户
// @Router   /api/v1/customer [post]
func (*CustomerApi) SaveCustomer(c *gin.Context) {
	Handle(c, func(req *model.CustomerReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Customer.SaveCustomer(c.Request.Context(), claims.TenantID, req)
	})
}

// GetCustomer 获取指定客户详情
// @Router   /api/v1/customer/:id [get]
func (*CustomerApi) GetCustomer(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	id := c.Param("id")
	if id == "" {
		c.Error(errors.New("客户ID不能为空"))
		return
	}

	tenantID := claims.TenantID
	if claims.Authority == "SYS_ADMIN" {
		tenantID = "" // 超管可跨租户查
	}

	res, err := service.GroupApp.Customer.GetCustomer(c.Request.Context(), id, tenantID)
	if err != nil {
		c.Error(errors.New("未找到该客户或无权访问"))
		return
	}
	c.Set("data", res)
}

// DeleteCustomer 删除指定客户
// @Router   /api/v1/customer/:id [delete]
func (*CustomerApi) DeleteCustomer(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	id := c.Param("id")
	if id == "" {
		c.Error(errors.New("客户ID不能为空"))
		return
	}

	if err := service.GroupApp.Customer.DeleteCustomer(c.Request.Context(), id, claims.TenantID); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", "ok")
}

// ListCustomers 分页获取客户列表
// @Router   /api/v1/customers [get]
func (*CustomerApi) ListCustomers(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	tenantID := claims.TenantID
	if claims.Authority == "SYS_ADMIN" && c.Query("tenant_id") != "" {
		tenantID = c.Query("tenant_id")
	}

	list, total, err := service.GroupApp.Customer.ListCustomers(c.Request.Context(), tenantID, search, page, pageSize)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// AssignDevices 分配设备到客户
// @Router   /api/v1/customer/:id/devices [post]
func (*CustomerApi) AssignDevices(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	id := c.Param("id")
	if id == "" {
		c.Error(errors.New("客户ID不能为空"))
		return
	}

	var req model.CustomerAssignDevicesReq
	if !BindAndValidate(c, &req) {
		return
	}

	if err := service.GroupApp.Customer.AssignDevices(c.Request.Context(), id, claims.TenantID, req.DeviceIDs); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", "ok")
}

// UnassignDevice 从客户解除绑定设备
// @Router   /api/v1/customer/:id/device/:device_id [delete]
func (*CustomerApi) UnassignDevice(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	id := c.Param("id")
	deviceID := c.Param("device_id")
	if id == "" || deviceID == "" {
		c.Error(errors.New("客户ID和设备ID不能为空"))
		return
	}

	if err := service.GroupApp.Customer.UnassignDevice(c.Request.Context(), id, claims.TenantID, deviceID); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", "ok")
}

// ListCustomerDevices 获取客户分配的设备列表
// @Router   /api/v1/customer/:id/devices [get]
func (*CustomerApi) ListCustomerDevices(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	id := c.Param("id")
	if id == "" {
		c.Error(errors.New("客户ID不能为空"))
		return
	}

	deviceIDs, err := service.GroupApp.Customer.ListCustomerDevices(c.Request.Context(), id, claims.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{
		"device_ids": deviceIDs,
	})
}
