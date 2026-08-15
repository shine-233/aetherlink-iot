// 文件用途：定义 device model custom control 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

type CreateDeviceModelCustomControlReq struct {
	DeviceTemplateId string  `json:"device_template_id" validate:"required,max=36"` // 设备物模型ID
	Name             string  `json:"name" validate:"required,max=36"`               // 名称
	ControlType      string  `json:"control_type" validate:"required,max=50"`       // 控制类型
	Description      *string `json:"description" validate:"omitempty,max=500"`      // 描述
	Content          *string `json:"content" validate:"omitempty"`                  // 指令内容
	EnableStatus     string  `json:"enable_status" validate:"required,max=10"`      // 启用状态
	Remark           *string `json:"remark" validate:"omitempty,max=255"`           // 备注
}

type UpdateDeviceModelCustomControlReq struct {
	ID               string  `json:"id" validate:"required,max=36"`                  // ID
	DeviceTemplateId *string `json:"device_template_id" validate:"omitempty,max=36"` // 设备物模型ID
	Name             *string `json:"name" validate:"omitempty,max=36"`               // 名称
	ControlType      *string `json:"control_type" validate:"omitempty,max=50"`       // 控制类型
	Description      *string `json:"description" validate:"omitempty,max=500"`       // 描述
	Content          *string `json:"content" validate:"omitempty"`                   // 指令内容
	EnableStatus     *string `json:"enable_status" validate:"omitempty,max=10"`      // 启用状态
	Remark           *string `json:"remark" validate:"omitempty,max=255"`            // 备注
}
