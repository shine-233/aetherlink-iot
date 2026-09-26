// 文件用途：TP-21 MSET 特征维度对接层的单元测试——配置开关、取数编排、降级落因与评分折入。
// 核心逻辑：钉死四条口径——①开关默认关（未配置返回 nil，评分行为与旧版逐位一致）；
// ②正常/异常推理样本的偏差方向与扣分折算（偏差分×权重折入 anomaly_penalty）；
// ③取数失败/无特征键/冷启动/样本不足/奇异矩阵全部 fail-closed 降级且记对原因；
// ④nil 维度下 ComputeDeviceHealthWithMSET 与 ComputeDeviceHealth 深度相等（回归护栏）。
// 关键注意事项：viper 键与 deviceHealthMSETOps 缝都在 t.Cleanup 里恢复，防止污染同包其他测试；
// 合成序列用正弦扰动（确定性，无随机源），偏差断言不依赖具体分布实现。
// 重构建议：后续加"观察模式"专项用例（penalty_weight=0 挂分不扣分）时复用本文件的序列构造器。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/healthmset"
	"aetherlink-iot/backend/internal/model"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	msetTestBucketMs   = int64(60000)
	msetTestMinSamples = 8
)

// setMSETTestConfig 设置测试配置并在测试结束后恢复 unset 等价状态（本加载器把 unset 与零值等同处理）。
func setMSETTestConfig(t *testing.T, key string, value interface{}) {
	t.Helper()
	previous := viper.Get(key)
	viper.Set(key, value)
	t.Cleanup(func() {
		if previous == nil {
			viper.Set(key, false) // bool/数值键的 unset 等价态：false/0/空串按类型给零值
			viper.Set(key, "")
			return
		}
		viper.Set(key, previous)
	})
}

// withMSETTestOps 注入取数缝并在测试结束后还原。
func withMSETTestOps(t *testing.T, fetch func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error), latest func(deviceID string) ([]model.TelemetryData, error)) {
	t.Helper()
	previous := deviceHealthMSETOps
	deviceHealthMSETOps = deviceHealthMSETOperations{fetchSeries: fetch, latestKeys: latest, now: previous.now}
	t.Cleanup(func() { deviceHealthMSETOps = previous })
}

// msetNoise 确定性 LCG 噪声（[-1,1)，固定种子跨版本可复现）。不同键用不同种子即近似独立，
// 避免跨键完全共线把协方差矩阵推成奇异（那会触发 singular_matrix 降级盖过用例本意）。
func msetNoise(seed uint64, count int) []float64 {
	state := seed
	noise := make([]float64, count)
	for i := range noise {
		state = state*6364136223846793005 + 1442695040888963407
		noise[i] = float64(int64(state>>11))/float64(1<<53)*2 - 1
	}
	return noise
}

// msetBuckets 构造分桶序列：i 号桶 x = startSec + i×60s，y = base + amp×noise[i]。
func msetBuckets(startSec int64, count int, base, amp float64, noise []float64) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, count)
	for i := 0; i < count; i++ {
		rows = append(rows, map[string]interface{}{
			"x": (startSec + int64(i)*msetTestBucketMs/1000) * 1000,
			"y": base + amp*noise[i],
		})
	}
	return rows
}

// msetStableSeries 两特征稳定序列：temperature≈20、humidity≈50（各自独立噪声）。
func msetStableSeries(startSec int64, count int) map[string][]map[string]interface{} {
	return map[string][]map[string]interface{}{
		"temperature": msetBuckets(startSec, count, 20, 0.5, msetNoise(11, count)),
		"humidity":    msetBuckets(startSec, count, 50, 0.5, msetNoise(22, count)),
	}
}

func msetSeriesToRows(series map[string][]map[string]interface{}, key string) []map[string]interface{} {
	return series[key]
}

func TestDeviceHealthMSETFeature_DisabledByDefault(t *testing.T) {
	svc := &DeviceHealthService{}
	// 不设置任何 health.mset.* 配置：开关必须默认关，直接返回 nil。
	feature := svc.deviceHealthMSETFeature("dev-001")
	assert.Nil(t, feature, "MSET feature must be nil when the switch is not configured (default off)")
}

// msetWithLastValue 返回把最后一桶 y 替换为指定值后的序列副本（最后一桶即"当前工况"推理样本）。
func msetWithLastValue(rows []map[string]interface{}, y float64) []map[string]interface{} {
	updated := append([]map[string]interface{}(nil), rows...)
	updated[len(updated)-1] = map[string]interface{}{"x": updated[len(updated)-1]["x"], "y": y}
	return updated
}

