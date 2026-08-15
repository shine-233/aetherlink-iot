//go:build external_integration

// 文件用途：验证直连设备属性上报和属性设置下行响应闭环。
// 核心逻辑：发布属性后查询属性表，通过 API 下发属性设置，设备响应后核对日志。
// 关键注意事项：该用例依赖真实 API、MQTT broker 和 PostgreSQL，默认 go test 不应声明集成闭环。
// 重构建议：可统一 message_id 提取工具，并把日志轮询封装为带超时的断言 helper。

/*
Purpose: 验证直连设备属性上报和属性设置下行响应闭环。
Core logic: 发布属性后查询属性表；通过 API 下发属性设置，设备订阅消息、提取 message_id、发送响应并核对日志。
Important notes: JSON 数字会被反序列化为 float64，测试里包含类型兼容逻辑；响应日志可能存在延迟写入。
Refactor suggestion: 可统一 message_id 提取工具，并把日志轮询封装为带超时的断言 helper。
*/
package tests

import (
	"encoding/json"
	"strings"
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

func TestAttributePublish(t *testing.T) {
	requireExternalIntegration(t)
	// 初始化 logger。
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	cfg, err := config.Load("../../config-community.yaml")
	require.NoError(t, err)

	dev, err := device.NewDevice(cfg, logger)
	require.NoError(t, err)
	require.NoError(t, dev.Connect())
	defer dev.Disconnect()

	dbClient, err := platform.NewDBClient(&cfg.Database, logger)
	require.NoError(t, err)
	defer dbClient.Close()

	// 构造测试数据。
	messageID := utils.GenerateMessageID()
	testData := utils.BuildAttributeData()

	// 发送属性数据。
	err = dev.PublishAttribute(testData, messageID)
	require.NoError(t, err)

	// 等待数据同步。
	time.Sleep(time.Duration(cfg.Test.WaitDBSyncSeconds) * time.Second)

	// 验证数据入库。
	for key, expectedValue := range testData {
		record, err := dbClient.QueryAttributeData(cfg.Device.DeviceID, key)
		require.NoError(t, err)
		require.NotNil(t, record, "No attribute data found for key: %s", key)

		if record != nil {
			assert.Equal(t, cfg.Device.DeviceID, record.DeviceID)
			err = utils.ValidateAttributeData(testData, record.Key, record.BoolV, record.NumberV, record.StringV)
			assert.NoError(t, err, "Value validation failed for key: %s", key)

			logger.Info("Attribute data verified",
				zap.String("key", key),
				zap.Any("expected_value", expectedValue))
		}
	}
}

// compareValues 比较两个值是否相等（处理 JSON 数字类型转换）。
func compareValues(expected, actual interface{}) bool {
	// 处理数字类型：兼容 int、int64 和 float64。
	switch exp := expected.(type) {
	case int:
		if act, ok := actual.(float64); ok {
			return float64(exp) == act
		}
	case int64:
		if act, ok := actual.(float64); ok {
			return float64(exp) == act
		}
	case float64:
		if act, ok := actual.(float64); ok {
			return exp == act
		}
		if act, ok := actual.(int); ok {
			return exp == float64(act)
		}
	}
	// 其他类型直接比较。
	return expected == actual
}

func TestAttributeSet(t *testing.T) {
	requireExternalIntegration(t)
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	cfg, err := config.Load("../../config-community.yaml")
	require.NoError(t, err)

	dev, err := device.NewDevice(cfg, logger)
	require.NoError(t, err)
	require.NoError(t, dev.Connect())
	defer dev.Disconnect()

	// 订阅所有设备相关 topic。
	require.NoError(t, dev.SubscribeAll())

	apiClient := platform.NewAPIClient(&cfg.API, logger)
	dbClient, err := platform.NewDBClient(&cfg.Database, logger)
	require.NoError(t, err)
	defer dbClient.Close()

	topics := utils.NewMQTTTopics(cfg.Device.DeviceNumber)
	dev.ClearReceivedMessages("")

	// 构造属性设置数据，使用 float64 以匹配 JSON 解析结果。
	attributeData := map[string]interface{}{
		"ip":   "192.168.1.100",
		"port": float64(8080),
	}

	// 下发属性设置。
	err = apiClient.PublishAttributeSet(cfg.Device.DeviceID, attributeData)
	require.NoError(t, err)

	// 等待设备接收。
	timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
	messages := dev.GetReceivedMessages(topics.AttributeSet(), timeout)
	require.NotEmpty(t, messages, "Device did not receive attribute set message")

	receivedMsg := messages[0]

	var receivedData map[string]interface{}
	err = json.Unmarshal(receivedMsg.Payload, &receivedData)
	require.NoError(t, err)

	// 验证接收到的数据。
	for key, expectedValue := range attributeData {
		actualValue, ok := receivedData[key]
		assert.True(t, ok, "Attribute key not found: %s", key)
		assert.Equal(t, expectedValue, actualValue, "Attribute value mismatch for key: %s", key)
	}

	// 从 topic 中提取 message_id。
	topicParts := strings.Split(receivedMsg.Topic, "/")
	messageID := topicParts[len(topicParts)-1]

	logger.Info("Extracted message_id from topic",
		zap.String("topic", receivedMsg.Topic),
		zap.String("message_id", messageID))

	// 使用提取的 message_id 发送响应。
	err = dev.PublishAttributeSetResponse(messageID, true)
	require.NoError(t, err)

	logger.Info("Attribute set response sent",
		zap.String("message_id", messageID))

	// 等待响应被处理，保留额外等待时间。
	time.Sleep(3 * time.Second)

	// 使用重试机制验证属性设置日志。
	var log *platform.AttributeSetLog
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		log, err = dbClient.QueryAttributeSetLogs(cfg.Device.DeviceID, messageID)
		require.NoError(t, err)

		if log != nil && log.RspData != nil {
			// 找到响应数据后跳出循环。
			break
		}

		if i < maxRetries-1 {
			logger.Info("Response data not yet recorded, retrying...",
				zap.Int("attempt", i+1),
				zap.Int("max_retries", maxRetries))
			time.Sleep(2 * time.Second)
		}
	}

	require.NotNil(t, log, "Attribute set log not found")
	require.NotNil(t, log.MessageID, "Attribute set log message ID should be recorded")
	assert.Equal(t, cfg.Device.DeviceID, log.DeviceID, "Device ID mismatch")
	assert.Equal(t, messageID, *log.MessageID, "Message ID should match")

	// 验证状态，"3" 表示响应成功。
	assert.Equal(t, "3", log.Status, "Status should be '3' (response success)")

	// 重试循环已给平台最多约 11 秒完成异步落库；此处必须断言响应数据确实被记录，
	// 否则就是“等待后仍放行 NULL”的假覆盖。若 RspData 仍为空则判定失败。
	require.NotNil(t, log.RspData, "Response data should be recorded after retries")
	logger.Info("Response data recorded successfully",
		zap.String("rsp_data", *log.RspData))

	// 验证操作类型。
	assert.Equal(t, "1", log.OperationType, "Operation type should be '1' (manual)")

	logger.Info("Attribute set log verified",
		zap.String("log_id", log.ID),
		zap.String("message_id", messageID),
		zap.String("status", log.Status),
		zap.String("operation_type", log.OperationType),
		zap.Any("rsp_data", log.RspData))
}
