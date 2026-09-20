// 文件用途：实现设备地理空间追踪与历史轨迹业务逻辑（TB-13 Geospatial Map Tracking）。
// 核心逻辑：提供多设备最新 GPS 经纬度位置标绘聚合与单设备历史行驶轨迹点阵抽取。
package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

// GetLatestDeviceLocations 获取当前租户/分组下全部设备的最新地理空间坐标与状态
func (*Device) GetLatestDeviceLocations(ctx context.Context, req *model.DeviceLocationLatestReq, claims *utils.UserClaims) (*model.DeviceLocationLatestResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}

	limit := 200
	if req.Limit > 0 {
		limit = req.Limit
	}
	listReq := &model.GetDeviceListByPageReq{
		PageReq: model.PageReq{
			Page:     1,
			PageSize: limit,
		},
		Search:  req.SearchKey,
		GroupId: req.GroupID,
	}
	if req.IsOnline != nil {
		status := int(*req.IsOnline)
		listReq.IsOnline = &status
	}

	scopes, err := resolveDeviceListScopes(listReq, claims)
	if err != nil {
		return nil, err
	}
	applyDeviceListOwnerFilterForClaims(listReq, claims)

	total, deviceList, err := dal.GetDeviceListByPageForScopes(listReq, scopes)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	resp := &model.DeviceLocationLatestResp{
		List:  make([]model.DeviceLocationLatestItem, 0, len(deviceList)),
		Total: total,
	}

	if len(deviceList) == 0 {
		return resp, nil
	}

	deviceIDs := make([]string, 0, len(deviceList))
	for _, d := range deviceList {
		deviceIDs = append(deviceIDs, d.ID)
	}

	telemetryMap, err := dal.GetCurrentTelemetryDataEvolutionByDeviceIDs(deviceIDs)
	if err != nil {
		logrus.Warnf("failed to load telemetry map for locations: %v", err)
		telemetryMap = make(map[string][]*model.TelemetryCurrentData)
	}

	for _, d := range deviceList {
		item := model.DeviceLocationLatestItem{
			DeviceID:     d.ID,
			DeviceName:   d.Name,
			DeviceNumber: d.DeviceNumber,
			TenantID:     d.TenantID,
			IsOnline:     int16(d.IsOnline),
			Attributes:   make(map[string]interface{}),
		}

		telemetries := telemetryMap[d.ID]
		var latestTs int64
		for _, t := range telemetries {
			if t == nil {
				continue
			}
			tMillis := t.T.UnixMilli()
			if tMillis > latestTs {
				latestTs = tMillis
			}

			key := strings.ToLower(strings.TrimSpace(t.Key))
			val := extractTelemetryCurrentValue(t)
			item.Attributes[t.Key] = val

			switch key {
			case "latitude", "lat":
				if num := parseCoordinateFloat(val); num != nil && isValidLatitude(*num) {
					item.Latitude = num
				}
			case "longitude", "lng", "lon", "long":
				if num := parseCoordinateFloat(val); num != nil && isValidLongitude(*num) {
					item.Longitude = num
				}
			case "speed":
				if num := parseCoordinateFloat(val); num != nil {
					item.Speed = num
				}
			case "altitude", "alt":
				if num := parseCoordinateFloat(val); num != nil {
					item.Altitude = num
				}
			case "location", "gps":
				lat, lng, speed, addr := parseLocationJSONOrString(val)
				if item.Latitude == nil && lat != nil {
					item.Latitude = lat
				}
				if item.Longitude == nil && lng != nil {
					item.Longitude = lng
				}
				if item.Speed == nil && speed != nil {
					item.Speed = speed
				}
				if item.Address == nil && addr != nil {
					item.Address = addr
				}
			case "address":
				if str, ok := val.(string); ok && str != "" {
					item.Address = &str
				}
			}
		}

		// 若遥测中未提供经纬度，尝试从设备静态属性 location 中解析
		if item.Latitude == nil || item.Longitude == nil {
			if d.Location != "" {
				lat, lng, _, addr := parseLocationJSONOrString(d.Location)
				if item.Latitude == nil && lat != nil {
					item.Latitude = lat
				}
				if item.Longitude == nil && lng != nil {
					item.Longitude = lng
				}
				if item.Address == nil && addr != nil {
					item.Address = addr
				}
			}
		}

		if latestTs > 0 {
			item.LastTime = &latestTs
		} else if d.Ts != nil {
			ts := d.Ts.UnixMilli()
			item.LastTime = &ts
		}

		resp.List = append(resp.List, item)
	}

	return resp, nil
}

