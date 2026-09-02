// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"fmt"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func operationLogLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return fmt.Sprintf("%%%s%%", replacer.Replace(value))
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

	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		logrus.Error(err)
		return count, operationLogList, err
	}

	listBuilder := base.Session(&gorm.Session{}).
		Select("operation_logs.*, users.name AS user_name, users.email AS email").
		Order("operation_logs.created_at DESC")
	if operationLog.Page != 0 && operationLog.PageSize != 0 {
		listBuilder = listBuilder.Limit(operationLog.PageSize).
			Offset((operationLog.Page - 1) * operationLog.PageSize)
	}
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
