// 文件用途：提供设备模型相关的 HTTP 接口处理器。
// 核心逻辑：绑定请求参数、读取上下文用户或路径信息，并调用 service 层完成业务处理后返回统一响应。
// 关键注意事项：该层应保持薄控制器职责，避免绕过鉴权、参数校验或错误码约定。
// 重构建议：若分支继续增加，优先抽取请求绑定、ID 解析和响应封装等通用辅助函数。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

// device_model.go 负责设备物模型相关 HTTP Handler。
// 核心职责：绑定物模型请求参数、读取 claims 注入租户与权限边界、调用 DeviceModel service 完成遥测/属性/事件/命令/自定义控制的增删改查。
// 主链路：前端模型编辑页或控制视图发起请求 -> BindAndValidate 绑定 DTO/路径参数 -> Handler 读取 claims 与补充 what/deviceId 等上下文 -> service.GroupApp.DeviceModel 处理 -> 写回统一响应。
// 使用注意：物模型是设备配置、自动化、控制视图与上行解析的重要上游契约，API 层应避免直接拼字段或在这里硬编码模型类别判断。
// 静态审查建议：
// 1. “通用物模型”“自定义命令”“自定义控制”已经在一个文件里共存，后续可按模型类别拆分，降低单文件体积。
// 2. Delete/Update 系列对 what、source、deviceId 等语义比较敏感，建议继续强化注释和 DTO 命名，避免前后端误用。
// 3. 目前多个 Handler 重复 claims 注入与统一响应写回，后续可提炼共用辅助层，但不要牺牲可读性。
type DeviceModelApi struct{}

// DeviceModelApi 是设备物模型控制层入口。
// 该结构体无状态，主要承担路由接收器职责，便于把请求生命周期信息留在 gin 上下文中处理。

