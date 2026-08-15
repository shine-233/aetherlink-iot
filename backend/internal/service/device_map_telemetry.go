// device_map_telemetry.go 负责地图场景下的设备遥测摘要查询，
// 会同时返回设备在线态、最近上报时间以及模板标签/单位映射后的遥测值。
package service

import (
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

func (*Device) GetMapTelemetry(deviceID string, claims *utils.UserClaims) (map[string]interface{}, error) {
	device, err := loadMapTelemetryDevice(deviceID, claims)
	if err != nil {
		return nil, err
	}

	telemetry, err := loadMapTelemetryCurrentData(deviceID)
	if err != nil {
		return nil, err
	}

	labelMap, err := loadMapTelemetryLabels(device, deviceID, telemetry)
	if err != nil {
		return nil, err
	}

	return buildMapTelemetryResponse(device, telemetry, labelMap), nil
}

// loadMapTelemetryDevice 先完成设备存在性与 owner/租户/RDI 共享权限校验，
// 后续遥测与标签查询都以这一步拿到的设备信息为准。地图遥测是只读场景，
// 因此走 allowSharedRead=true 的读权限门（与 ensureTelemetryDeviceReadAccess 同级），
// 保证同租户非属主用户在没有共享授权时被拒绝，而不是仅做存在性校验。
func loadMapTelemetryDevice(deviceID string, claims *utils.UserClaims) (*model.Device, error) {
	accessContext, err := ensureTelemetryDeviceAccess(deviceID, claims, "no permission to query device map telemetry", true)
	if err != nil {
		return nil, err
	}
	return accessContext.device, nil
}

func loadMapTelemetryCurrentData(deviceID string) ([]*model.TelemetryCurrentData, error) {
	telemetry, err := dal.GetCurrentTelemetryDataEvolution(deviceID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get device current telemetry failed:" + err.Error(),
			"id":    deviceID,
		})
	}
	return telemetry, nil
}

func loadMapTelemetryLabels(
	device *model.Device,
	deviceID string,
	telemetry []*model.TelemetryCurrentData,
) ([]*model.DeviceModelTelemetry, error) {
	// 物模型标签是可选增强信息；没有设备配置时仍允许返回原始遥测键和值。
	if device.DeviceConfigID == nil {
		return nil, nil
	}

	deviceConfig, err := dal.GetDeviceConfigByID(*device.DeviceConfigID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get device config failed:" + err.Error(),
			"id":    deviceID,
		})
	}

	if deviceConfig.DeviceTemplateID == nil {
		return nil, nil
	}

	labelMap, err := dal.GetDataNameByIdentifierAndTemplateId(*deviceConfig.DeviceTemplateID, mapTelemetryKeys(telemetry)...)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get thing model failed:" + err.Error(),
			"id":    deviceID,
		})
	}
	return labelMap, nil
}

func mapTelemetryKeys(telemetry []*model.TelemetryCurrentData) []string {
	keys := make([]string, 0)
	for _, v := range telemetry {
		if v != nil {
			keys = append(keys, v.Key)
		}
	}
	return keys
}

func buildMapTelemetryResponse(
	device *model.Device,
	telemetry []*model.TelemetryCurrentData,
	labelMap []*model.DeviceModelTelemetry,
) map[string]interface{} {
	res := make(map[string]interface{}, 0)
	res["device_id"] = device.ID
	res["is_online"] = device.IsOnline
	if len(telemetry) > 0 {
		res["last_push_time"] = telemetry[0].T
	} else {
		res["last_push_time"] = nil
	}
	res["telemetry_data"] = buildMapTelemetryRows(telemetry, labelMap)
	res["device_name"] = device.Name

	return res
}

type mapTelemetryLabel struct {
	label *string
	unit  *string
}

func buildMapTelemetryRows(
	telemetry []*model.TelemetryCurrentData,
	labelMap []*model.DeviceModelTelemetry,
) []map[string]interface{} {
	telemetryData := make([]map[string]interface{}, 0)
	if len(telemetry) == 0 {
		return telemetryData
	}

	// 标签索引按 key 预聚合，避免逐条遥测反复扫描模板定义。
	labelIndex := indexMapTelemetryLabels(labelMap)
	for _, v := range telemetry {
		telemetryData = append(telemetryData, buildMapTelemetryRow(v, labelIndex))
	}
	return telemetryData
}

func indexMapTelemetryLabels(labelMap []*model.DeviceModelTelemetry) map[string]mapTelemetryLabel {
	labelIndex := make(map[string]mapTelemetryLabel, len(labelMap))
	for _, v := range labelMap {
		labelIndex[v.DataIdentifier] = mapTelemetryLabel{
			label: v.DataName,
			unit:  v.Unit,
		}
	}
	return labelIndex
}

func buildMapTelemetryRow(
	telemetry *model.TelemetryCurrentData,
	labelIndex map[string]mapTelemetryLabel,
) map[string]interface{} {
	row := make(map[string]interface{})
	row["key"] = telemetry.Key

	if telemetry.BoolV != nil {
		row["value"] = telemetry.BoolV
	} else if telemetry.NumberV != nil {
		row["value"] = telemetry.NumberV
	} else if telemetry.StringV != nil {
		row["value"] = telemetry.StringV
	}

	var label *string
	var unit *string
	if item, ok := labelIndex[telemetry.Key]; ok {
		label = item.label
		unit = item.unit
	}
	row["label"] = label
	row["unit"] = unit
	return row
}
