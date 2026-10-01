// 文件用途：
// 提供告警配置、告警信息、告警历史与设备告警状态相关的 HTTP Handler。
//
// 核心链路：
// 1. 通过 BindAndValidate 绑定并校验请求参数。
// 2. 从上下文 claims 中提取用户与租户信息。
// 3. 将资源 ID、租户边界和查询条件转交 service.GroupApp.Alarm。
// 4. 统一通过 c.Error 或 c.Set("data", ...) 交给响应中间件输出结果。
//
// 使用注意：
// - 本层只负责参数装配、身份透传和错误转交，不应承载业务分支。
// - 涉及 TenantID 的写入必须以 claims 为准，避免客户端请求体覆盖租户边界。
// - 多个接口直接依赖路径参数 id，新增类似接口时要保持空值校验与错误码语义一致。
//
// 静态审查建议：
// - 重点关注 history、info、config 三组接口的入参模型与路由语义是否持续一致。
// - 可持续审查重复的 id 判空与 claims 提取逻辑，后续适合抽公共辅助函数降重复。
package api

import (
	"fmt"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type AlarmApi struct{}

// CreateAlarmConfig 创建告警配置。
// 核心是使用当前登录租户覆盖请求中的 TenantID 后再进入 service，
// 静态审查时要重点确认不存在客户端绕过租户隔离字段的可能。
// /api/v1/alarm/config [post]
func (*AlarmApi) CreateAlarmConfig(c *gin.Context) {
	Handle(c, func(req *model.CreateAlarmConfigReq, userClaims *utils.UserClaims) (interface{}, error) {
		req.TenantID = userClaims.TenantID
		return service.GroupApp.Alarm.CreateAlarmConfig(req, userClaims)
	})
}

// DeleteAlarmConfig 删除指定告警配置。
// 该接口依赖路径参数 id 精确定位资源，静态审查时建议确认空 id、非法 id
// 和越权删除在 service 层是否具有一致的错误处理语义。
// /api/v1/alarm/config/{id} [Delete]
func (*AlarmApi) DeleteAlarmConfig(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		if id == "" {
			return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"err": fmt.Sprintf("id is %s", id),
			})
		}
		return service.GroupApp.Alarm.DeleteAlarmConfig(id, userClaims)
	})
}

// UpdateAlarmConfig 更新告警配置。
// 这里通过 claims 回填 TenantID 指针，避免更新请求误改租户归属，
// 审查时可重点关注可选字段更新是否会带来误覆盖风险。
// /api/v1/alarm/config [PUT]
func (*AlarmApi) UpdateAlarmConfig(c *gin.Context) {
	Handle(c, func(req *model.UpdateAlarmConfigReq, userClaims *utils.UserClaims) (interface{}, error) {
		req.TenantID = &userClaims.TenantID
		return service.GroupApp.Alarm.UpdateAlarmConfig(req, userClaims)
	})
}

// ServeAlarmConfigListByPage 分页查询告警配置列表。
// 该接口通常是配置页的数据入口，静态审查时建议核对分页、筛选和租户条件
// 是否全部在 service 层闭合，避免返回超范围数据。
// /api/v1/alarm/config [GET]
func (*AlarmApi) ServeAlarmConfigListByPage(c *gin.Context) {
	Handle(c, func(req *model.GetAlarmConfigListByPageReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Alarm.GetAlarmConfigListByPage(req, userClaims)
	})
}

// /api/v1/alarm/info [put]
func (*AlarmApi) UpdateAlarmInfo(c *gin.Context) {
	Handle(c, func(req *model.UpdateAlarmInfoReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Alarm.UpdateAlarmInfo(req, userClaims)
	})
}

// BatchUpdateAlarmInfo 批量更新告警信息。
// 这是告警处理的批量入口，静态审查时应重点关注批量操作的部分失败语义、
// 原子性边界以及每条记录是否都经过相同的租户与权限校验。
// /api/v1/alarm/info/batch [put]
func (*AlarmApi) BatchUpdateAlarmInfo(c *gin.Context) {
	HandleAction(c, func(req *model.UpdateAlarmInfoBatchReq, userClaims *utils.UserClaims) error {
		return service.GroupApp.Alarm.UpdateAlarmInfoBatch(req, userClaims)
	})
}

