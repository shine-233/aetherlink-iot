// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"github.com/sirupsen/logrus"
)

func GetTelemetrySetLogsListByPage(req *model.GetTelemetrySetLogsListByPageReq) (int64, []map[string]interface{}, error) {

	var count int64
	q := query.TelemetrySetLog
	queryBuilder := q.WithContext(context.Background())

	queryBuilder = queryBuilder.Where(q.DeviceID.Eq(req.DeviceId))

	if req.Status != nil {
		queryBuilder = queryBuilder.Where(q.Status.Eq(*req.Status))
	}
	if req.OperationType != nil {
		queryBuilder = queryBuilder.Where(q.OperationType.Eq(*req.OperationType))
	}

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	if req.Page != 0 && req.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(req.PageSize)
		queryBuilder = queryBuilder.Offset((req.Page - 1) * req.PageSize)
	}

	queryBuilder = queryBuilder.LeftJoin(query.User, q.UserID.EqCol(query.User.ID))

	queryBuilder = queryBuilder.Order(q.CreatedAt.Desc())

	list := make([]map[string]interface{}, 0)

	err = queryBuilder.Select(q.ALL, query.User.Name.As("username")).Scan(&list)
	if err != nil {
		logrus.Error(err)
		return count, list, err
	}

	return count, list, nil

}

type TelemetrySetLogsQuery struct {
}

func (TelemetrySetLogsQuery) Create(ctx context.Context, info *model.TelemetrySetLog) (id string, err error) {
	telemetry := query.TelemetrySetLog

	err = telemetry.WithContext(ctx).Create(info)
	if err != nil {
		logrus.Error("[TelemetrySetLogsQuery]create failed:", err)
	}
	return info.ID, err
}

// 删除下发历史数据，带事务
func DeleteTelemetrySetLogsByDeviceId(deviceId string, tx *query.QueryTx) error {
	_, err := tx.TelemetrySetLog.WithContext(context.Background()).Where(query.TelemetrySetLog.DeviceID.Eq(deviceId)).Delete()
	return err
}

// GetTelemetrySetLogByID 根据日志ID查询遥测下发日志
func GetTelemetrySetLogByID(logID string) (*model.TelemetrySetLog, error) {
	q := query.TelemetrySetLog
	log, err := q.WithContext(context.Background()).
		Where(q.ID.Eq(logID)).
		First()
	if err != nil {
		logrus.Error("[GetTelemetrySetLogByID] query failed:", err)
		return nil, err
	}
	return log, nil
}

// UpdateTelemetrySetLog 更新遥测下发日志
func UpdateTelemetrySetLog(log *model.TelemetrySetLog) error {
	q := query.TelemetrySetLog
	_, err := q.WithContext(context.Background()).
		Where(q.ID.Eq(log.ID)).
		Updates(log)
	if err != nil {
		logrus.Error("[UpdateTelemetrySetLog] update failed:", err)
	}
	return err
}
