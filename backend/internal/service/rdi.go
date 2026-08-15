// 文件用途：维护 RDI 设备接入、告警映射和协议数据服务。
// 核心逻辑：解析 RDI payload、转换系统信息与告警等级，并串联设备、配置、邮件和历史数据。
// 关键注意事项：RDI 是外部协议边界，坏 JSON、字段别名、邮件接收人和权限失败都需保持 fail-safe。
// 重构建议：按配置、命令、分享和告警拆分模块，补齐事务、外部通知、协议兼容和权限测试。
// rdi.go owns RDI-specific business workflows.
//
// Purpose: activate RDI devices, build configuration responses, find matching
// firmware, send device commands, record direct-alarm history, and support
// share-aware read access. Core logic normalizes PIDs, validates tenant/user
// access, parses additional_info JSON, derives alarm email targets, and maps
// frontend RDI requests into DAL/MQTT/OTA operations. Important notes: RDI
// RDI views depend on the exact config, alarm, and history semantics, so changes
// should include focused service tests plus API-level checks for visible
// behavior. Refactor suggestion: split alarm-history derivation, firmware
// matching, and share/access checks into smaller collaborators once their test
// coverage is locked down.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	rdiConfigKey               = "rdi_config"
	rdiSystemInfoKey           = "rdi_system_info"
	rdiShareTokensKey          = "rdi_share_tokens"
	rdiShareRecipientsKey      = "rdi_share_recipients"
	rdiSW3ShortPressEvent      = "sw3_short_press"
	rdiTemperatureLowerBound   = -40
	rdiTemperatureUpperBound   = 125
	rdiCollectionIntervalMin   = 45
	rdiCollectionIntervalMax   = 60
	rdiDryContactDelayMin      = 0
	rdiDryContactDelayMax      = 86400
	rdiAlarmDurationMinSeconds = 0
	rdiAlarmDurationMaxSeconds = 24 * 60 * 60
	rdiShareTokenDefaultTTL    = 7 * 24 * 60 * 60
	rdiShareTokenMaxTTL        = 30 * 24 * 60 * 60
)

var promotedRDISystemInfoExtraKeys = []string{
	"address",
	"installation_date",
	"installer_company",
	"installer_contact",
	"installer_name",
	"installer_phone",
	"installer_email",
	"controller_serial_number",
}

type RDI struct{}

func (*RDI) ActivateDeviceByPID(req model.ActivateRDIDeviceReq, claims *utils.UserClaims) (*model.RDIDeviceConfigResponse, error) {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	pid, err := NormalizeRDIPID(req.PIDNumber)
	if err != nil {
		return nil, err
	}
	if pid == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "pid_number is required")
	}

	device, err := dal.GetDeviceByDeviceNumber(pid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.WithVars(204001, map[string]interface{}{"error": pid})
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": "database operation failed"})
	}
	// A physical SW3 unbind intentionally clears tenant_id so the next account
	// can claim the inactive controller by PID. A still-bound device remains
	// tenant-scoped and cannot be taken over by another tenant.
	if claims.Authority != constant.SYS_ADMIN &&
		strings.TrimSpace(device.TenantID) != "" &&
		device.TenantID != claims.TenantID {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	if device.ActivateFlag == "active" {
		return nil, errcode.New(204002)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = pid
	}
	now := nextRDIStateVersion(device.UpdateAt, time.Now().UTC())
	updates := map[string]interface{}{
		"name":          name,
		"tenant_id":     claims.TenantID,
		"device_number": pid,
		"activate_flag": "active",
		"is_enabled":    "enabled",
		"activate_at":   now,
		"update_at":     now,
	}
	if ownerUserID := createdDeviceOwnerUserID(claims); ownerUserID != nil {
		updates["owner_user_id"] = *ownerUserID
	}
	if _, err := dal.UpdateDeviceByMap(device.ID, updates); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": "database operation failed"})
	}
	initialize.DelDeviceCache(device.ID)

	return GroupApp.RDI.DeviceConfig(device.ID, claims)
}

func (*RDI) DeviceConfig(deviceID string, claims *utils.UserClaims) (*model.RDIDeviceConfigResponse, error) {
	device, err := getRDIDeviceForRead(deviceID, claims)
	if err != nil {
		return nil, err
	}
	return rdiDeviceConfigResponse(device, rdiDeviceConfigResponseOptionsForClaims(device, claims)), nil
}

