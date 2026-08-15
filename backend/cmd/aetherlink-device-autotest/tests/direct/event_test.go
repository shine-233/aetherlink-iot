//go:build external_integration

// 文件用途：验证直连设备事件上报及平台事件响应格式。
// 核心逻辑：发布 method/params 事件后查询事件表，并尝试接收平台事件响应 topic 进行 result 校验。
// 关键注意事项：平台事件响应是该闭环的必需结果，缺失或格式错误必须失败，不能只记录警告。
// 重构建议：可根据协议期望明确响应是否必需，并把可选响应变成单独测试场景。

/*
Purpose: 验证直连设备事件上报及平台事件响应格式。
Core logic: 发布 method/params 事件后查询事件表，并尝试接收平台事件响应 topic 进行 result 校验。
Important notes: 平台事件响应是该闭环的必需结果；缺失响应不再被警告路径掩盖。
Refactor suggestion: 可根据协议期望明确响应是否必需，并把可选响应变成单独测试场景。
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

func TestEventPublish(t *testing.T) {
	requireExternalIntegration(t)
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	cfg, err := config.Load("../../config-community.yaml")
	require.NoError(t, err)

	dev, err := device.NewDevice(cfg, logger)
	require.NoError(t, err)
	require.NoError(t, dev.Connect())
	defer dev.Disconnect()

	// 订阅设备相关 topic，确保能接收平台侧事件响应。
	require.NoError(t, dev.SubscribeAll())

	dbClient, err := platform.NewDBClient(&cfg.Database, logger)
	require.NoError(t, err)
	defer dbClient.Close()

	topics := utils.NewMQTTTopics(cfg.Device.DeviceNumber)
	dev.ClearReceivedMessages("")

	// 构造事件上报数据。
	messageID := utils.GenerateMessageID()
	method := "AlarmTriggered"
	params := map[string]interface{}{
		"alarm_type": "temperature_high",
		"level":      "critical",
		"value":      85.5,
	}

	startTime := time.Now()

	logger.Info("Publishing event",
		zap.String("message_id", messageID),
		zap.String("method", method),
		zap.Any("params", params))

	// 发布事件数据。
	err = dev.PublishEvent(method, params, messageID)
	require.NoError(t, err)

	// 等待数据同步到数据库。
	time.Sleep(time.Duration(cfg.Test.WaitDBSyncSeconds) * time.Second)

	// 查询并验证事件入库记录。
	records, err := dbClient.QueryEventData(cfg.Device.DeviceID, method, startTime)
	require.NoError(t, err)
	require.NotEmpty(t, records, "No event data found")

	record := records[0]

	assert.Equal(t, cfg.Device.DeviceID, record.DeviceID)
	assert.Equal(t, method, record.Identify)

	// 校验事件数据内容。
	err = utils.ValidateEventData(method, params, record.Data)
	assert.NoError(t, err, "Event data validation failed")

	logger.Info("Event data verified in database",
		zap.String("method", method),
		zap.String("event_id", record.ID),
		zap.Time("event_time", record.TS))

	// 等待并读取平台事件响应消息。
	timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
	responseMessages := dev.GetReceivedMessages(topics.EventResponse(), timeout)
	require.NotEmpty(t, responseMessages, "No event response received from platform")

	logger.Info("Received event response from platform",
		zap.Int("response_count", len(responseMessages)))

	for _, msg := range responseMessages {
		var response map[string]interface{}
		err := json.Unmarshal(msg.Payload, &response)
		require.NoError(t, err, "Failed to parse response")

		// 校验响应格式。
		err = utils.ValidateResponse(response)
		assert.NoError(t, err, "Response validation failed")

		logger.Info("Event response validated",
			zap.String("topic", msg.Topic),
			zap.Any("response", response))
	}
}