// CreateDeviceModelTelemetry 创建设备遥测物模型。
// 参数绑定：请求体绑定 CreateDeviceModelReq。
// claims 注入：claims 决定当前用户是否可向目标物模型或设备写入遥测定义。
// 链路说明：遥测定义会影响设备详情展示、图表绑定与自动化数据源，API 层不应在这里自行补默认字段。
// /api/v1/device/model/telemetry
func (*DeviceModelApi) CreateDeviceModelTelemetry(c *gin.Context) {
	var req model.CreateDeviceModelReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceModel.CreateDeviceModelGeneral(req, model.DEVICE_MODEL_TELEMETRY, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// CreateDeviceModelAttributes 创建设备属性物模型。
// 参数绑定：请求体绑定 CreateDeviceModelReq。
// 链路说明：属性模型通常面向最新状态展示与读写控件配置，细粒度校验应继续由 service 层集中负责。
// /api/v1/device/model/attributes [post]
func (*DeviceModelApi) CreateDeviceModelAttributes(c *gin.Context) {
	var req model.CreateDeviceModelReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceModel.CreateDeviceModelGeneral(req, model.DEVICE_MODEL_ATTRIBUTES, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// CreateDeviceModelEvents 创建设备事件物模型。
// 参数绑定：请求体绑定 CreateDeviceModelV2Req，说明事件模型与历史通用模型 DTO 已发生分化。
// 静态审查建议：V1/V2 DTO 并存意味着物模型协议正在演进，后续可在 README 或 DTO 注释中进一步明确差异边界。
func (*DeviceModelApi) CreateDeviceModelEvents(c *gin.Context) {
	var req model.CreateDeviceModelV2Req
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceModel.CreateDeviceModelGeneralV2(req, model.DEVICE_MODEL_EVENTS, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// CreateDeviceModelCommands 创建设备命令物模型。
// 参数绑定：请求体绑定 CreateDeviceModelV2Req。
// 链路说明：命令模型会影响设备控制、下发菜单与自动化动作选择，API 层应避免在这里额外拼接控制项。
func (*DeviceModelApi) CreateDeviceModelCommands(c *gin.Context) {
	var req model.CreateDeviceModelV2Req
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceModel.CreateDeviceModelGeneralV2(req, model.DEVICE_MODEL_COMMANDS, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DeleteDeviceModelGeneral 删除通用物模型项。
// 参数绑定：模型 ID 来自路径参数，模型类别通过查询参数 what 指定。
// claims 注入：claims 控制删除权限和租户隔离。
// 静态审查建议：what 使用字符串分发，后续可考虑收敛成更显式的枚举或常量，降低拼写错误带来的运行时问题。
func (*DeviceModelApi) DeleteDeviceModelGeneral(c *gin.Context) {
	id := c.Param("id")
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	var what string

	// 通过URI判断来自哪个接口
	uri := c.Request.RequestURI
	if strings.Contains(uri, "telemetry") {
		what = model.DEVICE_MODEL_TELEMETRY
	} else if strings.Contains(uri, "attributes") {
		what = model.DEVICE_MODEL_ATTRIBUTES
	} else if strings.Contains(uri, "events") {
		what = model.DEVICE_MODEL_EVENTS
	} else if strings.Contains(uri, "commands") {
		what = model.DEVICE_MODEL_COMMANDS
	} else {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"param_err": "url param is not a valid JSON",
		}))
		return
	}
	err := service.GroupApp.DeviceModel.DeleteDeviceModelGeneral(id, what, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// UpdateDeviceModelGeneral 更新通用物模型项。
// 参数绑定：请求体绑定 UpdateDeviceModelReq。
// 链路说明：该入口更多面向历史通用模型更新，变更后会影响展示、控制与自动化条件匹配。
// /api/v1/device/model/telemetry  [put]
func (*DeviceModelApi) UpdateDeviceModelGeneral(c *gin.Context) {
	var req model.UpdateDeviceModelReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	var what string

	// 通过URI判断来自哪个接口
	uri := c.Request.RequestURI
	if strings.Contains(uri, "telemetry") {
		what = model.DEVICE_MODEL_TELEMETRY
	} else if strings.Contains(uri, "attributes") {
		what = model.DEVICE_MODEL_ATTRIBUTES
	} else {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"param_err": "url param is not a valid JSON",
		}))
		return
	}

	data, err := service.GroupApp.DeviceModel.UpdateDeviceModelGeneral(req, what, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// UpdateDeviceModelGeneralV2 更新新版物模型项。
// 参数绑定：请求体绑定 UpdateDeviceModelV2Req。
// 静态审查建议：V1/V2 同时存在时，建议持续记录各自适配的页面与设备协议来源，避免调用方误走旧接口。
func (*DeviceModelApi) UpdateDeviceModelGeneralV2(c *gin.Context) {
	var req model.UpdateDeviceModelV2Req
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	var what string

	// 通过URI判断来自哪个接口
	uri := c.Request.RequestURI

	if strings.Contains(uri, "events") {
		what = model.DEVICE_MODEL_EVENTS
	} else if strings.Contains(uri, "commands") {
		what = model.DEVICE_MODEL_COMMANDS
	} else {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"param_err": "url param is not a valid JSON",
		}))
		return
	}

	data, err := service.GroupApp.DeviceModel.UpdateDeviceModelGeneralV2(req, what, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleDeviceModelGeneral 分页查询通用物模型。
// 参数绑定：绑定 GetDeviceModelListByPageReq，通常包含设备、模板、类别与分页条件。
// claims 注入：通过 claims 控制当前用户可见模型范围。
// 链路说明：这是设备模型管理页和配置弹窗的核心读取入口，查询条件语义应保持与前端筛选字段一致。
func (*DeviceModelApi) HandleDeviceModelGeneral(c *gin.Context) {
	var req model.GetDeviceModelListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	var what string

	// 通过URI判断来自哪个接口
	uri := c.Request.RequestURI
	if strings.Contains(uri, "telemetry") {
		what = model.DEVICE_MODEL_TELEMETRY
	} else if strings.Contains(uri, "attributes") {
		what = model.DEVICE_MODEL_ATTRIBUTES
	} else if strings.Contains(uri, "events") {
		what = model.DEVICE_MODEL_EVENTS
	} else if strings.Contains(uri, "commands") {
		what = model.DEVICE_MODEL_COMMANDS
	} else {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"param_err": "url param is not a valid JSON",
		}))
		return
	}

	data, err := service.GroupApp.DeviceModel.GetDeviceModelListByPageGeneral(req, what, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleModelSourceAT 返回 AT 指令来源相关模型列表。
// 参数绑定：绑定 ParamID，用于指向目标设备或模板。
// 链路说明：该接口反映设备协议来源与物模型的耦合，适合作为接入层协议配置的辅助数据源。
// 静态审查建议：source/AT 一类概念较专业，建议继续保持注释与目录 README 同步，方便后续维护者理解。
// /api/v1/device/model/source/at/list
func (*DeviceModelApi) HandleModelSourceAT(c *gin.Context) {
	var param model.ParamID
	if !BindAndValidate(c, &param) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceModel.GetModelSourceAT(c, &param, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// CreateDeviceModelCustomCommands 创建自定义命令。
// 参数绑定：请求体绑定 CreateDeviceModelCustomCommandReq。
// 链路说明：自定义命令通常服务于设备详情页自定义控制区，不应在 API 层混入命令编排或权限拼装逻辑。
// /api/v1/device/model/custom/commands/
func (*DeviceModelApi) CreateDeviceModelCustomCommands(c *gin.Context) {
	var req model.CreateDeviceModelCustomCommandReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)

	err := service.GroupApp.DeviceModel.CreateDeviceModelCustomCommands(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func (*DeviceModelApi) DeleteDeviceModelCustomCommands(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceModel.DeleteDeviceModelCustomCommands(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func (*DeviceModelApi) UpdateDeviceModelCustomCommands(c *gin.Context) {
	var req model.UpdateDeviceModelCustomCommandReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceModel.UpdateDeviceModelCustomCommands(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func (*DeviceModelApi) HandleDeviceModelCustomCommandsByPage(c *gin.Context) {
	var req model.GetDeviceModelListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)

	data, err := service.GroupApp.DeviceModel.GetDeviceModelCustomCommandsByPage(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleDeviceModelCustomCommandsByDeviceId 按设备读取自定义命令。
// 参数绑定：deviceId 来自路径参数。
// claims 注入：claims 用于校验当前用户是否可读取该设备的控制命令定义。
// 静态审查建议：路径参数读取方式直观，但若同类接口持续增加，可继续收敛成统一的“按设备读取模型”辅助封装。
func (*DeviceModelApi) HandleDeviceModelCustomCommandsByDeviceId(c *gin.Context) {
	deviceId := c.Param("deviceId")
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceModel.GetDeviceModelCustomCommandsByDeviceId(deviceId, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// CreateDeviceModelCustomControl 创建自定义控制项。
// 参数绑定：请求体绑定 CreateDeviceModelCustomControlReq。
// 链路说明：自定义控制与命令并列但语义不同，通常更偏前端控制视图渲染定义，后续维护时不要与命令模型混淆。
func (*DeviceModelApi) CreateDeviceModelCustomControl(c *gin.Context) {
	var req model.CreateDeviceModelCustomControlReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)

	err := service.GroupApp.DeviceModel.CreateDeviceModelCustomControl(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

func (*DeviceModelApi) DeleteDeviceModelCustomControl(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceModel.DeleteDeviceModelCustomControl(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

func (*DeviceModelApi) UpdateDeviceModelCustomControl(c *gin.Context) {
	var req model.UpdateDeviceModelCustomControlReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceModel.UpdateDeviceModelCustomControl(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// HandleDeviceModelCustomControl 分页查询自定义控制项。
// 参数绑定：当前复用了 GetDeviceModelListByPageReq，说明控制项列表与通用物模型列表拥有相似筛选契约。
// 静态审查建议：如果自定义控制后续出现专属筛选字段，建议及时拆分 DTO，避免复用请求模型掩盖语义差异。
// /api/v1/device/model/custom/control GET
func (*DeviceModelApi) HandleDeviceModelCustomControl(c *gin.Context) {
	var req model.GetDeviceModelListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceModel.GetDeviceModelCustomControlByPage(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
