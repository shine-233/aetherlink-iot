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
	Handle(c, func(req *model.CreateDataConverterReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DataConverter.CreateDataConverter(c.Request.Context(), req, claims)
	})
}

// UpdateDataConverter 更新数据转换器
// @Router   /api/v1/converters [put]
func (*DataConverterApi) UpdateDataConverter(c *gin.Context) {
	Handle(c, func(req *model.UpdateDataConverterReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DataConverter.UpdateDataConverter(c.Request.Context(), req, claims)
	})
}

// GetDataConverterByID 查询单个数据转换器
// @Router   /api/v1/converters/:id [get]
func (*DataConverterApi) GetDataConverterByID(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DataConverter.GetDataConverterByID(c.Request.Context(), id, claims)
	})
}

// DeleteDataConverter 删除数据转换器
// @Router   /api/v1/converters/:id [delete]
func (*DataConverterApi) DeleteDataConverter(c *gin.Context) {
	// 迁移形态：HandlePath（路径参数 id + claims）。成功包络保留旧的 data 对象 {"id": ...}，
	// 因此不能改用 HandlePathAction（其成功时 data 为 nil 并从包络中省略该字段）。
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		if err := service.GroupApp.DataConverter.DeleteDataConverter(c.Request.Context(), id, claims); err != nil {
			return nil, err
		}
		return map[string]interface{}{"id": id}, nil
	})
}

// ListDataConverters 分页查询列表
// @Router   /api/v1/converters [get]
func (*DataConverterApi) ListDataConverters(c *gin.Context) {
	Handle(c, func(req *model.GetDataConverterListReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DataConverter.ListDataConverters(c.Request.Context(), req, claims)
	})
}

// TestDataConverter 运行仿真测试
// @Router   /api/v1/converters/test [post]
func (*DataConverterApi) TestDataConverter(c *gin.Context) {
	Handle(c, func(req *model.TestDataConverterReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DataConverter.TestDataConverter(c.Request.Context(), req, claims)
	})
}
