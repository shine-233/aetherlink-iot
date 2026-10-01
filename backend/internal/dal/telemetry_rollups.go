// 文件用途：P2.3 遥测降采样（冷层）的数据访问——汇总写入与冷窗口聚合读取。
// 核心逻辑：
//   - 写入：把 cutoff 之前的原始遥测按桶 GROUP BY 汇总进 telemetry_rollups，
//     ON CONFLICT 覆盖，作业可重跑（幂等）。
//   - 读取：冷窗口（早于降采样边界）的分析查询回落到 rollup 表，
//     在 Go 侧把桶行重新聚合成请求的窗口粒度（count 加权），
//     输出与原始聚合路径同形的 x/y 行。
//
// 关键注意事项：
//  1. 降采样只覆盖**直连数据库**的遥测栈；grpc.tptodb_type 指向外部 TSDB 时
//     原始数据不在本库，作业与冷读都必须显式退出，而不是空转制造"已降采样"假象。
//  2. 冷读按 count 加权合并 avg（avg_i*count_i 之和 / count 之和），
//     min/max/count/last/sum 都是精确合并——不是对均值再求均值。
//  3. 汇总 SQL 必须带 number_v IS NOT NULL：字符串/布尔遥测不进数值冷层。
package dal

import (
	"fmt"
	"math"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// TelemetryRollupBucketMs 冷层固定桶宽：1 小时。
const TelemetryRollupBucketMs int64 = 3600 * 1000

// TelemetryDownsamplingActive 当前遥测栈是否为直连数据库模式（降采样仅此模式可用）。
func TelemetryDownsamplingActive() bool {
	return !usesTelemetryQueryClient()
}

// ListTelemetryDownsampleTargets 列出 cutoff 之前仍有原始数据的 (设备, 键) 对，限量返回。
// tenant-scope: system-job —— 降采样是全局后台作业，必须跨租户扫描才能覆盖所有设备；
// 它只做聚合写入（telemetry_rollups），不向任何租户返回原始数据，故不构成越权读取。
func ListTelemetryDownsampleTargets(cutoff int64, limit int) ([]model.TelemetryRollupTarget, error) {
	if limit <= 0 {
		limit = 100
	}
	rows := make([]model.TelemetryRollupTarget, 0, limit)
	err := global.DB.Raw(`
		SELECT DISTINCT device_id, key
		FROM telemetry_datas
		WHERE ts < ? AND number_v IS NOT NULL
		LIMIT ?`, cutoff, limit).Scan(&rows).Error
	return rows, err
}

// UpsertTelemetryRollups 把一个目标的 cutoff 之前数据汇总进冷层，返回涉及的桶数。
// ON CONFLICT 覆盖：重复执行得到相同结果（作业可重跑）。
func UpsertTelemetryRollups(deviceID, key string, cutoff int64) (int64, error) {
	bucket := TelemetryRollupBucketMs
	result := global.DB.Exec(`
		INSERT INTO telemetry_rollups (device_id, key, bucket_start, bucket_ms, min_v, max_v, avg_v, last_v, count_v, updated_at)
		SELECT device_id, key, (ts / ?) * ?, ?, MIN(number_v), MAX(number_v), AVG(number_v),
		       (array_agg(number_v ORDER BY ts DESC))[1], COUNT(*), now()
		FROM telemetry_datas
		WHERE device_id = ? AND key = ? AND ts < ? AND number_v IS NOT NULL
		GROUP BY device_id, key, (ts / ?) * ?
		ON CONFLICT (device_id, key, bucket_ms, bucket_start) DO UPDATE SET
			min_v = EXCLUDED.min_v,
			max_v = EXCLUDED.max_v,
			avg_v = EXCLUDED.avg_v,
			last_v = EXCLUDED.last_v,
			count_v = EXCLUDED.count_v,
			updated_at = now()`,
		bucket, bucket, bucket, deviceID, key, cutoff, bucket, bucket)
	return result.RowsAffected, result.Error
}

// GetTelemetryRollupAggregate 从冷层读取一个 (设备,键) 的窗口聚合，
// 输出行与 GetTelemetrStatisticaAgregationData 同形（x=时间戳, y=数值）。
// tenant-scope: caller-enforced —— 本函数只按 (deviceID, key) 取数，
// 设备是否属于调用方租户由上层 ensureTelemetryDeviceReadAccess 判定；
// 在此加租户过滤会与既有热层聚合路径的作用域语义分叉。
func GetTelemetryRollupAggregate(deviceID, key string, sTime, eTime, aggregateWindow int64, aggregateFunc string) ([]map[string]interface{}, error) {
	if aggregateWindow <= 0 {
		aggregateWindow = TelemetryRollupBucketMs
	}
	var buckets []model.TelemetryRollup
	err := global.DB.
		Where("device_id = ? AND key = ? AND bucket_ms = ? AND bucket_start >= ? AND bucket_start < ?",
			deviceID, key, TelemetryRollupBucketMs, sTime, eTime).
		Order("bucket_start ASC").
		Find(&buckets).Error
	if err != nil {
		return nil, err
	}
	if len(buckets) == 0 {
		return []map[string]interface{}{}, nil
	}

	// 把 1h 桶重聚合成请求窗口。窗口与冷层桶宽不同时按桶起点对齐分桶，
	// 与原始路径"窗口对齐查询起点"的语义一致。
	type windowAcc struct {
		count           int64
		sum             float64 // Σ avg*count
		min             float64
		max             float64
		last            float64
		lastBucketStart int64
	}
	accs := make(map[int64]*windowAcc)
	order := make([]int64, 0, len(buckets))
	for i := range buckets {
		b := &buckets[i]
		if b.CountV <= 0 || b.AvgV == nil {
			continue
		}
		start := b.BucketStart - ((b.BucketStart - sTime) % aggregateWindow)
		acc := accs[start]
		if acc == nil {
			acc = &windowAcc{min: math.Inf(1), max: math.Inf(-1)}
			accs[start] = acc
			order = append(order, start)
		}
		acc.count += b.CountV
		acc.sum += *b.AvgV * float64(b.CountV)
		if b.MinV != nil && *b.MinV < acc.min {
			acc.min = *b.MinV
		}
		if b.MaxV != nil && *b.MaxV > acc.max {
			acc.max = *b.MaxV
		}
		if b.BucketStart >= acc.lastBucketStart && b.LastV != nil {
			acc.last = *b.LastV
			acc.lastBucketStart = b.BucketStart
		}
	}

	rows := make([]map[string]interface{}, 0, len(order))
	for _, start := range order {
		acc := accs[start]
		var y float64
		switch aggregateFunc {
		case "avg":
			y = acc.sum / float64(acc.count)
		case "min":
			if math.IsInf(acc.min, 1) {
				continue
			}
			y = acc.min
		case "max":
			if math.IsInf(acc.max, -1) {
				continue
			}
			y = acc.max
		case "sum":
			y = acc.sum
		case "count":
			y = float64(acc.count)
		case "last":
			y = acc.last
		default:
			return nil, fmt.Errorf("不支持的冷层聚合函数: %s", aggregateFunc)
		}
		rows = append(rows, map[string]interface{}{"x": start, "y": y})
	}
	return rows, nil
}
