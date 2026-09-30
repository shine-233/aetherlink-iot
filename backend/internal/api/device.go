// 文件用途：提供设备相关的 HTTP 接口处理器。
// 核心逻辑：绑定请求参数、读取上下文用户或路径信息，并调用 service 层完成业务处理后返回统一响应。
// 关键注意事项：该层应保持薄控制器职责，避免绕过鉴权、参数校验或错误码约定。
// 重构建议：若分支继续增加，优先抽取请求绑定、ID 解析和响应封装等通用辅助函数。
package api

import (
	"aetherlink-iot/backend/internal/middleware"
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// device.go 负责设备域 HTTP Handler 的路由入口编排。
// 核心职责：完成请求参数绑定与校验、从 gin 上下文注入 claims、调用 service 层处理设备与物模型/分组/连接等能力，并把结果写回统一响应上下文。
// 主链路：前端请求 -> BindAndValidate 绑定 DTO/路径参数 -> 读取 claims 与语言环境 -> 调用 GroupApp.Device -> c.Set("data") 交给统一响应中间件输出。
// 使用注意：本层应保持薄控制器职责，不应在 Handler 内堆积领域判断；涉及租户、用户、项目边界的鉴权信息应优先通过 claims 向下游透传。
// 静态审查建议：
// 1. 物模型、设备分组、设备接入等子域已经汇聚在同一文件，后续可继续按业务边界拆分，降低设备 API 文件体积与定位成本。
// 2. 多数 Handler 重复执行“绑定参数 -> 取 claims -> 调 service -> 设置 data”，后续可抽取更稳定的辅助封装，减少遗漏统一错误处理的风险。
// 3. DeviceConnect 相关辅助函数已经开始下沉为小函数，适合作为后续整理连接类接口的样板，继续统一语言解析、claims 注入和响应封装。
type DeviceApi struct{}

// DeviceApi 负责设备域控制层入口。
// 它本身不持有状态，主要作为 Gin 路由挂载时的接收器，避免把请求生命周期信息残留在结构体字段中。

// CreateDevice 处理单个设备创建请求。
// 参数绑定：从请求体绑定 CreateDeviceReq。
// claims 注入：从上下文提取登录用户 claims，向 service 传递租户/项目级访问边界。
// 链路说明：该入口承接设备基础信息创建，真正的设备编号规则、物模型联动与副作用由 service 层统一收口。
// @Router   /api/v1/device [post]
func (*DeviceApi) CreateDevice(c *gin.Context) {
	Handle(c, func(req *model.CreateDeviceReq, userClaims *utils.UserClaims) (interface{}, error) {
		if req.ConflictPolicy == nil || *req.ConflictPolicy == "" {
			if q := c.Query("conflict_policy"); q != "" {
				req.ConflictPolicy = &q
			}
		}
		return service.GroupApp.Device.CreateDevice(*req, userClaims)
	})
}

// CreateDeviceBatch 处理服务接入场景的批量创建设备请求。
// 参数绑定：请求体绑定 BatchCreateDeviceReq。
// claims 注入：通过 claims 约束批量落库时的租户与用户边界。
// 静态审查建议：批量接口通常更容易出现部分成功/失败的追踪盲区，后续可在 service 层继续加强逐条结果与审计信息表达。
// /api/v1/device/service/access/batch [post]
func (*DeviceApi) CreateDeviceBatch(c *gin.Context) {
	Handle(c, func(req *model.BatchCreateDeviceReq, userClaims *utils.UserClaims) (interface{}, error) {
		if req.ConflictPolicy == nil || *req.ConflictPolicy == "" {
			if q := c.Query("conflict_policy"); q != "" {
				req.ConflictPolicy = &q
			}
		}
		return service.GroupApp.Device.CreateDeviceBatch(*req, userClaims)
	})
}

// DeleteDevice 删除设备
// @Router   /api/v1/device/{id} [delete]
func (*DeviceApi) DeleteDevice(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		return service.GroupApp.Device.DeleteDevice(id, userClaims)
	})
}

