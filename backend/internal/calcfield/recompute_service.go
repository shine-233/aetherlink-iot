// 文件用途:计算字段历史重算任务服务(PHASE-D-D4)。
// 核心逻辑:创建任务(校验字段归属租户+类型可重算)→ goroutine 异步回放(进度回写)→ 查询。
// 放置说明:放 calcfield 包规避 service→calcfield→uplink→service 的导入环;API 层直接调用本包。
// 关键注意事项:字段必须属调用者租户;重放确定性保证幂等;派生写回经 StorageEnqueuer seam
// (app 装配注入平台 storage 入口,未注入时静默丢弃并记录于任务 emitted=0)。
package calcfield

import (
	"context"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
)

// RecomputeService 历史重算任务管理。
type RecomputeService struct{}

// StorageSink 派生写回缝:app 装配注入平台 storage 入口(默认 nil → 任务跑完 emitted=0)。
var RecomputeStorageSink StorageEnqueuer

// CreateRecomputeTask 创建并启动重算任务。
func (*RecomputeService) CreateRecomputeTask(fieldID, deviceID string, fromTS, toTS int64, claims *utils.UserClaims) (*model.CalcfieldRecomputeTask, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant scope")
	}
	if fromTS <= 0 || toTS <= fromTS {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "invalid time range")
	}
	field, err := dal.GetCalculatedFieldForScope(fieldID, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if field == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "calculated field not found")
	}
	fieldType := field.Type
	if fieldType == "" {
		fieldType = FieldTypeSimple
	}
	switch fieldType {
	case FieldTypeSimple, FieldTypeTimeseries, FieldTypeGeofence, FieldTypePropagation:
	default:
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "recompute unsupported for this field type")
	}

	task := &model.CalcfieldRecomputeTask{
		ID:       uuid.New(),
		TenantID: claims.TenantID,
		FieldID:  fieldID,
		DeviceID: deviceID,
		FromTS:   fromTS,
		ToTS:     toTS,
		Status:   model.RecomputeStatusPending,
	}
	if err := dal.CreateCalcfieldRecomputeTask(context.Background(), task); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}

	go runRecomputeTask(task, *field)
	return task, nil
}

// runRecomputeTask 回放执行体(独立 goroutine)。
func runRecomputeTask(task *model.CalcfieldRecomputeTask, field model.CalculatedField) {
	ctx := context.Background()
	task.Status = model.RecomputeStatusRunning
	_ = dal.UpdateCalcfieldRecomputeTask(ctx, task)

	source := &dalHistorySource{}
	report := func(processed, emitted int64) {
		task.Processed = processed
		task.Emitted = emitted
	}
	rule := FieldRule{
		ID:         field.ID,
		OutputKey:  field.OutputKey,
		Expression: field.Expression,
		Type:       field.Type,
		Config:     field.Config,
	}
	var sink StorageEnqueuer
	if RecomputeStorageSink != nil {
		sink = RecomputeStorageSink
	}
	_, _, err := RecomputeRange(ctx, rule, task.DeviceID, task.TenantID, task.FromTS, task.ToTS, source, sink, report)
	if err != nil {
		task.Status = model.RecomputeStatusFailed
		msg := err.Error()
		task.ErrorMsg = &msg
	} else {
		task.Status = model.RecomputeStatusDone
	}
	_ = dal.UpdateCalcfieldRecomputeTask(ctx, task)
}

// GetRecomputeTask 任务详情。
func (*RecomputeService) GetRecomputeTask(id string, claims *utils.UserClaims) (*model.CalcfieldRecomputeTask, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant scope")
	}
	task, err := dal.GetCalcfieldRecomputeTask(context.Background(), claims.TenantID, id)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "task not found")
	}
	return task, nil
}

// ListRecomputeTasks 任务列表。
func (*RecomputeService) ListRecomputeTasks(claims *utils.UserClaims) ([]model.CalcfieldRecomputeTask, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant scope")
	}
	rows, err := dal.ListCalcfieldRecomputeTasks(context.Background(), claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return rows, nil
}

// dalHistorySource dal 历史源适配(租户守卫在 SQL)。
type dalHistorySource struct{}

func (dalHistorySource) ListRange(ctx context.Context, tenantID, deviceID, key string, fromTS, toTS int64) ([]HistorySample, error) {
	rows, err := dal.HistorySamplesByRange(ctx, tenantID, deviceID, key, fromTS, toTS)
	if err != nil {
		return nil, err
	}
	samples := make([]HistorySample, 0, len(rows))
	for _, row := range rows {
		if row.NumberV != nil {
			samples = append(samples, HistorySample{TS: row.T, Value: *row.NumberV})
		}
	}
	return samples, nil
}

var _ = time.Now

// RecomputeSvc 服务实例(API 层入口)。
var RecomputeSvc = &RecomputeService{}
