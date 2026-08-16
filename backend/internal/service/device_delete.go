package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	protocolplugin "aetherlink-iot/backend/internal/service/protocol_plugin"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

// DeleteDevice removes a device after permission, dependency, data, cache, and runtime cleanup checks.
func (*Device) DeleteDevice(id string, userClaims *utils.UserClaims) error {
	deviceInfo, err := loadDeviceForDelete(id, userClaims)
	if err != nil {
		return err
	}
	if err := ensureDeviceDeleteDependenciesCleared(id); err != nil {
		return err
	}

	if err := deleteDeviceTransactionalData(id, deviceInfo); err != nil {
		return err
	}

	finalizeDeviceDelete(id, deviceInfo)
	return nil
}

func loadDeviceForDelete(id string, userClaims *utils.UserClaims) (*model.Device, error) {
	deviceInfo, err := ensureDeviceDeleteAccess(id, userClaims)
	if err != nil {
		return nil, normalizeDeviceDeleteAccessError(err)
	}
	return deviceInfo, nil
}

func normalizeDeviceDeleteAccessError(err error) error {
	var appErr *errcode.Error
	if errors.As(err, &appErr) {
		return err
	}
	return deviceDBError(err)
}

func ensureDeviceDeleteAccess(id string, userClaims *utils.UserClaims) (*model.Device, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	if userClaims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to delete device")
	}
	deviceInfo, err := dal.GetDeviceByID(id)
	if err != nil {
		return nil, err
	}
	if userClaims.Authority != constant.SYS_ADMIN && deviceInfo.TenantID != userClaims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to delete device")
	}
	if userClaims.Authority == constant.TENANT_USER && !deviceOwnerMatchesClaims(deviceInfo, userClaims) {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to delete device")
	}
	return deviceInfo, nil
}

func ensureDeviceDeleteDependenciesCleared(id string) error {
	if err := ensureDeviceDeleteHasNoSubDevices(id); err != nil {
		return err
	}

	if err := ensureDeviceDeleteHasNoSceneTriggers(id); err != nil {
		return err
	}

	return nil
}

func ensureDeviceDeleteHasNoSubDevices(id string) error {
	data, err := dal.GetSubDeviceListByParentID(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if len(data) > 0 {
		return errcode.WithData(200063, map[string]interface{}{
			"message": "device has sub device,please remove sub device first",
		})
	}

	return nil
}

func ensureDeviceDeleteHasNoSceneTriggers(id string) error {
	conditions, err := dal.GetDeviceTriggerConditionListByDeviceId(id)
	if err != nil {
		return err
	}
	if len(conditions) > 0 {
		return errcode.WithData(200062, map[string]interface{}{
			"message": "device has scene,please remove scene first",
		})
	}

	return nil
}

func deleteDeviceTransactionalData(id string, deviceInfo *model.Device) error {
	tx, err := dal.StartTransaction()
	if err != nil {
		return deviceDBError(err)
	}

	if err := applyDeviceDeleteTransaction(tx, id, deviceInfo.TenantID); err != nil {
		return deviceDBError(err)
	}

	return nil
}

func applyDeviceDeleteTransaction(tx *query.QueryTx, id, tenantID string) error {
	if err := deleteDeviceRelatedRows(tx, id, tenantID); err != nil {
		return rollbackDeviceDelete(tx, err)
	}

	return dal.Commit(tx)
}

func deleteDeviceRelatedRows(tx *query.QueryTx, id, tenantID string) error {
	if err := deleteDeviceTelemetryRows(tx, id); err != nil {
		return err
	}

	if err := deleteDeviceAttributeAndEventRows(tx, id); err != nil {
		return err
	}

	if err := deleteDeviceCommandRows(tx, id); err != nil {
		return err
	}

	if err := dal.DeleteOTAUpgradeTaskDetailsByDeviceIDTx(id, tx); err != nil {
		return err
	}

	return deleteDeviceRecord(tx, id, tenantID)
}

func deleteDeviceTelemetryRows(tx *query.QueryTx, id string) error {
	if err := dal.DeleteCurrentTelemetryDataByDeviceId(id, tx); err != nil {
		return err
	}
	if err := dal.DeleteTelemetrDataByDeviceId(id, tx); err != nil {
		return err
	}
	if err := dal.DeleteTelemetrySetLogsByDeviceId(id, tx); err != nil {
		return err
	}
	return nil
}

func deleteDeviceAttributeAndEventRows(tx *query.QueryTx, id string) error {
	if err := dal.DeleteAttributeDataByDeviceId(id, tx); err != nil {
		return err
	}
	if err := dal.DeleteAttributeDataByDeviceIdTx(id, tx); err != nil {
		return err
	}
	if err := dal.DeleteEventDataByDeviceId(id, tx); err != nil {
		return err
	}

	return nil
}

func deleteDeviceCommandRows(tx *query.QueryTx, id string) error {
	if err := dal.DeleteCommandSetLogsByDeviceId(id, tx); err != nil {
		return err
	}
	return nil
}

func deleteDeviceRecord(tx *query.QueryTx, id, tenantID string) error {
	return dal.DeleteDeviceWithTx(id, tenantID, tx)
}

func finalizeDeviceDelete(id string, deviceInfo *model.Device) {
	deleteDeviceVoucherCache(id, deviceInfo.Voucher)
	if disconnectErr := protocolplugin.DisconnectDeviceByDeviceSnapshot(deviceInfo); disconnectErr != nil {
		logrus.Error("DisconnectDeviceByDeviceID failed")
	}
}

func deleteDeviceVoucherCache(deviceID, voucher string) {
	if global.REDIS == nil || strings.TrimSpace(voucher) == "" {
		return
	}
	if err := global.REDIS.Del(context.Background(), voucher).Err(); err != nil {
		logrus.Warn("delete device voucher cache failed")
	}
}

func deleteDeviceCache(deviceID string) {
	if global.REDIS == nil {
		return
	}
	if err := initialize.DelDeviceCache(deviceID); err != nil {
		logrus.Warn("delete device cache failed")
	}
}

func rollbackDeviceDelete(tx *query.QueryTx, cause error) error {
	if rollbackErr := dal.Rollback(tx); rollbackErr != nil {
		return joinDeviceDeleteRollbackError(cause, rollbackErr)
	}
	return cause
}

func joinDeviceDeleteRollbackError(cause, rollbackErr error) error {
	if rollbackErr == nil {
		return cause
	}
	return errors.Join(cause, fmt.Errorf("rollback device delete transaction: %w", rollbackErr))
}

func deviceDBError(err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}
