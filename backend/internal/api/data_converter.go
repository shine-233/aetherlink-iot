// 文件用途：提供数据转换器（ThingsBoard Data Converter）HTTP 处理器。
// 核心逻辑：入参绑定、鉴权边界注入、Service 分发与统一 API 响应包装。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DataConverterApi struct{}

// CreateDataConverter 创建数据转换器
// @Router   /api/v1/converters [post]
func (*DataConverterApi) CreateDataConverter(c *gin.Context) {
	var req model.CreateDataConverterReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DataConverter.CreateDataConverter(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// UpdateDataConverter 更新数据转换器
// @Router   /api/v1/converters [put]
func (*DataConverterApi) UpdateDataConverter(c *gin.Context) {
	var req model.UpdateDataConverterReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DataConverter.UpdateDataConverter(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetDataConverterByID 查询单个数据转换器
// @Router   /api/v1/converters/:id [get]
func (*DataConverterApi) GetDataConverterByID(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DataConverter.GetDataConverterByID(c.Request.Context(), id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DeleteDataConverter 删除数据转换器
// @Router   /api/v1/converters/:id [delete]
func (*DataConverterApi) DeleteDataConverter(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DataConverter.DeleteDataConverter(c.Request.Context(), id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"id": id})
}

// ListDataConverters 分页查询列表
// @Router   /api/v1/converters [get]
func (*DataConverterApi) ListDataConverters(c *gin.Context) {
	var req model.GetDataConverterListReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DataConverter.ListDataConverters(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// TestDataConverter 运行仿真测试
// @Router   /api/v1/converters/test [post]
func (*DataConverterApi) TestDataConverter(c *gin.Context) {
	var req model.TestDataConverterReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DataConverter.TestDataConverter(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
