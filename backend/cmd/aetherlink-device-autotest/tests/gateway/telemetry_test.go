//go:build external_integration

// 文件用途：验证网关遥测嵌套上报在平台中的入库效果。
// 核心逻辑：构造顶层网关、直连子设备、子网关及其子设备 telemetry payload，发布后按各 device_id 查询数据库。
// 关键注意事项：用例会根据配置中是否存在子设备/子网关动态覆盖路径，缺配置时对应分支不会被验证。
// 重构建议：可将拓扑样例固定化，并为缺失拓扑分支输出明确跳过原因。

/*
Purpose: 验证网关遥测嵌套上报在平台中的入库效果。
Core logic: 构造顶层网关、直连子设备、子网关及其子设备的 telemetry payload，发布后按各 device_id 查询数据库。
Important notes: 用例会根据配置中是否存在子设备/子网关动态覆盖路径，缺配置时对应分支不会被验证。
Refactor suggestion: 可将拓扑样例固定化，并为缺失拓扑分支输出明确跳过原因。
*/
package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"aetherlink-iot/aetherlink-device-autotest/internal/config"
	"aetherlink-iot/aetherlink-device-autotest/internal/device"
	"aetherlink-iot/aetherlink-device-autotest/internal/platform"
	"aetherlink-iot/aetherlink-device-autotest/internal/protocol"
	"aetherlink-iot/aetherlink-device-autotest/internal/utils"
)

func requireGatewayTelemetryRecord(t *testing.T, records []platform.TelemetryData, expected map[string]interface{}, key string) {
	t.Helper()
	require.NotEmpty(t, records, "No telemetry data found for key: %s", key)
	record := records[0]
	require.Equal(t, key, record.Key, "Telemetry key mismatch")
	require.NoError(t, utils.ValidateTelemetryData(expected, record.Key, record.BoolV, record.NumberV, record.StringV),
		"Telemetry value mismatch for key: %s", key)
}

