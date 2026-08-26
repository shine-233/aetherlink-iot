// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func GetTelemetrySetLogsListByPage(req *model.GetTelemetrySetLogsListByPageReq) (int64, []map[string]interface{}, error) {

	var count int64
	// P1 修复（2026-08-24，见 VALIDATION.md）：遥测下发日志列表改走 raw global.DB 链，
	// 杜绝包级单例 TelemetrySetLog LeftJoin(User)+ALL+Scan 在高并发下跨请求残留
	// Statement 读到空/旧数据；JOIN 形态、投影列名、排序与分页语义与收敛前逐条一致。
	base := global.DB.Table("telemetry_set_logs").
		Joins("LEFT JOIN users ON users.id = telemetry_set_logs.user_id").
		Where("telemetry_set_logs.device_id = ?", req.DeviceId)

	if req.Status != nil {
		base = base.Where("telemetry_set_logs.status = ?", *req.Status)
	}
	if req.OperationType != nil {
		base = base.Where("telemetry_set_logs.operation_type = ?", *req.OperationType)
	}

	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	listBuilder := base.Session(&gorm.Session{}).
		Select("telemetry_set_logs.*, users.name AS username").
		Order("telemetry_set_logs.created_at DESC")
	listBuilder = applyListPagination(listBuilder, req.Page, req.PageSize)
	list := make([]map[string]interface{}, 0)
	if err := listBuilder.Scan(&list).Error; err != nil {
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
