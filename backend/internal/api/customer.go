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
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errors.New("客户ID不能为空")
		}

		tenantID := claims.TenantID
		if claims.Authority == "SYS_ADMIN" {
			tenantID = "" // 超管可跨租户查
		}

		res, err := service.GroupApp.Customer.GetCustomer(c.Request.Context(), id, tenantID)
		if err != nil {
			return nil, errors.New("未找到该客户或无权访问")
		}
		return res, nil
	})
}

// DeleteCustomer 删除指定客户
// @Router   /api/v1/customer/:id [delete]
func (*CustomerApi) DeleteCustomer(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errors.New("客户ID不能为空")
		}

		if err := service.GroupApp.Customer.DeleteCustomer(c.Request.Context(), id, claims.TenantID); err != nil {
			return nil, err
		}
		// 迁移前 c.Set("data", "ok")：必须保留 "ok" 而非改用 respondAction（其会置 data 为 nil 并从包络中省略字段）。
		return "ok", nil
	})
}

// ListCustomers 分页获取客户列表
// @Router   /api/v1/customers [get]
func (*CustomerApi) ListCustomers(c *gin.Context) {
	HandleNoBody(c, func(claims *utils.UserClaims) (interface{}, error) {
		search := c.Query("search")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

		tenantID := claims.TenantID
		if claims.Authority == "SYS_ADMIN" && c.Query("tenant_id") != "" {
			tenantID = c.Query("tenant_id")
		}

		list, total, err := service.GroupApp.Customer.ListCustomers(c.Request.Context(), tenantID, search, page, pageSize)
		if err != nil {
			return nil, err
		}

		return gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		}, nil
	})
}

// AssignDevices 分配设备到客户
// @Router   /api/v1/customer/:id/devices [post]
// 编排约束：迁移前先取 claims、再校验路径参数、最后绑定请求体；HandlePathBody 会把绑定提到 claims 之前，
// 为保持编排顺序逐字一致这里保持手工编排，仅复用 RequireClaims / BindAndValidate / respond 出口。
func (*CustomerApi) AssignDevices(c *gin.Context) {
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.Error(errors.New("客户ID不能为空"))
		return
	}

	var req model.CustomerAssignDevicesReq
	if !BindAndValidate(c, &req) {
		return
	}

	// 迁移前成功时 c.Set("data", "ok")，故走 respond 而非 respondAction。
	respond(c, "ok", service.GroupApp.Customer.AssignDevices(c.Request.Context(), id, claims.TenantID, req.DeviceIDs))
}

// UnassignDevice 从客户解除绑定设备
// @Router   /api/v1/customer/:id/device/:device_id [delete]
func (*CustomerApi) UnassignDevice(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		deviceID := c.Param("device_id")
		if id == "" || deviceID == "" {
			return nil, errors.New("客户ID和设备ID不能为空")
		}

		if err := service.GroupApp.Customer.UnassignDevice(c.Request.Context(), id, claims.TenantID, deviceID); err != nil {
			return nil, err
		}
		// 迁移前 c.Set("data", "ok")：必须保留 "ok" 而非改用 respondAction（其会置 data 为 nil 并从包络中省略字段）。
		return "ok", nil
	})
}

// ListCustomerDevices 获取客户分配的设备列表
// @Router   /api/v1/customer/:id/devices [get]
func (*CustomerApi) ListCustomerDevices(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errors.New("客户ID不能为空")
		}

		deviceIDs, err := service.GroupApp.Customer.ListCustomerDevices(c.Request.Context(), id, claims.TenantID)
		if err != nil {
			return nil, err
		}
		return gin.H{
			"device_ids": deviceIDs,
		}, nil
	})
}
