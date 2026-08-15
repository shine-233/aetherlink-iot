//go:build external_integration

// 文件用途：验证平台向网关拓扑各层级下发属性设置的闭环。
// 核心逻辑：分别向顶层网关、子设备、子网关和子网关子设备下发属性设置，网关响应并查询设置日志。
// 关键注意事项：用例依赖网关下行 payload 嵌套结构和日志 data 内容，拓扑缺失时部分子用例不会执行。
// 重构建议：可把四类目标层级抽成表驱动用例，并统一 message_id、日志状态和 data 包含关系断言。

/*
Purpose: 验证平台向网关拓扑各层级下发属性设置的闭环。
Core logic: 分别向顶层网关、子设备、子网关和子网关子设备下发属性设置，网关接收消息、回响应并查询设置日志。
Important notes: 用例依赖网关下行 payload 的嵌套结构和日志 data 包含设备编号，拓扑缺失时部分子用例不会执行。
Refactor suggestion: 可把四类目标层级抽成表驱动用例，并统一 message_id、日志状态和 data 包含关系断言。
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

func TestGatewayAttributeSet(t *testing.T) {
	requireExternalIntegration(t)
	// 初始化日志。
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	// 加载网关测试配置。
	cfg, err := config.Load("../../config-gateway-community.yaml")
	require.NoError(t, err)

	// 确认当前配置使用网关设备类型。
	require.Equal(t, "gateway", cfg.DeviceType, "Test requires gateway device type")

	// 创建并连接网关设备。
	dev, err := device.NewDevice(cfg, logger)
	require.NoError(t, err)
	require.NoError(t, dev.Connect())
	defer dev.Disconnect()

	// 订阅网关相关主题。
	require.NoError(t, dev.SubscribeAll())

	// 创建 API 客户端。
	apiClient := platform.NewAPIClient(&cfg.API, logger)

	// 创建数据库客户端。
	dbClient, err := platform.NewDBClient(&cfg.Database, logger)
	require.NoError(t, err)
	defer dbClient.Close()

	// 获取网关 MQTT 主题构建器。
	topics := utils.NewGatewayMQTTTopics(cfg.Device.DeviceNumber)

	logger.Info("Starting gateway attribute set test",
		zap.String("gateway_number", cfg.Device.DeviceNumber))

	// 测试 1：向顶层网关下发属性设置。
	t.Run("AttributeSet_Gateway_Self", func(t *testing.T) {
		dev.ClearReceivedMessages(topics.AttributeSet())

		attributeData := map[string]interface{}{
			"device_name": "Gateway-001",
			"location":    "Building A",
			"firmware":    "v2.1.0",
		}

		logger.Info("Sending attribute set to gateway itself",
			zap.String("device_id", cfg.Device.DeviceID),
			zap.Any("attribute_data", attributeData))

		// 下发属性设置指令。
		err = apiClient.PublishAttributeSet(cfg.Device.DeviceID, attributeData)
		require.NoError(t, err)

		// 等待设备接收。
		timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
		messages := dev.GetReceivedMessages(topics.AttributeSet(), timeout)
		require.NotEmpty(t, messages, "Gateway did not receive attribute set message")

		var receivedMessageID string
		// 验证收到的消息。
		var receivedData map[string]interface{}
		err := json.Unmarshal(messages[0].Payload, &receivedData)
		require.NoError(t, err)

		logger.Info("Gateway received attribute set message",
			zap.Any("received_data", receivedData))

		// 从 topic 中提取 message_id。
		// Topic 格式：gateway/attributes/set/device_number_666666/6ef2ba79。
		topicParts := strings.Split(messages[0].Topic, "/")
		if len(topicParts) > 0 {
			receivedMessageID = topicParts[len(topicParts)-1]
			logger.Info("Extracted message_id from topic",
				zap.String("message_id", receivedMessageID),
				zap.String("topic", messages[0].Topic))
		}
		require.NotEmpty(t, receivedMessageID, "Gateway attribute set topic must carry a message id")

		// 验证 gateway_data；缺失或类型错误时必须失败，不能让 payload 分支静默通过。
		gatewayData, ok := receivedData["gateway_data"].(map[string]interface{})
		require.True(t, ok, "Attribute set payload must contain gateway_data object")
		for key, expectedValue := range attributeData {
			actualValue, exists := gatewayData[key]
			require.True(t, exists, "Attribute key not found in gateway_data: %s", key)
			assert.Equal(t, expectedValue, actualValue, "Attribute value mismatch for key: %s", key)
		}

		// 发送响应。
		if receivedMessageID != "" {
			err = dev.PublishAttributeSetResponse(receivedMessageID, true)
			require.NoError(t, err)
			logger.Info("Attribute set response sent", zap.String("message_id", receivedMessageID))
		}

		// 验证属性设置日志。
		if receivedMessageID != "" {
			time.Sleep(2 * time.Second)
			log, err := dbClient.QueryAttributeSetLogs(cfg.Device.DeviceID, receivedMessageID)
			require.NoError(t, err)
			require.NotNil(t, log, "No attribute set log found for gateway")

			if log != nil {
				// 状态：1=下发成功，3=响应成功。设备已回复响应，故状态应为已下发或已响应之一。
				assert.Contains(t, []string{"1", "3"}, log.Status,
					"Gateway attribute set status should be '1' (dispatched) or '3' (responded)")
				logger.Info("Gateway attribute set log verified",
					zap.String("status", log.Status),
					zap.String("data", log.Data),
					zap.Any("rsp_data", log.RspData))
			}
		}
	})

	// 测试 2：向子设备下发属性设置。
	if len(cfg.Gateway.SubDevices) > 0 {
		t.Run("AttributeSet_SubDevice", func(t *testing.T) {
			subDev := cfg.Gateway.SubDevices[0]
			dev.ClearReceivedMessages(topics.AttributeSet())

			attributeData := map[string]interface{}{
				"sensor_type": "temperature",
				"unit":        "celsius",
				"range":       float64(100),
			}

			logger.Info("Sending attribute set to sub-device",
				zap.String("sub_device_id", subDev.DeviceID),
				zap.String("sub_device_number", subDev.SubDeviceNumber),
				zap.Any("attribute_data", attributeData))

			// 向子设备下发属性设置指令。
			err = apiClient.PublishAttributeSet(subDev.DeviceID, attributeData)
			require.NoError(t, err)

			// 等待网关接收。
			timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
			messages := dev.GetReceivedMessages(topics.AttributeSet(), timeout)
			require.NotEmpty(t, messages, "Gateway did not receive attribute set message for sub-device")

			var receivedMessageID string
			var receivedData map[string]interface{}
			err := json.Unmarshal(messages[0].Payload, &receivedData)
			require.NoError(t, err)

			logger.Info("Gateway received attribute set message for sub-device",
				zap.Any("received_data", receivedData))

			// 从 topic 提取 message_id。
			topicParts := strings.Split(messages[0].Topic, "/")
			if len(topicParts) > 0 {
				receivedMessageID = topicParts[len(topicParts)-1]
			}
			require.NotEmpty(t, receivedMessageID, "Sub-device attribute set topic must carry a message id")

			// 验证 sub_device_data；缺失、目标设备缺失或类型错误均必须失败。
			subDeviceData, ok := receivedData["sub_device_data"].(map[string]interface{})
			require.True(t, ok, "Attribute set payload must contain sub_device_data object")
			subDevAttr, exists := subDeviceData[subDev.SubDeviceNumber]
			require.True(t, exists, "Sub-device attribute data not found for: %s", subDev.SubDeviceNumber)
			subDevAttrMap, ok := subDevAttr.(map[string]interface{})
			require.True(t, ok, "Sub-device attribute payload must be an object")
			for key, expectedValue := range attributeData {
				actualValue, keyExists := subDevAttrMap[key]
				require.True(t, keyExists, "Attribute key not found in sub-device data: %s", key)
				assert.Equal(t, expectedValue, actualValue, "Sub-device attribute value mismatch for key: %s", key)
			}

			// 发送响应。
			if receivedMessageID != "" {
				err = dev.PublishAttributeSetResponse(receivedMessageID, true)
				require.NoError(t, err)
			}

			// 验证日志。
			if receivedMessageID != "" {
				time.Sleep(2 * time.Second)
				log, err := dbClient.QueryAttributeSetLogs(subDev.DeviceID, receivedMessageID)
				require.NoError(t, err)
				require.NotNil(t, log, "No attribute set log found for sub-device")

				if log != nil {
					assert.NotEmpty(t, log.Data, "Sub-device attribute set log data should not be empty")
					assert.Contains(t, log.Data, subDev.SubDeviceNumber,
						"Log data should contain sub-device number: %s", subDev.SubDeviceNumber)

					logger.Info("Sub-device attribute set log verified",
						zap.String("sub_device_number", subDev.SubDeviceNumber),
						zap.String("status", log.Status),
						zap.String("data", log.Data))
				}
			}
		})
	}

	// 测试 3：向子网关下发属性设置。
	if len(cfg.Gateway.SubGateways) > 0 {
		t.Run("AttributeSet_SubGateway", func(t *testing.T) {
			subGw := cfg.Gateway.SubGateways[0]
			dev.ClearReceivedMessages(topics.AttributeSet())

			attributeData := map[string]interface{}{
				"gateway_type": "edge",
				"protocol":     "mqtt",
				"max_devices":  float64(50),
			}

			logger.Info("Sending attribute set to sub-gateway",
				zap.String("sub_gateway_id", subGw.DeviceID),
				zap.String("sub_gateway_number", subGw.SubGatewayNumber),
				zap.Any("attribute_data", attributeData))

			// 向子网关下发属性设置指令。
			err = apiClient.PublishAttributeSet(subGw.DeviceID, attributeData)
			require.NoError(t, err)

			// 等待网关接收。
			timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
			messages := dev.GetReceivedMessages(topics.AttributeSet(), timeout)
			require.NotEmpty(t, messages, "Gateway did not receive attribute set message for sub-gateway")

			var receivedMessageID string
			var receivedData map[string]interface{}
			err := json.Unmarshal(messages[0].Payload, &receivedData)
			require.NoError(t, err)

			logger.Info("Gateway received attribute set message for sub-gateway",
				zap.Any("received_data", receivedData))

			// 从 topic 提取 message_id。
			topicParts := strings.Split(messages[0].Topic, "/")
			if len(topicParts) > 0 {
				receivedMessageID = topicParts[len(topicParts)-1]
			}
			require.NotEmpty(t, receivedMessageID, "Sub-gateway attribute set topic must carry a message id")

			// 验证 sub_gateway_data 及其 gateway_data 内容；缺失时必须失败。
			subGatewayData, ok := receivedData["sub_gateway_data"].(map[string]interface{})
			require.True(t, ok, "Attribute set payload must contain sub_gateway_data object")
			subGwAttr, exists := subGatewayData[subGw.SubGatewayNumber]
			require.True(t, exists, "Sub-gateway attribute data not found for: %s", subGw.SubGatewayNumber)
			subGwAttrMap, ok := subGwAttr.(map[string]interface{})
			require.True(t, ok, "Sub-gateway attribute payload must be an object")
			gatewayData, ok := subGwAttrMap["gateway_data"].(map[string]interface{})
			require.True(t, ok, "Sub-gateway attribute payload must contain gateway_data object")
			for key, expectedValue := range attributeData {
				actualValue, keyExists := gatewayData[key]
				require.True(t, keyExists, "Attribute key not found in sub-gateway data: %s", key)
				assert.Equal(t, expectedValue, actualValue, "Sub-gateway attribute value mismatch for key: %s", key)
			}

			// 发送响应。
			if receivedMessageID != "" {
				err = dev.PublishAttributeSetResponse(receivedMessageID, true)
				require.NoError(t, err)
			}

			// 验证日志。
			if receivedMessageID != "" {
				time.Sleep(2 * time.Second)
				log, err := dbClient.QueryAttributeSetLogs(subGw.DeviceID, receivedMessageID)
				require.NoError(t, err)
				require.NotNil(t, log, "No attribute set log found for sub-gateway")

				if log != nil {
					assert.NotEmpty(t, log.Data, "Sub-gateway attribute set log data should not be empty")
					assert.Contains(t, log.Data, subGw.SubGatewayNumber,
						"Log data should contain sub-gateway number: %s", subGw.SubGatewayNumber)

					logger.Info("Sub-gateway attribute set log verified",
						zap.String("sub_gateway_number", subGw.SubGatewayNumber),
						zap.String("status", log.Status),
						zap.String("data", log.Data))
				}
			}
		})
	}

	// 测试 4：向子网关下的子设备下发属性设置。
	if len(cfg.Gateway.SubGateways) > 0 && len(cfg.Gateway.SubGateways[0].SubDevices) > 0 {
		t.Run("AttributeSet_SubGateway_SubDevice", func(t *testing.T) {
			subGw := cfg.Gateway.SubGateways[0]
			subGwSubDev := subGw.SubDevices[0]
			dev.ClearReceivedMessages(topics.AttributeSet())

			attributeData := map[string]interface{}{
				"sensor_model": "TH-100",
				"calibrated":   true,
				"interval":     float64(60),
			}

			logger.Info("Sending attribute set to sub-gateway's sub-device",
				zap.String("device_id", subGwSubDev.DeviceID),
				zap.String("sub_device_number", subGwSubDev.SubDeviceNumber),
				zap.String("parent_sub_gateway", subGw.SubGatewayNumber),
				zap.Any("attribute_data", attributeData))

			// 向子网关的子设备下发属性设置指令。
			err = apiClient.PublishAttributeSet(subGwSubDev.DeviceID, attributeData)
			require.NoError(t, err)

			// 等待网关接收。
			timeout := time.Duration(cfg.Test.WaitMQTTResponseSeconds) * time.Second
			messages := dev.GetReceivedMessages(topics.AttributeSet(), timeout)
			require.NotEmpty(t, messages, "Gateway did not receive attribute set message for sub-gateway's sub-device")

			var receivedMessageID string
			var receivedData map[string]interface{}
			err := json.Unmarshal(messages[0].Payload, &receivedData)
			require.NoError(t, err)

			logger.Info("Gateway received attribute set message for sub-gateway's sub-device",
				zap.Any("received_data", receivedData))

			// 从 topic 提取 message_id。
			topicParts := strings.Split(messages[0].Topic, "/")
			if len(topicParts) > 0 {
				receivedMessageID = topicParts[len(topicParts)-1]
			}
			require.NotEmpty(t, receivedMessageID, "Sub-gateway sub-device attribute set topic must carry a message id")

			// 验证最深层 sub_gateway_data/sub_device_data；此前仅取 message_id，payload 错误会被静默放行。
			subGatewayData, ok := receivedData["sub_gateway_data"].(map[string]interface{})
			require.True(t, ok, "Attribute set payload must contain sub_gateway_data object")
			subGwData, exists := subGatewayData[subGw.SubGatewayNumber]
			require.True(t, exists, "Sub-gateway attribute data not found: %s", subGw.SubGatewayNumber)
			subGwDataMap, ok := subGwData.(map[string]interface{})
			require.True(t, ok, "Sub-gateway attribute payload must be an object")
			subDeviceData, ok := subGwDataMap["sub_device_data"].(map[string]interface{})
			require.True(t, ok, "Sub-gateway attribute payload must contain sub_device_data object")
			subDevAttr, exists := subDeviceData[subGwSubDev.SubDeviceNumber]
			require.True(t, exists, "Sub-device attribute data not found: %s", subGwSubDev.SubDeviceNumber)
			subDevAttrMap, ok := subDevAttr.(map[string]interface{})
			require.True(t, ok, "Sub-device attribute payload must be an object")
			for key, expectedValue := range attributeData {
				actualValue, keyExists := subDevAttrMap[key]
				require.True(t, keyExists, "Attribute key not found in sub-device data: %s", key)
				assert.Equal(t, expectedValue, actualValue, "Sub-device attribute value mismatch for key: %s", key)
			}

			// 发送响应。
			if receivedMessageID != "" {
				err = dev.PublishAttributeSetResponse(receivedMessageID, true)
				require.NoError(t, err)
			}

			// 验证日志。
			if receivedMessageID != "" {
				time.Sleep(2 * time.Second)
				log, err := dbClient.QueryAttributeSetLogs(subGwSubDev.DeviceID, receivedMessageID)
				require.NoError(t, err)
				require.NotNil(t, log, "No attribute set log found for sub-gateway's sub-device")

				if log != nil {
					assert.NotEmpty(t, log.Data, "Sub-gateway's sub-device attribute set log data should not be empty")
					assert.Contains(t, log.Data, subGw.SubGatewayNumber,
						"Log data should contain sub-gateway number: %s", subGw.SubGatewayNumber)
					assert.Contains(t, log.Data, subGwSubDev.SubDeviceNumber,
						"Log data should contain sub-device number: %s", subGwSubDev.SubDeviceNumber)

					logger.Info("Sub-gateway's sub-device attribute set log verified",
						zap.String("sub_device_number", subGwSubDev.SubDeviceNumber),
						zap.String("parent_sub_gateway", subGw.SubGatewayNumber),
						zap.String("status", log.Status),
						zap.String("data", log.Data))
				}
			}
		})
	}

	logger.Info("Gateway attribute set test completed successfully")
}
