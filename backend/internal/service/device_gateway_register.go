// device_gateway_register.go 负责网关注册与子设备补注册，
// 核心关注点是租户边界、MQTT 凭证生成和网关子设备默认字段补齐。
package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

func resolveGatewayRegisterDeviceConfigID(configName string) *string {
	return normalizeCreateDeviceConfigID(dal.GetDeviceConfigIdByName(configName))
}

func applyGatewayRegisterModelFields(device *model.Device, modelName string) {
	device.Name = &modelName
	device.DeviceConfigID = resolveGatewayRegisterDeviceConfigID(modelName)
}

func applyGatewayRegisterDeviceDefaults(device *model.Device, tenantID string, timestamp *time.Time) {
	device.TenantID = tenantID
	device.CreatedAt = timestamp
	device.UpdateAt = timestamp
	device.IsOnline = 1
	device.ActivateFlag = "active"
}

func buildGatewayRegisterVoucher(username, password string) string {
	return fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)
}

func buildGatewaySubDeviceVoucher(username string) string {
	return fmt.Sprintf(`{"username":"%s"}`, username)
}

func gatewaySubDeviceExistsRegisterRes(subAddr string) model.DeviceSubRegisterRes {
	return model.DeviceSubRegisterRes{
		Result:    1,
		Errorcode: "exists",
		SubAddr:   subAddr,
	}
}

// buildGatewaySubDevice 基于网关和子设备描述组装一条待落库的子设备记录，
// 这里会继承父网关租户，并补齐在线默认状态与随机凭证。
func buildGatewaySubDevice(parentDeviceID string, tenantID string, timestamp *time.Time, item model.DeviceSubItem) model.Device {
	subDevice := model.Device{}
	subDevice.ID = uuid.New()
	applyGatewayRegisterModelFields(&subDevice, item.Model)
	subDevice.ParentID = &parentDeviceID
	subDevice.Voucher = buildGatewaySubDeviceVoucher(uuid.New())
	applyGatewayRegisterDeviceDefaults(&subDevice, tenantID, timestamp)
	subDevice.DeviceNumber = uuid.New()
	subDevice.SubDeviceAddr = &item.SubAddr
	return subDevice
}

func registerGatewaySubDevice(parentDeviceID string, tenantID string, timestamp *time.Time, item model.DeviceSubItem) model.DeviceSubRegisterRes {
	subDevice := buildGatewaySubDevice(parentDeviceID, tenantID, timestamp, item)
	subRegisterRes := model.DeviceSubRegisterRes{
		Result:    0,
		Errorcode: "",
		Message:   "成功",
		SubAddr:   item.SubAddr,
	}
	if err := dal.CreateDevice(&subDevice); err != nil {
		subRegisterRes.Result = 1
		subRegisterRes.Errorcode = "exists"
	}
	return subRegisterRes
}

// GatewayRegister returns existing MQTT credentials for a tenant gateway, or creates a new gateway access record.
func (*Device) GatewayRegister(req model.GatewayRegisterReq, claims *utils.UserClaims) (model.GatewayRegisterRes, error) {
	if claims == nil || claims.TenantID == "" {
		return model.GatewayRegisterRes{}, errcode.NewWithMessage(errcode.CodeNoPermission, "网关注册需要 API Key")
	}

	// 网关重复注册时复用既有 MQTT 身份，避免相同网关号产生多份凭证。
	device, err := dal.GetDeviceByDeviceNumber(req.GatewayId)
	if err == nil {
		if claims.Authority != constant.SYS_ADMIN && device.TenantID != claims.TenantID {
			return model.GatewayRegisterRes{}, errcode.NewWithMessage(errcode.CodeNoPermission, "无权为其他租户注册网关")
		}
		return buildExistingGatewayRegisterRes(device)
	}

	result, device := buildNewGatewayRegisterDevice(req, claims)
	return result, dal.CreateDevice(device)
}

func buildExistingGatewayRegisterRes(device *model.Device) (model.GatewayRegisterRes, error) {
	// Phase 2b：新行 voucher 列为空串，重复注册回显改从 24h 网页测试缓存解析；
	// 缓存也未命中（过期/SQL 直插行）时返回与模拟器一致的轮换指引，而非 DB 错误。
	voucherJSON := device.Voucher
	if strings.TrimSpace(voucherJSON) == "" {
		cached, err := dal.LoadDeviceCredentialTestCache(device.ID)
		if err != nil {
			return model.GatewayRegisterRes{}, errcode.NewWithMessage(errcode.CodeNotFound,
				"gateway credential test cache expired or absent; rotate the voucher to regenerate gateway credentials")
		}
		voucherJSON = cached
	}

	var voucher model.DeviceVoucher
	if err := json.Unmarshal([]byte(voucherJSON), &voucher); err != nil {
		return model.GatewayRegisterRes{}, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
			"message":   "解析网关凭证失败",
		})
	}

	return model.GatewayRegisterRes{
		MqttUsername: voucher.Username,
		MqttPassword: voucher.Password,
		MqttClientId: device.ID,
	}, nil
}

func buildNewGatewayRegisterDevice(req model.GatewayRegisterReq, claims *utils.UserClaims) (model.GatewayRegisterRes, *model.Device) {
	result := model.GatewayRegisterRes{
		MqttUsername: uuid.New()[0:22],
		MqttPassword: uuid.New()[0:7],
		MqttClientId: uuid.New(),
	}
	t := time.Now().UTC()

	device := &model.Device{}
	device.ID = result.MqttClientId
	applyGatewayRegisterModelFields(device, req.Model)
	logrus.Info(device.DeviceConfigID)
	device.Voucher = buildGatewayRegisterVoucher(result.MqttUsername, result.MqttPassword)
	applyGatewayRegisterDeviceDefaults(device, claims.TenantID, &t)
	device.DeviceNumber = req.GatewayId
	return result, device
}

// GatewayDeviceRegister fills missing sub-devices for a tenant gateway and returns per-address creation results.
func (*Device) GatewayDeviceRegister(req model.DeviceRegisterReq, claims *utils.UserClaims) (model.DeviceRegisterRes, error) {
	if claims == nil || claims.TenantID == "" {
		return model.DeviceRegisterRes{}, errcode.NewWithMessage(errcode.CodeNoPermission, "网关子设备注册需要 API Key")
	}

	device, err := dal.GetDeviceByIDUnscoped(req.DeviceId)
	if err != nil {
		return model.DeviceRegisterRes{
			Type:    "sub-register-response",
			Status:  "fail",
			Message: "未查询到网关设备信息",
		}, nil
	}
	if claims.Authority != constant.SYS_ADMIN && device.TenantID != claims.TenantID {
		return model.DeviceRegisterRes{}, errcode.NewWithMessage(errcode.CodeNoPermission, "无权为其他租户注册子设备")
	}
	res := model.DeviceRegisterRes{
		Type:         "sub-register-response",
		Status:       "success",
		Message:      "成功",
		RegistersRes: make(map[string]model.DeviceSubRegisterRes),
	}
	t := time.Now().UTC()

	for _, v := range req.Registers {
		// 子设备地址作为网关下的唯一键，已存在时返回 exists 而不是覆盖旧记录。
		if dal.GetSubDeviceExists(req.DeviceId, v.SubAddr) {
			res.RegistersRes[v.SubAddr] = gatewaySubDeviceExistsRegisterRes(v.SubAddr)
			continue
		}
		res.RegistersRes[v.SubAddr] = registerGatewaySubDevice(req.DeviceId, device.TenantID, &t, v)
	}

	return res, nil
}
