package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// device_templates.go 承接物模型与物模型市场相关 HTTP Handler。
// 这里保持同一个 DeviceApi receiver，不改变路由挂载与行为，只把物模型子域从 device.go 中抽离出来，提升定位与维护的局部性。

// CreateDeviceTemplate 创建物模型。
// 参数绑定：请求体绑定 CreateDeviceTemplateReq。
// 链路说明：物模型是设备配置与图表选择等能力的上游定义，API 层仅负责入参收口，模型内部校验与级联副作用应由 service 统一处理。
// @Router   /api/v1/device/template [post]
func (*DeviceApi) CreateDeviceTemplate(c *gin.Context) {
	var req model.CreateDeviceTemplateReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceTemplate.CreateDeviceTemplate(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// UpdateDeviceTemplate 更新设备物模型
// @Router   /api/v1/device/template [put]
func (*DeviceApi) UpdateDeviceTemplate(c *gin.Context) {
	var req model.UpdateDeviceTemplateReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceTemplate.UpdateDeviceTemplate(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// GetDeviceTemplateListByPage 分页获取设备物模型
// @Router   /api/v1/device/template [get]
func (*DeviceApi) HandleDeviceTemplateListByPage(c *gin.Context) {
	var req model.GetDeviceTemplateListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceTemplate.GetDeviceTemplateListByPage(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	serilizedData, err := utils.SerializeData(data, GetDeviceTemplateListData{})
	if err != nil {
		c.Error(errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	c.Set("data", serilizedData)
}

// @Router   /api/v1/device/template/menu [get]
func (*DeviceApi) HandleDeviceTemplateMenu(c *gin.Context) {
	var req model.GetDeviceTemplateMenuReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceTemplate.GetDeviceTemplateMenu(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleDeviceTemplateStats 获取设备物模型统计信息
// @Router   /api/v1/device/template/stats [get]
func (*DeviceApi) HandleDeviceTemplateStats(c *gin.Context) {
	var req model.GetDeviceTemplateStatsReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceTemplate.GetDeviceTemplateStats(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleDeviceTemplateSelector 获取设备物模型选择器
// @Router   /api/v1/device/template/selector [get]
func (*DeviceApi) HandleDeviceTemplateSelector(c *gin.Context) {
	var req model.GetDeviceTemplateSelectorReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceTemplate.GetDeviceTemplateSelector(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// DeleteDeviceTemplate 删除设备物模型
// @Router   /api/v1/device/template/{id} [delete]
func (*DeviceApi) DeleteDeviceTemplate(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceTemplate.DeleteDeviceTemplate(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// GetDeviceTemplate 获取设备物模型详情
// @Router   /api/v1/device/template/detail/{id} [get]
func (*DeviceApi) HandleDeviceTemplateById(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceTemplate.GetDeviceTemplateById(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	serilizedData, err := utils.SerializeData(data, DeviceTemplateReadSchema{})
	if err != nil {
		c.Error(errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}
	c.Set("data", serilizedData)
}

// 根据设备id获取物模型详情
// @Router   /api/v1/device/template/chart [get]
func (*DeviceApi) HandleDeviceTemplateByDeviceId(c *gin.Context) {
	deviceId := c.Query("device_id")
	if deviceId == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"device_id": deviceId,
			"msg":       "device_id is required",
		}))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceTemplate.GetDeviceTemplateByDeviceId(deviceId, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// MarketLogin 登录市场获取 Token
// @Router   /api/v1/device/template/market/login [post]
func (*DeviceApi) MarketLogin(c *gin.Context) {
	var req model.MarketLoginReq
	if !BindAndValidate(c, &req) {
		return
	}

	client := service.NewMarketClient()
	token, err := client.Login(c, req.Username, req.Password)
	if err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeSystemError, err.Error()))
		return
	}

	c.Set("data", map[string]string{
		"token": token,
	})
}

// PublishToMarket 发布物模型到市场
// @Router   /api/v1/device/template/market/publish [post]
func (*DeviceApi) PublishToMarket(c *gin.Context) {
	var req model.PublishToMarketReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	apiResp, err := service.GroupApp.DeviceTemplate.PublishToMarket(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", apiResp)
}

// ListMarketTemplates 获取市场物模型列表
// @Router   /api/v1/device/template/market/list [get]
func (*DeviceApi) ListMarketTemplates(c *gin.Context) {
	var req model.MarketTemplateListReq
	if !BindAndValidate(c, &req) {
		return
	}

	params := normalizeMarketTemplateListParams(req)
	data, err := listMarketTemplates(c, params)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

type marketTemplateListParams struct {
	keyword  string
	category string
	sortBy   string
	page     int
	pageSize int
}

func normalizeMarketTemplateListParams(req model.MarketTemplateListReq) marketTemplateListParams {
	params := marketTemplateListParams{
		page:     req.Page,
		pageSize: req.PageSize,
	}
	if req.Keyword != nil {
		params.keyword = *req.Keyword
	}
	if req.Category != nil {
		params.category = *req.Category
	}
	if req.SortBy != nil {
		params.sortBy = *req.SortBy
	}
	if params.page <= 0 {
		params.page = 1
	}
	if params.pageSize <= 0 {
		params.pageSize = 20
	}
	return params
}

func listMarketTemplates(c *gin.Context, params marketTemplateListParams) (interface{}, error) {
	client := service.NewMarketClient()
	data, err := client.ListMarketTemplates(c, params.keyword, params.category, params.sortBy, params.page, params.pageSize)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "Failed to list market thing models: " + err.Error(),
		})
	}
	return data, nil
}

// GetMarketTemplateDetail 获取市场物模型详情
// @Router   /api/v1/device/template/market/detail/:market_id [get]
func (*DeviceApi) GetMarketTemplateDetail(c *gin.Context) {
	marketID := c.Param("market_id")
	if marketID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "market_id is required"))
		return
	}

	client := service.NewMarketClient()
	data, err := client.GetMarketTemplateDetail(c, marketID)
	if err != nil {
		c.Error(errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "Failed to get market thing model detail: " + err.Error(),
		}))
		return
	}
	c.Set("data", data)
}

// InstallFromMarket 从市场安装物模型
// @Router   /api/v1/device/template/market/install [post]
func (*DeviceApi) InstallFromMarket(c *gin.Context) {
	var req model.InstallFromMarketReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	data, err := service.GroupApp.DeviceTemplate.InstallFromMarket(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ExportDeviceTemplate 模板市场导出：按读权限产出可移植模板描述符（JSON 载荷）。
// @Router   /api/v1/device/template/export/{id} [get]
func (*DeviceApi) ExportDeviceTemplate(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceTemplate.ExportDeviceTemplate(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ImportDeviceTemplate 模板市场导入：导出载荷原样回传，创建为调用者租户下的新模板。
// 幂等：同租户同名同版本返回既有模板（data.created=false）。
// @Router   /api/v1/device/template/import [post]
func (*DeviceApi) ImportDeviceTemplate(c *gin.Context) {
	var req model.ImportDeviceTemplateReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, created, err := service.GroupApp.DeviceTemplate.ImportDeviceTemplate(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{
		"template": data,
		"created":  created,
	})
}
