// 文件用途：验证设备生命周期、RDI 元数据和通用 helper 的高风险行为。
// 核心逻辑：覆盖设备创建删除、子设备绑定、租户权限、RDI 告警映射和结构体转换等服务分支。
// 关键注意事项：设备服务会级联清理告警、遥测和插件通知，测试必须证明权限失败发生在副作用之前。
// 重构建议：继续把大型设备测试拆成生命周期、权限、RDI 和 helper 套件，并补齐事务回滚断言。
// device_test.go verifies high-risk device service paths.
//
// Purpose: protect device create/delete permission checks, child-data cleanup ordering, and cross-tenant rejection behavior.
// Core logic: builds focused service tests that assert invalid claims or tenant mismatches fail before mutating device-related records.
// Important notes: device operations fan out to telemetry, attributes, events, configs, and broker-facing identifiers, so guard failures must remain fail-closed before side effects.
// Refactor suggestion: extract reusable device fixture builders as more service/DAL integration cases are added.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/pluginruntime"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestDeviceJoinDeviceDeleteRollbackError(t *testing.T) {
	cause := errors.New("delete failed")
	rollbackErr := errors.New("rollback failed")

	err := joinDeviceDeleteRollbackError(cause, rollbackErr)

	assert.ErrorIs(t, err, cause)
	assert.ErrorIs(t, err, rollbackErr)
	assert.Contains(t, err.Error(), "rollback device delete transaction")
}

func TestDeviceJoinDeviceDeleteRollbackError_NoRollbackError(t *testing.T) {
	cause := errors.New("delete failed")

	err := joinDeviceDeleteRollbackError(cause, nil)

	assert.Same(t, cause, err)
}

func TestDevicePrepareActiveDeviceUpdateUsesRequestedNameAndLifecycleFields(t *testing.T) {
	device := &model.Device{}
	activatedAt := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)

	prepareActiveDeviceUpdate(device, "0000000001A2", "sensor-a", activatedAt)

	assert.Equal(t, "0000000001A2", device.DeviceNumber)
	assert.NotNil(t, device.Name)
	assert.Equal(t, "sensor-a", *device.Name)
	assert.Equal(t, "active", device.ActivateFlag)
	assert.Equal(t, "enabled", device.IsEnabled)
	assert.Equal(t, activatedAt, *device.UpdateAt)
	assert.Equal(t, activatedAt, *device.ActivateAt)
}

func TestDevicePrepareActiveDeviceUpdateKeepsExistingNameWhenRequestNameBlank(t *testing.T) {
	name := "existing sensor"
	device := &model.Device{Name: &name}

	prepareActiveDeviceUpdate(device, "0000000001A2", "", time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC))

	assert.Equal(t, "existing sensor", *device.Name)
}

func TestDevicePrepareActiveDeviceUpdateFallsBackToDeviceNumberForBlankName(t *testing.T) {
	name := "   "
	device := &model.Device{Name: &name}

	prepareActiveDeviceUpdate(device, "0000000001A2", "", time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC))

	assert.NotNil(t, device.Name)
	assert.Equal(t, "0000000001A2", *device.Name)
}

func TestDeviceParseOptionEnumAdditionalInfo(t *testing.T) {
	raw := `[{"value_type":"int","value":1,"description":"on"}]`
	option := &model.Options{}

	parseOptionEnumAdditionalInfo(&raw, option, "status")

	assert.Len(t, option.Enum, 1)
	assert.Equal(t, "int", option.Enum[0].ValueType)
	assert.Equal(t, 1, option.Enum[0].Value)
	assert.Equal(t, "on", option.Enum[0].Description)
}

func TestDeviceParseOptionEnumAdditionalInfo_InvalidJSONLeavesEmptyEnum(t *testing.T) {
	raw := `not-json`
	option := &model.Options{}

	parseOptionEnumAdditionalInfo(&raw, option, "status")

	assert.Empty(t, option.Enum)
}

func TestDeviceBuildAttributeMetricOptionsMergesCurrentAndTemplateWithoutDuplicates(t *testing.T) {
	label := "Firmware"
	dataType := "Enum"
	enumRaw := `[{"value_type":"int","value":1,"description":"stable"}]`
	options := buildAttributeMetricOptions(
		[]*model.AttributeData{
			{Key: "firmware", StringV: StringPtr("1.0.0")},
		},
		map[string]*model.DeviceModelAttribute{
			"firmware": {
				DataIdentifier: "firmware",
				DataName:       &label,
				DataType:       &dataType,
				AdditionalInfo: &enumRaw,
			},
		},
	)

	if assert.Len(t, options, 1) {
		assert.Equal(t, "firmware", options[0].Key)
		assert.Equal(t, &label, options[0].Label)
		assert.Equal(t, &dataType, options[0].DataType)
		assert.Len(t, options[0].Enum, 1)
	}
}

func TestDeviceBuildTelemetryMetricOptionsAddsTemplateOnlyFields(t *testing.T) {
	label := "Temperature"
	dataType := "Number"
	options := buildTelemetryMetricOptions(
		nil,
		map[string]*model.DeviceModelTelemetry{
			"temperature": {
				DataIdentifier: "temperature",
				DataName:       &label,
				DataType:       &dataType,
			},
		},
	)

	if assert.Len(t, options, 1) {
		assert.Equal(t, "temperature", options[0].Key)
		assert.Equal(t, &label, options[0].Label)
		assert.Equal(t, &dataType, options[0].DataType)
	}
}

func TestDeviceBuildAutomationActionSourcesKeepsCustomBucketsAndReadWriteMetadata(t *testing.T) {
	label := "Temperature"
	dataType := "Number"
	unit := "C"
	readWrite := "RW"

	sources := buildDeviceActionSources(
		[]*model.TelemetryCurrentData{{Key: "temperature", NumberV: Float64Ptr(23)}},
		nil,
		&deviceMetricTemplate{
			telemetry: map[string]*model.DeviceModelTelemetry{
				"temperature": {
					DataIdentifier: "temperature",
					DataName:       &label,
					DataType:       &dataType,
					Unit:           &unit,
					ReadWriteFlag:  &readWrite,
				},
			},
			attributes: map[string]*model.DeviceModelAttribute{},
		},
	)

	assert.Len(t, sources, 4)
	assert.Equal(t, string(constant.TelemetrySource), sources[0].DataSourceTypeRes)
	if assert.Len(t, sources[0].Options, 1) {
		assert.Equal(t, &label, sources[0].Options[0].Label)
		assert.Equal(t, &dataType, sources[0].Options[0].DataType)
		assert.Equal(t, &unit, sources[0].Options[0].Uint)
		assert.Equal(t, &readWrite, sources[0].Options[0].ReadWriteFlag)
	}
	assert.Equal(t, "c_telemetry", sources[1].DataSourceTypeRes)
	assert.Equal(t, "c_attribute", sources[2].DataSourceTypeRes)
	assert.Equal(t, "c_command", sources[3].DataSourceTypeRes)
}

