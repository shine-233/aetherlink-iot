package aetherlink

import "github.com/DrmagicE/gmqtt/server"

type mqttDiagnosticEvent struct {
	deviceID    string
	clientID    string
	username    string
	action      string
	direction   string
	outcome     string
	error       string
	code        string
	topic       string
	mapped      bool
	sourceTopic string
	targetTopic string
	payload     []byte
	meta        map[string]interface{}
}

func recordMQTTDiagnosticEvent(event mqttDiagnosticEvent) {
	if event.deviceID == "" {
		return
	}
	// 每条上行/下行都会调用这里，而绝大多数设备未开调试：先查（本地缓存的）调试配置，
	// 未开启时直接返回，不再为注定被丢弃的日志构建 meta map 与条目。
	normalizedDeviceID, cfg, enabled, err := loadDeviceDebugConfigForWrite(event.deviceID)
	if err != nil || !enabled {
		return
	}

	entry := DeviceDebugLogEntry{
		Protocol:  "mqtt",
		Action:    event.action,
		Direction: event.direction,
		Outcome:   event.outcome,
		Error:     event.error,
		Meta:      buildMQTTDiagnosticMeta(event),
	}
	if event.payload != nil {
		entry.Payload = string(event.payload)
	}
	_, _ = writeDeviceDebugLogWithConfig(normalizedDeviceID, cfg, entry)
}

func buildMQTTDiagnosticMeta(event mqttDiagnosticEvent) map[string]interface{} {
	meta := map[string]interface{}{}
	if event.clientID != "" {
		meta["client_id"] = event.clientID
	}
	if event.username != "" {
		meta["username"] = event.username
	}
	if event.code != "" {
		meta["diagnostic_code"] = event.code
		if action := recommendedActionForMQTTDiagnosticCode(event.code); action != "" {
			meta["recommended_action"] = action
		}
	}
	if event.topic != "" {
		meta["topic"] = event.topic
	}
	if event.payload != nil {
		meta["payload_size"] = len(event.payload)
	}
	if event.mapped {
		meta["mapped"] = true
	}
	if event.sourceTopic != "" {
		meta["source_topic"] = event.sourceTopic
	}
	if event.targetTopic != "" {
		meta["target_topic"] = event.targetTopic
	}
	for key, value := range event.meta {
		meta[key] = value
	}
	return meta
}

func recordMQTTDiagnosticForClient(client server.Client, event mqttDiagnosticEvent) {
	event.clientID = client.ClientOptions().ClientID
	recordMQTTDiagnosticEvent(event)
}

func recommendedActionForMQTTDiagnosticCode(code string) string {
	switch code {
	case "auth_denied":
		return "check_device_credentials"
	case "transport_quota_exceeded":
		return "check_subscription_transport_quota_or_wait_next_utc_day"
	case "publish_deny":
		return "check_publish_topic_permission"
	case "subscribe_denied":
		return "check_subscribe_topic_permission"
	case "publish_error", "downlink_forward_error":
		return "check_topic_mapping_or_broker_route"
	case "publish_drop":
		return "check_custom_topic_mapping"
	case "disconnect_error":
		return "check_disconnect_reason"
	case "disconnect_normal":
		return "confirm_device_shutdown_or_reconnect_policy"
	default:
		return ""
	}
}
