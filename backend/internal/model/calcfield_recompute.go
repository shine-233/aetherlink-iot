package model

import "time"

// PHASE-D-D4 BEGIN 历史重算任务模型(70.sql)

// CalcfieldRecomputeTask 历史重算任务:对指定字段+设备的时间范围回放重算。
// 幂等语义:重算为确定性函数(同输入同输出),重复执行结果一致。
type CalcfieldRecomputeTask struct {
	ID        string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID  string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	FieldID   string    `gorm:"column:field_id;not null" json:"field_id"`
	DeviceID  string    `gorm:"column:device_id;not null" json:"device_id"`
	FromTS    int64     `gorm:"column:from_ts;not null" json:"from_ts"`
	ToTS      int64     `gorm:"column:to_ts;not null" json:"to_ts"`
	Status    string    `gorm:"column:status;not null;default:pending" json:"status"` // pending/running/done/failed
	Processed int64     `gorm:"column:processed;not null" json:"processed"`
	Emitted   int64     `gorm:"column:emitted;not null" json:"emitted"`
	ErrorMsg  *string   `gorm:"column:error_msg" json:"error_msg"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName 表名。
func (*CalcfieldRecomputeTask) TableName() string { return TableNameCalcfieldRecomputeTask }

// TableNameCalcfieldRecomputeTask 表名常量(70.sql)。
const TableNameCalcfieldRecomputeTask = "calcfield_recompute_tasks"

// 重算任务状态常量。
const (
	RecomputeStatusPending = "pending"
	RecomputeStatusRunning = "running"
	RecomputeStatusDone    = "done"
	RecomputeStatusFailed  = "failed"
)

// PHASE-D-D4 END
