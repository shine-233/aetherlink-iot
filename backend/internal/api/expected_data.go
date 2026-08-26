// expected_data.go 负责设备期望数据域的 HTTP 入口编排。
// 核心职责：
// 1. 提供期望数据列表查询、创建和删除接口，承接前端“期望值”或“期望状态”管理页面。
// 2. 在 API 层完成 DTO 绑定、claims 提取和统一错误出口，把租户、设备和业务规则边界交给 service 层处理。
// 3. 维持“入口只收口、不执行业务”的薄控制器模式，不在这里直接处理设备同步、状态比对或自动补偿逻辑。
// 上下游关系：
// 1. 上游通常来自设备详情期望值视图、批量控制页面或需要声明设备目标状态的运维入口。
// 2. 下游依赖 service.GroupApp.ExpectedData 处理分页、创建、删除以及与设备/属性/命令链路的联动。
// 3. `PageList` 使用独立 context，`Create/Delete` 则沿用 gin 上下文，说明下游可能对取消信号、链路追踪或日志上下文有不同依赖。
// 静态审查建议：
// 1. 当前列表、创建、删除三类入口已经覆盖完整 CRUD 的大半边界，若未来补更新或批量操作，建议统一设计期望数据生命周期接口而不是继续零散追加。
// 2. `c.MustGet("claims")` 对鉴权中间件顺序有硬依赖，若后续出现系统内调用或异步转发，需要补更稳妥的守卫。
// 3. 期望数据通常会影响自动化、状态对账或下发补偿，API 层虽然不处理这些副作用，但文档和字段语义变更时要同步核对前后端约定。
package api

import (
	"aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type ExpectedDataApi struct{}

// HandleExpectedDataList 分页查询期望数据。
// 参数绑定：
// 1. 通过分页 DTO 接收设备、期望项、状态等筛选条件。
// 2. `claims` 作为可见范围边界，保证只能查询当前租户或授权范围内的期望数据。
// 调用关系：
// 1. 上游通常来自设备期望值列表页或运维检索页。
// 2. 下游 service 负责分页查询、排序与状态字段解释，API 层不直接拼装业务数据。
// 静态审查建议：
// 1. 查询链路现已透传 c.Request.Context()（2026-08 #11 断链治理）；后续新增入口同样不要使用裸 Background。
// 2. 若期望数据列表后续支持导出或批量筛查，应避免在现有 DTO 中堆叠过多可选字段。
// /api/v1/expected/data/list
func (*ExpectedDataApi) HandleExpectedDataList(c *gin.Context) {
	var req model.GetExpectedDataPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ExpectedData.PageList(c.Request.Context(), &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// CreateExpectedData 新增期望数据。
// 参数与边界：
// 1. 请求体绑定创建 DTO，通常包含设备、目标属性或命令、期望值及相关策略字段。
// 2. `claims` 继续承担操作者身份和租户边界，真正的设备归属、字段合法性和冲突检测由 service 处理。
// 调用关系：
// 1. 上游通常来自设备详情中的期望值新增弹窗、批量控制流程或规则配置入口。
// 2. 下游 service 可能继续联动设备状态对账、缓存刷新或自动化逻辑，但这些副作用不在 API 层实现。
// 静态审查建议：
// 1. 创建操作属于高影响入口，若未来引入批量创建、审批或审计要求，建议把“请求来源”“操作备注”等上下文显式透传。
// 2. 当前成功后直接返回 service 响应，若不同创建场景需要不同回包摘要，应先收敛统一响应契约再扩展。
// /api/v1/expected/data POST
func (*ExpectedDataApi) CreateExpectedData(c *gin.Context) {
	var req model.CreateExpectedDataReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ExpectedData.Create(c, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// DeleteExpectedData 删除期望数据。
// 参数与边界：
// 1. 路径参数 `id` 表示期望数据记录主键，而不是设备 ID。
// 2. `claims` 用于下游判断当前用户是否有权限删除该期望项，以及是否属于当前租户。
// 调用关系：
// 1. 上游通常来自期望值列表行操作或设备详情中的撤销入口。
// 2. 下游 service 负责删除前校验、可能存在的缓存刷新和后续状态联动。
// 静态审查建议：
// 1. 删除后当前返回空 map，若前端未来需要保留被删对象摘要或撤销提示，可再统一评估删除类接口响应模型。
// 2. 期望数据删除可能影响自动化或状态比对结果，若后续需要更强审计，可在 API 层补操作来源或请求追踪信息透传。
// /api/v1/expected/data/{id} DELETE
func (*ExpectedDataApi) DeleteExpectedData(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.ExpectedData.Delete(c, id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}