func TestDeviceBuildAutomationConditionSourcesMergesCurrentAndTemplate(t *testing.T) {
	label := "Firmware"
	dataType := "String"

	sources := buildDeviceConditionSources(
		nil,
		[]*model.AttributeData{{Key: "firmware", StringV: StringPtr("1.0.0")}},
		&deviceMetricTemplate{
			telemetry: map[string]*model.DeviceModelTelemetry{},
			attributes: map[string]*model.DeviceModelAttribute{
				"firmware": {
					DataIdentifier: "firmware",
					DataName:       &label,
					DataType:       &dataType,
				},
			},
		},
	)

	if assert.Len(t, sources, 1) {
		assert.Equal(t, string(constant.AttributeSource), sources[0].DataSourceTypeRes)
		if assert.Len(t, sources[0].Options, 1) {
			assert.Equal(t, "firmware", sources[0].Options[0].Key)
			assert.Equal(t, &label, sources[0].Options[0].Label)
			assert.Equal(t, &dataType, sources[0].Options[0].DataType)
		}
	}
}

func TestDeviceCreateDeviceRejectsCrossTenantDeviceConfigWriteAccess(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	protocolType := "MQTT"
	voucherType := "BASIC"
	if err := db.Create(&model.DeviceConfig{
		ID:           "config-tenant-b",
		Name:         "tenant-b-config",
		DeviceType:   "1",
		TenantID:     "tenant-b",
		CreatedAt:    now,
		UpdatedAt:    now,
		ProtocolType: &protocolType,
		VoucherType:  &voucherType,
	}).Error; err != nil {
		t.Fatalf("create device config: %v", err)
	}

	deviceNumber := "device-cross-tenant"
	deviceConfigID := "config-tenant-b"
	claims := &utils.UserClaims{TenantID: "tenant-a"}

	_, err := (&Device{}).CreateDevice(model.CreateDeviceReq{
		DeviceNumber:   &deviceNumber,
		DeviceConfigId: &deviceConfigID,
	}, claims)

	assertDeviceConfigServiceError(t, err, "cross-tenant create device device config write access", errcode.CodeNoPermission, "no permission to query device config")

	var count int64
	if err := db.Model(&model.Device{}).Count(&count).Error; err != nil {
		t.Fatalf("count devices: %v", err)
	}
	assert.Equal(t, int64(0), count)
}

func TestDeviceCreateDeviceRejectsMissingClaims(t *testing.T) {
	deviceNumber := "device-missing-claims"

	_, err := (&Device{}).CreateDevice(model.CreateDeviceReq{
		DeviceNumber: &deviceNumber,
	}, nil)

	assertDeviceConfigServiceError(t, err, "create device missing claims", errcode.CodeNoPermission, "no permission to create device")
}

func TestDeviceCreateDeviceStoresOwnerUserID(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	deviceNumber := "device-owned-on-create"
	deviceName := "owned device"

	device, err := (&Device{}).CreateDevice(model.CreateDeviceReq{
		Name:         &deviceName,
		DeviceNumber: &deviceNumber,
	}, &utils.UserClaims{ID: "user-a", TenantID: "tenant-a", Authority: constant.TENANT_USER})

	assert.NoError(t, err)
	if assert.NotNil(t, device.OwnerUserID) {
		assert.Equal(t, "user-a", *device.OwnerUserID)
	}

	var stored model.Device
	if err := db.First(&stored, "id = ?", device.ID).Error; err != nil {
		t.Fatalf("query created device: %v", err)
	}
	if assert.NotNil(t, stored.OwnerUserID) {
		assert.Equal(t, "user-a", *stored.OwnerUserID)
	}
}

func TestDeviceGetDeviceTemplateChartSelectRejectsMissingClaims(t *testing.T) {
	_, err := (&Device{}).GetDeviceTemplateChartSelect(nil)
	assertDeviceConfigServiceError(t, err, "thing model chart selector missing claims", errcode.CodeNoPermission, "no permission to query thing model chart selector")

	_, err = (&Device{}).GetDeviceTemplateChartSelect(&utils.UserClaims{})
	assertDeviceConfigServiceError(t, err, "thing model chart selector blank tenant", errcode.CodeNoPermission, "no permission to query thing model chart selector")
}

func TestDeviceGetDeviceSelectorRejectsMissingClaims(t *testing.T) {
	_, err := (&Device{}).GetDeviceSelector(model.DeviceSelectorReq{}, nil)
	assertDeviceConfigServiceError(t, err, "device selector missing claims", errcode.CodeNoPermission, "no permission to query device selector")

	_, err = (&Device{}).GetDeviceSelector(model.DeviceSelectorReq{}, &utils.UserClaims{})
	assertDeviceConfigServiceError(t, err, "device selector blank tenant", errcode.CodeNoPermission, "no permission to query device selector")
}

func TestDeviceGetDeviceListByPageRejectsMissingClaims(t *testing.T) {
	// A default (non all-tenants) request runs the shared authority allowlist first, so nil
	// or empty-authority claims fail closed with the unified scope message before any
	// tenant/domain check — matching TestCustomerReadScopeEntryPointsRejectUnknownAuthorityBeforeDAL.
	_, err := (&Device{}).GetDeviceListByPage(&model.GetDeviceListByPageReq{}, nil)
	assertDeviceConfigServiceError(t, err, "device list missing claims", errcode.CodeNoPermission, unsupportedScopeAuthorityPermissionMessage)

	_, err = (&Device{}).GetDeviceListByPage(&model.GetDeviceListByPageReq{}, &utils.UserClaims{})
	assertDeviceConfigServiceError(t, err, "device list blank authority", errcode.CodeNoPermission, unsupportedScopeAuthorityPermissionMessage)
}

func TestDeviceListAndSelectorApplyTenantUserOwnerFilter(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	configID := createDeviceServiceConfig(t, db, "owner-filter-config", "tenant-a", strconv.Itoa(constant.DIRECT_CONNECTION))
	createDeviceServiceOwnedDevice(t, db, "device-owner-a", "owner-number-a", "tenant-a", "user-a", configID, now)
	createDeviceServiceOwnedDevice(t, db, "device-owner-b", "owner-number-b", "tenant-a", "user-b", configID, now)
	createDeviceServiceDevice(t, db, "device-unowned", "owner-number-unowned", "tenant-a", configID, now)

	deviceService := &Device{}
	listRsp, err := deviceService.GetDeviceListByPage(&model.GetDeviceListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10},
	}, &utils.UserClaims{ID: "user-a", TenantID: "tenant-a", Authority: constant.TENANT_USER})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), listRsp["total"])
	list := listRsp["list"].([]model.GetDeviceListByPageRsp)
	if assert.Len(t, list, 1) {
		assert.Equal(t, "device-owner-a", list[0].ID)
	}

	adminListRsp, err := deviceService.GetDeviceListByPage(&model.GetDeviceListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10},
	}, &utils.UserClaims{ID: "admin-a", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN})
	assert.NoError(t, err)
	assert.Equal(t, int64(3), adminListRsp["total"])

	selector, err := deviceService.GetDeviceSelector(model.DeviceSelectorReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10},
	}, &utils.UserClaims{ID: "user-a", TenantID: "tenant-a", Authority: constant.TENANT_USER})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), selector.Total)
	if assert.Len(t, selector.List, 1) {
		assert.Equal(t, "device-owner-a", selector.List[0].DeviceID)
	}
}