func TestDeviceHealthMSETFeature_NormalAndAbnormal(t *testing.T) {
	setMSETTestConfig(t, "health.mset.enabled", true)
	setMSETTestConfig(t, "health.mset.feature_keys", "temperature,humidity")
	setMSETTestConfig(t, "health.mset.min_samples", msetTestMinSamples)
	setMSETTestConfig(t, "health.mset.bucket_seconds", 60) // 与 msetBuckets 的 60s 序列间距对齐
	setMSETTestConfig(t, "health.mset.penalty_weight", 0.30)

	startSec := int64(1768000000) // 固定起点（秒），序列不依赖真实时钟

	svc := &DeviceHealthService{}

	// 正常现状（最后一桶贴着基线）：低偏差、扣分 = 偏差×权重。
	normalSeries := msetStableSeries(startSec, 20)
	normalSeries["temperature"] = msetWithLastValue(normalSeries["temperature"], 20.1)
	normalSeries["humidity"] = msetWithLastValue(normalSeries["humidity"], 50.1)
	withMSETTestOps(t,
		func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error) {
			return msetSeriesToRows(normalSeries, key), nil
		}, nil)
	normal := svc.deviceHealthMSETFeature("dev-001")
	require.NotNil(t, normal)
	assert.True(t, normal.Applied)
	assert.False(t, normal.Degraded)
	assert.Empty(t, normal.DegradeReason)
	assert.Less(t, normal.DeviationScore, 30.0, "normal current should deviate little from the baseline")
	assert.Equal(t, 19, normal.TrainSamples)
	assert.Equal(t, []string{"temperature", "humidity"}, normal.FeatureKeys)
	assert.InDelta(t, math.Round(normal.DeviationScore*0.30*100)/100, normal.Penalty, 1e-9)
	assert.True(t, normal.Penalty < 10, "normal penalty should be small, got %v", normal.Penalty)

	// 异常现状（最后一桶跳变到基线外 10+ 个标准差）：高偏差、扣分显著。
	abnormalSeries := msetStableSeries(startSec, 20)
	abnormalSeries["temperature"] = msetWithLastValue(abnormalSeries["temperature"], 32.0)
	abnormalSeries["humidity"] = msetWithLastValue(abnormalSeries["humidity"], 68.0)
	withMSETTestOps(t,
		func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error) {
			return msetSeriesToRows(abnormalSeries, key), nil
		}, nil)
	abnormal := svc.deviceHealthMSETFeature("dev-001")
	require.NotNil(t, abnormal)
	assert.True(t, abnormal.Applied)
	assert.Greater(t, abnormal.DeviationScore, 90.0, "abnormal current should deviate far from the baseline")
	assert.Greater(t, abnormal.Penalty, 27.0, "penalty = deviation × 0.30")
	assert.Less(t, abnormal.Penalty, 31.0)
	assert.Greater(t, abnormal.Mahalanobis, 0.0)
}

func TestDeviceHealthMSETFeature_DegradationPaths(t *testing.T) {
	setMSETTestConfig(t, "health.mset.enabled", true)
	setMSETTestConfig(t, "health.mset.min_samples", msetTestMinSamples)
	setMSETTestConfig(t, "health.mset.bucket_seconds", 60) // 与 msetBuckets 的 60s 序列间距对齐

	startSec := int64(1768000000)
	cases := []struct {
		name        string
		featureKeys string
		fetch       func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error)
		latest      func(deviceID string) ([]model.TelemetryData, error)
		wantReason  string
	}{
		{
			name:        "fetch error",
			featureKeys: "temperature,humidity",
			fetch: func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error) {
				return nil, errors.New("db down")
			},
			wantReason: "history_fetch_failed",
		},
		{
			name:        "no feature keys at all",
			featureKeys: "",
			fetch: func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error) {
				return nil, errors.New("should not be called")
			},
			latest:     func(deviceID string) ([]model.TelemetryData, error) { return nil, nil },
			wantReason: "no_feature_keys",
		},
		{
			name:        "cold start (single bucket)",
			featureKeys: "temperature,humidity",
			fetch: func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error) {
				return msetBuckets(startSec, 1, 20, 0.5, msetNoise(11, 1)), nil
			},
			wantReason: healthmset.ReasonColdStart,
		},
		{
			name:        "insufficient samples",
			featureKeys: "temperature,humidity",
			fetch: func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error) {
				return msetBuckets(startSec, 6, 20, 0.5, msetNoise(11, 6)), nil
			},
			wantReason: healthmset.ReasonInsufficientSamples,
		},
		{
			name:        "singular matrix (collinear features)",
			featureKeys: "temperature,humidity",
			fetch: func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error) {
				// 两键同种子噪声 → 序列完全相同 → 协方差奇异（固定种子保证可复现）。
				return msetBuckets(startSec, 20, 20, 0.5, msetNoise(11, 20)), nil
			},
			wantReason: healthmset.ReasonSingularMatrix,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setMSETTestConfig(t, "health.mset.feature_keys", tc.featureKeys)
			withMSETTestOps(t, tc.fetch, tc.latest)
			svc := &DeviceHealthService{}
			feature := svc.deviceHealthMSETFeature("dev-001")
			require.NotNil(t, feature)
			assert.True(t, feature.Degraded)
			assert.False(t, feature.Applied)
			assert.Equal(t, tc.wantReason, feature.DegradeReason)
			assert.Equal(t, 0.0, feature.DeviationScore, "degraded feature must be neutral")
			assert.Equal(t, 0.0, feature.Penalty, "degraded feature must not deduct points")
		})
	}
}

