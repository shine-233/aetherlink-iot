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
	deviceInfo, voucher, voucherChanged, err := prepareVoucherUpdate(ctx, param, claims)
	if err != nil {
		return "", err
	}
	if param.Voucher == "{}" {
		return "", nil
	}

	info, err := persistAndReloadVoucher(ctx, param.DeviceID, voucher)
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
		logrus.Error("[Device][UpdateDeviceVoucher] JsonToString failed")
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
	deviceID string,
	voucher string,
) (*model.Device, error) {
	// info.Voucher 仅作为 voucher_hash 与网页测试缓存的计算输入驻留内存，不落库。
	info := &model.Device{
		ID:      deviceID,
		Voucher: voucher,
	}
	// 凭证哈希存储 Phase 1/2b（references/backend-hardening-plan.md 车道1）：gen 模型无
	// VoucherHash 字段；Phase 2b 停写明文后不再经 gen Select(voucher) 回写列（避免明文
	// 出现在任何落库语句里），改由同事务内单条 raw UPDATE 写 voucher_hash 并把 voucher
	// 列置空串、逐设备写入网页测试缓存（dal.WriteVoucherHashInQueryTx 收口）。
	// 设备行不存在时 UPDATE 零命中静默通过，随后 reload First 报 NotFound，语义与原
	// gen 更新路径一致。
	if err := query.Q.Transaction(func(tx *query.Query) error {
		return dal.WriteVoucherHashInQueryTx(tx, []*model.Device{info})
	}); err != nil {
		return nil, err
	}

	reloaded, err := dal.DeviceQuery{}.First(ctx, query.Device.ID.Eq(deviceID))
	if err != nil {
		logrus.Error("[Device][UpdateDeviceVoucher] reload failed")
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
	// broker 侧缓存键为 sha256(voucher) 十六进制摘要，删键必须走同一哈希构造，否则旧凭证缓存无法失效。
	if err := global.REDIS.Del(ctx, utils.VoucherCacheKey(deviceInfo.Voucher)).Err(); err != nil {
		logrus.Warn("[Device][UpdateDeviceVoucher] delete old voucher cache failed")
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
		logrus.Error("[Device][UpdateDeviceVoucher] GetDeviceConfigByID failed")
		return
	}
	if !shouldDisconnectAfterVoucherChange(deviceConfig) {
		return
	}
	if disconnectErr := protocolplugin.DisconnectDeviceByDeviceID(deviceID); disconnectErr != nil {
		logrus.Error("[Device][UpdateDeviceVoucher] DisconnectDeviceByDeviceID failed")
	}
}

func shouldDisconnectAfterVoucherChange(deviceConfig *model.DeviceConfig) bool {
	if deviceConfig.ProtocolType == nil || *deviceConfig.ProtocolType == "MQTT" {
		return false
	}
	return deviceConfig.DeviceType == "1" || deviceConfig.DeviceType == "2"
}
