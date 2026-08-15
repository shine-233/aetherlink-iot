// 文件用途：覆盖下行消息模块 handler 行为的 Go 测试。
// 核心逻辑：验证总线发布订阅、处理器编解码和 MQTT 发布调用的下行契约，主要围绕 type publishCall、type mockPublisher、func (m *mockPublisher) PublishMessage、type mockProcessor 等声明展开。
// 关键注意事项：测试需保持消息类型、订阅关闭和发布错误路径的断言明确。
// 重构建议：后续可补充更多设备配置和编码失败场景，提升下行链路回归信心。

package downlink

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"aetherlink-iot/backend/internal/processor"

	"github.com/sirupsen/logrus"
)

type publishCall struct {
	deviceNumber string
	msgType      MessageType
	deviceType   string
	topicPrefix  string
	messageID    string
	qos          byte
	payload      []byte
}

type mockPublisher struct {
	calls []publishCall
	err   error
}

func (m *mockPublisher) PublishMessage(deviceNumber string, msgType MessageType, deviceType string, topicPrefix string, messageID string, qos byte, payload []byte) error {
	m.calls = append(m.calls, publishCall{
		deviceNumber: deviceNumber,
		msgType:      msgType,
		deviceType:   deviceType,
		topicPrefix:  topicPrefix,
		messageID:    messageID,
		qos:          qos,
		payload:      append([]byte(nil), payload...),
	})
	return m.err
}

type mockProcessor struct {
	inputs []processor.EncodeInput
	output *processor.EncodeOutput
	err    error
}

func (m *mockProcessor) Decode(context.Context, *processor.DecodeInput) (*processor.DecodeOutput, error) {
	return nil, errors.New("decode is not used by downlink handler")
}

func (m *mockProcessor) Encode(_ context.Context, input *processor.EncodeInput) (*processor.EncodeOutput, error) {
	m.inputs = append(m.inputs, *input)
	return m.output, m.err
}

func newHandlerTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logger
}

func TestHandlerPublishesRawDownlinkMessagesWhenNoDeviceConfigIsSet(t *testing.T) {
	publisher := &mockPublisher{}
	handler := NewHandler(publisher, &mockProcessor{}, newHandlerTestLogger())

	payload := json.RawMessage(`{"identify":"reboot","value":"{}"}`)
	msg := &Message{
		DeviceID:     "dev-1",
		DeviceNumber: "rdi-device-001",
		DeviceType:   "1",
		Type:         MessageTypeCommand,
		Data:         payload,
		TopicPrefix:  "tenant/custom/",
	}

	handler.HandleCommand(context.Background(), msg)

	if len(publisher.calls) != 1 {
		t.Fatalf("publisher calls = %d, want 1", len(publisher.calls))
	}
	call := publisher.calls[0]
	if call.deviceNumber != msg.DeviceNumber || call.msgType != MessageTypeCommand || call.deviceType != "1" {
		t.Fatalf("publisher call metadata mismatch: %+v", call)
	}
	if call.topicPrefix != "tenant/custom/" || call.qos != 1 {
		t.Fatalf("publisher call routing mismatch: %+v", call)
	}
	if string(call.payload) != string(payload) {
		t.Fatalf("publisher payload = %s, want %s", string(call.payload), string(payload))
	}
}

