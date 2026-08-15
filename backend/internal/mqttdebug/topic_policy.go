package mqttdebug

import (
	"fmt"
	"strings"
)

const maxMQTTDebugTopicBytes = 512

const (
	debugDeviceIDSegment     = "{device_id}"
	debugDeviceNumberSegment = "{device_number}"
)

// Device-scoped topics mirror the platform broker's canonical downlink topic
// shapes. Keeping the identity at a known position matters: merely finding a
// device number somewhere in a topic can mistake a shared uplink message_id
// for an identity and would authorize a cross-device payload.
var subscribableDeviceDebugTopicPatterns = []string{
	"devices/status/{device_id}",
	"devices/telemetry/control/{device_number}",
	"devices/telemetry/control/{device_number}/+",
	"devices/attributes/set/{device_number}/+",
	"devices/attributes/get/{device_number}",
	"devices/command/{device_number}/+",
	"devices/attributes/response/{device_number}/+",
	"devices/event/response/{device_number}/+",
	"gateway/telemetry/control/{device_number}",
	"gateway/attributes/set/{device_number}/+",
	"gateway/attributes/get/{device_number}",
	"gateway/command/{device_number}/+",
	"gateway/attributes/response/{device_number}/+",
	"gateway/event/response/{device_number}/+",
	"ota/devices/inform/{device_number}",
	"{device_number}/down",
}

// Publishing is narrower than subscribing. Status and acknowledgement topics
// are intentionally read-only so a platform-credential debug client cannot
// forge device online state or protocol acknowledgements into platform flows.
var publishableDeviceDebugTopicPatterns = []string{
	"devices/telemetry/control/{device_number}",
	"devices/telemetry/control/{device_number}/+",
	"devices/attributes/set/{device_number}/+",
	"devices/attributes/get/{device_number}",
	"devices/command/{device_number}/+",
	"gateway/telemetry/control/{device_number}",
	"gateway/attributes/set/{device_number}/+",
	"gateway/attributes/get/{device_number}",
	"gateway/command/{device_number}/+",
	"ota/devices/inform/{device_number}",
	"{device_number}/down",
}

// Shared uplink filters are observed from the production adapter's accepted
// message stream. They are never subscribed once per debug session and their
// device/tenant identity comes from the adapter lookup, not untrusted JSON.
var trustedUplinkDebugTopicPatterns = []string{
	"devices/telemetry",
	"devices/attributes/+",
	"devices/event/+",
	"devices/command/response/+",
	"devices/attributes/set/response/+",
	"gateway/telemetry",
	"gateway/attributes/+",
	"gateway/event/+",
	"gateway/command/response/+",
	"gateway/attributes/set/response/+",
}

func authorizeTopic(scope Scope, rawTopic string, subscription bool) (string, bool, error) {
	topic := strings.TrimSpace(rawTopic)
	if err := validateMQTTTopic(topic, subscription); err != nil {
		return "", false, err
	}
	if subscription {
		if matchesAnyDebugTopicPattern(topic, subscribableDeviceDebugTopicPatterns, scope) {
			return topic, false, nil
		}
		if matchesAnyDebugTopicPattern(topic, trustedUplinkDebugTopicPatterns, Scope{}) {
			return topic, true, nil
		}
	} else if matchesAnyDebugTopicPattern(topic, publishableDeviceDebugTopicPatterns, scope) {
		return topic, false, nil
	}
	return "", false, ErrTopicDenied
}

func validateMQTTTopic(topic string, subscription bool) error {
	if topic == "" || len(topic) > maxMQTTDebugTopicBytes || strings.ContainsRune(topic, '\x00') {
		return fmt.Errorf("%w: topic must contain 1-%d bytes", ErrInvalidTopic, maxMQTTDebugTopicBytes)
	}
	if strings.HasPrefix(topic, "$") {
		return fmt.Errorf("%w: broker control and shared-subscription prefixes are not allowed", ErrTopicDenied)
	}
	segments := strings.Split(topic, "/")
	for index, segment := range segments {
		if strings.Contains(segment, "#") {
			if !subscription || segment != "#" || index != len(segments)-1 {
				return fmt.Errorf("%w: # must be the final complete subscription segment", ErrInvalidTopic)
			}
		}
		if strings.Contains(segment, "+") {
			if !subscription || segment != "+" {
				return fmt.Errorf("%w: + must be a complete subscription segment", ErrInvalidTopic)
			}
		}
	}
	return nil
}

func matchesAnyDebugTopicPattern(topic string, patterns []string, scope Scope) bool {
	for _, pattern := range patterns {
		if matchesDebugTopicPattern(topic, pattern, scope) {
			return true
		}
	}
	return false
}

func matchesDebugTopicPattern(topic string, pattern string, scope Scope) bool {
	topicSegments := strings.Split(topic, "/")
	patternSegments := strings.Split(pattern, "/")
	if len(topicSegments) != len(patternSegments) {
		return false
	}
	deviceID := strings.TrimSpace(scope.DeviceID)
	deviceNumber := strings.TrimSpace(scope.DeviceNumber)
	for index, expected := range patternSegments {
		actual := topicSegments[index]
		switch expected {
		case debugDeviceIDSegment:
			if actual == "" || actual != deviceID {
				return false
			}
		case debugDeviceNumberSegment:
			if actual == "" || actual != deviceNumber {
				return false
			}
		case "+":
			// A pattern wildcard accepts either a concrete segment or the same
			// single-level wildcard in a subscription filter, but never an empty
			// segment or a multi-level wildcard.
			if actual == "" || actual == "#" {
				return false
			}
		default:
			if actual != expected {
				return false
			}
		}
	}
	return true
}

func mqttTopicFilterMatches(filter string, topic string) bool {
	filterSegments := strings.Split(filter, "/")
	topicSegments := strings.Split(topic, "/")
	if len(filterSegments) != len(topicSegments) {
		return false
	}
	for index, expected := range filterSegments {
		if expected == "+" {
			if topicSegments[index] == "" {
				return false
			}
			continue
		}
		if expected != topicSegments[index] {
			return false
		}
	}
	return true
}
