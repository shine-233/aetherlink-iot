// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// operationLogLikePattern 复用包内统一的 LIKE 转义助手（like_escape.go），转义语义完全一致。
func operationLogLikePattern(value string) string {
	return ContainsLikePattern(value)
}

func GetListByPage(operationLog *model.GetOperationLogListByPageReq, userClaims *utils.UserClaims) (int64, interface{}, error) {
	var count int64
	operationLogList := make([]model.GetOperationLogListByPageRsp, 0)
	// P1 修复（2026-08-24，见 VALIDATION.md）：gen LeftJoin 改走 raw 链
	// （clone==1 根，每次链式起点均为全新 Statement），Count 与 Scan 用 Session 克隆防污染；
	// 过滤条件、JOIN 形态、投影列名、排序与分页语义与收敛前逐条一致。
	// 当前查询按登录用户租户过滤；如恢复系统管理员跨租户查询，必须补权限测试。
	base := global.DB.Table("operation_logs").
		Joins("LEFT JOIN users ON users.id = operation_logs.user_id").
		Where("operation_logs.tenant_id = ?", userClaims.TenantID)

	if operationLog.IP != nil && *operationLog.IP != "" {
		base = base.Where("operation_logs.ip LIKE ?", operationLogLikePattern(*operationLog.IP))
	}

	if operationLog.Method != nil && *operationLog.Method != "" {
		base = base.Where("operation_logs.name = ?", *operationLog.Method)
	}

	if operationLog.Path != nil && *operationLog.Path != "" {
		base = base.Where("operation_logs.path LIKE ?", operationLogLikePattern(*operationLog.Path))
	}

	if operationLog.StartTime != nil && operationLog.EndTime != nil {
		base = base.Where("operation_logs.created_at BETWEEN ? AND ?", *operationLog.StartTime, *operationLog.EndTime)
	}

	if operationLog.UserName != nil && *operationLog.UserName != "" {
		base = base.Where("users.name LIKE ?", operationLogLikePattern(*operationLog.UserName))
	}

	// TB-10 实体级审计筛选（127.sql）：动作精确匹配，实体类型/ID 精确匹配。
	// 存量行（127.sql 之前）这些列为 NULL，带筛选时自然不命中，符合"筛选旧数据为空"预期。
	if operationLog.Action != nil && *operationLog.Action != "" {
		base = base.Where("operation_logs.action = ?", *operationLog.Action)
	}

	if operationLog.EntityType != nil && *operationLog.EntityType != "" {
		base = base.Where("operation_logs.entity_type = ?", *operationLog.EntityType)
	}

	if operationLog.EntityID != nil && *operationLog.EntityID != "" {
		base = base.Where("operation_logs.entity_id = ?", *operationLog.EntityID)
	}

	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		logrus.Error(err)
		return count, operationLogList, err
	}

	// 分页收编（2026-09-28）：旧写法 Page=0 时不加 LIMIT、PageSize 无上限；applyListPagination
	// 对缺省分页兜底 defaultListLimit 并由 clampListPageSize 封顶单页。
	listBuilder := applyListPagination(base.Session(&gorm.Session{}).
		Select("operation_logs.*, users.name AS user_name, users.email AS email").
		Order("operation_logs.created_at DESC"), operationLog.Page, operationLog.PageSize)
	if err := listBuilder.Scan(&operationLogList).Error; err != nil {
		logrus.Error(err)
		return count, operationLogList, err
	}

	return count, operationLogList, nil
}

func DeleteOperationLogsByTime(t time.Time) error {
	_, err := query.OperationLog.Where(query.OperationLog.CreatedAt.Lte(t)).Delete()
	return err
}

// operationLogExportColumns 是审计导出 CSV 实际使用的列。此前 Find 走 SELECT *，
// 会把 request_message/response_message（text 载荷）随至多 10 万行一起读入内存，
// 与下方"载荷列刻意不取"的约定不符；显式投影后两列保持 nil，service 层本就不输出它们。
var operationLogExportColumns = []string{
	"id", "created_at", "user_id", "tenant_id", "ip", "path", "name",
	"action", "entity_type", "entity_id", "status_code", "latency", "remark",
}

// ListOperationLogsForExport 按时间窗读取租户操作日志（P3 审计导出，CSV 渲染用）。
// 只投影导出列所需的字段级结构；message 载荷列刻意不取——审计最小化（见 service 层注释）。
// TB-10（127.sql）：filters.Action/EntityType/EntityID 为可选实体级筛选，空串/nil 不收窄结果。
func ListOperationLogsForExport(tenantID string, start, end time.Time, filters model.AuditLogExportReq, limit int) ([]model.OperationLog, error) {
	if limit <= 0 || limit > 200000 {
		limit = 100000
	}
	query := global.DB.Table("operation_logs").
		Where("tenant_id = ? AND created_at >= ? AND created_at < ?", tenantID, start, end)
	if filters.Action != nil && *filters.Action != "" {
		query = query.Where("action = ?", *filters.Action)
	}
	if filters.EntityType != nil && *filters.EntityType != "" {
		query = query.Where("entity_type = ?", *filters.EntityType)
	}
	if filters.EntityID != nil && *filters.EntityID != "" {
		query = query.Where("entity_id = ?", *filters.EntityID)
	}
	rows := make([]model.OperationLog, 0, 512)
	err := query.
		Select(operationLogExportColumns).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}