func (*RDI) LatestFirmwarePackage(deviceID string, claims *utils.UserClaims) (*model.RDILatestFirmwareResponse, error) {
	// Firmware package metadata contains the package URL, tenant, signature and
	// provider-specific additional information. It belongs to the OTA operation
	// surface, so share-read access must not expose it even though the endpoint is
	// read-only at the HTTP level.
	device, err := getRDIDevice(deviceID, claims)
	if err != nil {
		return nil, err
	}

	currentVersion := strings.TrimSpace(SafeDeref(device.CurrentVersion))
	packages, err := latestRDIFirmwarePackages(device, true)
	if err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		packages, err = latestRDIFirmwarePackages(device, false)
		if err != nil {
			return nil, err
		}
	}

	response := &model.RDILatestFirmwareResponse{
		DeviceID:       device.ID,
		CurrentVersion: currentVersion,
	}
	for _, pkg := range packages {
		if pkg == nil {
			continue
		}
		if strings.TrimSpace(pkg.Version) == "" {
			continue
		}
		if currentVersion != "" && strings.EqualFold(strings.TrimSpace(pkg.Version), currentVersion) {
			continue
		}
		response.UpdateAvailable = true
		response.Package = pkg
		break
	}
	return response, nil
}

func latestRDIFirmwarePackages(device *model.Device, matchDeviceConfig bool) ([]*model.OtaUpgradePackage, error) {
	q := query.OtaUpgradePackage
	queryBuilder := q.WithContext(context.Background()).Where(q.TenantID.Eq(device.TenantID))
	if matchDeviceConfig && device.DeviceConfigID != nil {
		deviceConfigID := strings.TrimSpace(*device.DeviceConfigID)
		if deviceConfigID != "" {
			queryBuilder = queryBuilder.Where(q.DeviceConfigID.Eq(deviceConfigID))
		}
	}
	return queryBuilder.Order(q.CreatedAt.Desc()).Limit(50).Find()
}

type rdiDeviceConfigResponseOptions struct {
	IncludeAdditionalInfo bool
	ExposeAlarmEmails     bool
}

// rdiDeviceConfigResponseOptionsForClaims is the disclosure seam for the
// authenticated config endpoint. Share-only recipients receive the modeled RDI
// config/system-information view, but not the raw extensible additional_info
// map or alarm recipient addresses.
func rdiDeviceConfigResponseOptionsForClaims(device *model.Device, claims *utils.UserClaims) rdiDeviceConfigResponseOptions {
	return rdiDeviceConfigResponseOptions{
		IncludeAdditionalInfo: hasTelemetryTenantAccess(device, claims, false),
		ExposeAlarmEmails:     rdiMayExposeAlarmEmails(device, claims),
	}
}

var rdiAlarmEmailConfigKeys = [...]string{
	"sensor_alarm_emails",
	"switch_alarm_emails",
	"warranty_alarm_emails",
	"sensor_1_alarm_emails",
	"sensor_2_alarm_emails",
	"switch_1_alarm_emails",
	"switch_2_alarm_emails",
}

func rdiDeviceConfigResponse(device *model.Device, options rdiDeviceConfigResponseOptions) *model.RDIDeviceConfigResponse {
	additional := parseAdditionalInfo(device.AdditionalInfo)
	cfg := configFromAdditionalInfo(additional)
	systemInfo := systemInfoFromAdditionalInfo(additional)
	responseAdditional := map[string]interface{}{}
	if options.IncludeAdditionalInfo {
		for key, value := range additional {
			if key == rdiShareTokensKey || key == rdiShareRecipientsKey {
				continue
			}
			responseAdditional[key] = value
		}
	}
	if !options.ExposeAlarmEmails {
		cfg = rdiConfigWithoutAlarmEmails(cfg)
		redactRDIAlarmEmailsFromAdditionalInfo(responseAdditional)
	}
	if !options.IncludeAdditionalInfo {
		// Only promoted, explicitly modeled system-information fields belong to
		// the share view. Arbitrary legacy extension fields stay owner/admin-only.
		systemInfo.ExtraFields = nil
	}

	return &model.RDIDeviceConfigResponse{
		DeviceID:        device.ID,
		PIDNumber:       device.DeviceNumber,
		DeviceName:      SafeDeref(device.Name),
		FirmwareVersion: SafeDeref(device.CurrentVersion),
		Online:          device.IsOnline == 1,
		ConnectionType:  readString(additional, "connection_type", "unknown"),
		Config:          cfg,
		SystemInfo:      systemInfo,
		AdditionalInfo:  responseAdditional,
		ThingModel:      RDIThingModelDefinition(),
	}
}

