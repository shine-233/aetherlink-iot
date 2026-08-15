// 文件用途：维护设备当前遥测数据读取、筛选和删除服务。
// 核心逻辑：校验设备读取权限，从时序/缓存数据中取当前值和 key 列表，并支持 WebSocket 查询路径。
// 关键注意事项：当前值直接驱动前端实时视图，空设备、跨租户和缓存缺失必须有稳定错误语义。
// 重构建议：拆出当前值仓储和权限 helper，补齐缓存失败、WS 路径、删除权限和时间戳测试。
package service

import (
	"encoding/json"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

type telemetryCurrentModelIndex struct {
	byKey          map[string]*model.DeviceModelTelemetry
	unitByKey      map[string]interface{}
	readWriteByKey map[string]interface{}
}

func loadTelemetryCurrentModelIndex(deviceInfo *model.Device) (telemetryCurrentModelIndex, error) {
	index := telemetryCurrentModelIndex{
		byKey:          make(map[string]*model.DeviceModelTelemetry),
		unitByKey:      make(map[string]interface{}),
		readWriteByKey: make(map[string]interface{}),
	}
	if deviceInfo == nil || deviceInfo.DeviceConfigID == nil {
		return index, nil
	}

	deviceConfig, err := dal.GetDeviceConfigByID(*deviceInfo.DeviceConfigID)
	if err != nil {
		return index, err
	}
	if deviceConfig.DeviceTemplateID == nil {
		return index, nil
	}

	telemetryModel, err := dal.GetDeviceModelTelemetryDataList(*deviceConfig.DeviceTemplateID)
	if err != nil {
		return index, err
	}
	for _, v := range telemetryModel {
		index.byKey[v.DataIdentifier] = v
		index.unitByKey[v.DataIdentifier] = v.Unit
		index.readWriteByKey[v.DataIdentifier] = v.ReadWriteFlag
	}
	return index, nil
}

func telemetryCurrentValue(v *model.TelemetryCurrentData) interface{} {
	var value interface{}
	if v.BoolV != nil {
		value = v.BoolV
	}
	if v.NumberV != nil {
		value = v.NumberV
	}
	if v.StringV != nil {
		value = v.StringV
	}
	return value
}

func applyTelemetryCurrentModel(row map[string]interface{}, key string, index telemetryCurrentModelIndex, includeReadWrite bool) {
	telemetryModel, ok := index.byKey[key]
	if !ok {
		return
	}

	row["label"] = telemetryModel.DataName
	row["unit"] = index.unitByKey[key]
	if includeReadWrite {
		row["read_write_flag"] = index.readWriteByKey[key]
	}
	row["data_type"] = telemetryModel.DataType
	if telemetryModel.DataType != nil && *telemetryModel.DataType == "Enum" && telemetryModel.AdditionalInfo != nil {
		var enumItems []model.EnumItem
		json.Unmarshal([]byte(*telemetryModel.AdditionalInfo), &enumItems)
		row["enum"] = enumItems
	}
}

func buildTelemetryCurrentRows(data []*model.TelemetryCurrentData, index telemetryCurrentModelIndex, includeReadWrite bool) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, len(data))
	for _, v := range data {
		tmp := map[string]interface{}{
			"device_id": v.DeviceID,
			"key":       v.Key,
			"ts":        v.T,
			"tenant_id": v.TenantID,
		}
		if value := telemetryCurrentValue(v); value != nil {
			tmp["value"] = value
		}
		applyTelemetryCurrentModel(tmp, v.Key, index, includeReadWrite)
		rows = append(rows, tmp)
	}
	return rows
}

func (*TelemetryData) GetCurrentTelemetrData(device_id string, claims *utils.UserClaims) (interface{}, error) {
	deviceInfo, err := ensureTelemetryDeviceReadAccess(device_id, claims)
	if err != nil {
		return nil, err
	}
	// d, err := dal.GetCurrentTelemetrData(device_id)
	// 数据源替换
	d, err := dal.GetCurrentTelemetryDataEvolution(device_id)
	if err != nil {
		return nil, err
	}

	modelIndex, err := loadTelemetryCurrentModelIndex(deviceInfo)
	if err != nil {
		return nil, err
	}

	return buildTelemetryCurrentRows(d, modelIndex, true), err
}