func TestDeviceListRDISystemInfoSummaryIsOptInAndRedacted(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	configID := createDeviceServiceConfig(t, db, "rdi-summary-config", "tenant-a", strconv.Itoa(constant.DIRECT_CONNECTION))
	createDeviceServiceDevice(t, db, "rdi-summary-device", "PID-001", "tenant-a", configID, now)
	additionalInfo := `{"rdi_system_info":{"installation_location":"Plant A","installer_name":"Alex","customer_name":"Private Customer","contact_email":"private@example.com","extra_fields":{"private_note":"do not expose"}},"rdi_share_tokens":[{"token":"must-not-leak"}]}`
	if err := db.Model(&model.Device{}).
		Where("id = ?", "rdi-summary-device").
		Update("additional_info", additionalInfo).Error; err != nil {
		t.Fatalf("set RDI system info: %v", err)
	}

	claims := &utils.UserClaims{ID: "admin-a", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN}
	defaultRsp, err := (&Device{}).GetDeviceListByPage(&model.GetDeviceListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10},
	}, claims)
	if !assert.NoError(t, err) {
		return
	}
	defaultRows := defaultRsp["list"].([]model.GetDeviceListByPageRsp)
	if !assert.Len(t, defaultRows, 1) {
		return
	}
	assert.Nil(t, defaultRows[0].RDISystemInfoSummary)

	summaryRsp, err := (&Device{}).GetDeviceListByPage(&model.GetDeviceListByPageReq{
		PageReq:                     model.PageReq{Page: 1, PageSize: 10},
		IncludeRDISystemInfoSummary: true,
	}, claims)
	if !assert.NoError(t, err) {
		return
	}
	summaryRows := summaryRsp["list"].([]model.GetDeviceListByPageRsp)
	if !assert.Len(t, summaryRows, 1) {
		return
	}
	if assert.NotNil(t, summaryRows[0].RDISystemInfoSummary) {
		assert.Equal(t, "Plant A", summaryRows[0].RDISystemInfoSummary.InstallationLocation)
		assert.Equal(t, "Alex", summaryRows[0].RDISystemInfoSummary.InstallerName)
	}

	encoded, marshalErr := json.Marshal(summaryRows[0])
	assert.NoError(t, marshalErr)
	assert.NotContains(t, string(encoded), "Private Customer")
	assert.NotContains(t, string(encoded), "private@example.com")
	assert.NotContains(t, string(encoded), "do not expose")
	assert.NotContains(t, string(encoded), "must-not-leak")
}

func TestDeviceMapTelemetryReadScopeRejectsSameTenantNonOwner(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	configID := createDeviceServiceConfig(t, db, "map-owner-config", "tenant-a", "1")
	ownerUserID := "owner-b"
	device := &model.Device{
		ID:             "map-owner-device",
		DeviceNumber:   "map-owner-number",
		Voucher:        "map-owner-voucher",
		TenantID:       "tenant-a",
		OwnerUserID:    &ownerUserID,
		IsEnabled:      "enabled",
		ActivateFlag:   "active",
		CreatedAt:      &now,
		UpdateAt:       &now,
		DeviceConfigID: &configID,
	}
	assert.NoError(t, db.Create(device).Error)

	_, err := loadMapTelemetryDevice(device.ID, &utils.UserClaims{
		ID:        "owner-a",
		TenantID:  "tenant-a",
		Authority: constant.TENANT_USER,
	})
	assert.Error(t, err)

	loaded, err := loadMapTelemetryDevice(device.ID, &utils.UserClaims{
		ID:        ownerUserID,
		TenantID:  "tenant-a",
		Authority: constant.TENANT_USER,
	})
	assert.NoError(t, err)
	assert.Equal(t, device.ID, loaded.ID)
}

