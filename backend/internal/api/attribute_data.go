// attribute_data.go 负责设备属性数据域的 HTTP 入口编排。
// 核心职责：
// 1. 为设备详情页属性视图提供“当前属性快照”“按 key 精确查询”“属性下发日志”三类读接口。
// 2. 为前端或运维入口提供属性写入与属性主动读取两类命令型接口。
// 3. 在 API 层完成参数绑定、claims 提取与统一错误出口，再把租户、设备和权限边界交给 service 层落地。
// 上下游关系：
// 1. 上游通常来自设备详情、调试视图或属性历史回溯页面，请求会携带设备 ID、属性 key 或分页筛选条件。
// 2. 下游依赖 service.GroupApp.AttributeData 统一处理属性快照查询、下发日志检索、MQTT/协议层消息投递等细节。
// 3. 该层不负责属性值格式转换、设备在线校验、消息投递重试或下发审计，只负责入口收口。
// 静态审查建议：
// 1. 当前文件混合“属性读模型”和“属性命令模型”，若接口继续增加，可按查询型/下发型 handler 拆分，降低阅读成本。
// 2. `c.MustGet("claims")` 强依赖鉴权中间件先行注入，若后续开放内部调用链路，建议补统一守卫或显式断言错误。
// 3. `AttributePutMessage` 把 `constant.Manual` 以字符串透传给 service，若后续来源枚举扩展，需同步收口为更稳定的类型约束。
// 4. 属性快照、单 key 查询和下发日志都直接服务前端设备详情页，字段命名、排序和分页语义调整时要同步核对前端表格契约。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AttributeDataApi struct{}

