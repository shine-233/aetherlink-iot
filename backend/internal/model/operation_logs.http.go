// 文件用途：定义 operation logs 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

import "time"

type CreateOperationLogReq struct {
	IP              string  `json:"ip" validate:"required,max=36" `                 // 请求IP
	Path            *string `json:"path" validate:"omitempty,max=2000" `            // 请求url
	UserID          string  `json:"user_id" validate:"required,max=36"`             // 操作用户
	Name            *string `json:"name" validate:"omitempty,max=255" `             // 接口名称
	Latency         int64   `json:"latency" validate:"omitempty"`                   // 耗时(ms)
	RequestMessage  *string `json:"request_message" validate:"omitempty,max=2000"`  // 请求内容
	ResponseMessage *string `json:"response_message" validate:"omitempty,max=2000"` // 响应内容
	TenantID        string  `json:"tenant_id" validate:"required,max=36"`           // 租户id
	Remark          *string `json:"remark" validate:"omitempty,max=255"`
}

type GetOperationLogListByPageReq struct {
	PageReq
	IP         *string    `json:"ip" form:"ip" validate:"omitempty,max=36"`                    // 请求IP
	StartTime  *time.Time `json:"start_time,omitempty" form:"start_time" validate:"omitempty"` // 开始日期
	EndTime    *time.Time `json:"end_time,omitempty" form:"end_time" validate:"omitempty"`     // 结束日期
	UserName   *string    `json:"username" form:"username" validate:"omitempty,max=255"`
	Method     *string    `json:"method" form:"method" validate:"omitempty,max=255"`
	Path       *string    `json:"path" form:"path" validate:"omitempty,max=2000"`
	Action     *string    `json:"action" form:"action" validate:"omitempty,max=32"`           // 实体级动作（TB-10，127.sql）
	EntityType *string    `json:"entity_type" form:"entity_type" validate:"omitempty,max=64"` // 审计实体类型
	EntityID   *string    `json:"entity_id" form:"entity_id" validate:"omitempty,max=36"`     // 审计实体ID
}

type GetOperationLogListByPageRsp struct {
	ID              string     `json:"id" `               // 主键
	IP              string     `json:"ip" `               // 请求IP
	Path            *string    `json:"path" `             // 请求url
	UserID          string     `json:"user_id" `          // 操作用户
	Name            *string    `json:"name" `             // 接口名称
	Latency         int64      `json:"latency" `          // 耗时(ms)
	RequestMessage  *string    `json:"request_message" `  // 请求内容
	ResponseMessage *string    `json:"response_message" ` // 响应内容
	TenantID        string     `json:"tenant_id" `        // 租户id
	CreatedAt       *time.Time `json:"created_at" `       // 创建时间
	Remark          *string    `json:"remark" `           // 备注
	UserName        *string    `json:"username"`          // 用户名
	Email           *string    `json:"email"`             // 邮箱
	Action          *string    `json:"action"`            // 实体级动作（create/update/delete/read/other；127.sql 前存量行为空）
	EntityType      *string    `json:"entity_type"`       // 审计实体类型（自请求路径解析）
	EntityID        *string    `json:"entity_id"`         // 审计实体ID（自请求路径解析）
	StatusCode      *int32     `json:"status_code"`       // HTTP响应状态码
}

// AuditLogExportReq 操作日志导出请求（P3 审计导出）。
// StartTime/EndTime 必填：审计导出必须有界，静默全量导出等于把审计表变成下载站。
// Action/EntityType/EntityID 为可选实体级筛选（TB-10，127.sql）：缺省不缩小窗口范围。
type AuditLogExportReq struct {
	StartTime  *time.Time `json:"start_time" form:"start_time" validate:"omitempty"`
	EndTime    *time.Time `json:"end_time" form:"end_time" validate:"omitempty"`
	Action     *string    `json:"action" form:"action" validate:"omitempty,max=32"`
	EntityType *string    `json:"entity_type" form:"entity_type" validate:"omitempty,max=64"`
	EntityID   *string    `json:"entity_id" form:"entity_id" validate:"omitempty,max=36"`
}
