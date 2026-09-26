// 文件用途：定义 data policy 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

type UpdateDataPolicyReq struct {
	Id            string  `json:"id" validate:"required,max=36"`
	RetentionDays int32   `json:"retention_days" validate:"required,gte=1,lte=3650"`
	Enabled       string  `json:"enabled" validate:"required,oneof=1 2"`
	Remark        *string `json:"remark" validate:"required,max=2000"`
}

// CreateDataPolicyReq 创建行级（租户/档案粒度）数据保留策略入参（TB-15R，138.sql）。
// TenantID 必填；DeviceConfigID 可空=租户级（覆盖该租户全部设备），非空=精确档案行。
// 行级只支持设备数据（data_type=1），服务层强校验；全局默认行不走本入口。
type CreateDataPolicyReq struct {
	DataType       string  `json:"data_type" validate:"required,oneof=1 2"`
	TenantID       string  `json:"tenant_id" validate:"required,max=36"`
	DeviceConfigID *string `json:"device_config_id" validate:"omitempty,max=36"`
	RetentionDays  int32   `json:"retention_days" validate:"required,gte=1,lte=3650"`
	Enabled        string  `json:"enabled" validate:"required,oneof=1 2"`
	Remark         *string `json:"remark" validate:"omitempty,max=2000"`
}

type GetDataPolicyListByPageReq struct {
	PageReq
}