func rdiMayExposeAlarmEmails(device *model.Device, claims *utils.UserClaims) bool {
	if device == nil || claims == nil {
		return false
	}
	if claims.Authority == constant.SYS_ADMIN {
		return true
	}
	deviceTenantID := strings.TrimSpace(device.TenantID)
	claimsTenantID := strings.TrimSpace(claims.TenantID)
	if deviceTenantID == "" || deviceTenantID != claimsTenantID {
		return false
	}
	if claims.Authority == constant.TENANT_ADMIN {
		return true
	}
	return deviceOwnerMatchesClaims(device, claims)
}

func rdiConfigWithoutAlarmEmails(cfg model.RDIConfig) model.RDIConfig {
	cfg.SensorAlarmEmails = ""
	cfg.SwitchAlarmEmails = ""
	cfg.WarrantyAlarmEmails = ""
	cfg.Sensor1AlarmEmails = ""
	cfg.Sensor2AlarmEmails = ""
	cfg.Switch1AlarmEmails = ""
	cfg.Switch2AlarmEmails = ""
	return cfg
}

func redactRDIAlarmEmailsFromAdditionalInfo(additional map[string]interface{}) {
	if len(additional) == 0 {
		return
	}
	for _, key := range rdiAlarmEmailConfigKeys {
		if _, exists := additional[key]; exists {
			additional[key] = ""
		}
	}
	rawStoredConfig, exists := additional[rdiConfigKey]
	if !exists {
		return
	}
	storedConfig, ok := rawStoredConfig.(map[string]interface{})
	if !ok {
		// Malformed legacy config cannot be safely field-redacted. Omit only that
		// invalid value instead of returning an opaque string or array containing
		// recipient addresses.
		delete(additional, rdiConfigKey)
		return
	}
	redactedConfig := make(map[string]interface{}, len(storedConfig))
	for key, value := range storedConfig {
		redactedConfig[key] = value
	}
	for _, key := range rdiAlarmEmailConfigKeys {
		if _, exists := redactedConfig[key]; exists {
			redactedConfig[key] = ""
		}
	}
	additional[rdiConfigKey] = redactedConfig
}

func (*RDI) UpdateDeviceConfig(ctx context.Context, deviceID string, req *model.UpdateRDIConfigReq, claims *utils.UserClaims) (*model.RDIDeviceConfigResponse, error) {
	if err := validateRDIConfig(req.Config); err != nil {
		return nil, err
	}

	// 开启事务并对设备行加 FOR UPDATE 行级锁，
	// 保护 additional_info 的 read-modify-write 并发安全。
	tx, err := dal.StartTransaction()
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": "database operation failed"})
	}
	committed := false
	defer func() {
		if !committed {
			_ = dal.Rollback(tx)
		}
	}()

	device, err := dal.GetDeviceByIDForUpdate(tx, deviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "device not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": "database operation failed"})
	}
	// 复用 getRDIDevice 的权限校验逻辑（基于已加载的 device）
	if err := assertRDIDeviceAccess(device, claims); err != nil {
		return nil, err
	}

	additional := parseAdditionalInfo(device.AdditionalInfo)
	additional[rdiConfigKey] = req.Config
	if req.SystemInfo != nil {
		additional[rdiSystemInfoKey] = normalizeRDISystemInfoForStorage(*req.SystemInfo)
	}

	nextAdditional, err := json.Marshal(additional)
	if err != nil {
		logrus.Errorf("RDI 序列化 additional_info 失败: %v", err)
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "failed to serialize config")
	}

	if err := dal.UpdateDeviceAdditionalInfoWithTx(tx, device.ID, string(nextAdditional)); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": "database operation failed"})
	}

	var commandTracking *model.RDICommandTracking
	if req.ApplyToDevice {
		params, _ := json.Marshal(req.Config)
		value := string(params)
		putReq := &model.PutMessageForCommand{
			DeviceID: device.ID,
			Identify: "set_alarm_config",
			Value:    &value,
		}
		tracking, err := GroupApp.CommandData.CommandPutMessageWithTracking(ctx, claims.ID, putReq, fmt.Sprintf("%d", constant.Manual), claims)
		if err != nil {
			return nil, err
		}
		commandTracking = rdiCommandTrackingFromDelivery(tracking)
	}

	if err := dal.Commit(tx); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": "database operation failed"})
	}
	committed = true

	response, err := GroupApp.RDI.DeviceConfig(deviceID, claims)
	if err != nil {
		return nil, err
	}
	response.CommandTracking = commandTracking
	return response, nil
}

