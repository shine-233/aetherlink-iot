// 文件用途：验证可选遥测生命周期、配置兜底、状态文件和实例 ID 持久化。
// 核心逻辑：使用 httptest 服务接收 payload，配合临时目录和 Viper 配置覆盖注册、心跳、升级和错误分支。
// 关键注意事项：测试只连接本地 httptest，不应依赖真实 PostHog；每个用例需要清理 Viper 全局配置。
// 重构建议：后续可把 Viper 和 HTTP client 注入，减少全局状态清理成本。
package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestReportTelemetryCycleLifecycle(t *testing.T) {
	t.Cleanup(resetTelemetryConfig)

	var (
		mu      sync.Mutex
		events  []string
		payload []map[string]interface{}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		mu.Lock()
		defer mu.Unlock()
		events = append(events, body["event"].(string))
		payload = append(payload, body)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	viper.Set("telemetry.enabled", true)
	viper.Set("telemetry.posthog_key", "phc_test")
	viper.Set("telemetry.posthog_host", server.URL)
	viper.Set("telemetry.state_file", filepath.Join(tmpDir, ".telemetry_state.json"))
	viper.Set("telemetry.instance_id_file", filepath.Join(tmpDir, ".instance_id"))

	ins := &InstanceInfo{
		InstanceID:  "instance-1",
		DeviceCount: 12,
		UserCount:   3,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Timestamp:   1775800000,
	}

	firstAt := time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)
	require.NoError(t, reportTelemetryCycleAt(ins, "startup", firstAt))

	mu.Lock()
	require.Equal(t, []string{EventInstanceRegistered, EventInstanceHeartbeat}, events)
	require.Equal(t, "instance-1", payload[0]["properties"].(map[string]interface{})["distinct_id"])
	require.NotEmpty(t, payload[0]["timestamp"])
	mu.Unlock()

	secondAt := firstAt.Add(30 * time.Minute)
	require.NoError(t, reportTelemetryCycleAt(ins, "heartbeat", secondAt))

	mu.Lock()
	require.Len(t, events, 2)
	mu.Unlock()

	ins.Version = "1.1.0"
	thirdAt := firstAt.Add(90 * time.Minute)
	require.NoError(t, reportTelemetryCycleAt(ins, "heartbeat", thirdAt))

	mu.Lock()
	require.Equal(t, []string{
		EventInstanceRegistered,
		EventInstanceHeartbeat,
		EventInstanceUpgraded,
		EventInstanceHeartbeat,
	}, events)
	require.Equal(t, "1.0.0", payload[2]["properties"].(map[string]interface{})["from_version"])
	require.Equal(t, "1.1.0", payload[2]["properties"].(map[string]interface{})["to_version"])
	mu.Unlock()
}

func resetTelemetryConfig() {
	keys := []string{
		"telemetry.enabled",
		"telemetry.posthog_key",
		"telemetry.posthog_host",
		"telemetry.state_file",
		"telemetry.instance_id_file",
		"telemetry.heartbeat_interval",
	}

	for _, key := range keys {
		viper.Set(key, nil)
	}
}

func TestTelemetryEnabledAndHeartbeatIntervalConfigFallbacks(t *testing.T) {
	t.Cleanup(resetTelemetryConfig)

	viper.Set("telemetry.enabled", false)
	require.False(t, TelemetryEnabled())

	viper.Set("telemetry.enabled", true)
	require.True(t, TelemetryEnabled())

	for _, raw := range []string{"", "not-a-duration", "0s", "-1m"} {
		viper.Set("telemetry.heartbeat_interval", raw)
		require.Equal(t, defaultHeartbeatInterval, HeartbeatInterval(), "raw interval %q", raw)
	}

	viper.Set("telemetry.heartbeat_interval", "15m")
	require.Equal(t, 15*time.Minute, HeartbeatInterval())
}

func TestReportTelemetryCycleSkipsWhenDisabledAndRejectsEmptyInstanceID(t *testing.T) {
	t.Cleanup(resetTelemetryConfig)
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, ".telemetry_state.json")
	viper.Set("telemetry.state_file", statePath)
	viper.Set("telemetry.enabled", false)

	require.NoError(t, reportTelemetryCycleAt(&InstanceInfo{}, "startup", time.Now().UTC()))
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("disabled telemetry should not create state file, stat err=%v", err)
	}

	viper.Set("telemetry.enabled", true)
	err := reportTelemetryCycleAt(&InstanceInfo{}, "startup", time.Now().UTC())
	require.ErrorContains(t, err, "instance_id is empty")
}

func TestTelemetryStateLoadSaveAndHeartbeatBucket(t *testing.T) {
	t.Cleanup(resetTelemetryConfig)
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "nested", ".telemetry_state.json")
	viper.Set("telemetry.state_file", statePath)

	state, err := loadTelemetryState()
	require.NoError(t, err)
	require.Equal(t, &TelemetryState{}, state)

	saved := &TelemetryState{
		RegisteredAt:        "2026-06-27T17:00:00Z",
		LastHeartbeatAt:     "2026-06-27T18:00:00Z",
		LastHeartbeatBucket: "2026-06-27T18:00:00Z",
		LastVersion:         "1.2.3",
	}
	require.NoError(t, saveTelemetryState(saved))

	loaded, err := loadTelemetryState()
	require.NoError(t, err)
	require.Equal(t, saved, loaded)

	now := time.Date(2026, 6, 27, 18, 59, 30, 0, time.UTC)
	require.Equal(t, "2026-06-27T18:45:00Z", heartbeatBucket(now, 15*time.Minute))

	require.NoError(t, os.WriteFile(statePath, []byte("{bad json"), 0o600))
	_, err = loadTelemetryState()
	require.Error(t, err)
}

func TestGetPersistentInstanceIDReadsExistingOrCreatesNewFile(t *testing.T) {
	t.Cleanup(resetTelemetryConfig)
	tmpDir := t.TempDir()
	instanceIDPath := filepath.Join(tmpDir, "nested", ".instance_id")
	viper.Set("telemetry.instance_id_file", instanceIDPath)

	require.NoError(t, os.MkdirAll(filepath.Dir(instanceIDPath), 0o755))
	require.NoError(t, os.WriteFile(instanceIDPath, []byte(" existing-instance \n"), 0o600))
	require.Equal(t, "existing-instance", GetPersistentInstanceID())

	require.NoError(t, os.Remove(instanceIDPath))
	generated := GetPersistentInstanceID()
	require.NotEmpty(t, generated)
	require.False(t, strings.Contains(generated, "\n"))

	persisted, err := os.ReadFile(instanceIDPath)
	require.NoError(t, err)
	require.Equal(t, generated, string(persisted))

	require.Equal(t, generated, GetPersistentInstanceID())
}

func TestNewInstanceUsesRuntimeAndVersionDefaults(t *testing.T) {
	ins := NewInstance()

	require.Equal(t, TelemetryEdition, "community")
	require.NotEmpty(t, ins.OS)
	require.NotEmpty(t, ins.Arch)
}
