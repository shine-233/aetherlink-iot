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
// 绑定/校验/claims/响应等样板统一收敛到 handler_adapter.go 的适配器（Handle/HandlePath/HandleNoBody/HandlePublic），
// JSON 包络逐字节不变（见 handler_adapter_golden_4_test.go 的新旧对比测试）。

// CreateDeviceTemplate 创建物模型。
// 参数绑定：请求体绑定 CreateDeviceTemplateReq。
// 链路说明：物模型是设备配置与图表选择等能力的上游定义，API 层仅负责入参收口，模型内部校验与级联副作用应由 service 统一处理。
// @Router   /api/v1/device/template [post]
func (*DeviceApi) CreateDeviceTemplate(c *gin.Context) {
	Handle(c, func(req *model.CreateDeviceTemplateReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTemplate.CreateDeviceTemplate(*req, userClaims)
	})
}

// UpdateDeviceTemplate 更新设备物模型
// @Router   /api/v1/device/template [put]
func (*DeviceApi) UpdateDeviceTemplate(c *gin.Context) {
	Handle(c, func(req *model.UpdateDeviceTemplateReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTemplate.UpdateDeviceTemplate(*req, userClaims)
	})
}

// GetDeviceTemplateListByPage 分页获取设备物模型
// @Router   /api/v1/device/template [get]
func (*DeviceApi) HandleDeviceTemplateListByPage(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceTemplateListByPageReq, userClaims *utils.UserClaims) (interface{}, error) {
		data, err := service.GroupApp.DeviceTemplate.GetDeviceTemplateListByPage(*req, userClaims)
		if err != nil {
			return nil, err
		}
		serilizedData, err := utils.SerializeData(data, GetDeviceTemplateListData{})
		if err != nil {
			return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
				"error": err.Error(),
			})
		}
		return serilizedData, nil
	})
}

// @Router   /api/v1/device/template/menu [get]
func (*DeviceApi) HandleDeviceTemplateMenu(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceTemplateMenuReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTemplate.GetDeviceTemplateMenu(*req, userClaims)
	})
}

// HandleDeviceTemplateStats 获取设备物模型统计信息
// @Router   /api/v1/device/template/stats [get]
func (*DeviceApi) HandleDeviceTemplateStats(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceTemplateStatsReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTemplate.GetDeviceTemplateStats(*req, userClaims)
	})
}

// HandleDeviceTemplateSelector 获取设备物模型选择器
// @Router   /api/v1/device/template/selector [get]
func (*DeviceApi) HandleDeviceTemplateSelector(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceTemplateSelectorReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTemplate.GetDeviceTemplateSelector(*req, userClaims)
	})
}

// DeleteDeviceTemplate 删除设备物模型
// @Router   /api/v1/device/template/{id} [delete]
func (*DeviceApi) DeleteDeviceTemplate(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		return service.GroupApp.DeviceTemplate.DeleteDeviceTemplate(id, userClaims)
	})
}

// GetDeviceTemplate 获取设备物模型详情
// @Router   /api/v1/device/template/detail/{id} [get]
func (*DeviceApi) HandleDeviceTemplateById(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		data, err := service.GroupApp.DeviceTemplate.GetDeviceTemplateById(id, userClaims)
		if err != nil {
			return nil, err
		}
		serilizedData, err := utils.SerializeData(data, DeviceTemplateReadSchema{})
		if err != nil {
			return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
				"error": err.Error(),
			})
		}
		return serilizedData, nil
	})
}

// 根据设备id获取物模型详情
// @Router   /api/v1/device/template/chart [get]
func (*DeviceApi) HandleDeviceTemplateByDeviceId(c *gin.Context) {
	HandleNoBody(c, func(userClaims *utils.UserClaims) (interface{}, error) {
		deviceId := c.Query("device_id")
		if deviceId == "" {
			return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"device_id": deviceId,
				"msg":       "device_id is required",
			})
		}
		data, err := service.GroupApp.DeviceTemplate.GetDeviceTemplateByDeviceId(deviceId, userClaims)
		if err != nil {
			return nil, err
		}
		return data, nil
	})
}

// MarketLogin 登录市场获取 Token
// @Router   /api/v1/device/template/market/login [post]
func (*DeviceApi) MarketLogin(c *gin.Context) {
	HandlePublic(c, func(req *model.MarketLoginReq) (interface{}, error) {
		client := service.NewMarketClient()
		token, err := client.Login(c, req.Username, req.Password)
		if err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
		}
		return map[string]string{
			"token": token,
		}, nil
	})
}

// PublishToMarket 发布物模型到市场
// @Router   /api/v1/device/template/market/publish [post]
func (*DeviceApi) PublishToMarket(c *gin.Context) {
	Handle(c, func(req *model.PublishToMarketReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTemplate.PublishToMarket(*req, userClaims)
	})
}

// ListMarketTemplates 获取市场物模型列表
// @Router   /api/v1/device/template/market/list [get]
func (*DeviceApi) ListMarketTemplates(c *gin.Context) {
	HandlePublic(c, func(req *model.MarketTemplateListReq) (interface{}, error) {
		params := normalizeMarketTemplateListParams(*req)
		return listMarketTemplates(c, params)
	})
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
	HandlePath(c, "market_id", func(marketID string, _ *utils.UserClaims) (interface{}, error) {
		if marketID == "" {
			return nil, errcode.WithData(errcode.CodeParamError, "market_id is required")
		}

		client := service.NewMarketClient()
		data, err := client.GetMarketTemplateDetail(c, marketID)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
				"error": "Failed to get market thing model detail: " + err.Error(),
			})
		}
		return data, nil
	})
}

// InstallFromMarket 从市场安装物模型
// @Router   /api/v1/device/template/market/install [post]
func (*DeviceApi) InstallFromMarket(c *gin.Context) {
	Handle(c, func(req *model.InstallFromMarketReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTemplate.InstallFromMarket(*req, userClaims)
	})
}

// ExportDeviceTemplate 模板市场导出：按读权限产出可移植模板描述符（JSON 载荷）。
// @Router   /api/v1/device/template/export/{id} [get]
func (*DeviceApi) ExportDeviceTemplate(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTemplate.ExportDeviceTemplate(id, userClaims)
	})
}

// ImportDeviceTemplate 模板市场导入：导出载荷原样回传，创建为调用者租户下的新模板。
// 幂等：同租户同名同版本返回既有模板（data.created=false）。
// @Router   /api/v1/device/template/import [post]
func (*DeviceApi) ImportDeviceTemplate(c *gin.Context) {
	Handle(c, func(req *model.ImportDeviceTemplateReq, userClaims *utils.UserClaims) (interface{}, error) {
		data, created, err := service.GroupApp.DeviceTemplate.ImportDeviceTemplate(*req, userClaims)
		if err != nil {
			// 失败同样审计：只记成功的审计回答不了"这个模板为什么没导进来"。
			service.EmitMarketTemplateImportAudit(service.BuildMarketTemplateImportAudit(
				userClaims.TenantID, userClaims.ID, "", "", false, err))
			return nil, err
		}
		// created=false 是幂等命中（同名同版本已存在），不是"没发生"，同样留痕。
		service.EmitMarketTemplateImportAudit(service.BuildMarketTemplateImportAudit(
			userClaims.TenantID, userClaims.ID, "", "", created, nil))
		return gin.H{
			"template": data,
			"created":  created,
		}, nil
	})
}