// /api/v1/alarm/info [get]
func (*AlarmApi) HandleAlarmInfoListByPage(c *gin.Context) {
	Handle(c, func(req *model.GetAlarmInfoListByPageReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Alarm.GetAlarmInfoListByPage(req, userClaims)
	})
}

// HandleAlarmHisttoryListByPage 分页查询告警历史。
// 该接口承接历史页检索，静态审查时建议关注时间范围、排序字段与大分页场景
// 下的性能和数据边界控制是否收口在 service 层。
// all_tenants 仅允许 SYS_ADMIN 显式启用，默认仍按租户/owner 范围查询。
// /api/v1/alarm/info/history [get]
func (*AlarmApi) HandleAlarmHisttoryListByPage(c *gin.Context) {
	//
	Handle(c, func(req *model.GetAlarmHisttoryListByPage, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Alarm.GetAlarmHisttoryListByPage(req, userClaims)
	})
}

// HandleAlarmHistoryMonthlyTrend returns monthly alarm occurrence counts for a selected year.
// @Summary 获取年度月度告警趋势
// @Description 按当前账号可见设备范围，返回所选年份 1-12 月的告警触发次数；已重置的历史告警仍计入原触发月份。
// @Tags 告警管理
// @Accept json
// @Produce json
// @Param year query int true "年份" minimum(2000) maximum(2100)
// @Param timezone query string false "IANA 时区，例如 Asia/Shanghai；默认 UTC"
// @Param all_tenants query bool false "仅 SYS_ADMIN 可显式汇总全部租户"
// @Success 200 {object} model.AlarmHistoryMonthlyTrendResp
// @Router /api/v1/alarm/info/history/monthly [get]
func (*AlarmApi) HandleAlarmHistoryMonthlyTrend(c *gin.Context) {
	Handle(c, func(req *model.AlarmHistoryMonthlyTrendReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Alarm.GetAlarmHistoryMonthlyTrend(req, userClaims)
	})
}

// /api/v1/alarm/info/history [put]
func (*AlarmApi) AlarmHistoryDescUpdate(c *gin.Context) {
	//
	HandleAction(c, func(req *model.AlarmHistoryDescUpdateReq, userClaims *utils.UserClaims) error {
		return service.GroupApp.Alarm.AlarmHistoryDescUpdate(req, userClaims)
	})
}

// AcknowledgeAlarmHistory 确认指定告警历史记录。
// 核心是将路径 id 与当前用户身份一并传入 service，
// 静态审查时应确认重复确认、状态迁移和审计信息写入是否被统一处理。
// /api/v1/alarm/info/history/{id}/acknowledge [put]
func (*AlarmApi) AcknowledgeAlarmHistory(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"err": fmt.Sprintf("id is %s", id),
			})
		}
		return service.GroupApp.Alarm.AcknowledgeAlarmHistory(id, userClaims)
	})
}

// ResetAlarmHistory 重置指定告警历史状态。
// 这类状态流转接口容易与确认、删除形成竞态，静态审查时建议重点核对
// 当前状态校验、并发更新处理和越权操作防护。
// /api/v1/alarm/info/history/{id}/reset [put]
func (*AlarmApi) ResetAlarmHistory(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"err": fmt.Sprintf("id is %s", id),
			})
		}
		return service.GroupApp.Alarm.ResetAlarmHistory(id, userClaims)
	})
}

// ClearAlarmHistory 清除指定告警历史记录（对齐 ThingsBoard 4.3 告警清除生命周期）。
// /api/v1/alarm/info/history/{id}/clear [post, put]
func (*AlarmApi) ClearAlarmHistory(c *gin.Context) {
	// 迁移注意：请求体是可选的（ContentLength>0 时尽力绑定且失败忽略），不适配 Handle 的强制绑定，
	// 因此保留闭包内软绑定，仅收敛 claims 与响应出口。
	HandleNoBody(c, func(userClaims *utils.UserClaims) (interface{}, error) {
		id := c.Param("id")
		if id == "" {
			return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"err": fmt.Sprintf("id is %s", id),
			})
		}
		var req model.ClearAlarmReq
		if c.Request.ContentLength > 0 {
			_ = c.ShouldBindJSON(&req)
		}
		note := ""
		if req.Note != nil {
			note = *req.Note
		}
		return service.GroupApp.Alarm.ClearAlarmHistory(id, note, userClaims)
	})
}

