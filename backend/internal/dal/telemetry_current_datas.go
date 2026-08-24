// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	tptodb "aetherlink-iot/backend/third_party/grpc/tptodb_client"
	pb "aetherlink-iot/backend/third_party/grpc/tptodb_client/grpc_tptodb"
)

// 从 telemetry_current_datas 中获取遥测当前数据，用于替换 telemetry_datas
func GetCurrentTelemetryDataEvolution(deviceId string) ([]*model.TelemetryCurrentData, error) {
	dbType := viper.GetString("grpc.tptodb_type")
	if dbType == "TSDB" || dbType == "KINGBASE" || dbType == "POLARDB" {
		var telemetry []*model.TelemetryCurrentData
		request := &pb.GetDeviceAttributesCurrentsRequest{
			DeviceId: deviceId,
		}

		r, err := tptodb.TelemetryQueryClient.GetDeviceAttributesCurrents(context.Background(), request)
		if err != nil {
			logrus.Printf("GetDeviceAttributesCurrents err:%+v", err)
			return nil, err
		}
		err = json.Unmarshal([]byte(r.Data), &telemetry)
		if err != nil {
			logrus.Printf("Unmarshal err:%v", err)
			return nil, err
		}
		return telemetry, nil
	}

	data, err := query.TelemetryCurrentData.Where(query.TelemetryCurrentData.DeviceID.Eq(deviceId)).Order(query.TelemetryCurrentData.T.Desc()).Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetCurrentTelemetryReadiness(deviceId string) (int64, *model.TelemetryCurrentData, error) {
	dbType := viper.GetString("grpc.tptodb_type")
	if dbType == "TSDB" || dbType == "KINGBASE" || dbType == "POLARDB" {
		telemetry, err := GetCurrentTelemetryDataEvolution(deviceId)
		if err != nil {
			return 0, nil, err
		}
		if len(telemetry) == 0 {
			return 0, nil, nil
		}
		return int64(len(telemetry)), telemetry[0], nil
	}

	return getCurrentTelemetryReadinessFromDB(deviceId)
}

func getCurrentTelemetryReadinessFromDB(deviceId string) (int64, *model.TelemetryCurrentData, error) {
	if global.DB != nil {
		var latest model.TelemetryCurrentData
		latestResult := global.DB.
			Where("device_id = ?", deviceId).
			Order("ts DESC").
			Limit(1).
			Take(&latest)
		if latestResult.Error == nil {
			var currentCount int64
			countResult := global.DB.
				Model(&model.TelemetryCurrentData{}).
				Where("device_id = ?", deviceId).
				Count(&currentCount)
			if countResult.Error == nil {
				if currentCount == 0 {
					currentCount = 1
				}
				return currentCount, &latest, nil
			}
		} else if errors.Is(latestResult.Error, gorm.ErrRecordNotFound) {
			return 0, nil, nil
		}
	}

	latest, err := query.TelemetryCurrentData.
		Where(query.TelemetryCurrentData.DeviceID.Eq(deviceId)).
		Order(query.TelemetryCurrentData.T.Desc()).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil, nil
		}
		return 0, nil, err
	}

	currentCount, err := query.TelemetryCurrentData.
		Where(query.TelemetryCurrentData.DeviceID.Eq(deviceId)).
		Count()
	if err != nil {
		return 0, nil, err
	}
	if currentCount == 0 {
		currentCount = 1
	}
	return currentCount, latest, nil
}

// 从 telemetry_current_datas 中获取遥测当前数据，用于替换 telemetry_datas
func GetCurrentTelemetryDataEvolutionByDeviceIDs(deviceIDs []string) (map[string][]*model.TelemetryCurrentData, error) {
	normalizedIDs := make([]string, 0, len(deviceIDs))
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" {
			continue
		}
		if _, ok := seen[deviceID]; ok {
			continue
		}
		seen[deviceID] = struct{}{}
		normalizedIDs = append(normalizedIDs, deviceID)
	}

	result := make(map[string][]*model.TelemetryCurrentData, len(normalizedIDs))
	if len(normalizedIDs) == 0 {
		return result, nil
	}

	dbType := viper.GetString("grpc.tptodb_type")
	if dbType == "TSDB" || dbType == "KINGBASE" || dbType == "POLARDB" {
		for _, deviceID := range normalizedIDs {
			telemetry, err := GetCurrentTelemetryDataEvolution(deviceID)
			if err != nil {
				return nil, err
			}
			result[deviceID] = telemetry
		}
		return result, nil
	}

	data, err := query.TelemetryCurrentData.
		Where(query.TelemetryCurrentData.DeviceID.In(normalizedIDs...)).
		Order(query.TelemetryCurrentData.DeviceID, query.TelemetryCurrentData.T.Desc()).
		Find()
	if err != nil {
		return nil, err
	}
	for _, item := range data {
		if item == nil {
			continue
		}
		result[item.DeviceID] = append(result[item.DeviceID], item)
	}
	return result, nil
}

