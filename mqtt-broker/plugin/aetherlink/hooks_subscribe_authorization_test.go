package aetherlink

import "testing"

func TestMQTTSubscribePolicyStandardTopicDeviceBinding(t *testing.T) {
	policy := mqttSubscribePolicy{fallbackDevice: "device-id", subTopic: "devices/command/device-001/reboot"}

	decision, handled := policy.decideStandardTopic("device-001")
	if !handled || !decision.allow {
		t.Fatalf("own standard topic: handled = %v, allow = %v; want true, true", handled, decision.allow)
	}

	decision, handled = policy.decideStandardTopic("device-002")
	if !handled || decision.allow {
		t.Fatalf("other device standard topic: handled = %v, allow = %v; want true, false", handled, decision.allow)
	}
}

func TestMQTTSubscribePolicyRejectsWildcardEmptyAndUnboundStandardTopics(t *testing.T) {
	tests := []string{
		"devices/command/+/reboot",
		"devices/command//reboot",
		"/down",
		"devices/register/response/+",
		"devices/config/down/response/+",
	}

	for _, topic := range tests {
		t.Run(topic, func(t *testing.T) {
			policy := mqttSubscribePolicy{fallbackDevice: "device-id", subTopic: topic}
			decision, handled := policy.decideStandardTopic("device-001")
			if !handled || decision.allow {
				t.Fatalf("handled = %v, allow = %v; want true, false", handled, decision.allow)
			}
		})
	}
}

func TestMQTTSubscribePolicyDoesNotFallbackReservedTopicToCustomMapping(t *testing.T) {
	policy := mqttSubscribePolicy{fallbackDevice: "device-id", subTopic: "devices/command/+/reboot"}

	decision := policy.decideTopicContract(t.Context(), "device-001")
	if decision.allow || decision.customMappingAllowed {
		t.Fatalf("allow = %v, customMappingAllowed = %v; want false, false", decision.allow, decision.customMappingAllowed)
	}
}

func TestMQTTSubscribePolicyLeavesCustomMappingTopicForMappingAuthorization(t *testing.T) {
	policy := mqttSubscribePolicy{fallbackDevice: "device-id", subTopic: "vendor/device-001/down"}

	decision, handled := policy.decideStandardTopic("device-001")
	if handled || decision.allow {
		t.Fatalf("custom mapping topic: handled = %v, allow = %v; want false, false", handled, decision.allow)
	}
}
