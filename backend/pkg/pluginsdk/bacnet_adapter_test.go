package pluginsdk

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var _ ProtocolAdapter = (*BACnetAdapter)(nil)

func TestBACnetAdapterManifestValidAndSigned(t *testing.T) {
	manifest := BACnetAdapterManifest()
	require.NoError(t, manifest.Validate(), "BACnet adapter manifest must be valid")

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	err = SignManifest(manifest, "vendor-honeywell-bacnet", priv)
	require.NoError(t, err)
	require.NotEmpty(t, manifest.Signature)
	require.Equal(t, "vendor-honeywell-bacnet", manifest.SignedBy)

	trustedKeys := map[string]ed25519.PublicKey{
		"vendor-honeywell-bacnet": pub,
	}
	err = VerifyManifestSignature(manifest, trustedKeys)
	require.NoError(t, err, "Signed BACnet manifest must verify against vendor public key")
}

func TestBACnetAdapterValidateConfig(t *testing.T) {
	adapter := NewBACnetAdapter()
	ctx := context.Background()

	validConfig := AdapterConfig{
		DeviceID: "bacnet-ahu-01",
		Values: map[string]any{
			"ip_address":      "192.168.1.100",
			"port":            float64(47808),
			"device_instance": float64(1001),
			"network_number":  float64(1),
			"apdu_timeout_ms": float64(3000),
		},
	}
	require.NoError(t, adapter.ValidateConfig(ctx, validConfig))

	// 缺少 device_id
	badNoID := validConfig
	badNoID.DeviceID = ""
	require.Error(t, adapter.ValidateConfig(ctx, badNoID))

	// 非法 IP 地址
	badIP := validConfig
	badIP.Values = map[string]any{
		"ip_address":      "999.999.999.999",
		"port":            float64(47808),
		"device_instance": float64(1001),
	}
	require.Error(t, adapter.ValidateConfig(ctx, badIP))

	// 端口超出范围
	badPort := validConfig
	badPort.Values = map[string]any{
		"ip_address":      "192.168.1.100",
		"port":            float64(80), // < 1024
		"device_instance": float64(1001),
	}
	require.Error(t, adapter.ValidateConfig(ctx, badPort))

	// device_instance 超过 22-bit 上限 4194303
	badInstance := validConfig
	badInstance.Values = map[string]any{
		"ip_address":      "192.168.1.100",
		"port":            float64(47808),
		"device_instance": float64(5000000),
	}
	require.Error(t, adapter.ValidateConfig(ctx, badInstance))
}

func TestBACnetAdapterConnectAndDiscover(t *testing.T) {
	adapter := NewBACnetAdapter()
	ctx := context.Background()

	config := AdapterConfig{
		DeviceID: "bacnet-chiller-01",
		Values: map[string]any{
			"ip_address":      "10.0.0.50",
			"port":            float64(47808),
			"device_instance": float64(2002),
		},
	}
	require.NoError(t, adapter.Connect(ctx, config))

	health, err := adapter.Health(ctx)
	require.NoError(t, err)
	require.Equal(t, HealthOK, health.Status)

	devices, err := adapter.Discover(ctx)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.Equal(t, "bacnet-chiller-01", devices[0].ID)
	require.Equal(t, "BACnet-Device-2002", devices[0].Name)
	require.Equal(t, "bacnet-2002", devices[0].Number)
	require.Equal(t, "10.0.0.50", devices[0].Metadata["ip_address"])

	require.NoError(t, adapter.Close(ctx))
	hClose, err := adapter.Health(ctx)
	require.NoError(t, err)
	require.Equal(t, HealthDown, hClose.Status)
}

func TestBACnetAdapterReadTelemetry(t *testing.T) {
	adapter := NewBACnetAdapter()
	ctx := context.Background()

	config := AdapterConfig{
		DeviceID: "bacnet-vav-01",
		Values: map[string]any{
			"ip_address":      "192.168.10.20",
			"port":            float64(47808),
			"device_instance": float64(3003),
		},
	}
	require.NoError(t, adapter.Connect(ctx, config))

	// 设置模拟属性
	now := time.Now().UTC()
	adapter.SetProperty("room_temp", BACnetValue{Value: 24.2, Timestamp: now})
	adapter.SetProperty("air_flow", BACnetValue{Value: 520.0, Timestamp: now})
	adapter.SetProperty("fan_running", BACnetValue{Value: false, Timestamp: now})

	telemetry, err := adapter.ReadTelemetry(ctx, "bacnet-vav-01", nil)
	require.NoError(t, err)
	require.Equal(t, "bacnet-vav-01", telemetry.DeviceID)

	pointsMap := make(map[string]any)
	for _, pt := range telemetry.Points {
		pointsMap[pt.Key] = pt.Value
	}

	require.Equal(t, 24.2, pointsMap["room_temp"])
	require.Equal(t, 520.0, pointsMap["air_flow"])
	require.Equal(t, false, pointsMap["fan_running"])
	require.Equal(t, 7.2, pointsMap["chilled_water_temp"]) // 默认底数
	require.Equal(t, 65.0, pointsMap["cooling_valve_pos"])

	// 过滤点
	filtered, err := adapter.ReadTelemetry(ctx, "bacnet-vav-01", []string{"room_temp"})
	require.NoError(t, err)
	require.Len(t, filtered.Points, 1)
	require.Equal(t, "room_temp", filtered.Points[0].Key)
}

func TestBACnetAdapterWriteCommand(t *testing.T) {
	adapter := NewBACnetAdapter()
	ctx := context.Background()

	config := AdapterConfig{
		DeviceID: "bacnet-ahu-zone1",
		Values: map[string]any{
			"ip_address":      "172.16.0.10",
			"port":            float64(47808),
			"device_instance": float64(4004),
		},
	}
	require.NoError(t, adapter.Connect(ctx, config))

	// 控制 1: set_room_temp
	cmdTemp := Command{
		DeviceID: "bacnet-ahu-zone1",
		Name:     "set_room_temp",
		Payload:  []byte(`{"target": 22.5}`),
	}
	require.NoError(t, adapter.WriteCommand(ctx, cmdTemp))

	// 验证修改后 Present_Value 生效
	telemetry, err := adapter.ReadTelemetry(ctx, "bacnet-ahu-zone1", []string{"room_temp"})
	require.NoError(t, err)
	require.Equal(t, 22.5, telemetry.Points[0].Value)

	// 温度超出安全边界
	cmdTempOut := Command{
		DeviceID: "bacnet-ahu-zone1",
		Name:     "set_room_temp",
		Payload:  []byte(`{"target": 50.0}`),
	}
	require.Error(t, adapter.WriteCommand(ctx, cmdTempOut))

	// 控制 2: override_fan
	cmdFan := Command{
		DeviceID: "bacnet-ahu-zone1",
		Name:     "override_fan",
		Payload:  []byte(`{"state": true}`),
	}
	require.NoError(t, adapter.WriteCommand(ctx, cmdFan))
	telemetryFan, err := adapter.ReadTelemetry(ctx, "bacnet-ahu-zone1", []string{"fan_running"})
	require.NoError(t, err)
	require.Equal(t, true, telemetryFan.Points[0].Value)

	// 未知命令拒绝
	cmdUnknown := Command{
		DeviceID: "bacnet-ahu-zone1",
		Name:     "explode_boiler",
	}
	require.Error(t, adapter.WriteCommand(ctx, cmdUnknown))
}