// UpdateDevice 更新设备
// @Router   /api/v1/device [put]
func (*DeviceApi) UpdateDevice(c *gin.Context) {
	Handle(c, func(req *model.UpdateDeviceReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.UpdateDevice(*req, userClaims)
	})
}

// ActiveDevice 激活设备
// @Router   /api/v1/device/active [put]
func (*DeviceApi) ActiveDevice(c *gin.Context) {
	Handle(c, func(req *model.ActiveDeviceReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.ActiveDevice(*req, userClaims)
	})
}

// HandleDeviceByID 返回单个设备详情。
// 参数绑定：设备 ID 来自路由参数。
// claims 注入：通过 claims 控制是否允许读取该设备所属租户/项目数据。
// 链路说明：该接口通常是设备详情页、编辑弹窗或二级关联页的数据入口，API 层不应额外拼装领域字段，避免与 service 返回结构漂移。
// @Router   /api/v1/device/detail/{id} [get]
func (*DeviceApi) HandleDeviceByID(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetDeviceByIDV1(id, userClaims)
	})
}

// HandleDeviceListByPage 返回设备分页列表。
// 参数绑定：通过查询参数或请求体绑定分页、筛选、排序条件。
// claims 注入：claims 决定当前用户可见的数据域，避免前端单靠筛选参数越权读取。
// 静态审查建议：列表检索条件较多时，推荐在 DTO 注释和 README 中同步维护查询语义，降低前后端对筛选字段理解不一致的风险。
// @Param all_tenants query bool false "仅 SYS_ADMIN 可显式查询全部租户设备"
// @Router   /api/v1/device [get]
func (*DeviceApi) HandleDeviceListByPage(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceListByPageReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetDeviceListByPage(req, userClaims)
	})
}

// @Tags     设备管理
// @Router   /api/v1/device/check/{deviceNumber} [get]
func (*DeviceApi) CheckDeviceNumber(c *gin.Context) {
	// 迁移注意：本接口没有错误分支——占用信息通过 data.message 透出，永远走成功信封。
	HandlePath(c, "deviceNumber", func(deviceNumber string, userClaims *utils.UserClaims) (interface{}, error) {
		checkErr, ok := service.GroupApp.Device.CheckDeviceNumber(deviceNumber, userClaims)
		data := map[string]interface{}{"is_available": ok}
		if checkErr != nil {
			data["message"] = checkErr.Error()
		}
		return data, nil
	})
}

// 移除子设备
// /api/v1/device/sub-remove
func (*DeviceApi) RemoveSubDevice(c *gin.Context) {
	HandleAction(c, func(req *model.RemoveSonDeviceReq, userClaims *utils.UserClaims) error {
		return service.GroupApp.Device.RemoveSubDevice(req.SubDeviceId, userClaims)
	})
}

// GetTenantDeviceList
// @AUTHOR:zxq
// @DATE: 2024-04-06 18:04
// @DESCRIPTIONS: 获得租户下设备列表
// /api/v1/device/tenant/list [get]
func (*DeviceApi) HandleTenantDeviceList(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceMenuReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetTenantDeviceList(req, userClaims)
	})
}

// GetDeviceList
// @AUTHOR:zxq
// @DATE: 2024-04-07 17:04
// @DESCRIPTIONS: 获得未绑定的设备列表（支持网关设备和子设备，可通过device_type参数过滤）
// /api/v1/device/list [get]
func (*DeviceApi) HandleDeviceList(c *gin.Context) {
	Handle(c, func(req *model.GetUnboundGatewaySubDeviceReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetDeviceList(c, userClaims, req)
	})
}

// CreateSonDevice
// @AUTHOR:zxq
// @DATE: 2024-04-07 17:04
// @DESCRIPTIONS: 添加子设备
// /api/v1/device/son/add
func (*DeviceApi) CreateSonDevice(c *gin.Context) {
	HandleAction(c, func(param *model.CreateSonDeviceRes, userClaims *utils.UserClaims) error {
		return service.GroupApp.Device.CreateSonDevice(c, param, userClaims)
	})
}