// GetDeviceLocationHistory 查询单设备在指定时间范围内的运动轨迹点
func (*Device) GetDeviceLocationHistory(ctx context.Context, deviceID string, req *model.DeviceLocationHistoryReq, claims *utils.UserClaims) (*model.DeviceLocationHistoryResp, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device id is required")
	}

	device, err := ensureTelemetryDeviceReadAccess(deviceID, claims)
	if err != nil {
		return nil, err
	}

	to := req.To
	if to <= 0 {
		to = time.Now().UnixMilli()
	}
	from := req.From
	if from <= 0 {
		from = to - 24*3600*1000 // 默认拉取最近 24 小时轨迹
	}
	limit := 1000
	if req.Limit > 0 && req.Limit <= 2000 {
		limit = req.Limit
	}

	rawPoints, err := dal.GetDeviceTrajectoryTelemetry(deviceID, from, to, limit*2)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	type trajectoryBuilder struct {
		ts       int64
		lat      *float64
		lng      *float64
		speed    *float64
		altitude *float64
	}

	pointsByTs := make(map[int64]*trajectoryBuilder)
	tsList := make([]int64, 0)

	for _, pt := range rawPoints {
		if pt == nil {
			continue
		}
		pb, exists := pointsByTs[pt.T]
		if !exists {
			pb = &trajectoryBuilder{ts: pt.T}
			pointsByTs[pt.T] = pb
			tsList = append(tsList, pt.T)
		}

		key := strings.ToLower(strings.TrimSpace(pt.Key))
		var numVal *float64
		if pt.NumberV != nil {
			numVal = pt.NumberV
		} else if pt.StringV != nil {
			if f, parseErr := strconv.ParseFloat(strings.TrimSpace(*pt.StringV), 64); parseErr == nil {
				numVal = &f
			}
		}

		switch key {
		case "latitude", "lat":
			if numVal != nil && isValidLatitude(*numVal) {
				pb.lat = numVal
			}
		case "longitude", "lng", "lon", "long":
			if numVal != nil && isValidLongitude(*numVal) {
				pb.lng = numVal
			}
		case "speed":
			if numVal != nil {
				pb.speed = numVal
			}
		case "altitude", "alt":
			if numVal != nil {
				pb.altitude = numVal
			}
		case "location", "gps":
			if pt.StringV != nil {
				lat, lng, speed, _ := parseLocationJSONOrString(*pt.StringV)
				if pb.lat == nil && lat != nil {
					pb.lat = lat
				}
				if pb.lng == nil && lng != nil {
					pb.lng = lng
				}
				if pb.speed == nil && speed != nil {
					pb.speed = speed
				}
			}
		}
	}

	points := make([]model.DeviceLocationHistoryPoint, 0, len(tsList))
	for _, ts := range tsList {
		pb := pointsByTs[ts]
		if pb != nil && pb.lat != nil && pb.lng != nil {
			points = append(points, model.DeviceLocationHistoryPoint{
				Timestamp: pb.ts,
				Latitude:  *pb.lat,
				Longitude: *pb.lng,
				Speed:     pb.speed,
				Altitude:  pb.altitude,
			})
			if len(points) >= limit {
				break
			}
		}
	}

	deviceName := ""
	if device.Name != nil {
		deviceName = *device.Name
	}

	return &model.DeviceLocationHistoryResp{
		DeviceID:   deviceID,
		DeviceName: deviceName,
		Points:     points,
		Total:      len(points),
	}, nil
}

func extractTelemetryCurrentValue(t *model.TelemetryCurrentData) any {
	if t.NumberV != nil {
		return *t.NumberV
	}
	if t.StringV != nil {
		return *t.StringV
	}
	if t.BoolV != nil {
		return *t.BoolV
	}
	return nil
}

func parseCoordinateFloat(v any) *float64 {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case float64:
		return &val
	case float32:
		f := float64(val)
		return &f
	case int:
		f := float64(val)
		return &f
	case int64:
		f := float64(val)
		return &f
	case int32:
		f := float64(val)
		return &f
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			return &f
		}
	}
	return nil
}

func isValidLatitude(lat float64) bool {
	return lat >= -90.0 && lat <= 90.0
}

func isValidLongitude(lng float64) bool {
	return lng >= -180.0 && lng <= 180.0
}

func parseLocationJSONOrString(raw any) (lat *float64, lng *float64, speed *float64, address *string) {
	if raw == nil {
		return
	}
	var str string
	switch v := raw.(type) {
	case string:
		str = strings.TrimSpace(v)
	case map[string]interface{}:
		return parseLocationMap(v)
	default:
		return
	}

	if str == "" {
		return
	}

	// 1. 尝试解析为 JSON 对象
	if strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}") {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(str), &m); err == nil {
			return parseLocationMap(m)
		}
	}

	// 2. 尝试解析 "lng,lat" 或 "lat,lng"
	delims := []string{",", ";", " "}
	for _, delim := range delims {
		if strings.Contains(str, delim) {
			parts := strings.Split(str, delim)
			validParts := make([]float64, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if f, err := strconv.ParseFloat(p, 64); err == nil {
					validParts = append(validParts, f)
				}
			}
			if len(validParts) >= 2 {
				p0 := validParts[0]
				p1 := validParts[1]
				// 若其中一个在 (90, 180] 之间，则该值为经度
				if p0 > 90 && p0 <= 180 && isValidLatitude(p1) {
					lng = &p0
					lat = &p1
					return
				}
				if p1 > 90 && p1 <= 180 && isValidLatitude(p0) {
					lat = &p0
					lng = &p1
					return
				}
				// 否则按照国内/高德惯例 [lng, lat] 或通用 [lat, lng] 判定有效区间
				if isValidLongitude(p0) && isValidLatitude(p1) {
					lng = &p0
					lat = &p1
					return
				}
				if isValidLatitude(p0) && isValidLongitude(p1) {
					lat = &p0
					lng = &p1
					return
				}
			}
		}
	}

	// 3. 作为普通地址文字
	address = &str
	return
}

func parseLocationMap(m map[string]interface{}) (lat *float64, lng *float64, speed *float64, address *string) {
	for k, v := range m {
		kLower := strings.ToLower(strings.TrimSpace(k))
		switch kLower {
		case "latitude", "lat":
			if num := parseCoordinateFloat(v); num != nil && isValidLatitude(*num) {
				lat = num
			}
		case "longitude", "lng", "lon", "long":
			if num := parseCoordinateFloat(v); num != nil && isValidLongitude(*num) {
				lng = num
			}
		case "speed":
			speed = parseCoordinateFloat(v)
		case "address", "addr":
			if s, ok := v.(string); ok {
				address = &s
			}
		}
	}
	return
}
