// 文件用途：P2.2 基础异常检测决策核心的表驱动用例（纯函数，无 DB 依赖）。
// 说明：DetectSeriesAnomalies 是纯决策函数；规则校验与命中判定逐表核对，
// "没数据"与"无异常"的语义边界由 RunTelemetryAnomalyDetection 编排层保证（另测）。
package service

import (
	"math"
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func f64(v float64) *float64 { return &v }

func TestDetectSeriesAnomaliesBounds(t *testing.T) {
	rule := model.TelemetryAnomalyRuleSpec{
		Type: model.TelemetryAnomalyRuleBounds,
		Min:  f64(0),
		Max:  f64(10),
	}
	_, hits, err := DetectSeriesAnomalies(rule, []float64{5, -1, 11, 10, 0})
	if err != nil {
		t.Fatalf("bounds rule rejected: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2 (index 1 below, index 2 above): %+v", len(hits), hits)
	}
	if hits[0].Index != 1 || hits[0].Value != -1 {
		t.Fatalf("hit[0] = %+v, want index 1 value -1", hits[0])
	}
	if hits[1].Index != 2 || hits[1].Value != 11 {
		t.Fatalf("hit[1] = %+v, want index 2 value 11", hits[1])
	}
}

func TestDetectSeriesAnomaliesBoundsOneSided(t *testing.T) {
	rule := model.TelemetryAnomalyRuleSpec{Type: model.TelemetryAnomalyRuleBounds, Min: f64(0)}
	_, hits, err := DetectSeriesAnomalies(rule, []float64{1, -2, 3})
	if err != nil || len(hits) != 1 || hits[0].Index != 1 {
		t.Fatalf("min-only rule: hits=%+v err=%v", hits, err)
	}
}

func TestDetectSeriesAnomaliesDeviation(t *testing.T) {
	rule := model.TelemetryAnomalyRuleSpec{Type: model.TelemetryAnomalyRuleDeviation}
	// 11 点序列，均值≈13.18、总体 σ≈10.12：45 明显越界（z≈3.15 > 3），
	// 其余点最大 |z|≈0.51，落在 ±3σ 容差内。
	//
	// 注意样本长度不是随便定的：当序列是"n-1 个相同值 + 1 个离群值"时，
	// 该离群点的 z 恒等于 sqrt(n-1)，与具体数值无关。因此 n=8 时 z 上限仅 2.65，
	// 永远触发不了 ±3σ——这正是本用例早先给出错误期望的原因。
	// 要验证"能命中"，样本至少需要 11 点（sqrt(10)≈3.16）。
	_, hits, err := DetectSeriesAnomalies(rule, []float64{10, 11, 9, 10, 12, 8, 10, 11, 9, 10, 45})
	if err != nil {
		t.Fatalf("deviation rule rejected: %v", err)
	}
	if len(hits) != 1 || hits[0].Index != 10 {
		t.Fatalf("deviation hits = %+v, want only index 10", hits)
	}
}

func TestDetectSeriesAnomaliesDeviationSigmaZeroIsNoHit(t *testing.T) {
	rule := model.TelemetryAnomalyRuleSpec{Type: model.TelemetryAnomalyRuleDeviation}
	_, hits, err := DetectSeriesAnomalies(rule, []float64{7, 7, 7})
	if err != nil || len(hits) != 0 {
		t.Fatalf("constant series: hits=%d err=%v; sigma=0 must be zero-hit, not fabricated anomalies", len(hits), err)
	}
}

func TestDetectSeriesAnomaliesEmptySeries(t *testing.T) {
	rule := model.TelemetryAnomalyRuleSpec{Type: model.TelemetryAnomalyRuleBounds, Max: f64(10)}
	_, hits, err := DetectSeriesAnomalies(rule, nil)
	if err != nil || len(hits) != 0 {
		t.Fatalf("empty series must not error and must not hit: %v %v", hits, err)
	}
}

func TestValidateTelemetryAnomalyRuleRejects(t *testing.T) {
	cases := []model.TelemetryAnomalyRuleSpec{
		{Type: "magic"},                          // 未知类型
		{Type: model.TelemetryAnomalyRuleBounds}, // bounds 缺上下限
		{Type: model.TelemetryAnomalyRuleBounds, Min: f64(5), Max: f64(1)}, // min > max
		{Type: model.TelemetryAnomalyRuleDeviation, K: f64(0)},             // k 非正
		{Type: model.TelemetryAnomalyRuleDeviation, K: f64(-1)},            // k 负
	}
	for i, rule := range cases {
		if _, err := validateTelemetryAnomalyRule(rule); err == nil {
			t.Fatalf("invalid rule #%d accepted: %+v", i, rule)
		}
	}
}

func TestValidateTelemetryAnomalyRuleDefaultsK(t *testing.T) {
	validated, err := validateTelemetryAnomalyRule(model.TelemetryAnomalyRuleSpec{Type: model.TelemetryAnomalyRuleDeviation})
	if err != nil {
		t.Fatal(err)
	}
	if validated.K == nil || *validated.K != 3 {
		t.Fatalf("default k = %v, want 3", validated.K)
	}
}

func TestMeanAndStdDev(t *testing.T) {
	mean, sigma := meanAndStdDev([]float64{2, 4, 4, 4, 5, 5, 7, 9})
	if math.Abs(mean-5) > 1e-9 || math.Abs(sigma-2) > 1e-9 {
		t.Fatalf("mean=%f sigma=%f, want 5 and 2", mean, sigma)
	}
	if m, s := meanAndStdDev(nil); m != 0 || s != 0 {
		t.Fatalf("empty series mean/sigma = %f/%f, want 0/0", m, s)
	}
}