func TestBoardDeviceTotalAppliesTenantUserOwnerScope(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	configID := createDeviceServiceConfig(t, db, "board-owner-config", "tenant-a", "1")
	ownerA := "owner-a"
	ownerB := "owner-b"
	for _, device := range []*model.Device{
		{ID: "board-owner-a", DeviceNumber: "board-owner-a", Voucher: "board-owner-a-voucher", TenantID: "tenant-a", OwnerUserID: &ownerA, IsEnabled: "enabled", ActivateFlag: "active", CreatedAt: &now, UpdateAt: &now, DeviceConfigID: &configID},
		{ID: "board-owner-b", DeviceNumber: "board-owner-b", Voucher: "board-owner-b-voucher", TenantID: "tenant-a", OwnerUserID: &ownerB, IsEnabled: "enabled", ActivateFlag: "active", CreatedAt: &now, UpdateAt: &now, DeviceConfigID: &configID},
	} {
		assert.NoError(t, db.Create(device).Error)
	}

	total, err := (&Board{}).GetDeviceTotal(context.Background(), &utils.UserClaims{
		ID:        ownerA,
		TenantID:  "tenant-a",
		Authority: constant.TENANT_USER,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)

	total, err = (&Board{}).GetDeviceTotal(context.Background(), &utils.UserClaims{
		ID:        "tenant-admin",
		TenantID:  "tenant-a",
		Authority: constant.TENANT_ADMIN,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
}

func TestDeviceDeleteDeviceRejectsCrossTenantBeforeChildDataCleanup(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	tenantID := "tenant-b"
	deviceID := "device-delete-cross-tenant"
	stringValue := "online"

	if err := db.Create(&model.Device{
		ID:           deviceID,
		DeviceNumber: "delete-cross-tenant-number",
		Voucher:      "delete-cross-tenant-voucher",
		TenantID:     tenantID,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&model.TelemetryCurrentData{
		DeviceID: deviceID,
		Key:      "status",
		T:        now,
		StringV:  &stringValue,
		TenantID: &tenantID,
	}).Error; err != nil {
		t.Fatalf("create telemetry current data: %v", err)
	}

	err := (&Device{}).DeleteDevice(deviceID, &utils.UserClaims{TenantID: "tenant-a"})

	assertDeviceConfigServiceError(t, err, "cross-tenant delete device", errcode.CodeNoPermission, "no permission to delete device")
	assertDeviceServiceRowCount(t, db, &model.Device{}, "id = ?", deviceID, 1)
	assertDeviceServiceRowCount(t, db, &model.TelemetryCurrentData{}, "device_id = ?", deviceID, 1)
}

func TestDeviceDeleteDeviceRejectsSameTenantForeignOwner(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	deviceID := "device-delete-foreign-owner"
	ownerUserID := "user-b"

	if err := db.Create(&model.Device{
		ID:           deviceID,
		DeviceNumber: "delete-foreign-owner-number",
		Voucher:      "delete-foreign-owner-voucher",
		TenantID:     "tenant-a",
		OwnerUserID:  &ownerUserID,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	err := (&Device{}).DeleteDevice(deviceID, &utils.UserClaims{
		ID:        "user-a",
		TenantID:  "tenant-a",
		Authority: constant.TENANT_USER,
	})

	assertDeviceConfigServiceError(t, err, "same-tenant foreign-owner delete device", errcode.CodeNoPermission, "no permission to delete device")
	assertDeviceServiceRowCount(t, db, &model.Device{}, "id = ?", deviceID, 1)
}

func TestDeviceDeleteDeviceAllowsTenantUserOwner(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	deviceID := "device-delete-owner"
	ownerUserID := "user-a"

	if err := db.Create(&model.Device{
		ID:           deviceID,
		DeviceNumber: "delete-owner-number",
		Voucher:      "delete-owner-voucher",
		TenantID:     "tenant-a",
		OwnerUserID:  &ownerUserID,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	err := (&Device{}).DeleteDevice(deviceID, &utils.UserClaims{
		ID:        ownerUserID,
		TenantID:  "tenant-a",
		Authority: constant.TENANT_USER,
	})

	assert.NoError(t, err)
	assertDeviceServiceRowCount(t, db, &model.Device{}, "id = ?", deviceID, 0)
}

func TestDeviceDeleteDeviceSysAdminUsesTargetTenantForTransactionalDelete(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	tenantID := "tenant-b"
	deviceID := "device-delete-sys-admin"
	stringValue := "online"

	if err := db.Create(&model.Device{
		ID:           deviceID,
		DeviceNumber: "delete-sys-admin-number",
		Voucher:      "delete-sys-admin-voucher",
		TenantID:     tenantID,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&model.TelemetryCurrentData{
		DeviceID: deviceID,
		Key:      "status",
		T:        now,
		StringV:  &stringValue,
		TenantID: &tenantID,
	}).Error; err != nil {
		t.Fatalf("create telemetry current data: %v", err)
	}

	err := (&Device{}).DeleteDevice(deviceID, &utils.UserClaims{TenantID: "tenant-a", Authority: constant.SYS_ADMIN})

	assert.NoError(t, err)
	assertDeviceServiceRowCount(t, db, &model.Device{}, "id = ?", deviceID, 0)
	assertDeviceServiceRowCount(t, db, &model.TelemetryCurrentData{}, "device_id = ?", deviceID, 0)
}

func TestDeviceDeleteDevicePreservesAlarmHistoryDeviceReference(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	tenantID := "tenant-a"
	otherTenantID := "tenant-b"
	deviceID := "device-delete-alarm-history"
	keptDeviceID := "device-alarm-history-kept"

	if err := db.Create(&model.Device{
		ID:           deviceID,
		DeviceNumber: "delete-alarm-history-number",
		Voucher:      "delete-alarm-history-voucher",
		TenantID:     tenantID,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	histories := []model.AlarmHistory{
		{
			ID:                "alarm-history-only-deleted",
			AlarmConfigID:     "alarm-config-a",
			GroupID:           "group-a",
			SceneAutomationID: "scene-a",
			Name:              "single deleted device",
			AlarmStatus:       "H",
			TenantID:          tenantID,
			CreateAt:          now,
			AlarmDeviceList:   `["device-delete-alarm-history"]`,
		},
		{
			ID:                "alarm-history-keeps-other-device",
			AlarmConfigID:     "alarm-config-a",
			GroupID:           "group-a",
			SceneAutomationID: "scene-a",
			Name:              "multi device",
			AlarmStatus:       "M",
			TenantID:          tenantID,
			CreateAt:          now.Add(time.Second),
			AlarmDeviceList:   `["device-delete-alarm-history","device-alarm-history-kept"]`,
		},
		{
			ID:                "alarm-history-other-tenant",
			AlarmConfigID:     "alarm-config-b",
			GroupID:           "group-b",
			SceneAutomationID: "scene-b",
			Name:              "other tenant",
			AlarmStatus:       "H",
			TenantID:          otherTenantID,
			CreateAt:          now,
			AlarmDeviceList:   `["device-delete-alarm-history"]`,
		},
	}
	if err := db.Create(&histories).Error; err != nil {
		t.Fatalf("create alarm histories: %v", err)
	}

	err := (&Device{}).DeleteDevice(deviceID, &utils.UserClaims{TenantID: tenantID})

	assert.NoError(t, err)
	assertDeviceServiceRowCount(t, db, &model.Device{}, "id = ?", deviceID, 0)
	assert.Equal(t, []string{deviceID}, alarmHistoryDeviceListForTest(t, db, "alarm-history-only-deleted"))
	assert.Equal(t, []string{deviceID, keptDeviceID}, alarmHistoryDeviceListForTest(t, db, "alarm-history-keeps-other-device"))
	assert.Equal(t, []string{deviceID}, alarmHistoryDeviceListForTest(t, db, "alarm-history-other-tenant"))
}

func TestDeviceDeleteDeviceNotifiesProtocolPluginWithPreDeleteSnapshot(t *testing.T) {
	restoreRuntime := pluginruntime.Set(pluginruntime.RemoteHTTP())
	t.Cleanup(restoreRuntime)

	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	tenantID := "tenant-a"
	deviceID := "device-delete-plugin-http"
	configID := "device-delete-plugin-config"
	protocolType := "HTTP"
	voucherType := "BASIC"
	deviceType := strconv.Itoa(constant.DIRECT_CONNECTION)
	disconnectCalls := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/device/disconnect", r.URL.Path)
		var body struct {
			DeviceID string `json:"device_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode plugin request: %v", err)
		}
		disconnectCalls <- body.DeviceID
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
	}))
	defer server.Close()

	serviceConfig := fmt.Sprintf(`{"http_address":"%s"}`, strings.TrimPrefix(server.URL, "http://"))
	if err := db.Create(&model.ServicePlugin{
		ID:                "plugin-http-delete",
		Name:              "HTTP",
		ServiceIdentifier: protocolType,
		ServiceType:       1,
		CreateAt:          now,
		UpdateAt:          now,
		ServiceConfig:     &serviceConfig,
	}).Error; err != nil {
		t.Fatalf("create service plugin: %v", err)
	}
	if err := db.Create(&model.DeviceConfig{
		ID:           configID,
		Name:         "delete plugin config",
		DeviceType:   deviceType,
		TenantID:     tenantID,
		CreatedAt:    now,
		UpdatedAt:    now,
		ProtocolType: &protocolType,
		VoucherType:  &voucherType,
	}).Error; err != nil {
		t.Fatalf("create device config: %v", err)
	}
	createDeviceServiceDevice(t, db, deviceID, "delete-plugin-number", tenantID, configID, now)

	err := (&Device{}).DeleteDevice(deviceID, &utils.UserClaims{TenantID: tenantID})

	assert.NoError(t, err)
	assertDeviceServiceRowCount(t, db, &model.Device{}, "id = ?", deviceID, 0)
	select {
	case calledDeviceID := <-disconnectCalls:
		assert.Equal(t, deviceID, calledDeviceID)
	case <-time.After(time.Second):
		t.Fatal("protocol plugin disconnect was not called")
	}
}

func TestDeviceDeleteDeviceRejectsMissingClaims(t *testing.T) {
	err := (&Device{}).DeleteDevice("device-missing-claims", nil)

	assertDeviceConfigServiceError(t, err, "delete device missing claims", errcode.CodeNoPermission, "no permission to delete device")
}

func TestDeviceCreateSonDeviceBindsChildrenTransactionally(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	tenantID := "tenant-a"
	parentConfigID := createDeviceServiceConfig(t, db, "gateway-config", tenantID, strconv.Itoa(constant.GATEWAY_DEVICE))
	childConfigID := createDeviceServiceConfig(t, db, "child-config", tenantID, strconv.Itoa(constant.GATEWAY_SON_DEVICE))
	createDeviceServiceDevice(t, db, "parent-gateway", "parent-number", tenantID, parentConfigID, now)
	createDeviceServiceDevice(t, db, "child-1", "child-number-1", tenantID, childConfigID, now)
	createDeviceServiceDevice(t, db, "child-2", "child-number-2", tenantID, childConfigID, now)

	err := (&Device{}).CreateSonDevice(context.Background(), &model.CreateSonDeviceRes{
		ID:    "parent-gateway",
		SonID: "child-1, child-2, child-1",
	}, &utils.UserClaims{TenantID: tenantID})

	assert.NoError(t, err)
	assertDeviceChildBinding(t, db, "child-1", "parent-gateway", "child-1")
	assertDeviceChildBinding(t, db, "child-2", "parent-gateway", "child-2")
}

func TestDeviceCreateSonDeviceRollsBackWhenAnyChildIsInvalid(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	tenantID := "tenant-a"
	parentConfigID := createDeviceServiceConfig(t, db, "gateway-config", tenantID, strconv.Itoa(constant.GATEWAY_DEVICE))
	childConfigID := createDeviceServiceConfig(t, db, "child-config", tenantID, strconv.Itoa(constant.GATEWAY_SON_DEVICE))
	invalidConfigID := createDeviceServiceConfig(t, db, "direct-config", tenantID, strconv.Itoa(constant.DIRECT_CONNECTION))
	createDeviceServiceDevice(t, db, "parent-gateway", "parent-number", tenantID, parentConfigID, now)
	createDeviceServiceDevice(t, db, "child-1", "child-number-1", tenantID, childConfigID, now)
	createDeviceServiceDevice(t, db, "child-invalid", "child-number-invalid", tenantID, invalidConfigID, now)

	err := (&Device{}).CreateSonDevice(context.Background(), &model.CreateSonDeviceRes{
		ID:    "parent-gateway",
		SonID: "child-1, child-invalid",
	}, &utils.UserClaims{TenantID: tenantID})

	assert.Error(t, err)
	assertDeviceChildBinding(t, db, "child-1", "", "")
	assertDeviceChildBinding(t, db, "child-invalid", "", "")
}

func TestDeviceCreateSonDeviceRollsBackWhenChildAlreadyBound(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	tenantID := "tenant-a"
	parentConfigID := createDeviceServiceConfig(t, db, "gateway-config", tenantID, strconv.Itoa(constant.GATEWAY_DEVICE))
	childConfigID := createDeviceServiceConfig(t, db, "child-config", tenantID, strconv.Itoa(constant.GATEWAY_SON_DEVICE))
	createDeviceServiceDevice(t, db, "parent-gateway", "parent-number", tenantID, parentConfigID, now)
	createDeviceServiceDevice(t, db, "existing-parent", "existing-parent-number", tenantID, parentConfigID, now)
	createDeviceServiceDevice(t, db, "child-1", "child-number-1", tenantID, childConfigID, now)
	createDeviceServiceDevice(t, db, "child-bound", "child-number-bound", tenantID, childConfigID, now)
	setDeviceServiceChildBinding(t, db, "child-bound", "existing-parent", "child-bound")

	err := (&Device{}).CreateSonDevice(context.Background(), &model.CreateSonDeviceRes{
		ID:    "parent-gateway",
		SonID: "child-1, child-bound",
	}, &utils.UserClaims{TenantID: tenantID})

	assertDeviceConfigServiceError(t, err, "already-bound child bind", errcode.CodeParamError, "")
	assertDeviceChildBinding(t, db, "child-1", "", "")
	assertDeviceChildBinding(t, db, "child-bound", "existing-parent", "child-bound")
}

func TestDeviceCreateSonDeviceRollsBackWhenChildCrossTenant(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	tenantID := "tenant-a"
	otherTenantID := "tenant-b"
	parentConfigID := createDeviceServiceConfig(t, db, "gateway-config", tenantID, strconv.Itoa(constant.GATEWAY_DEVICE))
	childConfigID := createDeviceServiceConfig(t, db, "child-config", tenantID, strconv.Itoa(constant.GATEWAY_SON_DEVICE))
	otherChildConfigID := createDeviceServiceConfig(t, db, "other-child-config", otherTenantID, strconv.Itoa(constant.GATEWAY_SON_DEVICE))
	createDeviceServiceDevice(t, db, "parent-gateway", "parent-number", tenantID, parentConfigID, now)
	createDeviceServiceDevice(t, db, "child-1", "child-number-1", tenantID, childConfigID, now)
	createDeviceServiceDevice(t, db, "child-other-tenant", "child-number-other", otherTenantID, otherChildConfigID, now)

	err := (&Device{}).CreateSonDevice(context.Background(), &model.CreateSonDeviceRes{
		ID:    "parent-gateway",
		SonID: "child-1, child-other-tenant",
	}, &utils.UserClaims{TenantID: tenantID})

	assertDeviceConfigServiceError(t, err, "cross-tenant child bind", errcode.CodeNoPermission, "no permission to modify device telemetry")
	assertDeviceChildBinding(t, db, "child-1", "", "")
	assertDeviceChildBinding(t, db, "child-other-tenant", "", "")
}

func TestDeviceCreateSonDeviceRejectsNonGatewayParent(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	tenantID := "tenant-a"
	parentConfigID := createDeviceServiceConfig(t, db, "direct-config", tenantID, strconv.Itoa(constant.DIRECT_CONNECTION))
	childConfigID := createDeviceServiceConfig(t, db, "child-config", tenantID, strconv.Itoa(constant.GATEWAY_SON_DEVICE))
	createDeviceServiceDevice(t, db, "direct-parent", "direct-parent-number", tenantID, parentConfigID, now)
	createDeviceServiceDevice(t, db, "child-1", "child-number-1", tenantID, childConfigID, now)

	err := (&Device{}).CreateSonDevice(context.Background(), &model.CreateSonDeviceRes{
		ID:    "direct-parent",
		SonID: "child-1",
	}, &utils.UserClaims{TenantID: tenantID})

	assertDeviceConfigServiceError(t, err, "non-gateway parent bind", errcode.CodeParamError, "")
	assertDeviceChildBinding(t, db, "child-1", "", "")
}

func setupDeviceServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Device{},
		&model.DeviceConfig{},
		&model.TelemetryCurrentData{},
		&model.TelemetryData{},
		&model.TelemetrySetLog{},
		&model.AttributeData{},
		&model.AttributeSetLog{},
		&model.EventData{},
		&model.CommandSetLog{},
		&model.OtaUpgradeTask{},
		&model.OtaUpgradeTaskDetail{},
		&model.DeviceTriggerCondition{},
		&model.AlarmHistory{},
		&model.ServicePlugin{},
		&model.Group{},
	); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}

func createDeviceServiceConfig(t *testing.T, db *gorm.DB, id string, tenantID string, deviceType string) string {
	t.Helper()

	now := time.Now().UTC()
	protocolType := "MQTT"
	voucherType := "BASIC"
	if err := db.Create(&model.DeviceConfig{
		ID:           id,
		Name:         id,
		DeviceType:   deviceType,
		TenantID:     tenantID,
		CreatedAt:    now,
		UpdatedAt:    now,
		ProtocolType: &protocolType,
		VoucherType:  &voucherType,
	}).Error; err != nil {
		t.Fatalf("create device config %s: %v", id, err)
	}
	return id
}

func createDeviceServiceDevice(t *testing.T, db *gorm.DB, id string, number string, tenantID string, configID string, now time.Time) {
	t.Helper()

	if err := db.Create(&model.Device{
		ID:             id,
		DeviceNumber:   number,
		Voucher:        number + "-voucher",
		TenantID:       tenantID,
		IsEnabled:      "enabled",
		ActivateFlag:   "active",
		CreatedAt:      &now,
		UpdateAt:       &now,
		DeviceConfigID: &configID,
	}).Error; err != nil {
		t.Fatalf("create device %s: %v", id, err)
	}
}

func createDeviceServiceOwnedDevice(t *testing.T, db *gorm.DB, id string, number string, tenantID string, ownerUserID string, configID string, now time.Time) {
	t.Helper()

	createDeviceServiceDevice(t, db, id, number, tenantID, configID, now)
	if err := db.Model(&model.Device{}).
		Where("id = ?", id).
		Update("owner_user_id", ownerUserID).Error; err != nil {
		t.Fatalf("set owner for device %s: %v", id, err)
	}
}

func setDeviceServiceChildBinding(t *testing.T, db *gorm.DB, deviceID string, parentID string, subDeviceAddr string) {
	t.Helper()

	if err := db.Model(&model.Device{}).
		Where("id = ?", deviceID).
		Updates(map[string]interface{}{
			"parent_id":       parentID,
			"sub_device_addr": subDeviceAddr,
		}).Error; err != nil {
		t.Fatalf("set child binding %s: %v", deviceID, err)
	}
}

func assertDeviceChildBinding(t *testing.T, db *gorm.DB, deviceID string, wantParentID string, wantSubDeviceAddr string) {
	t.Helper()

	var device model.Device
	if err := db.First(&device, "id = ?", deviceID).Error; err != nil {
		t.Fatalf("query device %s: %v", deviceID, err)
	}
	if wantParentID == "" {
		assert.Nil(t, device.ParentID)
		assert.Nil(t, device.SubDeviceAddr)
		return
	}
	if assert.NotNil(t, device.ParentID) {
		assert.Equal(t, wantParentID, *device.ParentID)
	}
	if assert.NotNil(t, device.SubDeviceAddr) {
		assert.Equal(t, wantSubDeviceAddr, *device.SubDeviceAddr)
	}
}

func assertDeviceServiceRowCount(t *testing.T, db *gorm.DB, modelValue any, query string, arg any, want int64) {
	t.Helper()

	var count int64
	if err := db.Model(modelValue).Where(query, arg).Count(&count).Error; err != nil {
		t.Fatalf("count %T: %v", modelValue, err)
	}
	assert.Equal(t, want, count)
}

func alarmHistoryDeviceListForTest(t *testing.T, db *gorm.DB, historyID string) []string {
	t.Helper()

	var history model.AlarmHistory
	if err := db.First(&history, "id = ?", historyID).Error; err != nil {
		t.Fatalf("query alarm history %s: %v", historyID, err)
	}
	var ids []string
	if err := json.Unmarshal([]byte(history.AlarmDeviceList), &ids); err != nil {
		t.Fatalf("decode alarm device list for %s: %v", historyID, err)
	}
	return ids
}

// --- isValidDeviceID ---

func TestDeviceIsValidDeviceID_ValidCases(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"8字符字母数字", "abcd1234"},
		{"36字符UUID格式", "550e8400-e29b-41d4-a716-446655440000"},
		{"纯数字8位", "12345678"},
		{"纯字母8位", "abcdefgh"},
		{"含连字符和下划线", "dev_1234-5678"},
		{"36字符最大长度", "abcdef1234567890abcdef1234567890ab12"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, isValidDeviceID(tt.id), "expected valid for id: %s", tt.id)
		})
	}
}

func TestDeviceIsValidDeviceID_InvalidCases(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"空字符串", ""},
		{"7字符太短", "abc1234"},
		{"37字符太长", "550e8400-e29b-41d4-a716-4466554400001"},
		{"含空格", "abc 1234"},
		{"含中文", "设备1234567"},
		{"含特殊字符@", "abc@1234"},
		{"含特殊字符!", "abc!1234"},
		{"含点号", "abc.1234"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, isValidDeviceID(tt.id), "expected invalid for id: %s", tt.id)
		})
	}
}

// --- rdiDeviceSharedStatus ---

func TestDeviceRdiDeviceSharedStatus_NilInput(t *testing.T) {
	result := rdiDeviceSharedStatus(nil)
	assert.Equal(t, "unshared", result)
}

func TestDeviceRdiDeviceSharedStatus_EmptyString(t *testing.T) {
	s := ""
	result := rdiDeviceSharedStatus(&s)
	assert.Equal(t, "unshared", result)
}

func TestDeviceRdiDeviceSharedStatus_WhitespaceOnly(t *testing.T) {
	s := "   "
	result := rdiDeviceSharedStatus(&s)
	assert.Equal(t, "unshared", result)
}

func TestDeviceRdiDeviceSharedStatus_InvalidJSON(t *testing.T) {
	s := "not-json"
	result := rdiDeviceSharedStatus(&s)
	assert.Equal(t, "unshared", result)
}

func TestDeviceRdiDeviceSharedStatus_NoShareRecipientsKey(t *testing.T) {
	s := `{"other_key": "value"}`
	result := rdiDeviceSharedStatus(&s)
	assert.Equal(t, "unshared", result)
}

func TestDeviceRdiDeviceSharedStatus_EmptyRecipientsArray(t *testing.T) {
	s := `{"rdi_share_recipients": []}`
	result := rdiDeviceSharedStatus(&s)
	assert.Equal(t, "unshared", result)
}

func TestDeviceRdiDeviceSharedStatus_RecipientsWithEmptyUserID(t *testing.T) {
	s := `{"rdi_share_recipients": [{"user_id": "", "email": "test@example.com"}]}`
	result := rdiDeviceSharedStatus(&s)
	assert.Equal(t, "unshared", result)
}

func TestDeviceRdiDeviceSharedStatus_RecipientsWithNonEmptyUserID(t *testing.T) {
	s := `{"rdi_share_recipients": [{"user_id": "user123", "email": "test@example.com"}]}`
	result := rdiDeviceSharedStatus(&s)
	assert.Equal(t, "shared", result)
}

func TestDeviceRdiDeviceSharedStatus_RecipientsWithWhitespaceUserID(t *testing.T) {
	s := `{"rdi_share_recipients": [{"user_id": "   ", "email": "test@example.com"}]}`
	result := rdiDeviceSharedStatus(&s)
	assert.Equal(t, "unshared", result)
}

// --- NormalizeRDIPID ---

func TestDeviceNormalizeRDIPID_EmptyString(t *testing.T) {
	result, err := NormalizeRDIPID("")
	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestDeviceNormalizeRDIPID_WhitespaceOnly(t *testing.T) {
	result, err := NormalizeRDIPID("   ")
	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestDeviceNormalizeRDIPID_Valid12Chars(t *testing.T) {
	result, err := NormalizeRDIPID("abc123def456")
	assert.NoError(t, err)
	assert.Equal(t, "ABC123DEF456", result)
}

func TestDeviceNormalizeRDIPID_LowercaseToUpper(t *testing.T) {
	result, err := NormalizeRDIPID("abcdefghijkl")
	assert.NoError(t, err)
	assert.Equal(t, "ABCDEFGHIJKL", result)
}

func TestDeviceNormalizeRDIPID_TooShort(t *testing.T) {
	_, err := NormalizeRDIPID("abc123")
	assert.Error(t, err)
}

func TestDeviceNormalizeRDIPID_TooLong(t *testing.T) {
	_, err := NormalizeRDIPID("abc123def4567")
	assert.Error(t, err)
}

func TestDeviceNormalizeRDIPID_SpecialCharacters(t *testing.T) {
	_, err := NormalizeRDIPID("abc123def45!")
	assert.Error(t, err)
}

func TestDeviceNormalizeRDIPID_WithLeadingTrailingSpaces(t *testing.T) {
	result, err := NormalizeRDIPID("  abc123def456  ")
	assert.NoError(t, err)
	assert.Equal(t, "ABC123DEF456", result)
}

// --- GetVoucherTypeForm ---

func TestDeviceGetVoucherTypeForm_MQTT_BASIC(t *testing.T) {
	d := &Device{}
	result, err := d.GetVoucherTypeForm("BASIC", "1", "MQTT")
	assert.NoError(t, err)
	forms := result.([]*model.DeviceConnectFormRes)
	assert.Len(t, forms, 2)
	assert.Equal(t, "username", forms[0].DataKey)
	assert.Equal(t, "password", forms[1].DataKey)
}

func TestDeviceGetVoucherTypeForm_MQTT_ACCESSTOKEN(t *testing.T) {
	d := &Device{}
	result, err := d.GetVoucherTypeForm("ACCESSTOKEN", "1", "MQTT")
	assert.NoError(t, err)
	forms := result.([]*model.DeviceConnectFormRes)
	assert.Len(t, forms, 1)
	assert.Equal(t, "username", forms[0].DataKey)
	assert.Contains(t, forms[0].Label, "密码留空")
}

func TestDeviceGetVoucherTypeForm_MQTT_InvalidVoucherType(t *testing.T) {
	d := &Device{}
	_, err := d.GetVoucherTypeForm("INVALID", "1", "MQTT")
	assert.Error(t, err)
	assert.EqualError(t, err, "voucher type is error: INVALID")
}

// --- SafeDeref ---

func TestDeviceSafeDeref_Nil(t *testing.T) {
	assert.Equal(t, "", SafeDeref(nil))
}

func TestDeviceSafeDeref_NonNil(t *testing.T) {
	s := "hello"
	assert.Equal(t, "hello", SafeDeref(&s))
}

// --- StringPtr ---

func TestDeviceStringPtr(t *testing.T) {
	p := StringPtr("test")
	assert.NotNil(t, p)
	assert.Equal(t, "test", *p)
}

// --- IsJSON ---

func TestDeviceIsJSON_Valid(t *testing.T) {
	assert.True(t, IsJSON(`{"key":"value"}`))
	assert.True(t, IsJSON(`[1,2,3]`))
	assert.True(t, IsJSON(`null`))
	assert.True(t, IsJSON(`"string"`))
}

func TestDeviceIsJSON_Invalid(t *testing.T) {
	assert.False(t, IsJSON("not json"))
	assert.False(t, IsJSON("{invalid"))
}

// --- StructToMapAndVerifyJson ---

func TestDeviceStructToMapAndVerifyJson_NonStruct(t *testing.T) {
	_, err := StructToMapAndVerifyJson("not a struct")
	assert.Error(t, err)
}

func TestDeviceStructToMapAndVerifyJson_BasicStruct(t *testing.T) {
	type testStruct struct {
		Name  string  `json:"name"`
		Value *string `json:"value"`
	}
	v := "hello"
	s := testStruct{Name: "test", Value: &v}
	result, err := StructToMapAndVerifyJson(s)
	assert.NoError(t, err)
	assert.Equal(t, "test", result["name"])
	assert.Equal(t, &v, result["value"])
}

func TestDeviceStructToMapAndVerifyJson_NilFieldsSkipped(t *testing.T) {
	type testStruct struct {
		Name  string  `json:"name"`
		Value *string `json:"value"`
	}
	s := testStruct{Name: "test", Value: nil}
	result, err := StructToMapAndVerifyJson(s)
	assert.NoError(t, err)
	assert.Equal(t, "test", result["name"])
	_, hasValue := result["value"]
	assert.False(t, hasValue)
}

func TestDeviceStructToMapAndVerifyJson_JsonValidation(t *testing.T) {
	type testStruct struct {
		Info *string `json:"additional_info"`
	}
	validJSON := `{"key":"value"}`
	s := testStruct{Info: &validJSON}
	_, err := StructToMapAndVerifyJson(s, "additional_info")
	assert.NoError(t, err)
}

func TestDeviceStructToMapAndVerifyJson_InvalidJsonField(t *testing.T) {
	type testStruct struct {
		Info *string `json:"additional_info"`
	}
	invalidJSON := "not json"
	s := testStruct{Info: &invalidJSON}
	_, err := StructToMapAndVerifyJson(s, "additional_info")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not valid JSON")
}

// --- StructToMap ---

func TestDeviceStructToMap_NonStruct(t *testing.T) {
	result := StructToMap("not a struct")
	assert.Empty(t, result)
}

func TestDeviceStructToMap_BasicStruct(t *testing.T) {
	type testStruct struct {
		Name  string  `json:"name"`
		Value *string `json:"value"`
	}
	v := "hello"
	s := testStruct{Name: "test", Value: &v}
	result := StructToMap(s)
	assert.Equal(t, "test", result["name"])
	assert.Equal(t, &v, result["value"])
}

func TestDeviceStructToMap_NilPointerSkipped(t *testing.T) {
	type testStruct struct {
		Name  string  `json:"name"`
		Value *string `json:"value"`
	}
	s := testStruct{Name: "test", Value: nil}
	result := StructToMap(s)
	assert.Equal(t, "test", result["name"])
	_, hasValue := result["value"]
	assert.False(t, hasValue)
}

// --- parseAdditionalInfo ---

func TestDeviceParseAdditionalInfo_Nil(t *testing.T) {
	result := parseAdditionalInfo(nil)
	assert.Empty(t, result)
}

func TestDeviceParseAdditionalInfo_EmptyString(t *testing.T) {
	s := ""
	result := parseAdditionalInfo(&s)
	assert.Empty(t, result)
}

func TestDeviceParseAdditionalInfo_ValidJSON(t *testing.T) {
	s := `{"key":"value","number":42}`
	result := parseAdditionalInfo(&s)
	assert.Equal(t, "value", result["key"])
	// json.Unmarshal into map[string]interface{} decodes numbers as float64
	assert.Equal(t, float64(42), result["number"])
}

func TestDeviceParseAdditionalInfo_InvalidJSON(t *testing.T) {
	s := "not-json"
	result := parseAdditionalInfo(&s)
	assert.Empty(t, result)
}

// --- rdiAlarmHistoryMeta ---

func TestDeviceRdiAlarmHistoryMeta_TemperatureAlarm(t *testing.T) {
	eventType, status, ok := rdiAlarmHistoryMeta(&model.EventInfo{Method: "temperature_alarm"})
	assert.True(t, ok)
	assert.Equal(t, "Temperature Alarm", eventType)
	assert.Equal(t, "H", status) // default fallback
}

func TestDeviceRdiAlarmHistoryMeta_SwitchAlarm(t *testing.T) {
	eventType, status, ok := rdiAlarmHistoryMeta(&model.EventInfo{Method: "switch_alarm"})
	assert.True(t, ok)
	assert.Equal(t, "Switch Alarm", eventType)
	assert.Equal(t, "M", status) // default fallback
}

func TestDeviceRdiAlarmHistoryMeta_WarrantyAlarm(t *testing.T) {
	eventType, status, ok := rdiAlarmHistoryMeta(&model.EventInfo{Method: "warranty_alarm"})
	assert.True(t, ok)
	assert.Equal(t, "Warranty Alarm", eventType)
	assert.Equal(t, "L", status) // default fallback
}

func TestDeviceRdiAlarmHistoryMeta_SW3ShortPress(t *testing.T) {
	eventType, status, ok := rdiAlarmHistoryMeta(&model.EventInfo{Method: "sw3_short_press"})
	assert.True(t, ok)
	assert.Equal(t, "SW3 Short Press (Unbind)", eventType)
	assert.Equal(t, "N", status)
}

func TestDeviceRdiAlarmHistoryMeta_SW3LongPress(t *testing.T) {
	eventType, status, ok := rdiAlarmHistoryMeta(&model.EventInfo{Method: "sw3_long_press"})
	assert.True(t, ok)
	assert.Equal(t, "SW3 Long Press (Factory Reset)", eventType)
	assert.Equal(t, "N", status)
}

func TestDeviceRdiAlarmHistoryMeta_SW2LongPress(t *testing.T) {
	eventType, status, ok := rdiAlarmHistoryMeta(&model.EventInfo{Method: "sw2_long_press"})
	assert.True(t, ok)
	assert.Equal(t, "SW2 Long Press (WiFi Provisioning)", eventType)
	assert.Equal(t, "N", status)
}

func TestDeviceRdiAlarmHistoryMeta_UnknownMethod(t *testing.T) {
	_, _, ok := rdiAlarmHistoryMeta(&model.EventInfo{Method: "unknown_event"})
	assert.False(t, ok)
}

func TestDeviceRdiAlarmHistoryMeta_NilEventInfo(t *testing.T) {
	_, _, ok := rdiAlarmHistoryMeta(nil)
	assert.False(t, ok)
}

// --- rdiAlarmStatusFromParams ---

func TestDeviceRdiAlarmStatusFromParams_HighLevel(t *testing.T) {
	params := map[string]interface{}{"alarm_level": "HIGH"}
	assert.Equal(t, "H", rdiAlarmStatusFromParams(params, "N"))
}

func TestDeviceRdiAlarmStatusFromParams_MediumLevel(t *testing.T) {
	params := map[string]interface{}{"alarm_level": "MEDIUM"}
	assert.Equal(t, "M", rdiAlarmStatusFromParams(params, "N"))
}

func TestDeviceRdiAlarmStatusFromParams_LowLevel(t *testing.T) {
	params := map[string]interface{}{"alarm_level": "LOW"}
	assert.Equal(t, "L", rdiAlarmStatusFromParams(params, "N"))
}

func TestDeviceRdiAlarmStatusFromParams_NormalLevel(t *testing.T) {
	params := map[string]interface{}{"alarm_level": "NORMAL"}
	assert.Equal(t, "N", rdiAlarmStatusFromParams(params, "H"))
}

func TestDeviceRdiAlarmStatusFromParams_NoLevelKey(t *testing.T) {
	params := map[string]interface{}{"other_key": "value"}
	assert.Equal(t, "H", rdiAlarmStatusFromParams(params, "H"))
}

func TestDeviceRdiAlarmStatusFromParams_EmptyParams(t *testing.T) {
	params := map[string]interface{}{}
	assert.Equal(t, "M", rdiAlarmStatusFromParams(params, "M"))
}

// --- rdiDirectAlarmConfigID ---

func TestDeviceRdiDirectAlarmConfigID(t *testing.T) {
	tests := []struct {
		method   string
		expected string
	}{
		{"temperature_alarm", "rdi-direct-temperature-alarm"},
		{"switch_alarm", "rdi-direct-switch-alarm"},
		{"warranty_alarm", "rdi-direct-warranty-alarm"},
		{"sw3_short_press", "rdi-direct-sw3-short-press"},
		{"sw3_long_press", "rdi-direct-sw3-long-press"},
		{"sw2_long_press", "rdi-direct-sw2-long-press"},
		{"unknown", "rdi-direct-alarm"},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			assert.Equal(t, tt.expected, rdiDirectAlarmConfigID(tt.method))
		})
	}
}

// --- rdiAlarmSourceIndexFromValue ---

func TestDeviceRdiAlarmSourceIndexFromValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected int
	}{
		{"1", 1},
		{"2", 2},
		{"T1", 1},
		{"T2", 2},
		{"S1", 1},
		{"S2", 2},
		{"SENSOR1", 1},
		{"SENSOR_2", 2},
		{"SW1", 1},
		{"SW2", 2},
		{"SWITCH_1", 1},
		{"SWITCH_2", 2},
		{"3", 0},
		{"unknown", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input.(string), func(t *testing.T) {
			assert.Equal(t, tt.expected, rdiAlarmSourceIndexFromValue(tt.input))
		})
	}
}

// --- parseRDIEmailRecipients ---

func TestDeviceParseRDIEmailRecipients_Empty(t *testing.T) {
	assert.Nil(t, parseRDIEmailRecipients(""))
	assert.Nil(t, parseRDIEmailRecipients("   "))
}

func TestDeviceParseRDIEmailRecipients_SingleEmail(t *testing.T) {
	result := parseRDIEmailRecipients("test@example.com")
	assert.Equal(t, []string{"test@example.com"}, result)
}

func TestDeviceParseRDIEmailRecipients_MultipleEmails(t *testing.T) {
	result := parseRDIEmailRecipients("a@example.com, b@example.com")
	assert.Equal(t, []string{"a@example.com", "b@example.com"}, result)
}

func TestDeviceParseRDIEmailRecipients_Dedup(t *testing.T) {
	result := parseRDIEmailRecipients("a@example.com, A@EXAMPLE.COM")
	assert.Equal(t, []string{"a@example.com"}, result)
}

func TestDeviceParseRDIEmailRecipients_InvalidEmail(t *testing.T) {
	result := parseRDIEmailRecipients("not-an-email")
	assert.Empty(t, result)
}

func TestDeviceParseRDIEmailRecipients_MixedValidInvalid(t *testing.T) {
	result := parseRDIEmailRecipients("good@example.com, bad, ok@test.com")
	assert.Equal(t, []string{"good@example.com", "ok@test.com"}, result)
}

// --- readString ---

func TestDeviceReadString_KeyExists(t *testing.T) {
	m := map[string]interface{}{"key": "value"}
	assert.Equal(t, "value", readString(m, "key", "fallback"))
}

func TestDeviceReadString_KeyNotExists(t *testing.T) {
	m := map[string]interface{}{}
	assert.Equal(t, "fallback", readString(m, "key", "fallback"))
}

func TestDeviceReadString_EmptyValue(t *testing.T) {
	m := map[string]interface{}{"key": ""}
	assert.Equal(t, "fallback", readString(m, "key", "fallback"))
}

func TestDeviceReadString_NonStringValue(t *testing.T) {
	m := map[string]interface{}{"key": 42}
	assert.Equal(t, "fallback", readString(m, "key", "fallback"))
}

// --- contains ---

func TestDeviceContains_Found(t *testing.T) {
	assert.True(t, contains([]string{"a", "b", "c"}, "b"))
}

func TestDeviceContains_NotFound(t *testing.T) {
	assert.False(t, contains([]string{"a", "b", "c"}, "d"))
}

func TestDeviceContains_EmptySlice(t *testing.T) {
	assert.False(t, contains([]string{}, "a"))
}
