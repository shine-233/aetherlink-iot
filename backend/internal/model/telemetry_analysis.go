package model

// TelemetryAnalysisQuery 轻量分析查询（ROADMAP P2.2）。
// 放在非 .gen.go 文件中：按项目约定不手改生成产物。
type TelemetryAnalysisQuery struct {
	DeviceIDs   []string
	Key         string
	StartTime   int64
	EndTime     int64
	Granularity string
	// Aggregate 聚合方式：avg / sum / min / max / count / last。
	Aggregate string
	// Compare 对比方式：none / previous_period（环比）/ same_period_last（同比）。
	Compare string
	// CompareOffsets 同比时用于回退的周期倍数，默认 1（如"上周同段"）。
	CompareOffsets int
	Format         string
}

// TelemetryAnalysisCompareMode 对比模式常量。
const (
	TelemetryAnalysisCompareNone     = "none"
	TelemetryAnalysisComparePrevious = "previous_period"
	TelemetryAnalysisCompareSameLast = "same_period_last"
)

// TelemetryAnalysisAggregate 聚合方式常量。
const (
	TelemetryAnalysisAggAvg   = "avg"
	TelemetryAnalysisAggSum   = "sum"
	TelemetryAnalysisAggMin   = "min"
	TelemetryAnalysisAggMax   = "max"
	TelemetryAnalysisAggCount = "count"
	TelemetryAnalysisAggLast  = "last"
)

// TelemetryAnalysisFormat 导出格式常量。
const (
	TelemetryAnalysisFormatXLSX = "xlsx"
	TelemetryAnalysisFormatCSV  = "csv"
)

// TelemetryAnomalyRuleSpec 基础异常检测规则（P2.2）。
// Type=bounds 时 Min/Max 生效；Type=deviation 时 K 生效（默认 3，即 ±3σ）。
type TelemetryAnomalyRuleSpec struct {
	// Type 规则类型：bounds（静态上下限）/ deviation（均值±K倍标准差）。
	Type string   `json:"type"`
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
	K    *float64 `json:"k,omitempty"`
}

// TelemetryAnomalyQuery 异常检测查询：与分析查询同源的时间窗 + 检测规则。
type TelemetryAnomalyQuery struct {
	DeviceIDs []string
	Key       string
	StartTime int64
	EndTime   int64
	// WindowMs 序列分桶宽度（毫秒）。异常判定在分桶聚合序列上进行——
	// 逐点检测需要对全量原始点建模，超出"基础异常"边界。
	WindowMs  int64
	Aggregate string
	Rule      TelemetryAnomalyRuleSpec
}

// TelemetryAnomalyHit 单个异常命中。
type TelemetryAnomalyHit struct {
	Index  int     `json:"index"` // 序列中的位置（第几个分桶）
	Value  float64 `json:"value"`
	Reason string  `json:"reason"`
}

// TelemetryAnomalyDeviceResult 单设备检测结果。
type TelemetryAnomalyDeviceResult struct {
	DeviceID  string                `json:"device_id"`
	Error     string                `json:"error,omitempty"`
	Total     int                   `json:"total"` // 参与判定的分桶数
	Anomalies []TelemetryAnomalyHit `json:"anomalies"`
	Rate      float64               `json:"rate"` // 异常分桶占比
}

// TelemetryAnomalyResult 整次检测的结果。
type TelemetryAnomalyResult struct {
	Key       string                         `json:"key"`
	Aggregate string                         `json:"aggregate"`
	WindowMs  int64                          `json:"window_ms"`
	Rule      TelemetryAnomalyRuleSpec       `json:"rule"`
	Devices   []TelemetryAnomalyDeviceResult `json:"devices"`
}
