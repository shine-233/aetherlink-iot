//go:build external_integration

// 文件用途：验证直连设备遥测上报和平台遥测控制闭环。
// 核心逻辑：发布遥测后查询历史表与当前表，下发遥测控制后订阅设备 topic 并核对控制日志。
// 关键注意事项：测试依赖真实数据同步等待时间，运行前应确保设备 ID 与环境隔离。
// 重构建议：可引入唯一测试 key/message_id 和更严格时间窗口，降低历史数据造成的误判。

/*
Purpose: 验证直连设备遥测上报和平台遥测控制闭环。
Core logic: 发布遥测后查询历史表与当前表，下发遥测控制后订阅设备 topic 并核对控制日志。
Important notes: 测试依赖真实数据同步等待时间，数据库查询当前偏向最新记录，运行前应确保设备 ID 与环境隔离。
Refactor suggestion: 可引入唯一测试 key/message_id 和更严格时间窗口，降低历史数据造成的误判。
*/
package tests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"aetherlink-iot/aetherlink-device-autotest/internal/config"
	"aetherlink-iot/aetherlink-device-autotest/internal/device"
	"aetherlink-iot/aetherlink-device-autotest/internal/platform"
	"aetherlink-iot/aetherlink-device-autotest/internal/utils"
)

func TestTelemetryPublish(t *testing.T) {
	requireExternalIntegration(t)
	// 初始化 logger。
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	// 加载测试配置。
	cfg, err := config.Load("../../config-community.yaml")
	require.NoError(t, err)

	// 初始化设备并建立连接。
	dev, err := device.NewDevice(cfg, logger)
	require.NoError(t, err)
	require.NoError(t, dev.Connect())
	defer dev.Disconnect()

	// 初始化数据库客户端。
	dbClient, err := platform.NewDBClient(&cfg.Database, logger)
	require.NoError(t, err)
	defer dbClient.Close()

	// 构造遥测测试数据。
	testData := utils.BuildTelemetryData()
	startTime := time.Now()

	logger.Info("Starting telemetry test",
		zap.Time("start_time", startTime),
		zap.Any("test_data", testData))

	// 发布遥测数据。
	err = dev.PublishTelemetry(testData)
	require.NoError(t, err)

	// 等待数据同步到数据库。
	time.Sleep(time.Duration(cfg.Test.WaitDBSyncSeconds) * time.Second)

	// 验证遥测数据入库。
	for key, expectedValue := range testData {
		// 查询开始时间之后的历史遥测记录。
		records, err := dbClient.QueryTelemetryData(cfg.Device.DeviceID, key, startTime)
		require.NoError(t, err)
		require.NotEmpty(t, records, "No telemetry data found for key: %s", key)

		record := records[0]

		// 验证设备 ID。
		assert.Equal(t, cfg.Device.DeviceID, record.DeviceID)

		// 验证遥测值。
		err = utils.ValidateTelemetryData(testData, record.Key, record.BoolV, record.NumberV, record.StringV)
		assert.NoError(t, err, "Value validation failed for key: %s", key)

		logger.Info("Telemetry data verified from history table",
			zap.String("key", key),
			zap.Any("expected_value", expectedValue),
			zap.Time("data_time", time.Unix(record.TS, 0)))

		// 验证当前遥测表。
		currentRecord, err := dbClient.QueryCurrentTelemetry(cfg.Device.DeviceID, key)
		require.NoError(t, err)
		require.NotNil(t, currentRecord, "No current telemetry data found for key: %s", key)

		err = utils.ValidateTelemetryData(testData, currentRecord.Key, currentRecord.BoolV, currentRecord.NumberV, currentRecord.StringV)
		assert.NoError(t, err, "Current value validation failed for key: %s", key)

		logger.Info("Telemetry data verified from current table",
			zap.String("key", key),
			zap.Any("expected_value", expectedValue),
			zap.Time("data_time", time.Unix(currentRecord.TS, 0)))
	}

	logger.Info("Telemetry test completed successfully")
}

func TestTelemetryControl(t *testing.T) {
	requireExternalIntegration(t)
	// 初始化 logger。
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	// 加载测试配置。
	cfg, err := config.Load("../../config-community.yaml")
	require.NoError(t, err)

	// 初始化设备并建立连接。
	dev, err := device.NewDevice(cfg, logger)
	require.NoError(t, err)
	require.NoError(t, dev.Connect())
	defer dev.Disconnect()

	// 订阅设备相关 topic。
	require.NoError(t, dev.SubscribeAll())

	// 初始化 API 客户端。
	apiClient := platform.NewAPIClient(&cfg.API, logger)

	// 初始化数据库客户端。
	dbClient, err := platform.NewDBClient(&cfg.Database, logger)
	require.NoError(t, err)
	defer dbClient.Close()

	// 准备 topic helper 并清理旧消息。
	topics := utils.NewMQTTTopics(cfg.Device.DeviceNumber)
	dev.ClearReceivedMessages(topics.TelemetryControl())

	// 构造遥测控制数据。
	controlData := map[string]interface{}{
		"switch":      true,
		"temperature": 25.5,
	}

	startTime := time.Now()

	// 下发遥测控制。
	err = apiClient.PublishTelemetryControl(cfg.Device.DeviceID, controlData)
	require.NoError(t, err)

	// 等待设备接收控制消息。
	timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
	messages := dev.GetReceivedMessages(topics.TelemetryControl(), timeout)
	require.NotEmpty(t, messages, "Device did not receive control message")

	// 验证收到的控制 payload。
	var receivedData map[string]interface{}
	err = json.Unmarshal(messages[0].Payload, &receivedData)
	require.NoError(t, err)

	for key, expectedValue := range controlData {
		actualValue, ok := receivedData[key]
		assert.True(t, ok, "Control data key not found: %s", key)
		assert.Equal(t, expectedValue, actualValue, "Control data value mismatch for key: %s", key)
	}

	logger.Info("Control message received and verified",
		zap.Any("data", receivedData))

	// 验证遥测控制日志。
	time.Sleep(1 * time.Second)
	logs, err := dbClient.QueryTelemetrySetLogs(cfg.Device.DeviceID, startTime)
	require.NoError(t, err)
	require.NotEmpty(t, logs, "No telemetry set log found")

	log := logs[0]
	assert.Equal(t, cfg.Device.DeviceID, log.DeviceID)
	assert.Equal(t, "1", log.Status, "Telemetry control should be successful")

	logger.Info("Telemetry set log verified",
		zap.String("log_id", log.ID),
		zap.String("status", log.Status))
}