func TestGatewayTelemetryPublish(t *testing.T) {
	requireExternalIntegration(t)
	// 初始化日志。
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	// 加载网关测试配置。
	cfg, err := config.Load("../../config-gateway-community.yaml")
	require.NoError(t, err)

	// 验证配置中的设备类型为网关。
	require.Equal(t, "gateway", cfg.DeviceType, "Test requires gateway device type")

	// 创建并连接网关设备。
	dev, err := device.NewDevice(cfg, logger)
	require.NoError(t, err)
	require.NoError(t, dev.Connect())
	defer dev.Disconnect()

	// 创建数据库客户端。
	dbClient, err := platform.NewDBClient(&cfg.Database, logger)
	require.NoError(t, err)
	defer dbClient.Close()

	// 构建嵌套遥测数据。
	// 包含：网关自身数据、直连子设备数据、子网关数据。
	gatewayData := map[string]interface{}{
		"temperature": 26.8,
		"humidity":    65.0,
	}

	subDeviceData := make(map[string]interface{})
	// 如果配置中存在直连子设备，则添加其遥测数据。
	if len(cfg.Gateway.SubDevices) > 0 {
		subDev := cfg.Gateway.SubDevices[0]
		subDeviceData[subDev.SubDeviceNumber] = map[string]interface{}{
			"temperature": 25.0,
			"switch":      true,
		}
	}

	subGatewayData := make(map[string]interface{})
	// 如果配置中存在子网关，则添加其遥测数据。
	if len(cfg.Gateway.SubGateways) > 0 {
		subGw := cfg.Gateway.SubGateways[0]
		subGwData := map[string]interface{}{
			"gateway_data": map[string]interface{}{
				"temperature": 28.5,
				"version":     "v1.0",
			},
		}

		// 如果子网关下存在子设备，则添加这些子设备的遥测数据。
		if len(subGw.SubDevices) > 0 {
			subGwSubDevData := make(map[string]interface{})
			for _, subDev := range subGw.SubDevices {
				subGwSubDevData[subDev.SubDeviceNumber] = map[string]interface{}{
					"temperature": 27.0,
					"humidity":    60.0,
				}
			}
			subGwData["sub_device_data"] = subGwSubDevData
		}

		subGatewayData[subGw.SubGatewayNumber] = subGwData
	}

	// 使用辅助函数构建完整的嵌套遥测 payload。
	testData := protocol.BuildNestedTelemetry(gatewayData, subDeviceData, subGatewayData)

	startTime := time.Now()

	logger.Info("Starting gateway telemetry test",
		zap.Time("start_time", startTime),
		zap.Any("test_data", testData))

	// 发布遥测数据。
	err = dev.PublishTelemetry(testData)
	require.NoError(t, err)

	// 等待数据同步到数据库。
	time.Sleep(time.Duration(cfg.Test.WaitDBSyncSeconds) * time.Second)

	// 验证网关自身数据。
	logger.Info("Verifying gateway data",
		zap.String("gateway_device_id", cfg.Device.DeviceID))

	for key := range gatewayData {
		records, err := dbClient.QueryTelemetryData(cfg.Device.DeviceID, key, startTime)
		require.NoError(t, err)
		requireGatewayTelemetryRecord(t, records, gatewayData, key)

		logger.Info("Gateway telemetry data verified",
			zap.String("key", key),
			zap.String("device_id", cfg.Device.DeviceID))
	}

	// 验证直连子设备数据。
	if len(cfg.Gateway.SubDevices) > 0 {
		subDev := cfg.Gateway.SubDevices[0]
		logger.Info("Verifying sub-device data",
			zap.String("sub_device_id", subDev.DeviceID),
			zap.String("sub_device_number", subDev.SubDeviceNumber))

		records, err := dbClient.QueryTelemetryData(subDev.DeviceID, "temperature", startTime)
		require.NoError(t, err)
		requireGatewayTelemetryRecord(t, records, subDeviceData[subDev.SubDeviceNumber].(map[string]interface{}), "temperature")

		logger.Info("Sub-device telemetry data verified",
			zap.String("sub_device_number", subDev.SubDeviceNumber),
			zap.String("key", "temperature"))
	}

	// 验证子网关数据。
	if len(cfg.Gateway.SubGateways) > 0 {
		subGw := cfg.Gateway.SubGateways[0]
		logger.Info("Verifying sub-gateway data",
			zap.String("sub_gateway_id", subGw.DeviceID),
			zap.String("sub_gateway_number", subGw.SubGatewayNumber))

		subGwData := subGatewayData[subGw.SubGatewayNumber].(map[string]interface{})
		subGwGatewayData := subGwData["gateway_data"].(map[string]interface{})
		records, err := dbClient.QueryTelemetryData(subGw.DeviceID, "temperature", startTime)
		require.NoError(t, err)
		requireGatewayTelemetryRecord(t, records, subGwGatewayData, "temperature")

		logger.Info("Sub-gateway telemetry data verified",
			zap.String("sub_gateway_number", subGw.SubGatewayNumber),
			zap.String("key", "temperature"))

		// 验证子网关下的子设备数据。
		if len(subGw.SubDevices) > 0 {
			subGwSubDev := subGw.SubDevices[0]
			logger.Info("Verifying sub-gateway's sub-device data",
				zap.String("device_id", subGwSubDev.DeviceID),
				zap.String("sub_device_number", subGwSubDev.SubDeviceNumber))

			subGwSubDevData := subGwData["sub_device_data"].(map[string]interface{})
			records, err := dbClient.QueryTelemetryData(subGwSubDev.DeviceID, "temperature", startTime)
			require.NoError(t, err)
			requireGatewayTelemetryRecord(t, records, subGwSubDevData[subGwSubDev.SubDeviceNumber].(map[string]interface{}), "temperature")

			logger.Info("Sub-gateway's sub-device telemetry data verified",
				zap.String("sub_device_number", subGwSubDev.SubDeviceNumber),
				zap.String("key", "temperature"))
		}
	}

	logger.Info("Gateway telemetry test completed successfully")
}
