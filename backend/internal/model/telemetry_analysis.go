package model

// TelemetryAnalysisQuery 轻量分析查询（ROADMAP P2.2）。
// 放在非 .gen.go 文件中：按项目约定不手改生成产物。
type TelemetryAnalysisQuery struct {
	DeviceIDs  []string
	Key        string
	StartTime  int64
	EndTime    int64
	Granularity string
	// Aggregate 聚合方式：avg / sum / min / max / count / last。
	Aggregate string
	// Compare 对比方式：none / previous_period（环比）/ same_period_last（同比）。
	Compare string
	// CompareOffsets 同比时用于回退的周期倍数，默认 1（如"上周同段"）。
	CompareOffsets int
	Format string
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