// DeviceConnectForm
// DeviceConnectForm 返回设备连接表单所需的凭证字段定义。
// 参数绑定：绑定 DeviceConnectFormReq，通常包含设备配置、协议或物模型上下文。
// claims 注入：当前实现未显式下传 claims，说明该接口更偏配置推导；若后续表单内容受租户能力影响，建议补齐 claims 透传。
// 静态审查建议：该接口与 DeviceConnect 共用同一业务链路，后续可把“表单定义”和“连接信息”相关 Handler 进一步聚合成独立文件。
// /api/v1/device/connect/form
func (*DeviceApi) DeviceConnectForm(c *gin.Context) {
	Handle(c, func(param *model.DeviceConnectFormReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.DeviceConnectForm(c, param, userClaims)
	})
}

// DeviceConnect
// DeviceConnect 返回设备连接说明或凭证信息。
// 参数绑定：调用 bindDeviceConnectParams 统一绑定 DeviceConnectFormReq。
// claims 注入：通过 deviceConnectClaims 读取上下文 claims，供 service 判断租户边界或可见性。
// 链路说明：该入口把语言解析、claims 注入和响应写回拆成了小函数，是当前设备 API 中较清晰的“薄控制器”样式。
// 静态审查建议：若未来连接协议继续增加，优先扩展 service 内部协议分发，而不是在 Handler 层添加分支。
// /api/v1/device/connect/info
func (*DeviceApi) DeviceConnect(c *gin.Context) {
	// 获取语言设置
	lang := deviceConnectLanguage(c)
	Handle(c, func(param *model.DeviceConnectFormReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.DeviceConnect(c, param, lang, userClaims)
	})
}

// deviceConnectLanguage 解析当前请求语言。
// 语言信息会继续传给 service，用于返回多语言凭证说明或安装指引文案。
func deviceConnectLanguage(c *gin.Context) string {
	lang := c.Request.Header.Get("Accept-Language")
	if lang == "" {
		return "zh_CN"
	}
	return lang
}

// UpdateDeviceVoucher
// @AUTHOR:zxq
// @DATE: 2024-04-15 16:04
// @DESCRIPTIONS: 更新
// /api/v1/device/update/voucher [post]
func (*DeviceApi) UpdateDeviceVoucher(c *gin.Context) {
	Handle(c, func(param *model.UpdateDeviceVoucherReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.UpdateDeviceVoucher(c, param, userClaims)
	})
}

// GetSubList
// @AUTHOR:wzc
// @DATE: 2024-03-15 16:04
// @DESCRIPTIONS: 更新
// /api/v1/device/sub-list/{id}
func (*DeviceApi) HandleSubList(c *gin.Context) {
	HandlePathQuery(c, "id", func(parant_id string, req *model.PageReq, userClaims *utils.UserClaims) (interface{}, error) {
		if parant_id == "" {
			return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"msg": "no parant_id",
			})
		}
		list, total, err := service.GroupApp.Device.GetSubList(c, parant_id, int64(req.Page), int64(req.PageSize), userClaims)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"total": total,
			"list":  list,
		}, nil
	})
}

// /api/v1/device/metrics/{id}
func (*DeviceApi) HandleMetrics(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetMetrics(id, userClaims)
	})
}

// HandleActionByDeviceID 返回单设备动作下拉菜单。
// 参数绑定：绑定设备 ID 及可能的物模型/配置上下文。
// 链路说明：该接口通常服务于自动化编排或动作选择弹窗，真正的动作过滤规则由 service 层依据设备模型和权限决定。
// /api/v1/device/metrics/menu [get]
func (*DeviceApi) HandleActionByDeviceID(c *gin.Context) {
	Handle(c, func(param *model.GetActionByDeviceIDReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetActionByDeviceID(param.DeviceID, userClaims)
	})
}

