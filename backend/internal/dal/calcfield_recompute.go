package dal

import (
	"context"
	"errors"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"
)

// PHASE-D-D4 BEGIN 历史重算任务 DAL

// CreateCalcfieldRecomputeTask 建任务。
func CreateCalcfieldRecomputeTask(ctx context.Context, task *model.CalcfieldRecomputeTask) error {
	if global.DB == nil {
		return errCalcfieldDBNotReady
	}
	return global.DB.WithContext(ctx).Table(model.TableNameCalcfieldRecomputeTask).Create(task).Error
}

// GetCalcfieldRecomputeTask 按 ID 读取(tenant 守卫)。
// tenant-scope: tenant_id 硬过滤。
func GetCalcfieldRecomputeTask(ctx context.Context, tenantID, id string) (*model.CalcfieldRecomputeTask, error) {
	if global.DB == nil {
		return nil, errCalcfieldDBNotReady
	}
	var row model.CalcfieldRecomputeTask
	err := global.DB.WithContext(ctx).
		Table(model.TableNameCalcfieldRecomputeTask).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListCalcfieldRecomputeTasks 租户任务列表(最近 50 条)。
// tenant-scope: tenant_id 硬过滤。
func ListCalcfieldRecomputeTasks(ctx context.Context, tenantID string) ([]model.CalcfieldRecomputeTask, error) {
	if global.DB == nil {
		return nil, errCalcfieldDBNotReady
	}
	rows := make([]model.CalcfieldRecomputeTask, 0)
	err := global.DB.WithContext(ctx).
		Table(model.TableNameCalcfieldRecomputeTask).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(50).
		Find(&rows).Error
	return rows, err
}

// UpdateCalcfieldRecomputeTask 回写状态/进度。
func UpdateCalcfieldRecomputeTask(ctx context.Context, task *model.CalcfieldRecomputeTask) error {
	if global.DB == nil {
		return errCalcfieldDBNotReady
	}
	task.UpdatedAt = time.Now()
	return global.DB.WithContext(ctx).Table(model.TableNameCalcfieldRecomputeTask).Save(task).Error
}

// HistorySamplesByRange 历史源 DAL 实现:telemetry_datas 数值样本(telemetry_datas.go 列结构)。
// tenant-scope: tenant_id 硬过滤(列可空,兼容 NULL 行按 IS NULL OR =)。
func HistorySamplesByRange(ctx context.Context, tenantID, deviceID, key string, fromTS, toTS int64) ([]model.TelemetryData, error) {
	if global.DB == nil {
		return nil, errCalcfieldDBNotReady
	}
	rows := make([]model.TelemetryData, 0)
	err := global.DB.WithContext(ctx).
		Table(model.TableNameTelemetryData).
		Where("device_id = ? AND `key` = ? AND ts >= ? AND ts < ?", deviceID, key, fromTS, toTS).
		Where("(tenant_id = ? OR tenant_id IS NULL)", tenantID).
		Where("number_v IS NOT NULL").
		Order("ts ASC").
		Find(&rows).Error
	return rows, err
}

var errCalcfieldDBNotReady = errors.New("db is not initialized")

// PHASE-D-D4 END
