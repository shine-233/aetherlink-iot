// 文件用途：设备 Modbus 点表服务层（ROADMAP B1）。
// 核心逻辑：前端在线编辑点表（按设备 ID），插件经 OpenAPI Key 按设备编号拉取。
// 关键注意事项：profile 只允许映射信息（target/registers 等），拒绝出现凭证字段；
//   大小上限 64KB 防止滥用；租户边界全部走设备访问守卫。
package service

import (
	"encoding/json"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

const deviceModbusProfileMaxBytes = 64 * 1024

// DeviceModbusProfile 设备 Modbus 点表服务入口。
type DeviceModbusProfile struct{}

// forbiddenProfileKeys 点表中不允许出现的凭证类字段（大小写不敏感）。
var forbiddenProfileKeys = []string{"username", "password", "voucher", "api_key", "apikey", "secret"}

// ModbusProfileResp 点表响应。
type ModbusProfileResp struct {
	DeviceID  string     `json:"device_id"`
	Profile   any        `json:"profile"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	UpdatedBy string     `json:"updated_by,omitempty"`
}

func validateModbusProfileShape(raw []byte) error {
	if len(raw) == 0 || !json.Valid(raw) {
		return errcode.NewWithMessage(errcode.CodeParamError, "profile must be valid json")
	}
	if len(raw) > deviceModbusProfileMaxBytes {
		return errcode.NewWithMessage(errcode.CodeParamError, "profile exceeds size limit")
	}
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "profile must be a json object")
	}
	for key := range generic {
		lower := strings.ToLower(key)
		for _, banned := range forbiddenProfileKeys {
			if lower == banned {
				return errcode.NewWithMessage(errcode.CodeParamError, "profile must not contain credential fields: "+key)
			}
		}
	}
	registersRaw, ok := generic["registers"]
	if ok {
		var registers []map[string]json.RawMessage
		if err := json.Unmarshal(registersRaw, &registers); err != nil {
			return errcode.NewWithMessage(errcode.CodeParamError, "profile.registers must be an array")
		}
	}
	return nil
}

// SaveProfile 前端保存点表。raw 为完整 profile 对象文本。
func (*DeviceModbusProfile) SaveProfile(deviceId string, raw []byte, claims *utils.UserClaims) (map[string]interface{}, error) {
	if claims == nil || claims.ID == "" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	if deviceId == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	if _, err := ensureTelemetryDeviceWriteAccess(deviceId, claims); err != nil {
		return nil, err
	}
	if err := validateModbusProfileShape(raw); err != nil {
		return nil, err
	}
	updatedAt, err := dal.UpsertDeviceModbusProfile(deviceId, string(raw), claims.ID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return map[string]interface{}{
		"device_id":  deviceId,
		"updated_at": updatedAt,
	}, nil
}

// GetProfileForUser 前端读取点表（读权限守卫）。
func (*DeviceModbusProfile) GetProfileForUser(deviceId string, claims *utils.UserClaims) (*ModbusProfileResp, error) {
	if _, err := ensureTelemetryDeviceReadAccess(deviceId, claims); err != nil {
		return nil, err
	}
	row, err := dal.GetDeviceModbusProfile(deviceId, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return buildProfileResp(deviceId, row), nil
}

// GetProfileByDeviceNumber 插件按设备编号拉取点表（x-api-key 等价 claims，仍做租户校验）。
func (*DeviceModbusProfile) GetProfileByDeviceNumber(deviceNumber string, claims *utils.UserClaims) (*ModbusProfileResp, error) {
	if claims == nil {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	if deviceNumber == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_number is required")
	}
	deviceInfo, err := dal.GetDeviceByDeviceNumber(deviceNumber)
	if err != nil || deviceInfo == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "device not found")
	}
	if deviceInfo.TenantID != claims.TenantID {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	row, err := dal.GetDeviceModbusProfile(deviceInfo.ID, deviceInfo.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	resp := buildProfileResp(deviceInfo.ID, row)
	return resp, nil
}

func buildProfileResp(deviceId string, row *model.DeviceModbusProfile) *ModbusProfileResp {
	resp := &ModbusProfileResp{DeviceID: deviceId}
	if row == nil {
		resp.Profile = map[string]interface{}{}
		return resp
	}
	var profile any
	if row.Profile != nil {
		_ = json.Unmarshal([]byte(*row.Profile), &profile)
	}
	if profile == nil {
		profile = map[string]interface{}{}
	}
	resp.Profile = profile
	resp.UpdatedAt = row.UpdatedAt
	if row.UpdatedBy != nil {
		resp.UpdatedBy = *row.UpdatedBy
	}
	return resp
}