// 根据设备ID和key获取当前遥测数据
func (*TelemetryData) GetCurrentTelemetrDataKeys(req *model.GetTelemetryCurrentDataKeysReq, claims *utils.UserClaims) (interface{}, error) {
	deviceInfo, err := ensureTelemetryDeviceReadAccess(req.DeviceID, claims)
	if err != nil {
		return nil, err
	}
	// d, err := dal.GetCurrentTelemetrData(device_id)
	// 数据源替换
	d, err := dal.GetCurrentTelemetryDataEvolutionByKeys(req.DeviceID, req.Keys)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	modelIndex, err := loadTelemetryCurrentModelIndex(deviceInfo)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return buildTelemetryCurrentRows(d, modelIndex, false), err
}

// 返回数据格式{"key":value,"key1":value1}
func (*TelemetryData) GetCurrentTelemetrDataForWs(device_id string, claims *utils.UserClaims) (interface{}, error) {
	if _, err := ensureTelemetryDeviceReadAccess(device_id, claims); err != nil {
		return nil, err
	}
	// d, err := dal.GetCurrentTelemetrData(device_id)

	// 数据源替换
	d, err := dal.GetCurrentTelemetryDataEvolution(device_id)
	if err != nil {
		return nil, err
	}

	// 格式化返回值
	data := make(map[string]interface{})
	if len(d) > 0 {
		for _, v := range d {
			if v.BoolV != nil {
				data[v.Key] = v.BoolV
			}
			if v.NumberV != nil {
				data[v.Key] = v.NumberV
			}
			if v.StringV != nil {
				data[v.Key] = v.StringV
			}
		}
	}
	return data, err
}

// 返回数据格式{"key":value,"key1":value1}
func (*TelemetryData) GetCurrentTelemetrDataKeysForWs(device_id string, keys []string, claims *utils.UserClaims) (interface{}, error) {
	if _, err := ensureTelemetryDeviceReadAccess(device_id, claims); err != nil {
		return nil, err
	}
	// d, err := dal.GetCurrentTelemetrData(device_id)

	// 数据源替换
	d, err := dal.GetCurrentTelemetryDataEvolutionByKeys(device_id, keys)
	if err != nil {
		return nil, err
	}

	// 格式化返回值
	data := make(map[string]interface{})
	if len(d) > 0 {
		for _, v := range d {
			if v.BoolV != nil {
				data[v.Key] = v.BoolV
			}
			if v.NumberV != nil {
				data[v.Key] = v.NumberV
			}
			if v.StringV != nil {
				data[v.Key] = v.StringV
			}
		}
	}
	return data, err
}

func (*TelemetryData) DeleteTelemetrData(req *model.DeleteTelemetryDataReq, claims *utils.UserClaims) error {
	if _, err := ensureTelemetryDeviceWriteAccess(req.DeviceID, claims); err != nil {
		return err
	}

	err := dal.DeleteTelemetrData(req.DeviceID, req.Key)
	if err != nil {
		return err
	}
	// 删除当前值
	err = dal.DeleteCurrentTelemetryData(req.DeviceID, req.Key)
	return err
}

func (*TelemetryData) GetCurrentTelemetrDetailData(device_id string, claims *utils.UserClaims) (interface{}, error) {
	if _, err := ensureTelemetryDeviceReadAccess(device_id, claims); err != nil {
		return nil, err
	}

	data, err := dal.GetCurrentTelemetrDetailData(device_id)
	if err != nil {
		return nil, err
	}

	dataMap := make(map[string]interface{})

	dataMap["device_id"] = data.DeviceID
	dataMap["key"] = data.Key
	dataMap["ts"] = data.T
	dataMap["tenant_id"] = data.TenantID

	if data.BoolV != nil {
		dataMap["value"] = data.BoolV
	}

	if data.NumberV != nil {
		dataMap["value"] = data.NumberV
	}

	if data.StringV != nil {
		dataMap["value"] = data.StringV
	}

	return dataMap, err
}
