// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	global "aetherlink-iot/backend/pkg/global"
	"context"
	"fmt"
)

type TelemetryDatasAggregate struct {
	AggregateWindow   int64  `json:"aggregate_window"`   // 聚合间隔
	AggregateFunction string `json:"aggregate_function"` // 聚合函数
	STime             int64  `json:"s_time"`
	ETime             int64  `json:"e_time"`
	Count             int64  `json:"count"`
	DeviceID          string `json:"device_id"`
	Key               string `json:"key"`
}

// 聚合查询
func GetTelemetryDatasAggregate(_ context.Context, telemetryDatasAggregate TelemetryDatasAggregate) ([]map[string]interface{}, error) {
	var data []map[string]interface{}
	var queryString string

	// 根据聚合方法获取不同的查询sql
	switch telemetryDatasAggregate.AggregateFunction {
	case "avg", "max", "min", "sum":
		queryString = GetQueryString1(telemetryDatasAggregate.AggregateFunction)
	case "diff":
		queryString = GetQueryString2(telemetryDatasAggregate.AggregateFunction)

	default:
		return nil, fmt.Errorf("不支持的聚合函数: %s", telemetryDatasAggregate.AggregateFunction)
	}

	resultData := global.DB.Raw(queryString, aggregateQueryArgs(telemetryDatasAggregate)...).Scan(&data)
	if resultData.Error != nil {
		return nil, resultData.Error
	}

	return data, nil

}

func aggregateQueryArgs(telemetryDatasAggregate TelemetryDatasAggregate) []interface{} {
	aggregateWindowSeconds := telemetryDatasAggregate.AggregateWindow / 1000
	if aggregateWindowSeconds < 1 {
		aggregateWindowSeconds = 1
	}

	return []interface{}{
		telemetryDatasAggregate.STime,
		telemetryDatasAggregate.ETime,
		telemetryDatasAggregate.Key,
		telemetryDatasAggregate.DeviceID,
		aggregateWindowSeconds,
		aggregateWindowSeconds,
	}
}

// 获取queryString，支持平均值，最大值，最小值，合计
func GetQueryString1(aggregateFunction string) string {
	queryString := fmt.Sprintf(
		`WITH FilteredData AS (
				SELECT
					ts / 1000 AS ts_sec,
					number_v
				FROM
					telemetry_datas
				WHERE
					ts BETWEEN ? AND ? AND key = ? AND device_id = ?
					AND number_v IS NOT NULL
					AND abs(number_v) < 1e15
			),
			TimeIntervals AS (
				SELECT
					ts_sec - (ts_sec %% ?) AS x,
					%s(number_v) AS y
				FROM
					FilteredData
				GROUP BY
					x
			)
			SELECT
				x * 1000 AS x,
				(x + ?) * 1000 AS x2,
				y
			FROM
				TimeIntervals
			WHERE
				y IS NOT NULL
			ORDER BY
				x ASC;`,
		aggregateFunction,
	)
	return queryString
}

// 获取queryString，支持差值计算
func GetQueryString2(_ string) string {
	queryString := fmt.Sprintf(
		`WITH FilteredData AS (
				SELECT
					ts / 1000 AS ts_sec,
					number_v
				FROM
					telemetry_datas
				WHERE
					ts BETWEEN ? AND ? AND key = ? AND device_id = ?
					AND number_v IS NOT NULL
					AND abs(number_v) < 1e15
			),
			TimeIntervals AS (
				SELECT
					ts_sec - (ts_sec %% ?) AS x,
					MAX(number_v) - MIN(number_v) AS y
				FROM
					FilteredData
				GROUP BY
					x
			)
			SELECT
				x * 1000 AS x,
				(x + ?) * 1000 AS x2,
				y
			FROM
				TimeIntervals
			WHERE
				y IS NOT NULL
			ORDER BY
				x ASC;`,
	)

	return queryString
}
