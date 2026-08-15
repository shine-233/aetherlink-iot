// 文件用途：提供场景自动化的创建、更新、开关、查询与日志查询接口。
// 核心链路：Handler 负责绑定请求、读取 claims 与路径参数，再把领域决策下沉给 scene automation 相关 service。
// 使用注意：该文件不应承担规则编排、告警关联判定或状态机逻辑，接口层只保留参数收敛与统一错误出口。
// 静态审查建议：重点检查 claims 透传是否完整、ID 路径参数是否统一校验、分页查询条件是否会放大结果集，以及重复控制器样板是否值得后续抽取。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	common "aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type SceneAutomationsApi struct{}

// CreateSceneAutomations 创建场景自动化。
// 静态审查重点：确认创建时的规则合法性、资源归属与默认状态初始化都在 service 层统一完成。
// /api/v1/scene_automations [post]
func (*SceneAutomationsApi) CreateSceneAutomations(c *gin.Context) {
	logrus.Info("create scene automation request")
	var req model.CreateSceneAutomationReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	id, err := service.GroupApp.SceneAutomation.CreateSceneAutomation(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"scene_automation_id": id})
}

// DeleteSceneAutomations 删除场景自动化。
// 静态审查重点：删除后是否需要级联清理依赖对象，应继续由 service 层集中约束。
// /api/v1/scene_automations/{id} [delete]
func (*SceneAutomationsApi) DeleteSceneAutomations(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.SceneAutomation.DeleteSceneAutomation(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", 1)
}

// SwitchSceneAutomations 切换场景自动化启停状态。
// 静态审查重点：当前接口仅依赖路径 ID 和 claims，状态切换幂等性需在 service 层保证。
// /api/v1/scene_automations/switch/{id} [post]
func (*SceneAutomationsApi) SwitchSceneAutomations(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.SceneAutomation.SwitchSceneAutomation(id, "", userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// UpdateSceneAutomations 更新场景自动化定义。
// 静态审查重点：关注更新请求是否会覆盖只读字段，避免接口层无意识放宽修改范围。
// /api/v1/scene_automations [put]
func (*SceneAutomationsApi) UpdateSceneAutomations(c *gin.Context) {
	var req model.UpdateSceneAutomationReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	id, err := service.GroupApp.SceneAutomation.UpdateSceneAutomation(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"scene_automation_id": id})
}

// DryRunSceneAutomations previews a rule without saving, executing, publishing, or refreshing runtime cache.
// @Summary Dry-run scene automations
// @Tags SceneAutomations
// @Accept json
// @Produce json
// @Param request body model.DryRunSceneAutomationReq true "Scene automation dry-run payload"
// @Success 200 {object} model.SceneAutomationDryRunResult "Dry-run validation result"
// @Router /api/v1/scene_automations/dry-run [post]
func (*SceneAutomationsApi) DryRunSceneAutomations(c *gin.Context) {
	var req model.DryRunSceneAutomationReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.SceneAutomation.DryRunSceneAutomation(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*SceneAutomationsApi) HandleSceneAutomations(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.SceneAutomation.GetSceneAutomation(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleSceneAutomationsByPage 分页查询场景自动化列表。
// 静态审查重点：关注查询条件缺省值带来的全量扫描风险，以及分页参数是否已在绑定阶段限幅。
// /api/v1/scene_automations/list [get]
func (*SceneAutomationsApi) HandleSceneAutomationsByPage(c *gin.Context) {
	var req model.GetSceneAutomationByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.SceneAutomation.GetSceneAutomationByPageReq(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleSceneAutomationsWithAlarmByPage 分页查询与告警关联的场景自动化。
// 关键点：额外补充 device_id/device_config_id 至少一项非空的接口层校验，避免无约束查询。
// 静态审查重点：如果后续加入更多筛选维度，建议继续保持“最少一个锚点条件”策略，防止查询面过宽。
// /api/v1/scene_automations/alarm [get]
func (*SceneAutomationsApi) HandleSceneAutomationsWithAlarmByPage(c *gin.Context) {
	var req model.GetSceneAutomationsWithAlarmByPageReq
	if !BindAndValidate(c, &req) {
		return
	}

	if common.IsStringEmpty(req.DeviceId) && common.IsStringEmpty(req.DeviceConfigId) {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "device_id and device_config_id can not be empty at the same time",
		}))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.SceneAutomation.GetSceneAutomationWithAlarmByPageReq(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleSceneAutomationsLog 查询场景自动化执行日志。
// 静态审查重点：日志查询通常容易放大数据量，建议持续关注时间范围、分页上限和敏感字段脱敏。
// /api/v1/scene_automations/log [get]
func (*SceneAutomationsApi) HandleSceneAutomationsLog(c *gin.Context) {
	var req model.GetSceneAutomationLogReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.SceneAutomationLog.GetSceneAutomationLog(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
