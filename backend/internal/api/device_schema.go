// 文件用途：集中定义物模型相关接口复用的响应 Schema，供 API 文档与返回结构声明使用。
// 核心链路：service 或 handler 将物模型聚合结果填充到本文件结构体中，再由统一响应层序列化返回前端。
// 使用注意：本文件承担“外部契约”角色，字段名、可空性和 example 标签会直接影响接口兼容性与文档表现。
// 静态审查建议：重点核对指针字段是否真实表达“可空/可缺省”语义，时间字段序列化格式是否满足前端约定，
// 以及配置类字符串字段是否需要额外的 JSON 合法性或大小约束说明，避免文档与真实返回不一致。
package api

import "time"

// DeviceTemplateReadSchema 描述单个物模型的读取视图，是列表与详情接口共享的数据载体。
type DeviceTemplateReadSchema struct {
	ID                string    `json:"id"`          // Id
	Name              string    `json:"name"`        // 物模型名称
	Author            *string   `json:"author"`      // 作者
	Version           *string   `json:"version"`     // 版本号
	Description       *string   `json:"description"` // 描述
	TenantID          string    `json:"tenant_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Flag              *int16    `json:"flag" example:"1"`    // 标志 默认1
	Label             *string   `json:"label"`               // 标签
	DeviceModelConfig *string   `json:"device_model_config"` // 物模型配置
	WebChartConfig    *string   `json:"web_chart_config"`    // web图表配置
	AppChartConfig    *string   `json:"app_chart_config"`    // app图表配置
	TypeKey           *string   `json:"type_key"`            // 行业类型（模板分类目录）
}

// GetDeviceTemplateListResponse 描述物模型列表接口的顶层响应结构。
type GetDeviceTemplateListResponse struct {
	Code    int                       `json:"code" example:"200"`
	Message string                    `json:"message" example:"success"`
	Data    GetDeviceTemplateListData `json:"data"`
}

// GetDeviceTemplateListData 描述分页列表中的总数与模板集合。
type GetDeviceTemplateListData struct {
	Total int64                      `json:"total"`
	List  []DeviceTemplateReadSchema `json:"list"`
}

// GetDeviceTemplateResponse 描述物模型详情接口的顶层响应结构。
type GetDeviceTemplateResponse struct {
	Code    int                      `json:"code" example:"200"`
	Message string                   `json:"message" example:"success"`
	Data    DeviceTemplateReadSchema `json:"data"`
}
