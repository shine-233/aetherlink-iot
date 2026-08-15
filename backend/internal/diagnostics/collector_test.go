// 文件用途：验证后端运行诊断采集器的默认配置与记录行为。
// 核心逻辑：覆盖保留策略、刷新策略和指标记录，确保排障数据保持可观测。
// 关键注意事项：测试应避免真实外部依赖，重点验证内存态诊断数据的一致性。
// 重构建议：可补充并发记录和序列化输出的表驱动用例。

package diagnostics

import (
	"testing"
	"time"
)

func TestDefaultConfigDocumentsDiagnosticsRetentionAndFlushPolicy(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Fatal("DefaultConfig should enable diagnostics")
	}
	if cfg.MaxFailures != 5 {
		t.Fatalf("MaxFailures = %d, want 5", cfg.MaxFailures)
	}
	if cfg.BatchFlushSize != 10 {
		t.Fatalf("BatchFlushSize = %d, want 10", cfg.BatchFlushSize)
	}
	if cfg.BatchFlushInterval != time.Second {
		t.Fatalf("BatchFlushInterval = %s, want 1s", cfg.BatchFlushInterval)
	}
}

func TestCollectorIsEnabledHandlesNilDisabledAndInitializedStates(t *testing.T) {
	var nilCollector *Collector
	if nilCollector.IsEnabled() {
		t.Fatal("nil collector should not be enabled")
	}

	collector := &Collector{}
	if collector.IsEnabled() {
		t.Fatal("uninitialized collector should not be enabled")
	}

	collector.initialized = true
	collector.config = Config{Enabled: false}
	if collector.IsEnabled() {
		t.Fatal("disabled collector should not be enabled")
	}

	collector.config = Config{Enabled: true}
	if !collector.IsEnabled() {
		t.Fatal("initialized enabled collector should be enabled")
	}
}

func TestRecordFailureShortCircuitsWhenCollectorIsNotEnabled(t *testing.T) {
	collector := &Collector{
		config: Config{Enabled: false, BatchFlushSize: 1},
		buffer: make([]failureItem, 0),
	}

	collector.RecordFailure("dev-1", DirectionUplink, StageAdapter, "bad payload")
	if len(collector.buffer) != 0 {
		t.Fatalf("RecordFailure disabled buffer length = %d, want 0", len(collector.buffer))
	}

	collector.config.Enabled = true
	collector.initialized = false
	collector.RecordFailure("dev-1", DirectionUplink, StageAdapter, "bad payload")
	if len(collector.buffer) != 0 {
		t.Fatalf("RecordFailure uninitialized buffer length = %d, want 0", len(collector.buffer))
	}
}

func TestCalculateMetricSuccessRates(t *testing.T) {
	tests := []struct {
		name        string
		total       int64
		success     int64
		wantRate    float64
		wantSuccess int64
	}{
		{name: "no traffic", total: 0, success: 0, wantRate: 0, wantSuccess: 0},
		{name: "all success", total: 10, success: 10, wantRate: 100, wantSuccess: 10},
		{name: "partial success", total: 8, success: 6, wantRate: 75, wantSuccess: 6},
		{name: "negative success is surfaced for inconsistent counters", total: 4, success: -1, wantRate: -25, wantSuccess: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateMetric(tt.total, tt.success)
			if got.Total != tt.total || got.Success != tt.wantSuccess || got.SuccessRate != tt.wantRate {
				t.Fatalf("calculateMetric(%d,%d) = %+v, want total=%d success=%d rate=%f", tt.total, tt.success, got, tt.total, tt.wantSuccess, tt.wantRate)
			}
		})
	}
}
