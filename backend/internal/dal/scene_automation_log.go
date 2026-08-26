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

func GetSceneAutomationLog(req *model.GetSceneAutomationLogReq, tenantId string) (int64, []*model.SceneAutomationLog, error) {
	var count int64
	q := query.SceneAutomationLog
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.SceneAutomationID.Eq(req.SceneAutomationId))
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantId))

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
