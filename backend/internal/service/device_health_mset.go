// 文件用途：TP-21 MSET 特征维度与健康评分扩展点的对接层——配置开关、训练窗口取数与对齐、fail-closed 降级落因。
// 核心逻辑：health.mset.enabled 默认关闭（关 = 返回 nil，评分行为与旧版逐位一致）；开启后按 feature_keys
// （未配置则从设备最新遥测数值键字典序推导，截前 MaxFeatures 个）拉取训练窗分桶均值序列，
// 经 healthmset.AlignMatrix 对齐成 N×M 矩阵，最近一桶作推理样本、其余作训练历史，交 healthmset.Evaluate
// 训练+推理，偏差分 × 权重折入 anomaly_penalty。
// 关键注意事项：遥测 DAL 查询均为 tenant-scope: caller-enforced——调用方必须先用
// GetTenantDeviceByID(deviceID, tenantID) 解析设备后再进本层，租户隔离由调用链保证；
// 任何取数失败/键缺失都降级为中性分并记原因（绝不阻断原评分、绝不伪装成"设备异常"或"设备健康"）；
// 批量评估路径每设备额外产生 M 次聚合查询，开启前应评估数据库压力。
// 重构建议：训练产物目前每次评估即时重算（无迁移、无持久化）；后续若需模型缓存或滚动训练，
// 在本文件 ops 缝上加缓存实现即可，healthmset 纯逻辑与评分折算无需变动。
package service

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/healthmset"
	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// viper 配置键与默认值。默认关：未配置即不启用，存量部署行为不变。
const (
	deviceHealthMSETEnabledKey       = "health.mset.enabled"
	deviceHealthMSETFeatureKeysKey   = "health.mset.feature_keys"
	deviceHealthMSETTrainHoursKey    = "health.mset.train_hours"
	deviceHealthMSETBucketSecondsKey = "health.mset.bucket_seconds"
	deviceHealthMSETMinSamplesKey    = "health.mset.min_samples"
	deviceHealthMSETWeightKey        = "health.mset.penalty_weight"

	deviceHealthMSETDefaultTrainHours    = 24
	deviceHealthMSETDefaultBucketSeconds = 600
	deviceHealthMSETDefaultWeight        = 0.30
)

// 服务侧降级原因（算法侧词表在 healthmset.Reason*，此处只补取数/配置层的原因）。
const (
	reasonNoFeatureKeys      = "no_feature_keys"      // 配置与设备遥测都拿不到数值特征键
	reasonHistoryFetchFailed = "history_fetch_failed" // 任一特征键的窗口聚合取数失败
)

// deviceHealthMSETConfig 一次评估的 MSET 配置快照。
type deviceHealthMSETConfig struct {
	Enabled       bool
	FeatureKeys   []string
	TrainHours    int
	BucketSeconds int64
	MinSamples    int
	PenaltyWeight float64
}

// loadDeviceHealthMSETConfig 读取配置并回填默认值/防御性钳制（错误配置降级为合法默认，不让坏配置击穿评分链路）。
func loadDeviceHealthMSETConfig() deviceHealthMSETConfig {
	cfg := deviceHealthMSETConfig{
		Enabled:       viper.GetBool(deviceHealthMSETEnabledKey),
		TrainHours:    viper.GetInt(deviceHealthMSETTrainHoursKey),
		BucketSeconds: viper.GetInt64(deviceHealthMSETBucketSecondsKey),
		MinSamples:    viper.GetInt(deviceHealthMSETMinSamplesKey),
		PenaltyWeight: viper.GetFloat64(deviceHealthMSETWeightKey),
	}
	if raw := strings.TrimSpace(viper.GetString(deviceHealthMSETFeatureKeysKey)); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if key := strings.TrimSpace(part); key != "" {
				cfg.FeatureKeys = append(cfg.FeatureKeys, key)
			}
		}
	}
	if cfg.TrainHours <= 0 {
		cfg.TrainHours = deviceHealthMSETDefaultTrainHours
	}
	if cfg.BucketSeconds <= 0 {
		cfg.BucketSeconds = deviceHealthMSETDefaultBucketSeconds
	}
	if cfg.MinSamples <= 0 {
		cfg.MinSamples = healthmset.DefaultMinSamples
	}
	if cfg.PenaltyWeight <= 0 {
		cfg.PenaltyWeight = 0 // 显式配 0 = 挂特征分但不扣分（观察模式），合法
	} else if cfg.PenaltyWeight > 1 {
		cfg.PenaltyWeight = 1
	}
	if len(cfg.FeatureKeys) > healthmset.MaxFeatures {
		cfg.FeatureKeys = cfg.FeatureKeys[:healthmset.MaxFeatures] // 确定性截断，超出部分忽略
	}
	return cfg
}

// deviceHealthMSETOperations 副作用缝：取数与时钟可注入，编排逻辑可无 DB 验证。
type deviceHealthMSETOperations struct {
	fetchSeries func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error)
	latestKeys  func(deviceID string) ([]model.TelemetryData, error)
	now         func() time.Time
}

var deviceHealthMSETOps = deviceHealthMSETOperations{
	fetchSeries: dal.GetTelemetrStatisticaAgregationData,
	latestKeys:  dal.GetCurrentTelemetrData,
	now:         time.Now,
}

