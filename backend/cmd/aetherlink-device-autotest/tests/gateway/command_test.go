//go:build external_integration

// 文件用途：验证平台向网关拓扑各层级下发命令的闭环。
// 核心逻辑：分别向顶层网关、子设备、子网关和子网关子设备下发命令，网关响应并查询命令日志。
// 关键注意事项：同时兼容扁平命令和 gateway_data 包装命令，日志断言依赖当前平台状态码与 data 内容约定。
// 重构建议：可与属性设置测试共享层级目标表和响应日志轮询 helper，并补充失败响应场景。

/*
Purpose: 验证平台向网关拓扑各层级下发命令的闭环。
Core logic: 分别向顶层网关、子设备、子网关和子网关子设备下发命令，网关接收消息、回响应并查询命令日志。
Important notes: 同时兼容扁平命令和 gateway_data 包装命令；日志断言依赖当前平台状态码与 data 内容约定。
Refactor suggestion: 可与属性设置测试共享层级目标表和响应日志轮询 helper，并补失败响应场景。
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

func TestGatewayCommand(t *testing.T) {
	requireExternalIntegration(t)
	// 初始化日志
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	// 加载配置
	cfg, err := config.Load("../../config-gateway-community.yaml")
	require.NoError(t, err)

	// 验证设备类型为网关
	require.Equal(t, "gateway", cfg.DeviceType, "Test requires gateway device type")

	// 创建网关设备
	dev, err := device.NewDevice(cfg, logger)
	require.NoError(t, err)
	require.NoError(t, dev.Connect())
	defer dev.Disconnect()

	// 订阅主题
	require.NoError(t, dev.SubscribeAll())

	// 创建 API 客户端
	apiClient := platform.NewAPIClient(&cfg.API, logger)

	// 创建数据库客户端
	dbClient, err := platform.NewDBClient(&cfg.Database, logger)
	require.NoError(t, err)
	defer dbClient.Close()

	// 获取主题构建器
	topics := utils.NewGatewayMQTTTopics(cfg.Device.DeviceNumber)

	logger.Info("Starting gateway command test",
		zap.String("gateway_number", cfg.Device.DeviceNumber))

	// 测试1：给顶层网关下发命令
	t.Run("Command_Gateway_Self", func(t *testing.T) {
		dev.ClearReceivedMessages(topics.Command())

		identify := "RestartGateway"
		commandData := map[string]interface{}{
			"delay_seconds": float64(5),
			"mode":          "safe",
		}

		logger.Info("Sending command to gateway itself",
			zap.String("device_id", cfg.Device.DeviceID),
			zap.String("identify", identify),
			zap.Any("command_data", commandData))

		// 下发命令
		err = apiClient.PublishCommand(cfg.Device.DeviceID, identify, commandData)
		require.NoError(t, err)

		// 等待设备接收
		timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
		messages := dev.GetReceivedMessages(topics.Command(), timeout)
		require.NotEmpty(t, messages, "Gateway did not receive command")

		var receivedMessageID string
		var receivedCmd map[string]interface{}
		err := json.Unmarshal(messages[0].Payload, &receivedCmd)
		require.NoError(t, err)

		logger.Info("Gateway received command",
			zap.Any("received_command", receivedCmd))

		// 从 topic 中提取 message_id
		topicParts := strings.Split(messages[0].Topic, "/")
		if len(topicParts) > 0 {
			receivedMessageID = topicParts[len(topicParts)-1]
			logger.Info("Extracted message_id from topic",
				zap.String("message_id", receivedMessageID),
				zap.String("topic", messages[0].Topic))
		}
		require.NotEmpty(t, receivedMessageID, "Gateway command topic must carry a message id")

		// 验证命令内容，兼容 gateway_data 包装
		if gatewayData, ok := receivedCmd["gateway_data"].(map[string]interface{}); ok {
			// 嵌套格式
			assert.Equal(t, identify, gatewayData["method"], "Command method mismatch")
			if params, ok := gatewayData["params"].(map[string]interface{}); ok {
				for key, expectedValue := range commandData {
					actualValue := params[key]
					assert.Equal(t, expectedValue, actualValue, "Command param mismatch for key: %s", key)
				}
			}
		} else {
			// 扁平格式
			assert.Equal(t, identify, receivedCmd["method"], "Command method mismatch")
			if params, ok := receivedCmd["params"].(map[string]interface{}); ok {
				for key, expectedValue := range commandData {
					actualValue := params[key]
					assert.Equal(t, expectedValue, actualValue, "Command param mismatch for key: %s", key)
				}
			}
		}

		// 发送响应
		if receivedMessageID != "" {
			err = dev.PublishCommandResponse(receivedMessageID, true, identify)
			require.NoError(t, err)
			logger.Info("Command response sent", zap.String("message_id", receivedMessageID))
		}

		// 验证命令日志
		if receivedMessageID != "" {
			time.Sleep(2 * time.Second)
			log, err := dbClient.QueryCommandSetLogs(cfg.Device.DeviceID, receivedMessageID)
			require.NoError(t, err)
			require.NotNil(t, log, "No command set log found for gateway")

			if log != nil {
				assert.Equal(t, identify, *log.Identify, "Command identify should match")
				logger.Info("Gateway command log verified",
					zap.String("identify", identify),
					zap.String("status", log.Status),
					zap.String("data", log.Data))
			}
		}
	})

	// 测试2：给子设备下发命令
	if len(cfg.Gateway.SubDevices) > 0 {
		t.Run("Command_SubDevice", func(t *testing.T) {
			subDev := cfg.Gateway.SubDevices[0]
			dev.ClearReceivedMessages(topics.Command())

			identify := "ReadSensor"
			commandData := map[string]interface{}{
				"type":     "temperature",
				"interval": float64(10),
			}

			logger.Info("Sending command to sub-device",
				zap.String("sub_device_id", subDev.DeviceID),
				zap.String("sub_device_number", subDev.SubDeviceNumber),
				zap.String("identify", identify),
				zap.Any("command_data", commandData))

			// 下发命令到子设备
			err = apiClient.PublishCommand(subDev.DeviceID, identify, commandData)
			require.NoError(t, err)

			// 等待网关接收
			timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
			messages := dev.GetReceivedMessages(topics.Command(), timeout)
			require.NotEmpty(t, messages, "Gateway did not receive command for sub-device")

			var receivedMessageID string
			var receivedCmd map[string]interface{}
			err := json.Unmarshal(messages[0].Payload, &receivedCmd)
			require.NoError(t, err)

			logger.Info("Gateway received command for sub-device",
				zap.Any("received_command", receivedCmd))

			// 从 topic 提取 message_id
			topicParts := strings.Split(messages[0].Topic, "/")
			if len(topicParts) > 0 {
				receivedMessageID = topicParts[len(topicParts)-1]
			}
			require.NotEmpty(t, receivedMessageID, "Sub-device command topic must carry a message id")

			// 验证 sub_device_data
			if subDeviceData, ok := receivedCmd["sub_device_data"].(map[string]interface{}); ok {
				subDevCmd, exists := subDeviceData[subDev.SubDeviceNumber]
				assert.True(t, exists, "Sub-device command not found for: %s", subDev.SubDeviceNumber)

				if exists {
					if subDevCmdMap, ok := subDevCmd.(map[string]interface{}); ok {
						assert.Equal(t, identify, subDevCmdMap["method"], "Command method mismatch")
					}
				}
			}

			// 发送响应
			if receivedMessageID != "" {
				err = dev.PublishCommandResponse(receivedMessageID, true, identify)
				require.NoError(t, err)
			}

			// 验证日志
			if receivedMessageID != "" {
				time.Sleep(2 * time.Second)
				log, err := dbClient.QueryCommandSetLogs(subDev.DeviceID, receivedMessageID)
				require.NoError(t, err)
				require.NotNil(t, log, "No command set log found for sub-device")

				if log != nil {
					assert.Equal(t, identify, *log.Identify, "Command identify should match")
					assert.NotEmpty(t, log.Data, "Sub-device command log data should not be empty")
					assert.Contains(t, log.Data, subDev.SubDeviceNumber,
						"Log data should contain sub-device number: %s", subDev.SubDeviceNumber)

					logger.Info("Sub-device command log verified",
						zap.String("sub_device_number", subDev.SubDeviceNumber),
						zap.String("identify", identify),
						zap.String("status", log.Status),
						zap.String("data", log.Data))
				}
			}
		})
	}

	// 测试3：给子网关下发命令
	if len(cfg.Gateway.SubGateways) > 0 {
		t.Run("Command_SubGateway", func(t *testing.T) {
			subGw := cfg.Gateway.SubGateways[0]
			dev.ClearReceivedMessages(topics.Command())

			identify := "SyncConfig"
			commandData := map[string]interface{}{
				"config_type": "network",
				"force":       true,
			}

			logger.Info("Sending command to sub-gateway",
				zap.String("sub_gateway_id", subGw.DeviceID),
				zap.String("sub_gateway_number", subGw.SubGatewayNumber),
				zap.String("identify", identify),
				zap.Any("command_data", commandData))

			// 下发命令到子网关
			err = apiClient.PublishCommand(subGw.DeviceID, identify, commandData)
			require.NoError(t, err)

			// 等待网关接收
			timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
			messages := dev.GetReceivedMessages(topics.Command(), timeout)
			require.NotEmpty(t, messages, "Gateway did not receive command for sub-gateway")

			var receivedMessageID string
			var receivedCmd map[string]interface{}
			err := json.Unmarshal(messages[0].Payload, &receivedCmd)
			require.NoError(t, err)

			logger.Info("Gateway received command for sub-gateway",
				zap.Any("received_command", receivedCmd))

			// 从 topic 提取 message_id
			topicParts := strings.Split(messages[0].Topic, "/")
			if len(topicParts) > 0 {
				receivedMessageID = topicParts[len(topicParts)-1]
			}
			require.NotEmpty(t, receivedMessageID, "Sub-gateway command topic must carry a message id")

			// 验证 sub_gateway_data
			if subGatewayData, ok := receivedCmd["sub_gateway_data"].(map[string]interface{}); ok {
				_, exists := subGatewayData[subGw.SubGatewayNumber]
				assert.True(t, exists, "Sub-gateway command not found for: %s", subGw.SubGatewayNumber)
			}

			// 发送响应
			if receivedMessageID != "" {
				err = dev.PublishCommandResponse(receivedMessageID, true, identify)
				require.NoError(t, err)
			}

			// 验证日志
			if receivedMessageID != "" {
				time.Sleep(2 * time.Second)
				log, err := dbClient.QueryCommandSetLogs(subGw.DeviceID, receivedMessageID)
				require.NoError(t, err)
				require.NotNil(t, log, "No command set log found for sub-gateway")

				if log != nil {
					assert.Equal(t, identify, *log.Identify, "Command identify should match")
					assert.NotEmpty(t, log.Data, "Sub-gateway command log data should not be empty")
					assert.Contains(t, log.Data, subGw.SubGatewayNumber,
						"Log data should contain sub-gateway number: %s", subGw.SubGatewayNumber)

					logger.Info("Sub-gateway command log verified",
						zap.String("sub_gateway_number", subGw.SubGatewayNumber),
						zap.String("identify", identify),
						zap.String("status", log.Status),
						zap.String("data", log.Data))
				}
			}
		})
	}

	// 测试4：给子网关下的子设备下发命令
	if len(cfg.Gateway.SubGateways) > 0 && len(cfg.Gateway.SubGateways[0].SubDevices) > 0 {
		t.Run("Command_SubGateway_SubDevice", func(t *testing.T) {
			subGw := cfg.Gateway.SubGateways[0]
			subGwSubDev := subGw.SubDevices[0]
			dev.ClearReceivedMessages(topics.Command())

			identify := "UpdateFirmware"
			commandData := map[string]interface{}{
				"version": "v1.2.3",
				"url":     "http://example.com/firmware.bin",
			}

			logger.Info("Sending command to sub-gateway's sub-device",
				zap.String("device_id", subGwSubDev.DeviceID),
				zap.String("sub_device_number", subGwSubDev.SubDeviceNumber),
				zap.String("parent_sub_gateway", subGw.SubGatewayNumber),
				zap.String("identify", identify),
				zap.Any("command_data", commandData))

			// 下发命令到子网关的子设备
			err = apiClient.PublishCommand(subGwSubDev.DeviceID, identify, commandData)
			require.NoError(t, err)

			// 等待网关接收
			timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
			messages := dev.GetReceivedMessages(topics.Command(), timeout)
			require.NotEmpty(t, messages, "Gateway did not receive command for sub-gateway's sub-device")

			var receivedMessageID string
			var receivedCmd map[string]interface{}
			err := json.Unmarshal(messages[0].Payload, &receivedCmd)
			require.NoError(t, err)

			logger.Info("Gateway received command for sub-gateway's sub-device",
				zap.Any("received_command", receivedCmd))

			// 从 topic 提取 message_id
			topicParts := strings.Split(messages[0].Topic, "/")
			if len(topicParts) > 0 {
				receivedMessageID = topicParts[len(topicParts)-1]
			}
			require.NotEmpty(t, receivedMessageID, "Sub-gateway sub-device command topic must carry a message id")

			// 发送响应
			if receivedMessageID != "" {
				err = dev.PublishCommandResponse(receivedMessageID, true, identify)
				require.NoError(t, err)
			}

			// 验证日志
			if receivedMessageID != "" {
				time.Sleep(2 * time.Second)
				log, err := dbClient.QueryCommandSetLogs(subGwSubDev.DeviceID, receivedMessageID)
				require.NoError(t, err)
				require.NotNil(t, log, "No command set log found for sub-gateway's sub-device")

				if log != nil {
					assert.Equal(t, identify, *log.Identify, "Command identify should match")
					assert.NotEmpty(t, log.Data, "Sub-gateway's sub-device command log data should not be empty")
					assert.Contains(t, log.Data, subGw.SubGatewayNumber,
						"Log data should contain sub-gateway number: %s", subGw.SubGatewayNumber)
					assert.Contains(t, log.Data, subGwSubDev.SubDeviceNumber,
						"Log data should contain sub-device number: %s", subGwSubDev.SubDeviceNumber)

					logger.Info("Sub-gateway's sub-device command log verified",
						zap.String("sub_device_number", subGwSubDev.SubDeviceNumber),
						zap.String("parent_sub_gateway", subGw.SubGatewayNumber),
						zap.String("identify", identify),
						zap.String("status", log.Status),
						zap.String("data", log.Data))
				}
			}
		})
	}

	logger.Info("Gateway command test completed successfully")
}
