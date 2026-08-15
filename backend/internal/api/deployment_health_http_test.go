package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/global"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// This contract keeps readiness fail-closed when required local dependencies
// are unavailable. Liveness remains the separate, dependency-free /health path.
func TestReadinessReturnsServiceUnavailableWhenCoreDependenciesAreMissing(t *testing.T) {
	oldDB, oldRedis, oldStatusRedis := global.DB, global.REDIS, global.STATUS_REDIS
	oldSettings := viper.AllSettings()
	global.DB, global.REDIS, global.STATUS_REDIS = nil, nil, nil
	t.Cleanup(func() {
		global.DB, global.REDIS, global.STATUS_REDIS = oldDB, oldRedis, oldStatusRedis
		service.SetMQTTHealthProbe(nil)
		viper.Reset()
		if err := viper.MergeConfigMap(oldSettings); err != nil {
			t.Errorf("restore viper settings: %v", err)
		}
	})
	// MQTT is optional when disabled, so enable it explicitly for this
	// fail-closed contract to exercise the missing-probe failure.
	viper.Reset()
	viper.Set("mqtt.enabled", true)
	service.SetMQTTHealthProbe(nil)

	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("change to temporary working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWorkingDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ready", nil)

	(&SystemApi{}).Readiness(ctx)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness HTTP status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("readiness Content-Type = %q, want JSON", got)
	}

	var report service.DeploymentHealthReport
	if err := json.Unmarshal(recorder.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	if report.Status != "down" {
		t.Fatalf("readiness report status = %q, want down", report.Status)
	}
	for _, key := range []string{"database", "redis", "mqtt"} {
		check, exists := report.Checks[key]
		if !exists {
			t.Fatalf("readiness report missing required check %q", key)
		}
		if !check.Required || check.OK {
			t.Fatalf("readiness check %q = %+v, want required failure", key, check)
		}
	}
}
