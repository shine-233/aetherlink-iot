// 文件用途：提供系统监控查询接口，负责读取登录用户 claims、解析查询参数，并把监控指标查询委托给系统监控 service。
// 核心逻辑：本文件包含“当前指标查询”和“历史指标查询”两个只读 Handler，前置做 SYS_ADMIN 权限校验和 query 参数归一化。
// 权限边界：API 层显式限制只有 claims.Authority 为 SYS_ADMIN 的调用者可以读取监控数据；service 层也会再次校验，形成双层保护，避免绕过路由直接访问内部逻辑。
// 静态审查建议：重点检查 MustGet("claims") 的安全性、Authority 常量是否应统一复用、hours 参数的容错与上限是否满足产品预期，以及 metricsManager 为空时返回 nil 数据是否需要更明确的错误语义。
package api

import (
	"fmt"
	"time"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// SystemMonitorApi 负责承接系统监控只读查询接口。
type SystemMonitorApi struct{}

// GetCurrentSystemMetrics 获取当前系统实时指标。
// 参数绑定：无请求体与 query 参数，只依赖上下文中的 claims。
// Claims：从 c.MustGet("claims") 读取 *utils.UserClaims，并使用 Authority 判断是否为 SYS_ADMIN。
// 权限边界：API 层先拒绝非 SYS_ADMIN 请求；即便上游误放行，service.GetCurrentMetrics 仍会再次校验 claims，避免敏感监控数据泄露。
// Service 调用链：api.GetCurrentSystemMetrics -> service.GroupApp.SystemMonitor.GetCurrentMetrics -> requireSystemMonitorAdmin -> metricsManager.GetCurrentMetrics。
// 静态审查建议：建议确认权限字符串是否统一使用常量，避免硬编码漂移；同时关注 metricsManager 为空时返回 nil 数据给前端是否足够可观测。
// @Summary 获取当前系统指标
// @Description 获取系统 CPU、内存、磁盘使用率等当前值
// @Tags 系统监控
// @Accept  json
// @Produce  json
// @Success 200 {object} response.Response
// @Router /api/v1/system/metrics/current [get]
func (api *SystemMonitorApi) GetCurrentSystemMetrics(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if userClaims.Authority != "SYS_ADMIN" {
		c.Error(errcode.New(errcode.CodeNoPermission))
		return
	}

	metrics, err := service.GroupApp.SystemMonitor.GetCurrentMetrics(userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", metrics)
}

// GetHistorySystemMetrics 获取系统历史指标时间序列。
// 参数绑定：从 query 参数读取 hours；缺失或解析失败时回退到 24，小于等于 0 时归一化为 1，大于 72 时截断为 72。
// Claims：从上下文读取 *utils.UserClaims，并要求 Authority 为 SYS_ADMIN。
// 权限边界：本层只开放固定时间窗口内的历史监控读取，不暴露任意跨度查询；service 层会再次校验管理员权限，防止通过内部调用链绕过限制。
// Service 调用链：api.GetHistorySystemMetrics -> fmt.Sscanf(query.hours) -> service.GroupApp.SystemMonitor.GetCombinedHistoryData -> requireSystemMonitorAdmin -> metricsManager.GetCombinedHistoryData。
// 静态审查建议：审查时建议确认 hours 上限 72 是否应提取为常量、解析失败直接回退默认值是否会掩盖客户端错误，以及是否需要为超大时间范围或空数据场景补充显式日志。
// @Summary 获取系统指标历史数据
// @Description 获取系统 CPU、内存、磁盘使用率的历史时间序列
// @Tags 系统监控
// @Accept  json
// @Produce  json
// @Param hours query int false "查询小时数，默认 24 小时" default(24)
// @Success 200 {object} response.Response
// @Router /api/v1/system/metrics/history [get]
func (api *SystemMonitorApi) GetHistorySystemMetrics(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if userClaims.Authority != "SYS_ADMIN" {
		c.Error(errcode.New(errcode.CodeNoPermission))
		return
	}

	hours := 24
	if hoursStr := c.Query("hours"); hoursStr != "" {
		if _, err := fmt.Sscanf(hoursStr, "%d", &hours); err != nil {
			hours = 24
		}
	}

	// 将查询窗口限制在 1~72 小时之间，避免无效请求或过大的时间跨度拖垮监控查询。
	if hours <= 0 {
		hours = 1
	} else if hours > 72 {
		hours = 72
	}

	duration := time.Duration(hours) * time.Hour
	data, err := service.GroupApp.SystemMonitor.GetCombinedHistoryData(duration, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}
