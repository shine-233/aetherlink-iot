package service

import (
	"context"
	"strconv"
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	protocolplugin "aetherlink-iot/backend/internal/service/protocol_plugin"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
)

// GetDeviceList 返回当前租户下可绑定到网关的候选子设备。
func (*Device) GetDeviceList(ctx context.Context, userClaims *utils.UserClaims, req *model.GetUnboundGatewaySubDeviceReq) ([]map[string]interface{}, error) {
	tenantID, err := requireDeviceTenantClaims(userClaims, "no permission to query unbound gateway devices")
	if err != nil {
		return nil, err
	}
	list, err := dal.DeviceQuery{}.GetGatewayUnrelatedDeviceList(ctx, tenantID, req.Search, req.DeviceType, deviceOwnerUserIDFilterForClaims(userClaims))
	if err != nil {
		logrus.Error(ctx, "[GetDeviceList]failed:", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return list, err
}

// CreateSonDevice 将子设备绑定到网关设备，并校验租户、设备类型和父设备拓扑。
func (*Device) CreateSonDevice(ctx context.Context, param *model.CreateSonDeviceRes, claims *utils.UserClaims) error {
	if param == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "bind sub-device request is required")
	}
	sonIDs := parseChildDeviceIDs(param.SonID)
	if len(sonIDs) == 0 {
		return errcode.NewWithMessage(errcode.CodeParamError, "sub-device id is required")
	}

	parentDevice, err := ensureParentGatewayDevice(ctx, param.ID, claims)
	if err != nil {
		return err
	}

	if err := bindChildDevicesTransaction(ctx, parentDevice, sonIDs, claims); err != nil {
		return err
	}

	if err := protocolplugin.DisconnectDeviceByDeviceID(parentDevice.ID); err != nil {
		logrus.Error(err)
	}
	return nil
}

func parseChildDeviceIDs(raw string) []string {
	seen := make(map[string]bool)
	ids := make([]string, 0)
	for _, rawID := range strings.Split(raw, ",") {
		id := strings.TrimSpace(rawID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

// ensureParentGatewayDevice 校验父设备存在、可写，并确认其配置是网关类型。
func ensureParentGatewayDevice(ctx context.Context, parentID string, claims *utils.UserClaims) (*model.Device, error) {
	parentDevice, err := ensureTelemetryDeviceWriteAccess(parentID, claims)
	if err != nil {
		return nil, err
	}
	if parentDevice.DeviceConfigID == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "parent gateway device config is required")
	}

	parentDeviceConfig, err := dal.DeviceConfigQuery{}.First(ctx, query.DeviceConfig.ID.Eq(*parentDevice.DeviceConfigID))
	if err != nil {
		logrus.Error(ctx, "[CreateSonDevice]First parent device_configs failed:", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if parentDeviceConfig.DeviceType != strconv.Itoa(constant.GATEWAY_DEVICE) {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error":       "parent device must be gateway device",
			"device_type": parentDeviceConfig.DeviceType,
		})
	}
	return parentDevice, nil
}

// bindChildDevicesTransaction 在事务中锁定父设备并逐个绑定子设备，避免并发修改拓扑。
func bindChildDevicesTransaction(ctx context.Context, parentDevice *model.Device, sonIDs []string, claims *utils.UserClaims) error {
	tx, err := dal.StartTransaction()
	if err != nil {
		return deviceDBError(err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = dal.Rollback(tx)
		}
	}()

	lockedParentDevice, err := dal.GetDeviceByIDForUpdateWithTenant(tx, parentDevice.ID, parentDevice.TenantID)
	if err != nil {
		return deviceDBError(err)
	}
	if err := ensureLockedParentGatewayDevice(ctx, tx, lockedParentDevice); err != nil {
		return err
	}
	for _, sonID := range sonIDs {
		if err := bindChildDevice(ctx, tx, lockedParentDevice, sonID, claims); err != nil {
			return err
		}
	}

	if err := dal.Commit(tx); err != nil {
		return deviceDBError(err)
	}
	committed = true
	return nil
}

// ensureLockedParentGatewayDevice 在事务锁内再次校验父设备仍然是网关，避免并发配置变更。
func ensureLockedParentGatewayDevice(ctx context.Context, tx *query.QueryTx, parentDevice *model.Device) error {
	if parentDevice.DeviceConfigID == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "parent gateway device config is required")
	}

	parentDeviceConfig, err := tx.DeviceConfig.WithContext(ctx).Where(tx.DeviceConfig.ID.Eq(*parentDevice.DeviceConfigID)).First()
	if err != nil {
		logrus.Error(ctx, "[CreateSonDevice]First locked parent device_configs failed:", err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if parentDeviceConfig.DeviceType != strconv.Itoa(constant.GATEWAY_DEVICE) {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error":       "parent device must be gateway device",
			"device_type": parentDeviceConfig.DeviceType,
		})
	}
	return nil
}

// bindChildDevice 校验单个子设备归属和类型后写入父子关系。
func bindChildDevice(ctx context.Context, tx *query.QueryTx, parentDevice *model.Device, sonID string, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify device telemetry")
	}

	deviceInfo, err := tx.Device.WithContext(ctx).
		Where(tx.Device.ID.Eq(sonID)).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First()
	if err != nil {
		logrus.Error(ctx, "[CreateSonDevice]First failed:", err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if claims.Authority != constant.SYS_ADMIN && deviceInfo.TenantID != claims.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify device telemetry")
	}
	if claims.Authority == constant.TENANT_USER && !deviceOwnerMatchesClaims(deviceInfo, claims) {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify device telemetry")
	}
	if deviceInfo.TenantID != parentDevice.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "parent and sub-device must belong to the same tenant")
	}
	if deviceInfo.ParentID != nil || deviceInfo.DeviceConfigID == nil {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "sub-device must be unbound and have device config",
			"id":    sonID,
		})
	}

	deviceConfig, err := dal.DeviceConfigQuery{}.First(ctx, query.DeviceConfig.ID.Eq(*deviceInfo.DeviceConfigID))
	if err != nil {
		logrus.Error(ctx, "[CreateSonDevice]First device_configs failed:", err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if deviceConfig.DeviceType != strconv.Itoa(constant.GATEWAY_DEVICE) && deviceConfig.DeviceType != strconv.Itoa(constant.GATEWAY_SON_DEVICE) {
		logrus.Error(ctx, "[CreateSonDevice]Invalid device type:", deviceConfig.DeviceType)
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error":       "子设备类型不支持绑定到当前网关",
			"device_type": deviceConfig.DeviceType,
		})
	}

	if err := dal.BindChildDeviceWithTx(tx, sonID, parentDevice.TenantID, parentDevice.ID, sonID); err != nil {
		logrus.Error(ctx, "[CreateSonDevice]update failed:", err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}