func TestDeviceHealthMSETFeature_DeriveKeysFromLatestTelemetry(t *testing.T) {
	setMSETTestConfig(t, "health.mset.enabled", true)
	setMSETTestConfig(t, "health.mset.feature_keys", "")
	setMSETTestConfig(t, "health.mset.min_samples", msetTestMinSamples)
	setMSETTestConfig(t, "health.mset.bucket_seconds", 60) // 与 msetBuckets 的 60s 序列间距对齐

	latest := []model.TelemetryData{
		{Key: "switch_state"},                             // 布尔/无数值 → 排除
		{Key: "humidity", NumberV: msetPtrFloat64(50)},    // 数值 → 收
		{Key: "temperature", NumberV: msetPtrFloat64(20)}, // 数值 → 收
		{Key: "voltage", NumberV: msetPtrFloat64(220)},    // 数值 → 收
	}
	withMSETTestOps(t,
		func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error) {
			// 每键不同基线与独立噪声，避免三键共线触发奇异矩阵降级（那会盖过本用例要验证的推导逻辑）。
			bases := map[string]float64{"humidity": 50, "temperature": 20, "voltage": 220}
			seeds := map[string]uint64{"humidity": 31, "temperature": 32, "voltage": 33}
			return msetBuckets(1768000000, 20, bases[key], 0.5, msetNoise(seeds[key], 20)), nil
		},
		func(deviceID string) ([]model.TelemetryData, error) { return latest, nil })

	svc := &DeviceHealthService{}
	feature := svc.deviceHealthMSETFeature("dev-001")
	require.NotNil(t, feature)
	// 推导键字典序排列、只取数值键。
	assert.Equal(t, []string{"humidity", "temperature", "voltage"}, feature.FeatureKeys)
	assert.True(t, feature.Applied, "derived keys should still allow training")
}

func TestComputeDeviceHealthWithMSET_NilEqualsLegacy(t *testing.T) {
	devName := "Fixture Device"
	offline := msetTimePtr()
	fixtures := []*model.Device{
		{ID: "dev-1", Name: &devName, IsOnline: 1, IsEnabled: "enabled", ActivateFlag: "active"},
		{ID: "dev-2", Name: &devName, IsOnline: 0, LastOfflineTime: offline, IsEnabled: "disabled", ActivateFlag: "inactive"},
	}
	alarms := []*model.AlarmHistory{{ID: "a1", Name: "High", AlarmStatus: "H", CreateAt: msetTimeNow()}}
	for _, device := range fixtures {
		baseScore, baseDetail := ComputeDeviceHealth(device, alarms)
		nilScore, nilDetail := ComputeDeviceHealthWithMSET(device, alarms, nil)
		// 两次调用各自取 time.Now，EvaluatedAt/CreatedAt 必然不同，逐字段比较其余全部语义字段。
		assert.Equal(t, baseScore.Score, nilScore.Score)
		assert.Equal(t, baseScore.HealthStatus, nilScore.HealthStatus)
		assert.Equal(t, baseScore.AlarmPenalty, nilScore.AlarmPenalty)
		assert.Equal(t, baseScore.OfflinePenalty, nilScore.OfflinePenalty)
		assert.Equal(t, baseScore.AnomalyPenalty, nilScore.AnomalyPenalty)
		assert.Equal(t, baseScore.Details, nilScore.Details)
		assert.Equal(t, baseDetail.DeviceID, nilDetail.DeviceID)
		assert.Equal(t, baseDetail.Score, nilDetail.Score)
		assert.Equal(t, baseDetail.HealthStatus, nilDetail.HealthStatus)
		assert.Equal(t, baseDetail.AlarmPenalty, nilDetail.AlarmPenalty)
		assert.Equal(t, baseDetail.OfflinePenalty, nilDetail.OfflinePenalty)
		assert.Equal(t, baseDetail.AnomalyPenalty, nilDetail.AnomalyPenalty)
		assert.Equal(t, baseDetail.Suggestions, nilDetail.Suggestions)
		assert.Equal(t, baseDetail.ActiveAlarms, nilDetail.ActiveAlarms)
		assert.Nil(t, nilDetail.MSET, "nil MSET input must not materialize an mset block")
		var details map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(nilScore.Details), &details))
		assert.NotContains(t, details, "mset", "legacy shape must stay free of the mset key")
	}
}

