// 文件用途：定义设备健康度评估（Device Health Score）领域模型与 HTTP 传输对象。
// 核心逻辑：持久化模型、健康分层级枚举、汇总大盘 DTO 与单设备多维诊断评分详情。
package model

import "time"

const TableNameDeviceHealthScore = "device_health_scores"

// 健康等级枚举 (ThingsBoard PE / ThingsPanel 算法中心标准)
const (
	HealthStatusHealthy    = "HEALTHY"     // 健康 (>=85)
	HealthStatusSubHealthy = "SUB_HEALTHY" // 亚健康 (70-84)
	HealthStatusWarning    = "WARNING"     // 告警/关注 (50-69)
	HealthStatusCritical   = "CRITICAL"    // 严重危险 (<50)
)

// DeviceHealthScore 数据库持久化实体
type DeviceHealthScore struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	DeviceID       string    `gorm:"column:device_id;not null" json:"device_id"`
	TenantID       string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Score          float64   `gorm:"column:score;not null;default:100.00" json:"score"`
	HealthStatus   string    `gorm:"column:health_status;not null;default:HEALTHY" json:"health_status"`
	AlarmPenalty   float64   `gorm:"column:alarm_penalty;not null;default:0.00" json:"alarm_penalty"`
	OfflinePenalty float64   `gorm:"column:offline_penalty;not null;default:0.00" json:"offline_penalty"`
	AnomalyPenalty float64   `gorm:"column:anomaly_penalty;not null;default:0.00" json:"anomaly_penalty"`
	Details        string    `gorm:"column:details;not null;default:{}" json:"details"`
	EvaluatedAt    time.Time `gorm:"column:evaluated_at;not null" json:"evaluated_at"`
	CreatedAt      time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*DeviceHealthScore) TableName() string {
	return TableNameDeviceHealthScore
}

// DeviceHealthScoreItem 列表项与大盘排行项
type DeviceHealthScoreItem struct {
	DeviceID       string  `json:"device_id"`
	DeviceName     string  `json:"device_name"`
	DeviceNumber   string  `json:"device_number,omitempty"`
	Score          float64 `json:"score"`
	HealthStatus   string  `json:"health_status"`
	AlarmPenalty   float64 `json:"alarm_penalty"`
	OfflinePenalty float64 `json:"offline_penalty"`
	AnomalyPenalty float64 `json:"anomaly_penalty"`
	IsOnline       bool    `json:"is_online"`
	EvaluatedAt    string  `json:"evaluated_at"`
}

// DeviceHealthSummaryResp 租户设备综合健康度汇总大盘
type DeviceHealthSummaryResp struct {
	TotalDevices     int                     `json:"total_devices"`
	HealthyCount     int                     `json:"healthy_count"`
	SubHealthyCount  int                     `json:"sub_healthy_count"`
	WarningCount     int                     `json:"warning_count"`
	CriticalCount    int                     `json:"critical_count"`
	AverageScore     float64                 `json:"average_score"`
	UnhealthyDevices []DeviceHealthScoreItem `json:"unhealthy_devices"`
	EvaluatedAt      string                  `json:"evaluated_at"`
}

// DeviceHealthDetailResp 单设备健康度多维诊断剖析
type DeviceHealthDetailResp struct {
	DeviceID               string                   `json:"device_id"`
	DeviceName             string                   `json:"device_name"`
	DeviceNumber           string                   `json:"device_number"`
	Score                  float64                  `json:"score"`
	HealthStatus           string                   `json:"health_status"`
	AlarmPenalty           float64                  `json:"alarm_penalty"`
	OfflinePenalty         float64                  `json:"offline_penalty"`
	AnomalyPenalty         float64                  `json:"anomaly_penalty"`
	IsOnline               bool                     `json:"is_online"`
	OfflineDurationSeconds int64                    `json:"offline_duration_seconds"`
	ActiveAlarmCount       int                      `json:"active_alarm_count"`
	ActiveAlarms           []map[string]interface{} `json:"active_alarms"`
	Suggestions            []string                 `json:"suggestions"`
	EvaluatedAt            string                   `json:"evaluated_at"`
	// MSET TP-21 多元状态估计特征维度。配置开关（health.mset.enabled，默认关）未开启时恒为 nil，
	// 报文形状与既有契约完全兼容；开启后降级路径也会带上 degraded 与原因。
	MSET *DeviceHealthMSETFeature `json:"mset,omitempty"`
}

// DeviceHealthMSETFeature TP-21 MSET（多元状态估计）特征维度在健康评分里的投影。
// 评分口径：偏差分 × 权重折入 anomaly_penalty（health_scores 表无独立列，本项不做迁移），
// 明细分解随 details JSON 与本对象留痕；降级（degraded=true）时偏差分恒为 0（中性），并记录原因。
type DeviceHealthMSETFeature struct {
	Applied        bool     `json:"applied"`                  // 是否完成一次有效推理
	Degraded       bool     `json:"degraded"`                 // fail-closed 降级（未产生有效判定）
	DegradeReason  string   `json:"degrade_reason,omitempty"` // cold_start | insufficient_samples | singular_matrix | ...
	DeviationScore float64  `json:"deviation_score"`          // 0~100 偏差评分（0=贴合历史基线）
	Penalty        float64  `json:"penalty"`                  // 折入 anomaly_penalty 的扣减（偏差分×权重）
	Mahalanobis    float64  `json:"mahalanobis,omitempty"`    // 马氏距离 d（非平方），仅有效推理时出现
	FeatureKeys    []string `json:"feature_keys,omitempty"`   // 参与训练/推理的特征键（按列序）
	TrainSamples   int      `json:"train_samples,omitempty"`  // 训练历史完整样本行数
}

// EvaluateDeviceHealthReq 手动或定时触发健康评估入参
type EvaluateDeviceHealthReq struct {
	DeviceID *string `json:"device_id" form:"device_id"`
}
