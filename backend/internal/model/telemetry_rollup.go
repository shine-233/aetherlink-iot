// 文件用途：P2.3 遥测降采样（冷层）的数据模型。
// 核心逻辑：telemetry_rollups 按 (设备, 键, 桶宽, 桶起点) 汇总原始遥测的
// min/max/avg/count/last，作为保留策略删掉原始数据后的冷层查询来源。
// 关键注意事项：
//   - 降采样是**加法**：原始行保留到保留策略到期，rollup 只多不少——
//     汇总错了可以重跑修正，删了原始数据就真的没了。
//   - 主键含 bucket_ms：未来引入多粒度（1h/1d）时同一行集可共存，不冲突。
package model

import "time"

const TableNameTelemetryRollup = "telemetry_rollups"

// TelemetryRollup 一个分桶的汇总行。
type TelemetryRollup struct {
	DeviceID    string    `gorm:"column:device_id;primaryKey" json:"device_id"`
	Key         string    `gorm:"column:key;primaryKey" json:"key"`
	BucketMs    int64     `gorm:"column:bucket_ms;primaryKey" json:"bucket_ms"`
	BucketStart int64     `gorm:"column:bucket_start;primaryKey" json:"bucket_start"` // unix ms
	MinV        *float64  `gorm:"column:min_v" json:"min_v"`
	MaxV        *float64  `gorm:"column:max_v" json:"max_v"`
	AvgV        *float64  `gorm:"column:avg_v" json:"avg_v"`
	LastV       *float64  `gorm:"column:last_v" json:"last_v"`
	CountV      int64     `gorm:"column:count_v" json:"count_v"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (*TelemetryRollup) TableName() string { return TableNameTelemetryRollup }

// TelemetryRollupTarget 降采样作业的一个目标（一对设备/键）。
type TelemetryRollupTarget struct {
	DeviceID string `gorm:"column:device_id" json:"device_id"`
	Key      string `gorm:"column:key" json:"key"`
}