// HandleDataList 查询设备属性当前值列表。
// 参数来源：
// 1. 路径参数 `id` 表示设备实例 ID。
// 2. `claims` 由鉴权中间件注入，用于限制当前租户或用户可见的设备范围。
// 调用关系：
// 1. 上游通常由设备详情页首次进入属性 tab 时触发。
// 2. 下游委托 AttributeData service 负责设备可见性校验、属性快照聚合和返回结构整理。
// 静态审查建议：
// 1. 当前直接使用字符串 ID 透传，若后续设备主键形态调整，建议统一在 API 层增加更显式的参数语义校验。
// 2. 若后续引入属性分页或大字段裁剪，这个“全量快照接口”要和详情页加载策略一起重审。
// @Router   /api/v1/attribute/datas/{id} [get]
func (*AttributeDataApi) HandleDataList(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.AttributeData.GetAttributeDataList(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleAttributeDataByKey 根据 key 查询设备属性。
// 参数绑定：
// 1. 通过 BindAndValidate 绑定查询参数，通常包含设备 ID、属性 key 等精确过滤条件。
// 2. `claims` 继续作为数据权限边界，避免跨租户或越权读取单个属性。
// 调用关系：
// 1. 上游适合设备详情局部刷新、单属性回读或调试场景。
// 2. 下游 service 负责 key 存在性、设备归属和最终属性值解析，不在 API 层做业务判断。
// 静态审查建议：
// 1. 单 key 查询与全量快照查询返回模型若长期并行，建议在文档或 DTO 命名上进一步拉开差异，减少误用。
// 2. 若后续支持模糊 key 或批量 key 查询，这个入口应避免继续堆参数分支，改为独立查询模型更清晰。
// /api/v1/attribute/datas/key [get]
func (*AttributeDataApi) HandleAttributeDataByKey(c *gin.Context) {
	var req model.GetDataListByKeyReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.AttributeData.GetAttributeDataByKey(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DeleteData 删除属性数据。
// 参数边界：
// 1. 路径参数 `id` 表示待删除的属性数据记录，而不是设备 ID。
// 2. `claims` 用于下游判断当前操作者是否具备删除权限以及可操作的数据范围。
// 下游影响：
// 1. service 层可能继续联动审计、缓存刷新或历史回溯逻辑，API 层不自行处理副作用。
// 静态审查建议：
// 1. 删除操作属于高影响入口，若后续产品要求更强审计，可在 API 层补充操作来源或请求追踪信息透传。
// 2. 当前成功后统一返回 nil，若前端未来需要回显被删对象摘要，可再评估响应契约。
// @Router   /api/v1/attribute/datas/{id} [delete]
func (*AttributeDataApi) DeleteData(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.AttributeData.DeleteAttributeData(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleAttributeSetLogsDataListByPage 查询属性下发记录。
// 参数绑定：
// 1. 通过分页 DTO 接收设备、属性、结果状态、时间范围等筛选条件。
// 2. `claims` 约束可见日志范围，避免跨租户查看其他设备的属性下发轨迹。
// 调用关系：
// 1. 上游常用于设备详情中的命令回溯、运维排障或审计页面。
// 2. 下游 service 负责日志排序、分页、状态解释及与设备实体的权限校验。
// 静态审查建议：
// 1. 下发日志通常带有敏感载荷或错误原因，若未来对外开放更多角色，应复核脱敏边界是否应前移到 service 或 serializer。
// 2. 这类分页 handler 与其他日志接口结构高度相似，后续可抽公共分页查询入口模板减少重复。
// @Router   /api/v1/attribute/datas/set/logs [get]
func (*AttributeDataApi) HandleAttributeSetLogsDataListByPage(c *gin.Context) {
	var req model.GetAttributeSetLogsListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.AttributeData.GetAttributeSetLogsDataListByPage(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// AttributePutMessage 发送属性写入消息。
// 参数与边界：
// 1. 请求体绑定到属性写入 DTO，通常包含目标设备、属性 key 与待写入的值。
// 2. `userClaims.ID` 作为操作者身份透传给下游，`claims` 本身继续承担租户、设备和权限边界。
// 3. 当前显式把来源标记为 `constant.Manual`，帮助下游区分人工写入与自动化、系统任务等其他来源。
// 调用关系：
// 1. 上游通常来自设备详情页手动写属性、调试视图或运维入口。
// 2. 下游 service 负责设备在线性判断、协议路由、消息投递和下发日志落库，API 层不介入具体发送逻辑。
// 静态审查建议：
// 1. 该入口既接收 gin 上下文又接收操作者 ID，说明下游可能依赖 trace 或请求取消；若后续抽象统一命令入口，要保留这层上下文语义。
// 2. 若属性写入后要支持幂等键、批量写入或异步任务回执，建议避免继续在现有 DTO 上叠加可选字段。
// /api/v1/attribute/datas/pub [post]
func (*AttributeDataApi) AttributePutMessage(c *gin.Context) {
	var req model.AttributePutMessage
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.AttributeData.AttributePutMessage(c, userClaims.ID, &req, strconv.Itoa(constant.Manual), userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// AttributeGetMessage 发送属性读取请求。
// 参数与边界：
// 1. 请求体绑定读取命令 DTO，通常描述目标设备、要主动拉取的属性范围。
// 2. `claims` 决定谁可以触发主动读取，以及可操作的设备归属范围。
// 调用关系：
// 1. 上游常见于设备详情页“主动刷新属性”或设备调试流程。
// 2. 下游 service 负责构造协议层读取命令、投递到上行/下行链路，并在后续由属性数据链路回写快照。
// 静态审查建议：
// 1. 该接口只返回提交结果，不保证设备立即上报；若前端后续要区分“已受理”和“已回读成功”，需要补异步结果契约。
// 2. 读取命令与写入命令共享相似的鉴权和消息投递边界，后续可考虑抽公共命令入口注释模板或 helper。
// /api/v1/attribute/datas/get
func (*AttributeDataApi) AttributeGetMessage(c *gin.Context) {
	var req model.AttributeGetMessageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.AttributeData.AttributeGetMessage(userClaims, &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}