// BatchAlarmHistoryAction 批量确认或重置告警历史，并返回每条记录的成功/失败明细。
// 该入口面向运维闭环场景，Handler 只负责绑定 action、ids、note 和当前操作者身份。
// /api/v1/alarm/info/history/batch-action [put]
func (*AlarmApi) BatchAlarmHistoryAction(c *gin.Context) {
	Handle(c, func(req *model.AlarmHistoryBatchActionReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Alarm.BatchAlarmHistoryAction(req, userClaims)
	})
}

// HandleDeviceAlarmStatus 查询设备当前是否处于告警状态。
// Handler 只负责把 service 返回值包装为 {alarm: bool} 响应，
// 静态审查时可重点留意设备标识与租户边界是否足以防止跨租户探测。
func (*AlarmApi) HandleDeviceAlarmStatus(c *gin.Context) {
	//
	Handle(c, func(req *model.GetDeviceAlarmStatusReq, userClaims *utils.UserClaims) (interface{}, error) {
		ok, err := service.GroupApp.Alarm.GetDeviceAlarmStatus(req, userClaims)
		if err != nil {
			return nil, err
		}
		return map[string]bool{
			"alarm": ok,
		}, nil
	})
}

// /api/v1/alarm/info/config/device [get]
func (*AlarmApi) HandleConfigByDevice(c *gin.Context) {
	//
	Handle(c, func(req *model.GetDeviceAlarmStatusReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Alarm.GetConfigByDevice(req, userClaims)
	})
}

// HandleAlarmInfoHistory 根据历史记录 ID 查询详情。
// 该接口常作为历史列表的详情入口，静态审查时建议确认返回内容是否包含敏感字段，
// 以及 id 所属租户校验是否完全下沉到 service 层。
// /api/v1/alarm/info/history/{id} [GET]
func (*AlarmApi) HandleAlarmInfoHistory(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"err": fmt.Sprintf("id is %s", id),
			})
		}
		return service.GroupApp.Alarm.GetAlarmInfoHistoryByID(id, userClaims)
	})
}

// GetAlarmDeviceCountsByTenant 获取当前账号可见范围内的告警设备数量统计。
// 租户管理员使用租户全量范围，TENANT_USER 还要叠加设备 owner 过滤；
// all_tenants 只允许 SYS_ADMIN 显式启用；其他账号始终保持租户/owner 范围。
// @Summary 获取当前账号可见的告警状态设备数量
// @Description 租户管理员查看租户全量，普通租户用户只统计自己拥有的设备
// @Tags 告警管理
// @Accept json
// @Produce json
// @Param all_tenants query bool false "仅 SYS_ADMIN 可显式汇总全部租户"
// @Success 200 {object} model.AlarmDeviceCountsResponse
// @Router /api/v1/alarm/device/counts [get]
func (api *AlarmApi) GetAlarmDeviceCountsByTenant(c *gin.Context) {
	Handle(c, func(req *model.AlarmDeviceCountsReq, userClaims *utils.UserClaims) (interface{}, error) {
		// 具体统计逻辑位于 service 层，Handler 只保留入参与响应包装职责。
		return service.GroupApp.Alarm.GetAlarmDeviceCounts(req, userClaims)
	})
}

// DeleteAlarmHistory 保留旧 DELETE 路由作为兼容入口；service 会先校验
// tenant/owner 权限，再按审计留存策略拒绝删除，确保触发过的告警仍可追溯。
// /api/v1/alarm/info/history/{id} [DELETE]
func (*AlarmApi) DeleteAlarmHistory(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		if id == "" {
			return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"err": fmt.Sprintf("id is %s", id),
			})
		}
		return service.GroupApp.Alarm.DeleteAlarmHistory(id, userClaims)
	})
}