// msetPtrFloat64 / msetTimePtr / msetTimeNow 本文件专用小助手（带前缀避免同包未来撞名）。
func msetPtrFloat64(v float64) *float64 { return &v }
func msetTimePtr() *time.Time           { t := time.Now(); return &t }
func msetTimeNow() time.Time            { return time.Now() }

func TestComputeDeviceHealthWithMSET_FoldsPenaltyIntoAnomaly(t *testing.T) {
	devName := "Fixture Device"
	device := &model.Device{ID: "dev-1", Name: &devName, IsOnline: 1, IsEnabled: "enabled", ActivateFlag: "active"}
	mset := &model.DeviceHealthMSETFeature{
		Applied:        true,
		DeviationScore: 80,
		Penalty:        24,
		Mahalanobis:    3.2,
		FeatureKeys:    []string{"temperature", "humidity"},
		TrainSamples:   19,
	}

	score, detail := ComputeDeviceHealthWithMSET(device, []*model.AlarmHistory{}, mset)
	assert.Equal(t, 24.0, score.AnomalyPenalty, "MSET penalty folds into anomaly_penalty")
	assert.Equal(t, 76.0, score.Score)
	assert.Equal(t, model.HealthStatusSubHealthy, score.HealthStatus)
	assert.Same(t, mset, detail.MSET)

	// details JSON 带 mset 明细。
	var details map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(score.Details), &details))
	require.Contains(t, details, "mset")
	msetJSON, _ := json.Marshal(details["mset"])
	assert.Contains(t, string(msetJSON), `"penalty":24`)

	// 偏差分 ≥70 追加 MSET 处置建议。
	found := false
	for _, suggestion := range detail.Suggestions {
		if contains := len(suggestion) > 0 && suggestion[:4] == "MSET"; contains {
			found = true
		}
	}
	assert.True(t, found, "high deviation should append an MSET suggestion, got %v", detail.Suggestions)
}

func TestComputeDeviceHealthWithMSET_DegradedRecordsReason(t *testing.T) {
	devName := "Fixture Device"
	device := &model.Device{ID: "dev-1", Name: &devName, IsOnline: 1, IsEnabled: "enabled", ActivateFlag: "active"}
	mset := &model.DeviceHealthMSETFeature{
		Degraded:      true,
		DegradeReason: healthmset.ReasonColdStart,
	}

	score, detail := ComputeDeviceHealthWithMSET(device, []*model.AlarmHistory{}, mset)
	assert.Equal(t, 100.0, score.Score, "degraded MSET must not deduct points")
	assert.Equal(t, 0.0, score.AnomalyPenalty)
	assert.Same(t, mset, detail.MSET)
	assert.Contains(t, detail.Suggestions, fmt.Sprintf("MSET 多元状态估计未生效（%s），本次评分未包含多元偏差扣分", healthmset.ReasonColdStart))
}

func TestDeviceHealthMSETConfigClamps(t *testing.T) {
	setMSETTestConfig(t, "health.mset.train_hours", -1)
	setMSETTestConfig(t, "health.mset.bucket_seconds", 0)
	setMSETTestConfig(t, "health.mset.min_samples", 0)
	setMSETTestConfig(t, "health.mset.penalty_weight", 5)
	setMSETTestConfig(t, "health.mset.feature_keys", " a , b ,, c , d , e , f , g , h , i , j ")

	cfg := loadDeviceHealthMSETConfig()
	assert.Equal(t, 24, cfg.TrainHours, "train_hours falls back to default when invalid")
	assert.Equal(t, int64(600), cfg.BucketSeconds)
	assert.Equal(t, healthmset.DefaultMinSamples, cfg.MinSamples)
	assert.Equal(t, 1.0, cfg.PenaltyWeight, "penalty weight clamps to [0,1]")
	assert.Len(t, cfg.FeatureKeys, healthmset.MaxFeatures, "feature keys truncate to MaxFeatures deterministically")
	assert.Equal(t, []string{"a", "b", "c", "d", "e", "f", "g", "h"}, cfg.FeatureKeys)
}
