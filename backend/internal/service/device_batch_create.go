package service

import (
	"encoding/json"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/pluginruntime"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

type batchCreateDeviceContext struct {
	serviceAccess *model.ServiceAccess
	createdAt     time.Time
	ownerUserID   *string
}

func buildBatchCreateDeviceContext(serviceAccessID string, claims *utils.UserClaims) (*batchCreateDeviceContext, error) {
	serviceAccess, err := ensureServiceAccessWriteAccess(serviceAccessID, claims)
	if err != nil {
		return nil, err
	}
	return &batchCreateDeviceContext{
		serviceAccess: serviceAccess,
		createdAt:     time.Now().UTC(),
		ownerUserID:   createdDeviceOwnerUserID(claims),
	}, nil
}

func shouldSkipBatchCreateDeviceItem(item model.BatchCreateDevice) bool {
	return item.DeviceName == "" && item.DeviceNumber == "" && item.DeviceConfigId == ""
}

func validateBatchCreateDeviceRequiredFields(item model.BatchCreateDevice) error {
	if item.DeviceNumber == "" {
		return errcode.WithVars(100005, map[string]interface{}{
			"field": "device_number",
		})
	}
	if item.DeviceConfigId == "" {
		return errcode.WithVars(100005, map[string]interface{}{
			"field": "device_config_id",
		})
	}
	if item.DeviceName == "" {
		return errcode.WithVars(100005, map[string]interface{}{
			"field": "device_name",
		})
	}
	return nil
}

func collectBatchCreateDeviceNumbers(items []model.BatchCreateDevice) ([]string, error) {
	deviceNumbers := make([]string, 0, len(items))
	seenDeviceNumbers := make(map[string]struct{}, len(items))
	for _, item := range items {
		if shouldSkipBatchCreateDeviceItem(item) {
			continue
		}
		if err := validateBatchCreateDeviceRequiredFields(item); err != nil {
			return nil, err
		}
		if _, ok := seenDeviceNumbers[item.DeviceNumber]; ok {
			continue
		}
		seenDeviceNumbers[item.DeviceNumber] = struct{}{}
		deviceNumbers = append(deviceNumbers, item.DeviceNumber)
	}
	return deviceNumbers, nil
}

func existingBatchCreateDeviceNumbers(items []model.BatchCreateDevice) (map[string]bool, error) {
	deviceNumbers, err := collectBatchCreateDeviceNumbers(items)
	if err != nil {
		return nil, err
	}
	existingNumbers, err := dal.CheckDeviceNumbersExists(deviceNumbers)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return existingNumbers, nil
}

func validateBatchCreateDeviceTenant(item model.BatchCreateDevice, claims *utils.UserClaims, serviceAccess *model.ServiceAccess) error {
	_, err := ensureWritableDeviceConfigForTenant(
		item.DeviceConfigId,
		claims,
		serviceAccess.TenantID,
		"device config and service access tenant mismatch",
	)
	return err
}

func buildBatchCreateDeviceModel(item model.BatchCreateDevice, ctx *batchCreateDeviceContext, serviceAccessID string) *model.Device {
	device := &model.Device{
		ID:              uuid.New(),
		Name:            &item.DeviceName,
		DeviceNumber:    item.DeviceNumber,
		Voucher:         `{"username":"` + uuid.New()[0:22] + `"}`,
		TenantID:        ctx.serviceAccess.TenantID,
		OwnerUserID:     ctx.ownerUserID,
		CreatedAt:       &ctx.createdAt,
		UpdateAt:        &ctx.createdAt,
		AccessWay:       StringPtr("B"),
		Description:     item.Description,
		DeviceConfigID:  &item.DeviceConfigId,
		IsOnline:        0,
		ActivateFlag:    "active",
		ServiceAccessID: &serviceAccessID,
	}
	return device
}

func buildBatchCreateDeviceList(req model.BatchCreateDeviceReq, claims *utils.UserClaims, ctx *batchCreateDeviceContext) ([]*model.Device, error) {
	deviceList := make([]*model.Device, 0, len(req.DeviceList))
	seenDeviceNumbers := make(map[string]struct{}, len(req.DeviceList))
	existingDeviceNumbers, err := existingBatchCreateDeviceNumbers(req.DeviceList)
	if err != nil {
		return nil, err
	}
	for _, item := range req.DeviceList {
		if shouldSkipBatchCreateDeviceItem(item) {
			continue
		}
		if err := validateBatchCreateDeviceRequiredFields(item); err != nil {
			return nil, err
		}
		if _, ok := seenDeviceNumbers[item.DeviceNumber]; ok {
			logrus.Warn("skip duplicate device number in batch create request")
			continue
		}
		seenDeviceNumbers[item.DeviceNumber] = struct{}{}
		if existingDeviceNumbers[item.DeviceNumber] {
			continue
		}
		if err := validateBatchCreateDeviceTenant(item, claims, ctx.serviceAccess); err != nil {
			return nil, err
		}
		deviceList = append(deviceList, buildBatchCreateDeviceModel(item, ctx, req.ServiceAccessId))
	}
	return deviceList, nil
}

// CreateDeviceBatch persists validated batch device rows, then best-effort
// refreshes related service-plugin metadata once the DB write succeeds.
func (*Device) CreateDeviceBatch(req model.BatchCreateDeviceReq, claims *utils.UserClaims) (data any, err error) {
	ctx, err := buildBatchCreateDeviceContext(req.ServiceAccessId, claims)
	if err != nil {
		return nil, err
	}
	deviceList, err := buildBatchCreateDeviceList(req, claims, ctx)
	if err != nil {
		return nil, err
	}
	if len(deviceList) == 0 {
		return deviceList, nil
	}
	err = dal.CreateDeviceBatch(deviceList)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if err := notifyCreateDeviceBatchServicePlugin(ctx.serviceAccess.ServicePluginID, req.ServiceAccessId); err != nil {
		logrus.Warn("batch create devices persisted but service plugin notification failed")
	}

	return deviceList, nil
}

func notifyCreateDeviceBatchServicePlugin(servicePluginID string, serviceAccessID string) error {
	// Notify the related service plugin after batch create so downstream
	// metadata and cached access state can refresh.
	_, host, err := dal.GetServicePluginHttpAddressByID(servicePluginID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
			"message":   "create device success, query service plugin failed",
		})
	}
	dataMap := make(map[string]interface{})
	dataMap["service_access_id"] = serviceAccessID
	// Serialize the notification payload before calling the plugin endpoint.
	dataBytes, err := json.Marshal(dataMap)
	if err != nil {
		return errcode.WithData(100004, map[string]interface{}{
			"message": "create device success, marshal data failed",
		})
	}
	// Notify the service plugin after batch create so related metadata can refresh.
	logrus.Debug("notify service plugin after batch create")
	_, err = pluginruntime.Current().Notify(host, "1", string(dataBytes))
	if err != nil {
		return errcode.WithVars(105001, map[string]interface{}{
			"error": "create device success, notification failed" + err.Error(),
		})
	}
	logrus.Debug("service plugin notification completed")
	return nil
}
