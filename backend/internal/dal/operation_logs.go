// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"fmt"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

func operationLogLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return fmt.Sprintf("%%%s%%", replacer.Replace(value))
}

func GetListByPage(operationLog *model.GetOperationLogListByPageReq, userClaims *utils.UserClaims) (int64, interface{}, error) {
	q := query.OperationLog
	var count int64
	operationLogList := make([]model.GetOperationLogListByPageRsp, 0)
	var queryBuilder query.IOperationLogDo
	// 当前查询按登录用户租户过滤；如恢复系统管理员跨租户查询，必须补权限测试。
	// if userClaims.Authority != "SYS_ADMIN" {
	// 	queryBuilder = q.WithContext(context.Background()).Where(q.TenantID.Eq(operationLog.TenantID))

	// }
	queryBuilder = q.WithContext(context.Background()).Where(q.TenantID.Eq(userClaims.TenantID))
	if operationLog.IP != nil && *operationLog.IP != "" {
		queryBuilder = queryBuilder.Where(q.IP.Like(operationLogLikePattern(*operationLog.IP)))
	}

	if operationLog.Method != nil && *operationLog.Method != "" {
		queryBuilder = queryBuilder.Where(q.Name.Eq(*operationLog.Method))
	}

	if operationLog.Path != nil && *operationLog.Path != "" {
		queryBuilder = queryBuilder.Where(q.Path.Like(operationLogLikePattern(*operationLog.Path)))
	}

	if operationLog.StartTime != nil && operationLog.EndTime != nil {
		queryBuilder = queryBuilder.Where(q.CreatedAt.Between(*operationLog.StartTime, *operationLog.EndTime))
	}

	u := query.User
	queryBuilder = queryBuilder.LeftJoin(u, u.ID.EqCol(q.UserID))
	if operationLog.UserName != nil && *operationLog.UserName != "" {
		queryBuilder = queryBuilder.Where(u.Name.Like(operationLogLikePattern(*operationLog.UserName)))
	}

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, operationLogList, err
	}

	if operationLog.Page != 0 && operationLog.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(operationLog.PageSize)
		queryBuilder = queryBuilder.Offset((operationLog.Page - 1) * operationLog.PageSize)
	}

	err = queryBuilder.Select(q.ALL, u.Name.As("user_name"), u.Email).
		Order(q.CreatedAt.Desc()).
		Scan(&operationLogList)
	if err != nil {
		logrus.Error(err)
		return count, operationLogList, err
	}

	return count, operationLogList, err
}

func DeleteOperationLogsByTime(t time.Time) error {
	_, err := query.OperationLog.Where(query.OperationLog.CreatedAt.Lte(t)).Delete()
	return err
}
