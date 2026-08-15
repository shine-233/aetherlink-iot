// 文件用途：定义 scene automations 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

import "time"

type CreateSceneAutomationReq struct {
	Name                   string        `json:"name" validate:"required,max=36"`
	Description            string        `json:"description"`
	Enabled                string        `json:"enabled" validate:"omitempty,oneof=Y N"`
	TriggerConditionGroups [][]Condition `json:"trigger_condition_groups" validate:"required"`
	Actions                []Action      `json:"actions" validate:"required"`
	Remark                 string        `json:"remark" `
}

type UpdateSceneAutomationReq struct {
	ID                     string        `json:"id" validate:"required,max=36"`
	Name                   string        `json:"name" validate:"required,max=36"`
	Description            string        `json:"description"`
	Enabled                string        `json:"enabled" validate:"required,oneof=Y N"`
	TriggerConditionGroups [][]Condition `json:"trigger_condition_groups" validate:"required"`
	Actions                []Action      `json:"actions" validate:"required"`
	Remark                 string        `json:"remark" `
}

type Condition struct {
	TriggerConditionsType string     `json:"trigger_conditions_type" validate:"required"`
	TriggerSource         *string    `json:"trigger_source" validate:"omitempty"`
	TriggerParamType      *string    `json:"trigger_param_type" validate:"omitempty"`
	TriggerParam          *string    `json:"trigger_param" validate:"omitempty"`
	TriggerOperator       *string    `json:"trigger_operator" validate:"omitempty"`
	TriggerValue          *string    `json:"trigger_value" validate:"omitempty,max=1024"`
	ExecutionTime         *time.Time `json:"execution_time" validate:"omitempty"`
	ExpirationTime        *int       `json:"expiration_time" validate:"omitempty"`
	TaskType              *string    `json:"task_type" validate:"omitempty"`
	Params                *string    `json:"params" validate:"omitempty"`
}

type Action struct {
	ActionType      string `json:"action_type" validate:"omitempty"`
	ActionTarget    string `json:"action_target" validate:"omitempty"`
	ActionParamType string `json:"action_param_type" validate:"omitempty"`
	ActionParam     string `json:"action_param" validate:"omitempty"`
	ActionValue     string `json:"action_value" validate:"omitempty"`
}

type GetSceneAutomationByPageReq struct {
	Name           *string `json:"name" form:"name" validate:"omitempty"`
	DeviceId       *string `json:"device_id"  form:"device_id"  validate:"omitempty"`
	DeviceConfigId *string `json:"device_config_id"  form:"device_config_id"  validate:"omitempty"`
	PageReq
}

type GetSceneAutomationsWithAlarmByPageReq struct {
	PageReq
	DeviceId       *string `json:"device_id"  form:"device_id"  validate:"omitempty"`
	DeviceConfigId *string `json:"device_config_id" form:"device_config_id" validate:"omitempty"`
}

type DryRunSceneAutomationReq struct {
	ID                     *string       `json:"id" validate:"omitempty,max=36"`
	Name                   *string       `json:"name" validate:"omitempty,max=36"`
	Description            *string       `json:"description" validate:"omitempty"`
	Enabled                string        `json:"enabled" validate:"omitempty,oneof=Y N"`
	TriggerConditionGroups [][]Condition `json:"trigger_condition_groups" validate:"required"`
	Actions                []Action      `json:"actions" validate:"required"`
}

type SceneAutomationDryRunSummary struct {
	ConditionGroupCount int            `json:"condition_group_count"`
	ConditionCount      int            `json:"condition_count"`
	ActionCount         int            `json:"action_count"`
	ConditionTypes      map[string]int `json:"condition_types"`
	ActionTypes         map[string]int `json:"action_types"`
	TargetKinds         map[string]int `json:"target_kinds"`
}

type SceneAutomationDryRunDiagnostic struct {
	Severity string `json:"severity"`
	Scope    string `json:"scope"`
	Message  string `json:"message"`
}

// SceneAutomationDryRunTraceStep 描述预演执行 trace 中的单个有序步骤。
// 该 trace 是对已有校验结果的重排,把触发条件与动作按真实评估顺序展开,
// 便于前端逐步呈现"会发生什么"。它不代表真实运行,只是静态推演。
type SceneAutomationDryRunTraceStep struct {
	Index      int      `json:"index"`
	Phase      string   `json:"phase"`  // trigger | action
	Kind       string   `json:"kind"`   // condition/action 的类型名
	Target     string   `json:"target"` // 被引用的资源类别
	Label      string   `json:"label"`  // 人类可读的步骤标签
	Status     string   `json:"status"` // evaluated | skipped | blocked
	Detail     string   `json:"detail"`
	Notes      []string `json:"notes"`
	GroupIndex *int     `json:"group_index,omitempty"`
}

// SceneAutomationDryRunTrace 是有序执行推演,phase 顺序为先触发后动作。
type SceneAutomationDryRunTrace struct {
	Steps        []SceneAutomationDryRunTraceStep `json:"steps"`
	StepCount    int                              `json:"step_count"`
	EvaluatedAt  string                           `json:"evaluated_at"`
	Explanation  string                           `json:"explanation"`
	IsSimulation bool                             `json:"is_simulation"`
}

type SceneAutomationDryRunResult struct {
	Supported          bool                              `json:"supported"`
	Valid              bool                              `json:"valid"`
	CanSave            bool                              `json:"can_save"`
	Summary            string                            `json:"summary"`
	DryRun             SceneAutomationDryRunSummary      `json:"dry_run"`
	ReferenceCounts    map[string]int                    `json:"reference_counts"`
	Warnings           []string                          `json:"warnings"`
	Errors             []string                          `json:"errors"`
	BlockingErrors     []string                          `json:"blocking_errors"`
	SkippedConditions  []string                          `json:"skipped_conditions"`
	UnavailableActions []string                          `json:"unavailable_actions"`
	MatchedDevices     *int                              `json:"matched_devices,omitempty"`
	Diagnostics        []SceneAutomationDryRunDiagnostic `json:"diagnostics"`
	NextSteps          []string                          `json:"next_steps"`
	ExecutionTrace     SceneAutomationDryRunTrace        `json:"execution_trace"`
}
