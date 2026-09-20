// 文件用途：提供设备地理空间与历史轨迹数据访问层（TB-13 Geospatial Map Tracking）。
// 核心逻辑：高效检索设备经纬度、速度与海拔时序点阵，用于看板地图部件实时标绘与历史轨迹回放。
package dal

import (
	"context"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
)

// GetDeviceTrajectoryTelemetry 查询指定设备在时间范围内的轨迹与位置遥测数据
func GetDeviceTrajectoryTelemetry(deviceID string, startTime, endTime int64, limit int) ([]*model.TelemetryData, error) {
	keys := []string{"latitude", "lat", "longitude", "lng", "lon", "long", "speed", "altitude", "location", "gps"}
	q := query.TelemetryData
	qb := q.WithContext(context.Background()).
		Where(q.DeviceID.Eq(deviceID)).
		Where(q.Key.In(keys...))
	if startTime > 0 {
		qb = qb.Where(q.T.Gte(startTime))
	}
	if endTime > 0 {
		qb = qb.Where(q.T.Lte(endTime))
	}
	if limit > 0 {
		qb = qb.Limit(limit)
	}
	return qb.Order(q.T.Asc()).Find()
}
