// 文件用途：承载上行消息模块的 response 处理逻辑。
// 核心逻辑：从 MQTT 或内部总线接收遥测、属性、事件、状态和响应消息并分发到处理、存储或通知链路，主要围绕 type ResponseUplink、type ResponseUplinkConfig、func NewResponseUplink、func (f *ResponseUplink) Start 等声明展开。
// 关键注意事项：上行链路包含 goroutine、缓存和外部服务调用，修改需关注并发关闭和消息类型兼容。
// 重构建议：后续可拆分通用解析、自动化触发和副作用发送逻辑，提升可测试性。

package uplink

import (
	"context"
	"encoding/json"
	"time"

	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
)

// ResponseUplink 响应流处理器
// 负责处理命令和属性设置的响应消息，更新日志表
type ResponseUplink struct {
	logger *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// ResponseUplinkConfig ResponseUplink 配置
type ResponseUplinkConfig struct {
	Logger *logrus.Logger
}

// NewResponseUplink 创建响应流处理器
func NewResponseUplink(config ResponseUplinkConfig) *ResponseUplink {
	ctx, cancel := context.WithCancel(context.Background())

	if config.Logger == nil {
		config.Logger = logrus.StandardLogger()
	}

	return &ResponseUplink{
		logger: config.Logger,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
}

// Start 启动响应流处理
func (f *ResponseUplink) Start(input <-chan *DeviceMessage) error {
	f.logger.Info("ResponseUplink starting...")

	go func() {
		defer close(f.done)
		f.logger.Info("ResponseUplink message loop started")
		for {
			select {
			case <-f.ctx.Done():
				f.logger.Info("ResponseUplink stopped")
				return
			case msg, ok := <-input:
				if !ok {
					f.logger.Info("ResponseUplink message channel closed")
					return
				}
				if msg == nil {
					f.logger.Warn("Received nil response message, skipping")
					continue
				}
				f.logger.WithFields(logrus.Fields{
					"device_id":  msg.DeviceID,
					"type":       msg.Type,
					"message_id": msg.Metadata["message_id"],
				}).Info("ResponseUplink received message")
				f.processMessage(msg)
			}
		}
	}()

	f.logger.Info("ResponseUplink started successfully")
	return nil
}

// Stop 停止响应流处理
func (f *ResponseUplink) Stop() error {
	f.cancel()
	return nil
}

func (f *ResponseUplink) Done() <-chan struct{} {
	return f.done
}

// processMessage 处理响应消息
func (f *ResponseUplink) processMessage(msg *DeviceMessage) {
	// 1. 提取 message_id
	messageID, ok := msg.Metadata["message_id"].(string)
	if !ok || messageID == "" {
		f.logger.WithField("metadata", msg.Metadata).Error("Missing message_id in metadata")
		return
	}

	// 2. 解析响应数据
	responseData, success := f.parseResponse(msg.Payload)
	rawPayload := string(msg.Payload)

	// 3. 根据消息类型更新对应日志表
	switch msg.Type {
	case MessageTypeCommandResponse, MessageTypeGatewayCommandResponse:
		f.updateCommandLog(msg.DeviceID, messageID, success, responseData, rawPayload)

	case MessageTypeAttributeSetResponse, MessageTypeGatewayAttributeSetResponse:
		f.updateAttributeLog(messageID, success, responseData)

	default:
		f.logger.WithField("type", msg.Type).Warn("Unknown response type")
	}
}

// parseResponse 解析响应数据
// 返回: (错误信息, 是否成功)
// 响应格式: {"result":0,"message":"success","ts":1609143039,"errcode":"","method":""}
// result: 0-成功 1-失败
func (f *ResponseUplink) parseResponse(payload []byte) (string, bool) {
	// 尝试解析响应格式
	var response struct {
		Result  int    `json:"result"`  // 0-成功 1-失败
		Message string `json:"message"` // 消息内容
		Errcode string `json:"errcode"` // 错误码（可选）
		Ts      int64  `json:"ts"`      // 时间戳（可选）
		Method  string `json:"method"`  // 事件和命令的方法（可选）
	}

	if err := json.Unmarshal(payload, &response); err != nil {
		// 解析失败，认为是原始响应数据，记录错误
		f.logger.WithError(err).WithField("payload", string(payload)).Warn("Failed to parse response as JSON")
		return string(payload), false // 解析失败视为失败
	}

	// 判断成功/失败: result 为 0 表示成功
	success := response.Result == 0

	errorMsg := ""
	if !success {
		// 失败时，优先使用 errcode，其次 message
		if response.Errcode != "" {
			errorMsg = response.Errcode
		} else if response.Message != "" {
			errorMsg = response.Message
		} else {
			errorMsg = string(payload)
		}
	}

	return errorMsg, success
}

// updateCommandLog 更新命令日志
func (f *ResponseUplink) updateCommandLog(deviceID string, messageID string, success bool, errorMsg string, rawPayload string) {
	// 状态: 3=成功, 4=失败
	status := "3" // 成功
	var errorMsgPtr *string

	if !success {
		status = "4" // 失败
		errorMsgPtr = &errorMsg
	}

	// 更新日志
	result, err := query.CommandSetLog.
		Where(query.CommandSetLog.MessageID.Eq(messageID)).
		Where(query.CommandSetLog.DeviceID.Eq(deviceID)).
		Updates(map[string]interface{}{
			"status":        &status,
			"error_message": errorMsgPtr,
			"rsp_data":      rawPayload,
		})

	if err != nil {
		f.logger.WithError(err).WithField("message_id", messageID).Error("Failed to update command log")
		return
	}

	if result.RowsAffected == 0 {
		f.logger.WithField("message_id", messageID).Warn("Command log not found")
		return
	}

	responseAt := time.Now().UTC()
	if err := service.RecordFleetCommandJobDeviceResponse(deviceID, messageID, status, rawPayload, errorMsg, responseAt); err != nil {
		f.logger.WithError(err).WithFields(logrus.Fields{
			"device_id":  deviceID,
			"message_id": messageID,
		}).Error("Failed to update command job detail response evidence")
	}

	f.logger.WithFields(logrus.Fields{
		"message_id": messageID,
		"status":     status,
		"success":    success,
	}).Info("Command log updated")
}

// updateAttributeLog 更新属性设置日志
func (f *ResponseUplink) updateAttributeLog(messageID string, success bool, errorMsg string) {
	// 状态: 3=成功, 4=失败
	status := "3" // 成功
	var errorMsgPtr *string

	if !success {
		status = "4" // 失败
		errorMsgPtr = &errorMsg
	}

	// 更新日志
	result, err := query.AttributeSetLog.
		Where(query.AttributeSetLog.MessageID.Eq(messageID)).
		Updates(map[string]interface{}{
			"status":        &status,
			"error_message": errorMsgPtr,
		})

	if err != nil {
		f.logger.WithError(err).WithField("message_id", messageID).Error("Failed to update attribute log")
		return
	}

	if result.RowsAffected == 0 {
		f.logger.WithField("message_id", messageID).Warn("Attribute log not found")
		return
	}

	f.logger.WithFields(logrus.Fields{
		"message_id": messageID,
		"status":     status,
		"success":    success,
	}).Info("Attribute log updated")
}
