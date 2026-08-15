// event_data.go 负责设备事件数据域的 HTTP 入口。
// 核心职责：
// 1. 接收事件历史查询请求，完成分页筛选参数绑定与 claims 提取。
// 2. 把设备、产品、租户和时间范围等查询边界交给 service 层统一校验和聚合。
// 3. 保持 API 层为薄控制器，不在这里掺入事件解释、规则关联或通知分发逻辑。
// 上下游关系：
// 1. 上游通常来自设备详情事件页、告警回放、运维排障或数据审计页面。
// 2. 下游依赖 service.GroupApp.EventData 查询事件存储、补齐分页结构，并按 claims 过滤可见范围。
// 静态审查建议：
// 1. 当前文件只有分页查询入口，后续若继续增加导出、详情或重放接口，建议按“查询类事件接口”独立分组，避免再次堆在单文件。
// 2. `c.MustGet("claims")` 对鉴权中间件注入顺序有强依赖，若未来开放内部任务入口，需要补更显式的空值保护或守卫。
// 3. 事件列表往往与告警、通知或自动化链路存在关联字段，字段命名、排序规则和时间过滤语义调整时，要同步核对前端事件表格与审计导出逻辑。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type EventDataApi struct{}

// HandleEventDatasListByPage 分页查询事件数据。
// 参数绑定：
// 1. 通过 DTO 绑定分页、设备、事件类型、时间范围等筛选条件。
// 2. `claims` 用于限定当前用户或租户可查询的事件范围，避免跨租户读取设备事件。
// 调用关系：
// 1. 上游通常由设备详情事件页或系统级事件检索页触发。
// 2. 下游 service 负责事件数据源查询、权限裁剪、排序和分页结果拼装。
// 静态审查建议：
// 1. 当前分页查询和其他数据域分页接口结构非常相似，后续可抽公共查询模板，统一注释、错误处理和审计留痕。
// 2. 事件数据可能包含原始 payload、告警上下文或设备标识，若未来开放更细粒度角色，应再次审查脱敏与字段裁剪边界。
// @Router   /api/v1/event/datas [get]
func (*EventDataApi) HandleEventDatasListByPage(c *gin.Context) {
	var req model.GetEventDatasListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.EventData.GetEventDatasListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
