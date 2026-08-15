// notification_histories_schema.go 定义通知历史 API 的响应结构。
// 核心职责：
// 1. 统一描述通知历史单条记录视图与分页列表包装，作为 handler、Swagger 和前端契约之间的共享结构。
// 2. 为“通知发送记录/通知历史列表”类接口提供稳定输出模型，避免不同接口各自复制字段定义。
// 3. 明确可空字段、时间展示字段与租户归属字段的语义边界，帮助调用方区分“没有值”和“值为空字符串”。
// 上下游关系：
// 1. 上游通常由 notification_histories 相关 handler 或 service 返回结果时引用，用来组织 HTTP 响应中的 data 部分。
// 2. 下游通常被前端通知历史列表、审计追踪页面以及 OpenAPI/Swagger 文档消费。
// 字段视图约定：
// 1. ReadNotificationHistorySchema 表示单条通知历史记录的完整读取视图，同时也作为分页列表中的元素结构复用。
// 2. 该视图更偏向“展示态”而不是“写入态”：例如 SendTime 已收敛为字符串，适合直接返回给前端展示。
// 3. 带 *string 的字段表示后端允许为空，调用方需要区分 null 与非 null 字符串，而不是默认其一定存在。
// 列表包装约定：
// 1. GetNotificationHistoryListByPageOutSchema 只负责表达分页结果本身，不额外承载 code/message 等更外层响应信封。
// 2. Total 表示满足筛选条件的总记录数，List 表示当前页实际返回的通知历史切片。
// 3. 当没有数据时，建议上游返回 total=0 且 list 为空数组，以减少前端对 null 列表的特殊分支处理。
// 静态审查建议：
// 1. SendTime 当前使用 string，说明时间格式化职责已前移到 service 或 assembler；若后续出现多时区、多格式需求，建议补充格式约束注释或改为显式时间字段加展示字段双轨输出。
// 2. SendContent、SendResult、Remark 采用 *string，能够表达缺省值；若后续需要更严格的接口契约，可评估哪些字段应该保证非空并在上游完成兜底。
// 3. 当前 schema 仅覆盖读取场景；若未来新增导出、详情审计或重试诊断接口，建议拆分独立视图模型，避免在基础读取结构上持续膨胀字段。
package api

// ReadNotificationHistorySchema 表示单条通知历史记录的读取视图。
// 它既可用于详情读取，也可作为分页列表中的单项元素返回给前端。
type ReadNotificationHistorySchema struct {
	ID               string  `json:"id"`                // 通知历史主键 ID。
	SendTime         string  `json:"send_time"`         // 发送时间展示值，通常已在上游完成格式化。
	SendContent      *string `json:"send_content"`      // 实际发送内容；为空表示该记录未保留内容或内容不可用。
	SendTarget       string  `json:"send_target"`       // 发送目标，例如手机号、邮箱、用户标识或其他渠道目标。
	SendResult       *string `json:"send_result"`       // 发送结果说明；可用于展示成功/失败原因或第三方返回摘要。
	NotificationType string  `json:"notification_type"` // 通知类型/渠道类型，例如短信、邮件、站内信等。
	TenantID         string  `json:"tenant_id"`         // 记录所属租户 ID，用于多租户隔离与前端定位归属。
	Remark           *string `json:"remark"`            // 备注信息；常用于补充人工说明、审计备注或异常上下文。
}

// GetNotificationHistoryListByPageOutSchema 封装通知历史分页查询结果。
// 该结构只表达列表数据本身，通常作为更外层统一响应体中的 data 字段出现。
type GetNotificationHistoryListByPageOutSchema struct {
	Total int                             `json:"total"` // 满足当前筛选条件的总记录数，而非当前页长度。
	List  []ReadNotificationHistorySchema `json:"list"`  // 当前页返回的通知历史记录列表。
}
