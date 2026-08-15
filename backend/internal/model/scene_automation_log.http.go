// 文件用途：定义 scene automation log 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

import "time"

type GetSceneAutomationLogReq struct {
	PageReq
	SceneAutomationId  string     `json:"scene_automation_id" form:"scene_automation_id" validate:"required,max=36"`
	ExecutionResult    *string    `json:"execution_result" form:"execution_result" validate:"omitempty"`
	ExecutionStartTime *time.Time `json:"execution_start_time" form:"execution_start_time" validate:"omitempty"`
	ExecutionEndTime   *time.Time `json:"execution_end_time" form:"execution_end_time" validate:"omitempty"`
}

type GetSceneByDeviceIdWhitDeviceConfigIdReq struct {
	PageReq
	DeviceId       string `json:"device_id" form:"device_id"`
	DeviceConfigId string `json:"device_config_id" form:"device_config_id"`
}
