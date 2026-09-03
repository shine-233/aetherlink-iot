// 文件用途：维护场景自动化执行日志查询和记录服务。
// 核心逻辑：记录场景执行结果、动作输出和失败原因，并按条件返回前端日志列表。
// 关键注意事项：执行日志是排障证据，不能误报成功、泄露跨租户数据或丢失失败原因。
// 重构建议：抽出日志仓储和结果序列化器，补齐权限、分页、事务和敏感字段测试。
package service

import (
	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

type SceneAutomationLog struct{}

func (*SceneAutomationLog) GetSceneAutomationLog(req *model.GetSceneAutomationLogReq, u *utils.UserClaims) (interface{}, error) {
	if u == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query scene automation log")
	}
	// 日志行按执行时租户归属存储；场景为租户级资源，TENANT_USER 保持 self-only，
	// 空租户管理员（SYS_ADMIN 平台场景）→ [""]，非空管理员 → self∪子孙（ROADMAP C2）。
	scopes := sceneAutomationReadScopes(u.TenantID, u)
	total, data, err := dal.GetSceneAutomationLog(req, scopes)
	logList := make(map[string]interface{})
	logList["total"] = total
	logList["list"] = data

	return logList, err
}
