// 文件用途：平台点表拉取回归测试（ROADMAP B1）。
// 核心逻辑：httptest 模拟平台端点，验证信封解析、target/registers 覆盖与归一化。
// 关键注意事项：不发起真实外网请求。
package platform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/config"
)

func TestFetchProfileMergesTargetAndRegisters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/device/modbus/profile/number/plc-01", r.URL.Path)
		require.Equal(t, "test-key", r.Header.Get("x-api-key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"device_id":"dev-1","profile":{` +
			`"target":{"host":"10.0.0.5","port":1502,"unit_id":2},` +
			`"registers":[{"key":"temperature","type":"input","address":100,"data_type":"i16","multiplier":0.1}]}}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", 3*time.Second)
	device := config.DeviceConfig{DeviceNumber: "plc-01"}

	changed, err := client.FetchProfile(context.Background(), &device)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "10.0.0.5", device.Target.Host)
	require.Equal(t, 1502, device.Target.Port)
	require.Len(t, device.Registers, 1)
	require.Equal(t, 0.1, device.Registers[0].Multiplier, "Normalize must default/scale multiplier")
}

func TestFetchProfileReportsPlatformError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":101001,"message":"device not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second)
	device := config.DeviceConfig{DeviceNumber: "ghost"}
	_, err := client.FetchProfile(context.Background(), &device)
	require.Error(t, err)
}
