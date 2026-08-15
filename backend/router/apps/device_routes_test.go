package apps

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDeviceRoutesRegisterDebugDiagnosticsAndOnboardingHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	(&Device{}).InitDevice(engine.Group("/api/v1"))

	routes := make(map[string]string)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = route.Handler
	}

	tests := []struct {
		method  string
		path    string
		handler string
	}{
		{method: "POST", path: "/api/v1/device/:id/mqtt-debug/session", handler: "OpenMQTTDebugSession"},
		{method: "GET", path: "/api/v1/device/:id/mqtt-debug/session/:session_id", handler: "GetMQTTDebugSession"},
		{method: "POST", path: "/api/v1/device/:id/mqtt-debug/session/:session_id/command", handler: "ApplyMQTTDebugCommand"},
		{method: "DELETE", path: "/api/v1/device/:id/mqtt-debug/session/:session_id", handler: "CloseMQTTDebugSession"},
		{method: "GET", path: "/api/v1/device/:id/connection/diagnostics", handler: "GetDeviceConnectionDiagnostics"},
		{method: "GET", path: "/api/v1/device/:id/onboarding/connection-guide", handler: "GetDeviceConnectionGuide"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			key := tt.method + " " + tt.path
			registeredHandler, ok := routes[key]
			if !ok {
				t.Fatalf("route %s is not registered", key)
			}
			if !strings.Contains(registeredHandler, tt.handler) {
				t.Fatalf("route %s handler = %q, want %q", key, registeredHandler, tt.handler)
			}
		})
	}
}

func TestDeviceRouteContractsDoNotUseConflictingDeviceWildcardNames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	(&Device{}).InitDevice(engine.Group("/api/v1"))

	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/api/v1/device/") && strings.Contains(route.Path, ":device_id") {
			t.Fatalf("device route %q uses :device_id; the device group consistently uses :id", route.Path)
		}
	}
}
