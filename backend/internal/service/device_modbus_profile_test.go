// 文件用途：设备 Modbus 点表服务层回归测试（ROADMAP B1）。
// 核心逻辑：sqlite 内存库验证保存/读取回环、凭证字段拒绝、按编号拉取的租户校验。
// 关键注意事项：profile 不允许携带 username/password 等凭证字段；跨租户拉取必须拒绝。
package service

import (
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/require"
)

func TestModbusProfileSaveAndGetRoundtrip(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.DeviceModbusProfile{}))
	deviceID := "dev-modbus-1"
	if err := db.Create(&model.Device{
		ID:           deviceID,
		DeviceNumber: "modbus-plc-01",
		Voucher:      "v",
		TenantID:     "tenant-1",
		IsEnabled:    "enabled",
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	claims := &utils.UserClaims{ID: "user-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}

	svc := &DeviceModbusProfile{}
	profile := `{"target":{"host":"192.168.1.50","port":502,"unit_id":1},"registers":[{"key":"temperature","type":"input","address":100,"data_type":"i16","multiplier":0.1}]}`
	result, err := svc.SaveProfile(deviceID, []byte(profile), claims)
	require.NoError(t, err)
	require.Equal(t, deviceID, result["device_id"])

	resp, err := svc.GetProfileForUser(deviceID, claims)
	require.NoError(t, err)
	require.Equal(t, deviceID, resp.DeviceID)
	profileMap, ok := resp.Profile.(map[string]interface{})
	require.True(t, ok)
	targetMap, ok := profileMap["target"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "192.168.1.50", targetMap["host"])

	// 二次保存走 upsert，不产生第二行。
	_, err = svc.SaveProfile(deviceID, []byte(`{"registers":[]}`), claims)
	require.NoError(t, err)
	resp2, err := svc.GetProfileForUser(deviceID, claims)
	require.NoError(t, err)
	profileMap2 := resp2.Profile.(map[string]interface{})
	require.Empty(t, profileMap2["registers"])
}

func TestModbusProfileRejectsCredentialFields(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.DeviceModbusProfile{}))
	deviceID := "dev-modbus-cred"
	if err := db.Create(&model.Device{
		ID: deviceID, DeviceNumber: "modbus-plc-02", Voucher: "v",
		TenantID: "tenant-1", IsEnabled: "enabled",
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	claims := &utils.UserClaims{ID: "user-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}
	svc := &DeviceModbusProfile{}

	_, err := svc.SaveProfile(deviceID, []byte(`{"username":"u","registers":[]}`), claims)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "credential"), err.Error())

	_, err = svc.SaveProfile(deviceID, []byte(`{"Password":"p"}`), claims)
	require.Error(t, err)

	_, err = svc.SaveProfile(deviceID, []byte(`not-json`), claims)
	require.Error(t, err)
}

func TestModbusProfileByDeviceNumberTenantCheck(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.DeviceModbusProfile{}))
	if err := db.Create(&model.Device{
		ID: "dev-num-1", DeviceNumber: "num-1", Voucher: "v",
		TenantID: "tenant-1", IsEnabled: "enabled",
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	owner := &utils.UserClaims{ID: "user-1", TenantID: "tenant-1", Authority: constant.SYS_ADMIN}
	svc := &DeviceModbusProfile{}

	_, err := svc.SaveProfile("dev-num-1", []byte(`{"registers":[]}`), &utils.UserClaims{ID: "user-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN})
	require.NoError(t, err)

	resp, err := svc.GetProfileByDeviceNumber("num-1", owner)
	require.NoError(t, err)
	require.Equal(t, "dev-num-1", resp.DeviceID)

	stranger := &utils.UserClaims{ID: "user-x", TenantID: "tenant-9", Authority: constant.SYS_ADMIN}
	_, err = svc.GetProfileByDeviceNumber("num-1", stranger)
	require.Error(t, err)
}
