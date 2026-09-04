// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"context"

	"github.com/sirupsen/logrus"
)

// GetSceneAutomationLog 分页返回指定场景在作用域内的执行日志（ROADMAP C2 自上而下读）。
// scopes 语义：0→fail-closed 空结果、1→tenant_id =（与旧单租户等价）、>1→tenant_id IN；
// 日志行以其执行时的租户归属存储，作用域内不含该场景租户时自然空结果（不泄露存在性）。
// tenant-scope: scopes 由 service 层展开并校验（TENANT_ADMIN/SYS_ADMIN self∪子孙；
// TENANT_USER 保持 self-only；空租户由 service 映射为 [""] 保持平台空租户旧行为）。
func GetSceneAutomationLog(req *model.GetSceneAutomationLogReq, scopes []string) (int64, []*model.SceneAutomationLog, error) {
	var count int64
	q := query.SceneAutomationLog
	queryBuilder := q.WithContext(context.Background())
	switch len(scopes) {
	case 0:
		return 0, []*model.SceneAutomationLog{}, nil
	case 1:
		queryBuilder = queryBuilder.Where(q.TenantID.Eq(scopes[0]))
	default:
		queryBuilder = queryBuilder.Where(q.TenantID.In(scopes...))
	}
	queryBuilder = queryBuilder.Where(q.SceneAutomationID.Eq(req.SceneAutomationId))

	if req.ExecutionResult != nil {
		queryBuilder = queryBuilder.Where(q.ExecutionResult.Eq(*req.ExecutionResult))
	}

	if req.ExecutionStartTime != nil && req.ExecutionEndTime != nil {
		queryBuilder = queryBuilder.Where(q.ExecutedAt.Between(*req.ExecutionStartTime, *req.ExecutionEndTime))
	}

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	queryBuilder = applyListPagination(queryBuilder, req.Page, req.PageSize)

	logList, err := queryBuilder.Order(q.ExecutedAt.Desc()).Find()
	if err != nil {
		return count, logList, err
	}
	return count, logList, err

}

func SceneAutomationLogInsert(data *model.SceneAutomationLog) error {
	err := query.SceneAutomationLog.Create(data)
	if err != nil {
		return err
	}
	return err
}