// HandleConditionByDeviceID 返回单设备条件下拉菜单。
// 参数绑定：当前复用了 GetActionByDeviceIDReq，说明动作与条件筛选参数契约相近。
// 静态审查建议：若后续动作与条件查询参数发生分化，建议拆出独立 DTO，避免复用请求体掩盖语义差异。
// /api/v1/device/metrics/condition/menu [get]
func (*DeviceApi) HandleConditionByDeviceID(c *gin.Context) {
	Handle(c, func(param *model.GetActionByDeviceIDReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetConditionByDeviceID(param.DeviceID, userClaims)
	})
}

// /api/v1/device/map/telemetry/{id}
func (*DeviceApi) HandleMapTelemetry(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetMapTelemetry(id, userClaims)
	})
}

// 有物模型且有图表配置的设备下拉列表
// /api/v1/device/template/chart/select
func (*DeviceApi) HandleDeviceTemplateChartSelect(c *gin.Context) {
	HandleNoBody(c, func(userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetDeviceTemplateChartSelect(userClaims)
	})
}

// UpdateDeviceConfig 更换设备配置。
// 参数绑定：请求体绑定 ChangeDeviceConfigReq。
// claims 注入：通过 claims 控制谁可以触发设备配置切换、物模型解绑或协议变更。
// 链路说明：API 层只做参数入口，配置变更后的缓存清理、协议副作用与凭证联动由 service 层统一完成。
// 静态审查建议：这是高副作用接口，后续适合补充更明确的审计记录与变更前后快照说明。
// /api/v1/device/update/config [put]
func (*DeviceApi) UpdateDeviceConfig(c *gin.Context) {
	HandleAction(c, func(param *model.ChangeDeviceConfigReq, userClaims *utils.UserClaims) error {
		return service.GroupApp.Device.UpdateDeviceConfig(param, userClaims)
	})
}

// HandleDeviceOnlineStatus 返回设备在线状态。
// 参数绑定：设备 ID 来自路径参数。
// claims 注入：claims 用于约束可见设备范围，避免通过枚举 ID 探测其他租户在线状态。
// 链路说明：在线状态通常由上行、缓存或状态服务汇总，API 层不应自行推断在线逻辑。
// /api/v1/device/online/status/{id} [get]
func (*DeviceApi) HandleDeviceOnlineStatus(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetDeviceOnlineStatus(id, userClaims)
	})
}

func (*DeviceApi) GatewayRegister(c *gin.Context) {
	var req model.GatewayRegisterReq
	if !BindAndValidate(c, &req) {
		return
	}
	if !middleware.OpenAPIKeyAuth(c) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Device.GatewayRegister(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

func (*DeviceApi) GatewaySubRegister(c *gin.Context) {
	var req model.DeviceRegisterReq
	if !BindAndValidate(c, &req) {
		logrus.Warning("GatewaySubRegister validation failed")
		return
	}
	logrus.Info("GatewaySubRegister request received")
	if !middleware.OpenAPIKeyAuth(c) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Device.GatewayDeviceRegister(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// 设备单指标图表数据查询
// /api/v1/device/metrics/chart [get]
func (*DeviceApi) HandleDeviceMetricsChart(c *gin.Context) {
	Handle(c, func(param *model.GetDeviceMetricsChartReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetDeviceMetricsChart(param, userClaims)
	})
}

// 设备选择器
// /api/v1/device/selector [get]
func (*DeviceApi) HandleDeviceSelector(c *gin.Context) {
	Handle(c, func(req *model.DeviceSelectorReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetDeviceSelector(*req, userClaims)
	})
}

// 获取租户下最近上报数据的三个设备的遥测数据
// /api/v1/device/telemetry/latest [get]
func (*DeviceApi) HandleTenantTelemetryData(c *gin.Context) {
	HandleNoBody(c, func(userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetTenantTelemetryData(userClaims)
	})
}

// GetDeviceStatusHistory 获取设备状态历史记录
// @Router   /api/v1/device/status/history [get]
func (*DeviceApi) GetDeviceStatusHistory(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceStatusHistoryReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Device.GetDeviceStatusHistory(req, userClaims)
	})
}