func rdiCommandTrackingFromDelivery(tracking *CommandDeliveryTracking) *model.RDICommandTracking {
	if tracking == nil {
		return nil
	}
	return &model.RDICommandTracking{
		MessageID:     tracking.MessageID,
		Status:        tracking.Status,
		DeviceID:      tracking.DeviceID,
		Identifier:    tracking.Identify,
		OperationType: tracking.OperationType,
		LogRecorded:   tracking.LogRecorded,
	}
}

func (*RDI) SendCommand(ctx context.Context, deviceID string, req *model.RDICommandReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	device, err := getRDIDevice(deviceID, claims)
	if err != nil {
		return nil, err
	}
	identifier := strings.TrimSpace(req.Identifier)
	if !allowedRDICommand(identifier) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "unsupported RDI command identifier")
	}
	if err := validateRDICommand(identifier, req.Params); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(req.Params)
	if err != nil {
		logrus.Errorf("RDI 序列化命令参数失败: %v", err)
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "failed to serialize command params")
	}
	value := string(payload)
	putReq := &model.PutMessageForCommand{
		DeviceID: device.ID,
		Identify: identifier,
		Value:    &value,
	}
	tracking, err := GroupApp.CommandData.CommandPutMessageWithTracking(
		ctx,
		claims.ID,
		putReq,
		fmt.Sprintf("%d", constant.Manual),
		claims,
	)
	if err != nil {
		return nil, err
	}
	commandTracking := rdiCommandTrackingFromDelivery(tracking)

	return map[string]interface{}{
		"device_id":        device.ID,
		"identifier":       identifier,
		"params":           req.Params,
		"status":           "queued",
		"message_id":       commandTracking.MessageID,
		"log_recorded":     commandTracking.LogRecorded,
		"command_tracking": commandTracking,
		"operation_type":   commandTracking.OperationType,
		"tracking_status":  commandTracking.Status,
	}, nil
}

