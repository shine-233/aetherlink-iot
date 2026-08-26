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

// isolatedTelemetryCurrent 返回从全新 gorm Statement 出发的 telemetry_current_datas 链起点。
// 批次三收敛（2026-08，见 references/gen-inheritance-audit.md）：这是全栈最热读面
// （board/twin/details/diagnostics 高频并发），包级表单例的继承式语句根在高负载下会
// 跨请求残留 Statement 条件，导致 SELECT 读到跨设备陈旧 rows（症状：CI 03_data
// telemetry snapshot 深比较失败）。Session{NewDB:true} 强制每次操作都使用零起点的
// 全新语句，与 device_config P1 修复同构；写侧 raw tx 链（storage/telemetry_writer.go）不受影响。
func isolatedTelemetryCurrent() query.ITelemetryCurrentDataDo {
	return query.TelemetryCurrentData.Session(&gorm.Session{NewDB: true})
}

// 从 telemetry_current_datas 中获取遥测当前数据，用于替换 telemetry_datas
// tenant-scope: caller-enforced?2026-08-26 ?????
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

	// 读侧收敛：从全新 Statement 出发，杜绝单例残留条件并入执行语句。
	data, err := isolatedTelemetryCurrent().
		Where(query.TelemetryCurrentData.DeviceID.Eq(deviceId)).
		Order(query.TelemetryCurrentData.T.Desc()).
		Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
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
	// 批次三收敛：本函数整体改走 raw global.DB 链（clone==1 根，每次链式起点全新
	// Statement），与 users.go 登录选择器同构。历史上 global.DB 为空时会回落到
	// 继承式 gen 兜底链；但 Session{NewDB} 起点会丢失 Statement.Model 表绑定，
	// 使 Count 直接报 "Table not set"（gorm v1.31.2 / gen v0.3.28 实测），
	// 且生产环境 global.DB 与 query.SetDefault 恒成对初始化，该兜底不可达，故一并移除。
	if global.DB == nil {
		return 0, nil, gorm.ErrInvalidDB
	}

	var latest model.TelemetryCurrentData
	latestResult := global.DB.
		Where("device_id = ?", deviceId).
		Order("ts DESC").
		Limit(1).
		Take(&latest)
	if latestResult.Error != nil {
		if errors.Is(latestResult.Error, gorm.ErrRecordNotFound) {
			return 0, nil, nil
		}
		return 0, nil, latestResult.Error
	}

	var currentCount int64
	if err := global.DB.
		Model(&model.TelemetryCurrentData{}).
		Where("device_id = ?", deviceId).
		Count(&currentCount).Error; err != nil {
		return 0, nil, err
	}
	if currentCount == 0 {
		currentCount = 1
	}
	return currentCount, &latest, nil
}

// 从 telemetry_current_datas 中获取遥测当前数据，用于替换 telemetry_datas
// tenant-scope: caller-enforced?2026-08-26 ?????
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

	// 读侧收敛：批量读从全新 Statement 出发，避免残留条件污染 In 集合过滤。
	data, err := isolatedTelemetryCurrent().
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

// tenant-scope: caller-enforced?2026-08-26 ?????
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

	// 读侧收敛：按 keys 过滤的热读从全新 Statement 出发。
	data, err := isolatedTelemetryCurrent().
		Where(query.TelemetryCurrentData.DeviceID.Eq(deviceId), query.TelemetryCurrentData.Key.In(keys...)).
		Order(query.TelemetryCurrentData.T.Desc()).
		Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetCurrentTelemetryDataOneKeys(deviceId string, keys string) (interface{}, error) {
	// 读侧收敛：单 key 读从全新 Statement 出发；既有 ErrRecordNotFound 返回语义保持逐字节一致。
	data, err := isolatedTelemetryCurrent().
		Where(query.TelemetryCurrentData.DeviceID.Eq(deviceId), query.TelemetryCurrentData.Key.Eq(keys)).
		Order(query.TelemetryCurrentData.T.Desc()).
		First()
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
	// 收敛说明：删除链同样从全新 Statement 出发，切断跨请求条件继承；
	// 既有"未命中行不报错"的返回语义保持不变（RowsAffected 守卫由调用方契约决定）。
	_, err := isolatedTelemetryCurrent().
		Where(query.TelemetryCurrentData.DeviceID.Eq(deviceId), query.TelemetryCurrentData.Key.Eq(key)).
		Delete()
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
