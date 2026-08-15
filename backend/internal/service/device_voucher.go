package service

import (
	"context"

	"aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	protocolplugin "aetherlink-iot/backend/internal/service/protocol_plugin"
	common "aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

// UpdateDeviceVoucher 更新设备凭证，校验唯一性后清理旧凭证缓存并按协议类型决定是否断连。
func (*Device) UpdateDeviceVoucher(ctx context.Context, param *model.UpdateDeviceVoucherReq, claims *utils.UserClaims) (string, error) {
	var (
		db = dal.DeviceQuery{}
	)

	deviceInfo, voucher, voucherChanged, err := prepareVoucherUpdate(ctx, param, claims)
	if err != nil {
		return "", err
	}
	if param.Voucher == "{}" {
		return "", nil
	}

	info, err := persistAndReloadVoucher(ctx, db, param.DeviceID, voucher)
	if err != nil {
		return voucher, err
	}

	// 设备配置或凭证变化后清理设备缓存，避免连接信息继续使用旧配置。
	initialize.DelDeviceCache(param.DeviceID)
	if voucherChanged {
		handleUpdatedVoucherSideEffects(ctx, param.DeviceID, deviceInfo)
	}

	return info.Voucher, nil
}

// serializeUpdateDeviceVoucher 将凭证请求统一序列化为字符串，兼容字符串和结构化凭证。
func serializeUpdateDeviceVoucher(ctx context.Context, voucher any) (string, error) {
	if v, ok := voucher.(string); ok {
		return v, nil
	}

	result, err := common.JsonToString(voucher)
	if err != nil {
		logrus.Error(ctx, "[Device][UpdateDeviceVoucher]JsonToString failed:", err)
		return "", err
	}
	return result, nil
}

// prepareVoucherUpdate 加载设备、序列化新凭证并校验凭证唯一性，返回后续持久化所需上下文。
func prepareVoucherUpdate(ctx context.Context, param *model.UpdateDeviceVoucherReq, claims *utils.UserClaims) (*model.Device, string, bool, error) {
	deviceInfo, err := ensureTelemetryDeviceWriteAccess(param.DeviceID, claims)
	if err != nil {
		return nil, "", false, err
	}

	voucher, err := serializeUpdateDeviceVoucher(ctx, param.Voucher)
	if err != nil {
		return nil, "", false, err
	}

	voucherChanged := deviceInfo.Voucher != voucher
	if err := ensureUpdatedVoucherAvailable(voucher, param.DeviceID, voucherChanged); err != nil {
		return nil, "", false, err
	}

	return deviceInfo, voucher, voucherChanged, nil
}

// persistAndReloadVoucher 封装凭证更新后的数据库写入和回读。
func persistAndReloadVoucher(
	ctx context.Context,
	db dal.DeviceQuery,
	deviceID string,
	voucher string,
) (*model.Device, error) {
	device := query.Device
	info := &model.Device{
		ID:      deviceID,
		Voucher: voucher,
	}
	if err := db.Update(ctx, info, device.Voucher); err != nil {
		logrus.Error(ctx, "[Device][UpdateDeviceVoucher]failed:", err)
		return nil, err
	}

	reloaded, err := db.First(ctx, device.ID.Eq(deviceID))
	if err != nil {
		logrus.Error(ctx, "[Device][UpdateDeviceVoucher]first failed:", err)
		return nil, err
	}
	return reloaded, nil
}

// ensureUpdatedVoucherAvailable 仅在凭证发生变化时检查唯一性，避免误报当前设备已有凭证。
func ensureUpdatedVoucherAvailable(voucher string, deviceID string, changed bool) error {
	if !changed {
		return nil
	}

	exists, err := dal.CheckVoucherExists(voucher, deviceID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
			"message":   "check voucher exists failed",
		})
	}
	if exists {
		return errcode.New(204005)
	}
	return nil
}

// handleUpdatedVoucherSideEffects 在凭证更新后删除旧缓存、刷新设备缓存，并按协议类型决定是否断连。
func handleUpdatedVoucherSideEffects(ctx context.Context, deviceID string, deviceInfo *model.Device) {
	if err := global.REDIS.Del(ctx, deviceInfo.Voucher).Err(); err != nil {
		logrus.WithError(err).WithField("device_id", deviceID).Warn("[Device][UpdateDeviceVoucher]delete old voucher cache failed")
	}
	disconnectNonMQTTDeviceAfterVoucherChange(ctx, deviceID, deviceInfo)
}

// disconnectNonMQTTDeviceAfterVoucherChange 针对非 MQTT 设备断开旧连接，让协议插件重新使用新凭证建链。
func disconnectNonMQTTDeviceAfterVoucherChange(ctx context.Context, deviceID string, deviceInfo *model.Device) {
	if deviceInfo.DeviceConfigID == nil {
		return
	}

	deviceConfig, err := dal.GetDeviceConfigByID(*deviceInfo.DeviceConfigID)
	if err != nil {
		logrus.Error(ctx, "[Device][UpdateDeviceVoucher]GetDeviceConfigByID failed:", err)
		return
	}
	if !shouldDisconnectAfterVoucherChange(deviceConfig) {
		return
	}
	if disconnectErr := protocolplugin.DisconnectDeviceByDeviceID(deviceID); disconnectErr != nil {
		logrus.Error(ctx, "[Device][UpdateDeviceVoucher]DisconnectDeviceByDeviceID failed:", disconnectErr)
	}
}

func shouldDisconnectAfterVoucherChange(deviceConfig *model.DeviceConfig) bool {
	if deviceConfig.ProtocolType == nil || *deviceConfig.ProtocolType == "MQTT" {
		return false
	}
	return deviceConfig.DeviceType == "1" || deviceConfig.DeviceType == "2"
}
