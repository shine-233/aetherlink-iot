// 文件用途：定义 scene info 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

import "time"

type CreateSceneReq struct {
	Name        string            `json:"name" validate:"required,max=36"`
	Description string            `json:"description"`
	Actions     []SceneActionsReq `json:"actions" validate:"required"`
}

type SceneActionsReq struct {
	ActionType      string  `json:"action_type" validate:"required,oneof=10 11 30"`
	ActionTarget    string  `json:"action_target" validate:"required"`
	ActionParamType *string `json:"action_param_type" validate:"omitempty"`
	ActionParam     *string `json:"action_param" validate:"omitempty"`
	ActionValue     *string `json:"action_value" validate:"omitempty"`
	Remark          *string `json:"remark" validate:"omitempty"`
}

type DryRunSceneActionReq struct {
	ActionType      string  `json:"action_type" validate:"omitempty,oneof=10 11 30"`
	ActionTarget    string  `json:"action_target" validate:"omitempty"`
	ActionParamType *string `json:"action_param_type" validate:"omitempty"`
	ActionParam     *string `json:"action_param" validate:"omitempty"`
	ActionValue     *string `json:"action_value" validate:"omitempty"`
	Remark          *string `json:"remark" validate:"omitempty"`
}

type DryRunSceneReq struct {
	ID          *string                `json:"id" validate:"omitempty,max=36"`
	Name        *string                `json:"name" validate:"omitempty,max=36"`
	Description *string                `json:"description" validate:"omitempty"`
	Actions     []DryRunSceneActionReq `json:"actions" validate:"omitempty"`
}

type UpdateSceneReq struct {
	ID          string            `json:"id" validate:"required,max=36"`
	Name        string            `json:"name" validate:"required,max=36"`
	Description string            `json:"description"`
	Actions     []SceneActionsReq `json:"actions" validate:"required"`
}

type GetSceneListByPageReq struct {
	PageReq
	Name *string `json:"name" form:"name" validate:"omitempty"`
}

type GetSceneLogListByPageReq struct {
	PageReq
	ID                 string     `json:"id" form:"id" validate:"required,max=36"`
	ExecutionResult    *string    `json:"execution_result" form:"execution_result" validate:"omitempty"`
	ExecutionStartTime *time.Time `json:"execution_start_time" form:"execution_start_time" validate:"omitempty"`
	ExecutionEndTime   *time.Time `json:"execution_end_time" form:"execution_end_time" validate:"omitempty"`
}
