// 文件用途：定义操作日志分页查询接口的 API Handler，用于输出审计/操作轨迹列表。
// 核心链路：请求进入 Handler 后先做参数绑定与校验，再读取上下文 claims 作为数据范围约束，
// 然后调用 OperationLogs service 查询分页结果，最终通过统一响应中间件返回给调用方。
// 使用注意：日志查询接口通常面向审计场景，应确保过滤条件、租户隔离和排序语义稳定，避免影响追溯结果。
// 静态审查建议：重点检查 claims 缺失时的 panic 风险、分页排序字段是否存在隐式默认值、
// 以及日志内容返回是否可能暴露敏感字段，必要时在 service 或 DTO 层做脱敏约束。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type OperationLogsApi struct{}

// HandleListByPage 分页查询当前权限范围内的操作日志。
// 核心步骤：绑定分页筛选条件、提取 claims、调用 service 查询，并把结果挂到上下文供统一响应输出。
// 审查重点：确认查询条件无法绕过租户/角色隔离，同时注意日志列表的返回字段是否满足最小暴露原则。
// @Router   /api/v1/operation_logs [get]
func (*OperationLogsApi) HandleListByPage(c *gin.Context) {
	var req model.GetOperationLogListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	list, err := service.GroupApp.OperationLogs.GetListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", list)
}
