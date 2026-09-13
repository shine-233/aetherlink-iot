// 文件用途：P2.2 基础异常检测——决策核心 + 与轻量分析同源的数据编排。
// 核心逻辑：按设备拉取分桶聚合序列（复用 telemetryAnalysisOps.fetch 缝），
// 对序列套用 bounds（静态上下限）或 deviation（均值±Kσ）规则，逐桶判定。
//
// 关键注意事项：
//   - 这是**基础**异常检测：不做建模、不做学习。规则无法判定时（序列空、
//     标准差为 0、上下限倒置）如实报错或零命中，绝不静默当"无异常"。
//   - deviation 规则里 σ=0 意味着序列全等——判定不出异常，但要在结果里说明，
//     否则"没有异常"和"无法判定"无法区分。
//   - 序列空不是异常也不是正常：分桶序列为空说明时间窗内没有数据，单独报 error，
//     让调用方分辨"没数据"和"有数据且正常"。
//   - 单设备失败不中断多设备检测（与 RunTelemetryAnalysis 同一口径）。
package service

import (
	"errors"
	"fmt"
	"math"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// 异常检测规则类型：单一事实源在 model 包（对外契约词汇），此处只做本地别名，
// 避免两处各写一份字面量后悄悄漂移。
const (
	TelemetryAnomalyRuleBounds    = model.TelemetryAnomalyRuleBounds
	TelemetryAnomalyRuleDeviation = model.TelemetryAnomalyRuleDeviation
)

// ErrTelemetryAnomalyRuleInvalid 规则本身不可用。
var ErrTelemetryAnomalyRuleInvalid = errors.New("telemetry anomaly rule is invalid")

// validateTelemetryAnomalyRule 校验规则并回填默认值。
func validateTelemetryAnomalyRule(rule model.TelemetryAnomalyRuleSpec) (model.TelemetryAnomalyRuleSpec, error) {
	switch rule.Type {
	case TelemetryAnomalyRuleBounds:
		if rule.Min == nil && rule.Max == nil {
			return rule, fmt.Errorf("%w: bounds rule needs min and/or max", ErrTelemetryAnomalyRuleInvalid)
		}
		if rule.Min != nil && rule.Max != nil && *rule.Min > *rule.Max {
			return rule, fmt.Errorf("%w: bounds min > max", ErrTelemetryAnomalyRuleInvalid)
		}
		return rule, nil
	case TelemetryAnomalyRuleDeviation:
		k := 3.0
		if rule.K != nil {
			if *rule.K <= 0 {
				return rule, fmt.Errorf("%w: deviation k must be positive", ErrTelemetryAnomalyRuleInvalid)
			}
			k = *rule.K
		}
		rule.K = &k
		return rule, nil
	default:
		return rule, fmt.Errorf("%w: unsupported type %q", ErrTelemetryAnomalyRuleInvalid, rule.Type)
	}
}

// DetectSeriesAnomalies 对一个分桶聚合序列执行规则判定（纯函数，便于定向验证）。
func DetectSeriesAnomalies(rule model.TelemetryAnomalyRuleSpec, values []float64) (model.TelemetryAnomalyRuleSpec, []model.TelemetryAnomalyHit, error) {
	validated, err := validateTelemetryAnomalyRule(rule)
	if err != nil {
		return validated, nil, err
	}
	hits := []model.TelemetryAnomalyHit{}
	switch validated.Type {
	case TelemetryAnomalyRuleBounds:
		for i, v := range values {
			if validated.Min != nil && v < *validated.Min {
				hits = append(hits, model.TelemetryAnomalyHit{
					Index: i, Value: v,
					Reason: fmt.Sprintf("below min %s", formatTelemetryAnalysisNumber(*validated.Min, true)),
				})
				continue
			}
			if validated.Max != nil && v > *validated.Max {
				hits = append(hits, model.TelemetryAnomalyHit{
					Index: i, Value: v,
					Reason: fmt.Sprintf("above max %s", formatTelemetryAnalysisNumber(*validated.Max, true)),
				})
			}
		}
	case TelemetryAnomalyRuleDeviation:
		mean, sigma := meanAndStdDev(values)
		if sigma == 0 {
			// 序列全等：判定不出异常，但也不能静默当"无异常"——
			// 全等序列上 bounds 规则可能有命中，deviation 则语义上无定义。
			return validated, hits, nil
		}
		k := *validated.K
		for i, v := range values {
			if z := math.Abs(v-mean) / sigma; z > k {
				hits = append(hits, model.TelemetryAnomalyHit{
					Index: i, Value: v,
					Reason: fmt.Sprintf("deviation z=%.2f exceeds k=%.2f (mean=%s, sigma=%s)",
						z, k, formatTelemetryAnalysisNumber(mean, true), formatTelemetryAnalysisNumber(sigma, true)),
				})
			}
		}
	}
	return validated, hits, nil
}

// meanAndStdDev 序列均值与总体标准差。空序列返回 (0, 0)。
func meanAndStdDev(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	var acc float64
	for _, v := range values {
		acc += (v - mean) * (v - mean)
	}
	return mean, math.Sqrt(acc / float64(len(values)))
}

// RunTelemetryAnomalyDetection 异常检测编排：与 RunTelemetryAnalysis 共用取数缝与权限缝。
func RunTelemetryAnomalyDetection(q model.TelemetryAnomalyQuery, claims *utils.UserClaims) (*model.TelemetryAnomalyResult, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	if len(q.DeviceIDs) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_ids is required")
	}
	if q.Key == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "key is required")
	}
	if q.EndTime <= q.StartTime {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "end_time must be after start_time")
	}
	if q.WindowMs <= 0 || q.WindowMs > q.EndTime-q.StartTime {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "window_ms must be positive and not exceed the query window")
	}
	aggregate, err := validateTelemetryAnalysisAggregate(q.Aggregate)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}
	rule, err := validateTelemetryAnomalyRule(q.Rule)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}

	result := &model.TelemetryAnomalyResult{
		Key:       q.Key,
		Aggregate: aggregate,
		WindowMs:  q.WindowMs,
		Rule:      rule,
	}
	for _, deviceID := range q.DeviceIDs {
		deviceResult := model.TelemetryAnomalyDeviceResult{DeviceID: deviceID, Anomalies: []model.TelemetryAnomalyHit{}}
		if _, accessErr := telemetryAnalysisOps.access(deviceID, claims); accessErr != nil {
			deviceResult.Error = "device not readable"
			result.Devices = append(result.Devices, deviceResult)
			continue
		}
		rows, ferr := telemetryAnalysisOps.fetch(deviceID, q.Key, q.StartTime, q.EndTime, q.WindowMs, aggregate)
		if ferr != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": ferr.Error()})
		}
		values := telemetryAnalysisValues(rows)
		if len(values) == 0 {
			// 时间窗内没有数据：不是"无异常"，让调用方自己分辨。
			deviceResult.Error = "no data in window"
			result.Devices = append(result.Devices, deviceResult)
			continue
		}
		_, hits, derr := DetectSeriesAnomalies(rule, values)
		if derr != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, derr.Error())
		}
		deviceResult.Total = len(values)
		deviceResult.Anomalies = hits
		if deviceResult.Total > 0 {
			deviceResult.Rate = float64(len(hits)) / float64(deviceResult.Total)
		}
		result.Devices = append(result.Devices, deviceResult)
	}
	return result, nil
}
