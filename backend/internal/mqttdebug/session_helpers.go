package mqttdebug

import (
	"fmt"
	"strings"
)

func normalizeScope(scope Scope) (Scope, error) {
	scope.TenantID = strings.TrimSpace(scope.TenantID)
	scope.UserID = strings.TrimSpace(scope.UserID)
	scope.DeviceID = strings.TrimSpace(scope.DeviceID)
	scope.DeviceNumber = strings.TrimSpace(scope.DeviceNumber)
	if scope.TenantID == "" || scope.UserID == "" || scope.DeviceID == "" || scope.DeviceNumber == "" {
		return Scope{}, fmt.Errorf("%w: tenant, user and device identity are required", ErrInvalidCommand)
	}
	if strings.ContainsAny(scope.DeviceID, "/+#\x00") || strings.ContainsAny(scope.DeviceNumber, "/+#\x00") {
		return Scope{}, fmt.Errorf("%w: device identity is not a safe mqtt topic segment", ErrInvalidCommand)
	}
	return scope, nil
}

func debugClientID(sessionID string) string {
	compact := strings.ReplaceAll(sessionID, "-", "")
	if len(compact) > 16 {
		compact = compact[:16]
	}
	// Keep the identifier at 23 bytes so strict MQTT 3.1 brokers do not reject
	// it when Paho negotiates the compatibility protocol.
	return "al-dbg-" + compact
}

// Message window selection lives in messageRing.selectAfter (message_ring.go).