// deviceHealthMSETFeature 快速路径：开关关闭（默认）直接返回 nil，零额外取数。
func (*DeviceHealthService) deviceHealthMSETFeature(deviceID string) *model.DeviceHealthMSETFeature {
	cfg := loadDeviceHealthMSETConfig()
	if !cfg.Enabled {
		return nil
	}
	return evaluateDeviceHealthMSETWithConfig(deviceID, cfg)
}

// evaluateDeviceHealthMSETWithConfig MSET 特征评估编排：配置 → 特征键 → 取数对齐 → 训练推理 → 折算扣分。
func evaluateDeviceHealthMSETWithConfig(deviceID string, cfg deviceHealthMSETConfig) *model.DeviceHealthMSETFeature {
	feature := &model.DeviceHealthMSETFeature{}

	keys := cfg.FeatureKeys
	if len(keys) == 0 {
		keys = deriveDeviceHealthMSETKeys(deviceID)
	}
	if len(keys) == 0 {
		feature.Degraded = true
		feature.DegradeReason = reasonNoFeatureKeys
		return feature
	}
	feature.FeatureKeys = append([]string(nil), keys...)

	bucketMs := cfg.BucketSeconds * 1000
	now := deviceHealthMSETOps.now()
	startMs := now.Add(-time.Duration(cfg.TrainHours) * time.Hour).UnixMilli()
	endMs := now.UnixMilli()

	series := make(map[string][]healthmset.Point, len(keys))
	for _, key := range keys {
		rows, err := deviceHealthMSETOps.fetchSeries(deviceID, key, startMs, endMs, bucketMs, "avg")
		if err != nil {
			logrus.Errorf("health mset: fetch series failed (device=%s key=%s): %v", deviceID, key, err)
			feature.Degraded = true
			feature.DegradeReason = reasonHistoryFetchFailed
			return feature
		}
		points := make([]healthmset.Point, 0, len(rows))
		for _, row := range rows {
			x, xok := healthMSETAsInt64(row["x"])
			y, yok := healthMSETAsFloat64(row["y"])
			if !xok || !yok {
				continue // 形状不符的行跳过：对齐层靠完整桶兜底
			}
			points = append(points, healthmset.Point{X: x, Y: y})
		}
		series[key] = points
	}

	aligned := healthmset.AlignMatrix(bucketMs, keys, series)
	if len(aligned.Rows) == 0 {
		feature.Degraded = true
		feature.DegradeReason = healthmset.ReasonColdStart
		return feature
	}
	// 最近一桶作推理样本（当前工况），其余作训练历史（历史基线）。
	current := aligned.Rows[len(aligned.Rows)-1]
	history := aligned.Rows[:len(aligned.Rows)-1]

	inference := healthmset.Evaluate(
		healthmset.Config{MinSamples: cfg.MinSamples},
		aligned.Keys, history, current,
	)
	feature.Applied = inference.Applied
	feature.Degraded = inference.Degraded
	feature.DegradeReason = inference.DegradeReason
	feature.DeviationScore = inference.DeviationScore
	feature.Mahalanobis = inference.Mahalanobis
	feature.TrainSamples = len(history)
	if inference.Applied {
		feature.Penalty = math.Round(inference.DeviationScore*cfg.PenaltyWeight*100) / 100
	}
	return feature
}

// deriveDeviceHealthMSETKeys 未配置 feature_keys 时，从设备最新遥测里取数值型键（字典序，确定性截前 MaxFeatures 个）。
func deriveDeviceHealthMSETKeys(deviceID string) []string {
	latest, err := deviceHealthMSETOps.latestKeys(deviceID)
	if err != nil {
		logrus.Errorf("health mset: derive feature keys failed (device=%s): %v", deviceID, err)
		return nil
	}
	keys := make([]string, 0, len(latest))
	for _, row := range latest {
		if row.NumberV != nil {
			keys = append(keys, row.Key)
		}
	}
	sort.Strings(keys)
	if len(keys) > healthmset.MaxFeatures {
		keys = keys[:healthmset.MaxFeatures]
	}
	return keys
}

// healthMSETAsInt64 / healthMSETAsFloat64 DAL 行值 → 数值。聚合结果经 JSON 或驱动反序列化，
// 数值可能以 int/int64/float64/json.Number/字符串出现；解析失败一律按缺值处理（对齐层兜底）。
func healthMSETAsInt64(v interface{}) (int64, bool) {
	switch value := v.(type) {
	case int64:
		return value, true
	case int:
		return int64(value), true
	case int32:
		return int64(value), true
	case uint64:
		return int64(value), true
	case float64:
		return int64(value), true
	case json.Number:
		parsed, err := value.Int64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func healthMSETAsFloat64(v interface{}) (float64, bool) {
	switch value := v.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int64:
		return float64(value), true
	case int:
		return float64(value), true
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

// deviceHealthMSETSummary 生成给 suggestions 的说明文本（评估层不直接拼中文到评分核心）。
func deviceHealthMSETSummary(feature *model.DeviceHealthMSETFeature) string {
	if feature == nil {
		return ""
	}
	if feature.Degraded {
		return fmt.Sprintf("MSET 多元状态估计未生效（%s），本次评分未包含多元偏差扣分", feature.DegradeReason)
	}
	return fmt.Sprintf("MSET 多元状态估计偏差分 %.2f（马氏距离 %.3f，特征 %s）",
		feature.DeviationScore, feature.Mahalanobis, strings.Join(feature.FeatureKeys, ","))
}
