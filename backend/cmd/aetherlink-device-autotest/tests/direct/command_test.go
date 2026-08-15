//go:build external_integration

// 文件用途：验证直连设备接收平台命令并回传命令响应的闭环。
// 核心逻辑：通过 API 下发命令，设备订阅 command topic，提取 message_id 后发布响应并查询命令日志。
// 关键注意事项：该用例依赖真实 API、MQTT broker 和 PostgreSQL，日志状态值按当前平台约定断言。
// 重构建议：可把 topic 解析、响应发送和日志轮询抽成共享 helper，并覆盖失败响应路径。

/*
Purpose: 验证直连设备接收平台命令并回传命令响应的闭环。
Core logic: 通过 API 下发命令，设备订阅 command topic，校验 method/params，提取 message_id 后发布响应并查询命令日志。
Important notes: 该测试依赖 topic 尾段作为 message_id，且日志状态值按当前平台约定断言为响应成功。
Refactor suggestion: 可把 topic 解析、响应发送和日志轮询抽成共享 helper，并覆盖失败响应路径。
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

func TestCommandPublish(t *testing.T) {
	requireExternalIntegration(t)
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	cfg, err := config.Load("../../config-community.yaml")
	require.NoError(t, err)

	dev, err := device.NewDevice(cfg, logger)
	require.NoError(t, err)
	require.NoError(t, dev.Connect())
	defer dev.Disconnect()

	// 订阅设备相关 topic，确保能接收平台命令。
	require.NoError(t, dev.SubscribeAll())

	apiClient := platform.NewAPIClient(&cfg.API, logger)
	dbClient, err := platform.NewDBClient(&cfg.Database, logger)
	require.NoError(t, err)
	defer dbClient.Close()

	topics := utils.NewMQTTTopics(cfg.Device.DeviceNumber)
	dev.ClearReceivedMessages("")

	// 构造命令下发数据。
	identify := "RestartDevice"
	commandData := map[string]interface{}{
		"delay_seconds": float64(5), // 使用 float64 以匹配 JSON 数值解码结果
		"mode":          "safe",
	}

	// 通过平台 API 下发命令。
	err = apiClient.PublishCommand(cfg.Device.DeviceID, identify, commandData)
	require.NoError(t, err)

	// 等待设备收到命令消息。
	timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
	messages := dev.GetReceivedMessages(topics.Command(), timeout)
	require.NotEmpty(t, messages, "Device did not receive command")

	receivedMsg := messages[0]

	var receivedCmd map[string]interface{}
	err = json.Unmarshal(receivedMsg.Payload, &receivedCmd)
	require.NoError(t, err)

	// 校验命令方法名。
	assert.Equal(t, identify, receivedCmd["method"], "Command method mismatch")

	params, ok := receivedCmd["params"].(map[string]interface{})
	require.True(t, ok, "Command params not found")

	for key, expectedValue := range commandData {
		actualValue := params[key]
		assert.Equal(t, expectedValue, actualValue, "Command param mismatch for key: %s", key)
	}

	// 从 topic 尾段提取 message_id。
	topicParts := strings.Split(receivedMsg.Topic, "/")
	messageID := topicParts[len(topicParts)-1]

	logger.Info("Extracted message_id from topic",
		zap.String("topic", receivedMsg.Topic),
		zap.String("message_id", messageID))

	// 使用提取到的 message_id 发布命令响应。
	err = dev.PublishCommandResponse(messageID, true, identify)
	require.NoError(t, err)

	logger.Info("Command response sent",
		zap.String("identify", identify),
		zap.String("message_id", messageID))

	// 等待响应数据写入平台日志。
	time.Sleep(3 * time.Second)

	// 查询命令日志，重试等待响应数据落库。
	var log *platform.CommandSetLog
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		log, err = dbClient.QueryCommandSetLogs(cfg.Device.DeviceID, messageID)
		require.NoError(t, err)

		if log != nil && log.RspData != nil {
			break
		}

		if i < maxRetries-1 {
			logger.Info("Response data not yet recorded, retrying...",
				zap.Int("attempt", i+1),
				zap.Int("max_retries", maxRetries))
			time.Sleep(2 * time.Second)
		}
	}

	require.NotNil(t, log, "Command set log not found")
	require.NotNil(t, log.MessageID, "Command set log message ID should be recorded")
	require.NotNil(t, log.Identify, "Command set log identify should be recorded")

	assert.Equal(t, cfg.Device.DeviceID, log.DeviceID, "Device ID mismatch")
	assert.Equal(t, messageID, *log.MessageID, "Message ID should match")
	assert.Equal(t, identify, *log.Identify, "Command identify should match")

	// 校验响应成功状态。
	assert.Equal(t, "3", log.Status, "Status should be '3' (response success)")

	// 检查数据库中的响应数据是否为空。
	require.NotNil(t, log.RspData, "Response data should be recorded after command response")
	logger.Info("Response data recorded successfully",
		zap.String("rsp_data", *log.RspData))

	assert.Equal(t, "1", log.OperationType, "Operation type should be '1' (manual)")

	logger.Info("Command log verified",
		zap.String("log_id", log.ID),
		zap.String("message_id", messageID),
		zap.String("identify", identify),
		zap.String("status", log.Status),
		zap.String("operation_type", log.OperationType))
}