func GetCurrentTelemetryDataEvolutionByKeys(deviceId string, keys []string) ([]*model.TelemetryCurrentData, error) {
	dbType := viper.GetString("grpc.tptodb_type")
	if dbType == "TSDB" || dbType == "KINGBASE" || dbType == "POLARDB" {
		data := make([]*model.TelemetryCurrentData, 0)
		request := &pb.GetDeviceAttributesCurrentsRequest{
			DeviceId:  deviceId,
			Attribute: keys,
		}
		r, err := tptodb.TelemetryQueryClient.GetDeviceAttributesCurrents(context.Background(), request)
		if err != nil {
			logrus.Printf("err: %+v", err)
			return nil, err
		}

		err = json.Unmarshal([]byte(r.Data), &data)
		if err != nil {
			logrus.Printf("Unmarshal err:%v", err)
			return nil, err
		}

		return data, nil
	}

	data, err := query.TelemetryCurrentData.Where(query.TelemetryCurrentData.DeviceID.Eq(deviceId), query.TelemetryCurrentData.Key.In(keys...)).Order(query.TelemetryCurrentData.T.Desc()).Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetCurrentTelemetryDataOneKeys(deviceId string, keys string) (interface{}, error) {
	data, err := query.TelemetryCurrentData.Where(query.TelemetryCurrentData.DeviceID.Eq(deviceId), query.TelemetryCurrentData.Key.Eq(keys)).Order(query.TelemetryCurrentData.T.Desc()).First()
	var result interface{}
	if err != nil {
		return result, err
		//} else if err == gorm.ErrRecordNotFound {
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, nil
	}
	if data.BoolV != nil {
		// result = fmt.Sprintf("%t", *data.BoolV)
		result = *data.BoolV
	}
	if data.NumberV != nil {
		// result = fmt.Sprintf("%f", *data.NumberV)
		result = *data.NumberV
	}
	if data.StringV != nil {
		result = *data.StringV
	}
	return result, nil
}

// 根据ID和key删除当前遥测数据
func DeleteCurrentTelemetryData(deviceId string, key string) error {
	_, err := query.TelemetryCurrentData.Where(query.TelemetryCurrentData.DeviceID.Eq(deviceId), query.TelemetryCurrentData.Key.Eq(key)).Delete()
	return err
}

// 根据ID删除当前遥测数据
func DeleteCurrentTelemetryDataByDeviceId(deviceId string, tx *query.QueryTx) error {
	_, err := tx.TelemetryCurrentData.Where(query.TelemetryCurrentData.DeviceID.Eq(deviceId)).Delete()
	return err
}

type NewDeviceData struct {
	DeviceID  string    `json:"device_id"`
	Timestamp time.Time `json:"timestamp"`
}

// 获取租户下最近上报数据的三个设备的遥测数据
func GetTenantTelemetryData(tenantId string, ownerUserID *string) ([]NewDeviceData, error) {
	type DeviceData struct {
		DeviceID string    `json:"device_id"`
		MaxT     time.Time `json:"max_t"`
	}

	var devices []DeviceData
	sql := `
		SELECT t.device_id, MAX(t.ts) AS max_t
		FROM telemetry_current_datas t
		JOIN devices d ON d.id = t.device_id
		WHERE t.tenant_id = ? AND d.tenant_id = ? AND d.activate_flag = 'active'`
	args := []interface{}{tenantId, tenantId}
	if ownerUserID != nil && strings.TrimSpace(*ownerUserID) != "" {
		sql += " AND d.owner_user_id = ?"
		args = append(args, strings.TrimSpace(*ownerUserID))
	}
	sql += " GROUP BY t.device_id ORDER BY MAX(t.ts) DESC LIMIT 3"

	err := global.DB.Raw(sql, args...).Scan(&devices).Error
	if err != nil {
		return nil, err
	}

	result := make([]NewDeviceData, 0, len(devices))
	for _, device := range devices {
		deviceInfo := NewDeviceData{
			DeviceID:  device.DeviceID,
			Timestamp: device.MaxT,
		}
		result = append(result, deviceInfo)
	}

	return result, nil
}