func TestHandlerEncodesConfiguredMessagesWithExpectedDataTypes(t *testing.T) {
	tests := []struct {
		name       string
		call       func(*Handler, context.Context, *Message)
		msgType    MessageType
		dataType   processor.DataType
		deviceData json.RawMessage
	}{
		{
			name:       "command",
			call:       (*Handler).HandleCommand,
			msgType:    MessageTypeCommand,
			dataType:   processor.DataTypeCommand,
			deviceData: json.RawMessage(`{"cmd":"reset"}`),
		},
		{
			name:       "attribute set",
			call:       (*Handler).HandleAttributeSet,
			msgType:    MessageTypeAttributeSet,
			dataType:   processor.DataTypeAttributeSet,
			deviceData: json.RawMessage(`{"mode":"auto"}`),
		},
		{
			name:       "attribute get uses attribute-set encoder",
			call:       (*Handler).HandleAttributeGet,
			msgType:    MessageTypeAttributeGet,
			dataType:   processor.DataTypeAttributeSet,
			deviceData: json.RawMessage(`{"key":"mode"}`),
		},
		{
			name:       "telemetry control",
			call:       (*Handler).HandleTelemetry,
			msgType:    MessageTypeTelemetry,
			dataType:   processor.DataTypeTelemetryControl,
			deviceData: json.RawMessage(`{"interval":30}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &mockPublisher{}
			processorMock := &mockProcessor{
				output: &processor.EncodeOutput{
					Success:     true,
					EncodedData: []byte(`{"encoded":true}`),
				},
			}
			handler := NewHandler(publisher, processorMock, newHandlerTestLogger())
			msg := &Message{
				DeviceID:       "dev-1",
				DeviceNumber:   "rdi-device-001",
				DeviceType:     "1",
				DeviceConfigID: "config-1",
				Type:           tt.msgType,
				Data:           tt.deviceData,
			}

			tt.call(handler, context.Background(), msg)

			if len(processorMock.inputs) != 1 {
				t.Fatalf("Encode calls = %d, want 1", len(processorMock.inputs))
			}
			input := processorMock.inputs[0]
			if input.DeviceConfigID != "config-1" || input.Type != tt.dataType || string(input.Data) != string(tt.deviceData) {
				t.Fatalf("Encode input = %+v data=%s, want type=%s data=%s", input, string(input.Data), tt.dataType, string(tt.deviceData))
			}
			if input.Timestamp <= 0 {
				t.Fatalf("Encode timestamp = %d, want positive", input.Timestamp)
			}
			if len(publisher.calls) != 1 {
				t.Fatalf("publisher calls = %d, want 1", len(publisher.calls))
			}
			if string(publisher.calls[0].payload) != `{"encoded":true}` {
				t.Fatalf("publisher payload = %s, want encoded payload", string(publisher.calls[0].payload))
			}
		})
	}
}

func TestHandlerDoesNotPublishInvalidEncodeFailedOrPublisherFailedMessages(t *testing.T) {
	t.Run("invalid message", func(t *testing.T) {
		publisher := &mockPublisher{}
		handler := NewHandler(publisher, &mockProcessor{}, newHandlerTestLogger())

		handler.HandleCommand(context.Background(), &Message{DeviceID: "dev-1", Type: MessageTypeCommand})

		if len(publisher.calls) != 0 {
			t.Fatalf("publisher calls = %d, want 0", len(publisher.calls))
		}
	})

	t.Run("encoder returns error", func(t *testing.T) {
		publisher := &mockPublisher{}
		handler := NewHandler(publisher, &mockProcessor{err: errors.New("encode failed")}, newHandlerTestLogger())

		handler.HandleCommand(context.Background(), &Message{
			DeviceID:       "dev-1",
			DeviceNumber:   "rdi-device-001",
			DeviceType:     "1",
			DeviceConfigID: "config-1",
			Type:           MessageTypeCommand,
			Data:           json.RawMessage(`{"cmd":"reset"}`),
		})

		if len(publisher.calls) != 0 {
			t.Fatalf("publisher calls = %d, want 0", len(publisher.calls))
		}
	})

	t.Run("encoder returns unsuccessful result", func(t *testing.T) {
		publisher := &mockPublisher{}
		handler := NewHandler(publisher, &mockProcessor{
			output: &processor.EncodeOutput{Success: false, Error: errors.New("script returned false")},
		}, newHandlerTestLogger())

		handler.HandleCommand(context.Background(), &Message{
			DeviceID:       "dev-1",
			DeviceNumber:   "rdi-device-001",
			DeviceType:     "1",
			DeviceConfigID: "config-1",
			Type:           MessageTypeCommand,
			Data:           json.RawMessage(`{"cmd":"reset"}`),
		})

		if len(publisher.calls) != 0 {
			t.Fatalf("publisher calls = %d, want 0", len(publisher.calls))
		}
	})

	t.Run("publisher failure is surfaced through publish call without retrying", func(t *testing.T) {
		publisher := &mockPublisher{err: errors.New("publish failed")}
		handler := NewHandler(publisher, &mockProcessor{}, newHandlerTestLogger())

		handler.HandleCommand(context.Background(), &Message{
			DeviceID:     "dev-1",
			DeviceNumber: "rdi-device-001",
			DeviceType:   "1",
			Type:         MessageTypeCommand,
			Data:         json.RawMessage(`{"cmd":"reset"}`),
		})

		if len(publisher.calls) != 1 {
			t.Fatalf("publisher calls = %d, want exactly 1 failed publish attempt", len(publisher.calls))
		}
	})
}

func TestHandlerPublishMessageRejectsMissingPublisher(t *testing.T) {
	handler := NewHandler(nil, nil, newHandlerTestLogger())

	err := handler.publishMessage(&Message{
		DeviceNumber: "rdi-device-001",
		Type:         MessageTypeCommand,
	}, []byte(`{"cmd":"reset"}`))

	if err == nil || err.Error() != "message publisher not initialized" {
		t.Fatalf("publishMessage error = %v, want publisher not initialized", err)
	}
}
