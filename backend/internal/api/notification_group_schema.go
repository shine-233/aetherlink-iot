// notification_group_schema.go 定义通知组 API 的响应结构。
// 核心职责：
// 1. 统一描述通知组单条详情、分页列表、创建/更新/删除等接口的输出模型。
// 2. 为 Gin handler、Swagger 注解和前后端契约提供可复用的结构体定义。
// 3. 将通知组基础字段与列表包装结构集中收口，避免不同接口各自复制响应模型。
// 上下游关系：
// 1. 上游由 notification_group.go 等 handler 使用，用于声明接口返回体。
// 2. 下游通常被前端通知分组页面、自动化通知配置页以及 OpenAPI/Swagger 文档消费。
// 静态审查建议：
// 1. 当前 schema 主要覆盖基础字段，若后续通知组需要包含渠道明细、成员统计或运行态指标，建议新增专门视图模型而不是直接膨胀基础结构。
// 2. `NotificationConfig` 仍以 `*string` 暴露，说明配置体可能是 JSON 字符串；若后续前后端都需要稳定字段级访问，可评估收敛为显式配置对象。
// 3. `Create`、`Get`、`Update` 响应结构存在一定重复，后续如同类 schema 增多，可考虑抽公共泛型响应包装模式。
package api

import "time"

// ReadNotificationGroupOutSchema 表示通知组完整详情视图。
// 它既用于单条详情，也作为分页列表中的元素结构。
type ReadNotificationGroupOutSchema struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	NotificationType   string    `json:"notification_type"`
	Status             string    `json:"status"`
	NotificationConfig *string   `json:"notification_config"`
	Description        *string   `json:"description"`
	TenantID           string    `json:"tenant_id"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	Remark             *string   `json:"remark"`
}

// CreateNotificationGroupResponse 描述创建通知组后的标准响应包装。
type CreateNotificationGroupResponse struct {
	Code    int                            `json:"code" example:"200"`
	Message string                         `json:"message" example:"success"`
	Data    ReadNotificationGroupOutSchema `json:"data"`
}

// GetNotificationGroupResponse 描述单条通知组详情响应。
type GetNotificationGroupResponse struct {
	Code    int                            `json:"code"`
	Message string                         `json:"message"`
	Data    ReadNotificationGroupOutSchema `json:"data"`
}

// UpdateNotificationGroupResponse 描述更新通知组后的响应。
type UpdateNotificationGroupResponse struct {
	Code    int                              `json:"code"`
	Message string                           `json:"message"`
	Data    UpdateNotificationGroupOutSchema `json:"data"`
}

// UpdateNotificationGroupOutSchema 描述更新后返回的通知组视图。
// 当前使用可选字段表达“哪些字段可能被本次更新影响”。
type UpdateNotificationGroupOutSchema struct {
	Name               *string   `json:"name"`
	NotificationType   *string   `json:"notification_type"`
	Status             *string   `json:"status"`
	NotificationConfig *string   `json:"notification_config"`
	Description        *string   `json:"description"`
	Remark             *string   `json:"remark"`
	UpdatedAt          time.Time `json:"updated_at"`
	TenantID           string    `json:"tenant_id"`
}

// DeleteNotificationGroupResponse 描述删除通知组后的基础响应。
type DeleteNotificationGroupResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// GetNotificationGroupListByPageResponse 描述通知组分页列表响应。
type GetNotificationGroupListByPageResponse struct {
	Code    int                                     `json:"code"`
	Message string                                  `json:"message"`
	Data    GetNotificationGroupListByPageOutSchema `json:"data"`
}

// GetNotificationGroupListByPageOutSchema 封装分页总数和通知组列表。
type GetNotificationGroupListByPageOutSchema struct {
	Total int                              `json:"total"`
	List  []ReadNotificationGroupOutSchema `json:"list"`
}