func (*RDI) NotifyAlarmEvent(device *model.Device, eventInfo *model.EventInfo) error {
	if device == nil || eventInfo == nil {
		return nil
	}

	plan := buildRDIAlarmEventPlan(device, eventInfo, rdiAlarmEventPlanOptions{
		HistoryID: uuid.New(),
		EventTime: time.Now(),
	})
	if plan.AlarmHistory != nil {
		if err := dal.AlarmHistorySave(plan.AlarmHistory); err != nil {
			return err
		}
	}
	if plan.Email == nil {
		return nil
	}

	recipients := plan.Email.Recipients
	if plan.Email.NeedsTenantWarningRecipients {
		recipients = warningEmailsForOwnedDevices(device.TenantID, device.ID)
	}
	if len(recipients) == 0 {
		return (&NotificationServicesConfig{}).saveTenantEmailFailure(
			device.TenantID,
			"",
			plan.Email.Body,
			tenantEmailFailureRecipientsEmpty,
			device.ID,
		)
	}

	var firstErr error
	for _, email := range recipients {
		if err := sendEmailMessageForDevices(plan.Email.Body, plan.Email.Subject, device.TenantID, []string{device.ID}, email); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (*RDI) HandlePhysicalUnbindEvent(device *model.Device, eventInfo *model.EventInfo) error {
	if !isRDIPhysicalUnbindEvent(eventInfo) || device == nil || strings.TrimSpace(device.ID) == "" {
		return nil
	}

	outboxEvent, err := persistRDIPhysicalUnbind(device.ID, time.Now().UTC())
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	// Invalidate both lookup directions immediately after the inactive/disabled state is persisted.
	// The device update, group cleanup, and durable revocation event have committed before cache
	// invalidation and the best-effort immediate broker notification.
	deleteDeviceVoucherCache(device.ID, device.Voucher)
	deleteDeviceCache(device.ID)
	if outboxEvent == nil {
		return nil
	}
	revocationContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := deliverMQTTSessionRevocationOutboxEvent(revocationContext, outboxEvent.ID); err != nil {
		// The unbind is already committed together with a durable pending event. Returning an
		// error here would invite duplicate device-event delivery without improving durability;
		// the background outbox worker owns retries instead.
		logrus.WithError(err).WithFields(logrus.Fields{
			"device_id":  device.ID,
			"outbox_id":  outboxEvent.ID,
			"revoked_at": outboxEvent.RevokedAt,
		}).Warn("mqtt session revocation queued for retry")
	}
	return nil
}

func persistRDIPhysicalUnbind(deviceID string, revokedAt time.Time) (*mqttSessionRevocationOutbox, error) {
	tx, err := dal.StartTransaction()
	if err != nil {
		return nil, err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = dal.Rollback(tx)
		}
	}()

	lockedDevice, err := dal.GetDeviceByIDForUpdate(tx, deviceID)
	if err != nil {
		return nil, err
	}
	revokedAt = nextRDIStateVersion(lockedDevice.UpdateAt, revokedAt)

	var outboxEvent *mqttSessionRevocationOutbox
	if rdiDeviceNeedsPhysicalUnbind(lockedDevice) {
		result, updateErr := tx.Device.Where(tx.Device.ID.Eq(deviceID)).Updates(rdiPhysicalUnbindUpdates(lockedDevice, revokedAt))
		if updateErr != nil {
			return nil, updateErr
		}
		if result.RowsAffected == 0 {
			return nil, fmt.Errorf("physical unbind device update affected no rows")
		}
		outboxEvent = newMQTTSessionRevocationOutbox(deviceID, revokedAt)
		if err := createMQTTSessionRevocationOutboxWithDB(tx.Device.UnderlyingDB(), outboxEvent); err != nil {
			return nil, err
		}
	} else {
		// Duplicate uplink delivery must not create a fresh revocation generation. Reuse the
		// still-actionable event; a previously published event means this duplicate is done.
		outboxEvent, err = findActionableMQTTSessionRevocationOutboxWithDB(tx.Device.UnderlyingDB(), deviceID)
		if err != nil {
			return nil, err
		}
	}
	if _, err := tx.RGroupDevice.Where(tx.RGroupDevice.DeviceID.Eq(deviceID)).Delete(); err != nil {
		return nil, err
	}
	if err := dal.Commit(tx); err != nil {
		return nil, err
	}
	rollback = false
	return outboxEvent, nil
}

func rdiDeviceNeedsPhysicalUnbind(device *model.Device) bool {
	if device == nil {
		return false
	}
	return strings.TrimSpace(device.TenantID) != "" ||
		device.OwnerUserID != nil ||
		!strings.EqualFold(strings.TrimSpace(device.ActivateFlag), "inactive") ||
		!strings.EqualFold(strings.TrimSpace(device.IsEnabled), "disabled")
}

func nextRDIStateVersion(current *time.Time, candidate time.Time) time.Time {
	candidate = candidate.UTC()
	if candidate.IsZero() {
		candidate = time.Now().UTC()
	}
	if current != nil {
		currentUTC := current.UTC()
		if !candidate.After(currentUTC) {
			// PostgreSQL timestamptz commonly persists microsecond precision. Advancing by
			// one microsecond keeps activation/unbind generations strictly ordered even
			// when application-node clocks move backwards.
			candidate = currentUTC.Add(time.Microsecond)
		}
	}
	return candidate
}

func isRDIPhysicalUnbindEvent(eventInfo *model.EventInfo) bool {
	return eventInfo != nil && strings.TrimSpace(eventInfo.Method) == rdiSW3ShortPressEvent
}

func rdiPhysicalUnbindUpdates(device *model.Device, now time.Time) map[string]interface{} {
	return map[string]interface{}{
		"tenant_id":       "",
		"owner_user_id":   nil,
		"activate_flag":   "inactive",
		"is_enabled":      "disabled",
		"is_online":       int16(0),
		"additional_info": rdiAdditionalInfoWithoutShareState(device.AdditionalInfo),
		"update_at":       now,
	}
}

func rdiAdditionalInfoWithoutShareState(info *string) string {
	additional := parseAdditionalInfo(info)
	delete(additional, rdiShareTokensKey)
	delete(additional, rdiShareRecipientsKey)
	bytes, err := json.Marshal(additional)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func getRDIDeviceForRead(deviceID string, claims *utils.UserClaims) (*model.Device, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	device, err := dal.GetDeviceByID(deviceID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": "database operation failed"})
	}
	if hasTelemetryTenantAccess(device, claims, true) {
		return device, nil
	}
	return nil, errcode.New(errcode.CodeNoPermission)
}

func getRDIDevice(deviceID string, claims *utils.UserClaims) (*model.Device, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	device, err := dal.GetDeviceByID(deviceID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": "database operation failed"})
	}
	if !hasTelemetryTenantAccess(device, claims, false) {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	return device, nil
}

// assertRDIDeviceAccess 对已加载的 device 执行与 getRDIDevice 相同的权限校验，
// 供事务内使用 GetDeviceByIDForUpdate 加载设备后复用，避免重复查询。
func assertRDIDeviceAccess(device *model.Device, claims *utils.UserClaims) error {
	if device == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	if !hasTelemetryTenantAccess(device, claims, false) {
		return errcode.New(errcode.CodeNoPermission)
	}
	return nil
}

func parseAdditionalInfo(info *string) map[string]interface{} {
	additional := map[string]interface{}{}
	if info == nil || strings.TrimSpace(*info) == "" {
		return additional
	}
	if err := json.Unmarshal([]byte(*info), &additional); err != nil {
		return map[string]interface{}{}
	}
	return additional
}

func systemInfoFromAdditionalInfo(additional map[string]interface{}) model.RDISystemInfo {
	info := model.RDISystemInfo{}
	if val, ok := additional[rdiSystemInfoKey]; ok {
		if bytes, err := json.Marshal(val); err == nil {
			_ = json.Unmarshal(bytes, &info)
		}
	}
	if info.ExtraFields == nil {
		info.ExtraFields = map[string]interface{}{}
	}
	promoteRDISystemInfoExtraFields(&info)
	return info
}

func normalizeRDISystemInfoForStorage(info model.RDISystemInfo) model.RDISystemInfo {
	if info.ExtraFields == nil {
		info.ExtraFields = map[string]interface{}{}
	}
	promoteRDISystemInfoExtraFields(&info)
	for _, key := range promotedRDISystemInfoExtraKeys {
		delete(info.ExtraFields, key)
	}
	return info
}

func promoteRDISystemInfoExtraFields(info *model.RDISystemInfo) {
	if info == nil || info.ExtraFields == nil {
		return
	}
	if info.Address == "" {
		info.Address = stringFromExtraField(info.ExtraFields, "address")
	}
	if info.InstallationDate == "" {
		info.InstallationDate = stringFromExtraField(info.ExtraFields, "installation_date")
	}
	if info.InstallerCompany == "" {
		info.InstallerCompany = stringFromExtraField(info.ExtraFields, "installer_company")
	}
	if info.InstallerContact == "" {
		info.InstallerContact = stringFromExtraField(info.ExtraFields, "installer_contact")
	}
	if info.InstallerName == "" {
		info.InstallerName = stringFromExtraField(info.ExtraFields, "installer_name")
	}
	if info.InstallerPhone == "" {
		info.InstallerPhone = stringFromExtraField(info.ExtraFields, "installer_phone")
	}
	if info.InstallerEmail == "" {
		info.InstallerEmail = stringFromExtraField(info.ExtraFields, "installer_email")
	}
	if info.ControllerSerialNumber == "" {
		info.ControllerSerialNumber = stringFromExtraField(info.ExtraFields, "controller_serial_number")
	}
}

func stringFromExtraField(fields map[string]interface{}, key string) string {
	if raw, ok := fields[key]; ok {
		if value, ok := raw.(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readString(values map[string]interface{}, key string, fallback string) string {
	if raw, ok := values[key]; ok {
		if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return fallback
}
