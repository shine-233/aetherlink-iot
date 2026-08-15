package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Frame is a replayable representation of the repository-observed direct
// device MQTT contract.  Payload is the exact bytes that would be published;
// the broker/plugin may add its own authenticated device envelope later.
type Frame struct {
	Sequence   int             `json:"sequence"`
	Kind       string          `json:"kind"`
	Topic      string          `json:"topic"`
	Payload    json.RawMessage `json:"payload"`
	DeviceID   string          `json:"device_id"`
	PID        string          `json:"pid"`
	Timestamp  int64           `json:"timestamp"`
	Provenance string          `json:"provenance"`
	Protocol   string          `json:"protocol"`
}

type commandPayload struct {
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type ackPayload struct {
	Message string `json:"message"`
	Method  string `json:"method,omitempty"`
	Result  int    `json:"result"`
	TS      int64  `json:"ts"`
}

func newFrame(identity SyntheticIdentity, sequence int, kind, topic string, payload []byte, timestamp int64) (Frame, error) {
	if err := validateSyntheticIdentity(identity); err != nil {
		return Frame{}, err
	}
	if sequence < 1 {
		return Frame{}, fmt.Errorf("frame sequence must be positive")
	}
	if strings.TrimSpace(topic) == "" {
		return Frame{}, fmt.Errorf("frame topic is required")
	}
	if len(payload) == 0 {
		return Frame{}, fmt.Errorf("frame payload is required")
	}
	return Frame{
		Sequence:   sequence,
		Kind:       kind,
		Topic:      topic,
		Payload:    json.RawMessage(append([]byte(nil), payload...)),
		DeviceID:   identity.DeviceID,
		PID:        identity.PID,
		Timestamp:  timestamp,
		Provenance: SyntheticMode,
		Protocol:   ObservedProtocol,
	}, nil
}

func BuildStatusFrame(identity SyntheticIdentity, online bool, timestamp int64, sequence int) (Frame, error) {
	payload := []byte("0")
	if online {
		payload = []byte("1")
	}
	return newFrame(identity, sequence, "status", "devices/status/"+identity.DeviceID, payload, timestamp)
}

func BuildTelemetryFrame(identity SyntheticIdentity, values map[string]any, timestamp int64, sequence int) (Frame, error) {
	if values == nil {
		return Frame{}, fmt.Errorf("telemetry values are required")
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return Frame{}, fmt.Errorf("marshal telemetry payload: %w", err)
	}
	return newFrame(identity, sequence, "telemetry", "devices/telemetry", payload, timestamp)
}

func BuildCommandFrame(identity SyntheticIdentity, messageID, method string, params any, timestamp int64, sequence int) (Frame, error) {
	messageID = strings.TrimSpace(messageID)
	method = strings.TrimSpace(method)
	if messageID == "" || method == "" {
		return Frame{}, fmt.Errorf("command message id and method are required")
	}
	payload, err := json.Marshal(commandPayload{Method: method, Params: params})
	if err != nil {
		return Frame{}, fmt.Errorf("marshal command payload: %w", err)
	}
	return newFrame(identity, sequence, "command", "devices/command/"+identity.PID+"/"+messageID, payload, timestamp)
}

func BuildACKFrame(identity SyntheticIdentity, command Frame, success bool, timestamp int64, sequence int) (Frame, error) {
	if err := validateReplayFrame(command); err != nil {
		return Frame{}, fmt.Errorf("invalid command frame: %w", err)
	}
	if command.PID != identity.PID {
		return Frame{}, fmt.Errorf("command PID %q does not match identity PID %q", command.PID, identity.PID)
	}
	messageID, err := commandTopicMessageID(command.Topic, identity.PID)
	if err != nil {
		return Frame{}, err
	}
	var request commandPayload
	if err := json.Unmarshal(command.Payload, &request); err != nil || strings.TrimSpace(request.Method) == "" {
		return Frame{}, fmt.Errorf("command payload must contain a method")
	}
	result := 1
	message := "failed"
	if success {
		result = 0
		message = "success"
	}
	payload, err := json.Marshal(ackPayload{Message: message, Method: request.Method, Result: result, TS: timestamp})
	if err != nil {
		return Frame{}, fmt.Errorf("marshal ACK payload: %w", err)
	}
	return newFrame(identity, sequence, "ack", "devices/command/response/"+messageID, payload, timestamp)
}

func validateReplayFrame(frame Frame) error {
	if frame.Provenance != SyntheticMode {
		return fmt.Errorf("replay accepts only provenance %q, got %q", SyntheticMode, frame.Provenance)
	}
	if frame.Protocol != ObservedProtocol {
		return fmt.Errorf("replay accepts only protocol %q, got %q", ObservedProtocol, frame.Protocol)
	}
	if frame.Sequence < 1 || strings.TrimSpace(frame.Kind) == "" || strings.TrimSpace(frame.Topic) == "" || len(frame.Payload) == 0 {
		return fmt.Errorf("replay frame is incomplete")
	}
	if frame.DeviceID == "" || frame.PID == "" {
		return fmt.Errorf("replay frame identity is incomplete")
	}

	switch frame.Kind {
	case "status":
		parts := strings.Split(frame.Topic, "/")
		if len(parts) != 3 || parts[0] != "devices" || parts[1] != "status" || parts[2] != frame.DeviceID {
			return fmt.Errorf("status topic does not match device ID: %q", frame.Topic)
		}
		if string(frame.Payload) != "0" && string(frame.Payload) != "1" {
			return fmt.Errorf("status payload must be 0 or 1, got %q", frame.Payload)
		}
	case "telemetry":
		if frame.Topic != "devices/telemetry" {
			return fmt.Errorf("telemetry topic must be devices/telemetry, got %q", frame.Topic)
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(frame.Payload, &values); err != nil || values == nil {
			return fmt.Errorf("telemetry payload must be a JSON object")
		}
	case "command":
		if _, err := commandTopicMessageID(frame.Topic, frame.PID); err != nil {
			return err
		}
		var request commandPayload
		if err := json.Unmarshal(frame.Payload, &request); err != nil || strings.TrimSpace(request.Method) == "" {
			return fmt.Errorf("command payload must contain a method")
		}
	case "ack":
		if _, err := responseTopicMessageID(frame.Topic); err != nil {
			return err
		}
		var response ackPayload
		if err := json.Unmarshal(frame.Payload, &response); err != nil {
			return fmt.Errorf("ACK payload must be valid JSON: %w", err)
		}
		if response.Result != 0 && response.Result != 1 {
			return fmt.Errorf("ACK result must be 0 or 1, got %d", response.Result)
		}
		if response.Message != "success" && response.Message != "failed" {
			return fmt.Errorf("ACK message must be success or failed, got %q", response.Message)
		}
		if (response.Result == 0 && response.Message != "success") || (response.Result == 1 && response.Message != "failed") {
			return fmt.Errorf("ACK result and message disagree")
		}
	default:
		return fmt.Errorf("unsupported replay frame kind %q", frame.Kind)
	}
	return nil
}

func commandTopicMessageID(topic, pid string) (string, error) {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) != 4 || parts[0] != "devices" || parts[1] != "command" || parts[2] != pid || parts[3] == "" {
		return "", fmt.Errorf("command topic is not a direct-device command topic for PID %q: %q", pid, topic)
	}
	return parts[3], nil
}

func responseTopicMessageID(topic string) (string, error) {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) != 4 || parts[0] != "devices" || parts[1] != "command" || parts[2] != "response" || parts[3] == "" {
		return "", fmt.Errorf("ACK topic is not a direct-device command response topic: %q", topic)
	}
	return parts[3], nil
}
