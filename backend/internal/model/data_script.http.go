// 文件用途：定义 data script 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

import "time"

type CreateDataScriptReq struct {
	Name            string  `json:"name" validate:"required,max=99"`
	DeviceConfigId  string  `json:"device_config_id"  validate:"required,max=36"`
	Content         *string `json:"content" validate:"omitempty"`
	ScriptType      string  `json:"script_type" validate:"required,oneof=A B C D E F H"`
	LastAnalogInput *string `json:"last_analog_input" validate:"omitempty"`
	Description     *string `json:"description" validate:"omitempty,max=255"`
	Remark          *string `json:"remark" validate:"omitempty,max=255"`
}

type UpdateDataScriptReq struct {
	Id              string     `json:"id" validate:"required,max=36"` // Id
	Name            string     `json:"name" validate:"required,max=99"`
	DeviceConfigId  string     `json:"device_config_id"  validate:"required,max=36"`
	Content         *string    `json:"content" validate:"omitempty"`
	ScriptType      string     `json:"script_type" validate:"required,oneof=A B C D E F H"` //  A-遥测上报预处理B-遥测下发预处理C-属性上报预处理D-属性下发预处理 E-命令下发预处理 F-事件上报预处理 H-事件下发预处理
	LastAnalogInput *string    `json:"last_analog_input" validate:"omitempty"`
	Description     *string    `json:"description" validate:"omitempty,max=255"`
	Remark          *string    `json:"remark" validate:"omitempty,max=255"`
	UpdatedAt       *time.Time `json:"updated_at" validate:"omitempty"`
}

type GetDataScriptListByPageReq struct {
	PageReq
	DeviceConfigId *string `json:"device_config_id" form:"device_config_id" validate:"required,max=36"`
	ScriptType     *string `json:"script_type" form:"script_type" validate:"omitempty"`
}

type QuizDataScriptReq struct {
	Content     string `json:"content" validate:"required,max=20000"`
	AnalogInput string `json:"last_analog_input" validate:"omitempty,max=8192"`
	Topic       string `json:"topic" validate:"omitempty,max=255"`
}

type EnableDataScriptReq struct {
	Id         string `json:"id" validate:"required,max=36"`
	EnableFlag string `json:"enable_flag" validate:"required,oneof=Y N"`
}
