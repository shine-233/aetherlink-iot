package pluginsdk

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var _ ProtocolAdapter = (*CANAdapter)(nil)

func TestCANAdapterManifestValid(t *testing.T) {
	manifest := CANAdapterManifest()
	require.NoError(t, manifest.Validate(), "CAN adapter manifest must pass self-validation")

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	err = SignManifest(manifest, "vendor-bosch-can", priv)
	require.NoError(t, err)
	require.NotEmpty(t, manifest.Signature)
	require.Equal(t, "vendor-bosch-can", manifest.SignedBy)

	trustedKeys := map[string]ed25519.PublicKey{
		"vendor-bosch-can": pub,
	}
	err = VerifyManifestSignature(manifest, trustedKeys)
	require.NoError(t, err, "Signed CAN adapter manifest must verify with trusted vendor key")
}

func TestCANAdapterValidateConfig(t *testing.T) {
	adapter := NewCANAdapter()
	ctx := context.Background()

	validConfig := AdapterConfig{
		DeviceID: "can-device-01",
		Values: map[string]any{
			"interface_name":   "vcan0",
			"baud_rate":        float64(500000),
			"node_id":          float64(12),
			"extended_id":      true,
			"poll_interval_ms": float64(100),
		},
	}
	require.NoError(t, adapter.ValidateConfig(ctx, validConfig))

	// 缺少 device_id
	badConfigNoID := validConfig
	badConfigNoID.DeviceID = ""
	require.Error(t, adapter.ValidateConfig(ctx, badConfigNoID))

	// 缺少必填属性 interface_name
	badConfigNoInterface := AdapterConfig{
		DeviceID: "can-device-01",
		Values: map[string]any{
			"baud_rate": float64(500000),
			"node_id":   float64(12),
		},
	}
	require.Error(t, adapter.ValidateConfig(ctx, badConfigNoInterface))

	// 非法波特率
	badBaudConfig := validConfig
	badBaudConfig.Values = map[string]any{
		"interface_name": "can0",
		"baud_rate":      float64(9600), // 不在 125k/250k/500k/1M
		"node_id":        float64(12),
	}
	require.Error(t, adapter.ValidateConfig(ctx, badBaudConfig))

	// 非法 node_id (0 或 > 127)
	badNodeConfig := validConfig
	badNodeConfig.Values = map[string]any{
		"interface_name": "can0",
		"baud_rate":      float64(250000),
		"node_id":        float64(255),
	}
	require.Error(t, adapter.ValidateConfig(ctx, badNodeConfig))
}

func TestCANAdapterConnectAndDiscover(t *testing.T) {
	adapter := NewCANAdapter()
	ctx := context.Background()

	// 未连接时 Discover 拒绝
	_, err := adapter.Discover(ctx)
	require.Error(t, err)

	config := AdapterConfig{
		DeviceID: "ecu-engine-main",
		Values: map[string]any{
			"interface_name": "can0",
			"baud_rate":      float64(250000),
			"node_id":        float64(42),
			"extended_id":    true,
		},
	}

	require.NoError(t, adapter.Connect(ctx, config))

	health, err := adapter.Health(ctx)
	require.NoError(t, err)
	require.Equal(t, HealthOK, health.Status)

	devices, err := adapter.Discover(ctx)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.Equal(t, "ecu-engine-main", devices[0].ID)
	require.Equal(t, "CAN-Node-42", devices[0].Name)
	require.Equal(t, "can-42", devices[0].Number)
	require.Equal(t, "can0", devices[0].Metadata["interface"])

	require.NoError(t, adapter.Close(ctx))
	hAfterClose, err := adapter.Health(ctx)
	require.NoError(t, err)
	require.Equal(t, HealthDown, hAfterClose.Status)
}

func TestCANAdapterReadTelemetryDecoding(t *testing.T) {
	adapter := NewCANAdapter()
	ctx := context.Background()

	config := AdapterConfig{
		DeviceID: "truck-ecu-01",
		Values: map[string]any{
			"interface_name": "can1",
			"baud_rate":      float64(500000),
			"node_id":        float64(10),
		},
	}
	require.NoError(t, adapter.Connect(ctx, config))

	// 模拟真实 CAN 帧注入
	// Frame 1: Engine 0x18F00400, RPM = 2400 (raw 19200 = 0x4B00), Temp = 85 degC (raw 125 = 0x7D)
	var f1 CANFrame
	f1.ID = 0x18F00400
	f1.DLC = 3
	binary.BigEndian.PutUint16(f1.Data[0:2], 19200)
	f1.Data[2] = 125
	f1.Timestamp = time.Date(2026, 9, 19, 23, 0, 0, 0, time.UTC)
	adapter.InjectFrame(f1)

	// Frame 2: Pressure 0x18F00500, Oil Pressure = 420 kPa (raw 210), Voltage = 24.50 V (raw 2450 = 0x0992)
	var f2 CANFrame
	f2.ID = 0x18F00500
	f2.DLC = 3
	f2.Data[0] = 210
	binary.BigEndian.PutUint16(f2.Data[1:3], 2450)
	f2.Timestamp = f1.Timestamp
	adapter.InjectFrame(f2)

	// Frame 3: Relay 0x18F00600, relay_1 on (bit 0 = 1)
	var f3 CANFrame
	f3.ID = 0x18F00600
	f3.DLC = 1
	f3.Data[0] = 0x01
	f3.Timestamp = f1.Timestamp
	adapter.InjectFrame(f3)

	// 读取全量点
	telemetry, err := adapter.ReadTelemetry(ctx, "truck-ecu-01", nil)
	require.NoError(t, err)
	require.Equal(t, "truck-ecu-01", telemetry.DeviceID)

	pointsMap := make(map[string]any)
	for _, pt := range telemetry.Points {
		pointsMap[pt.Key] = pt.Value
	}

	require.InDelta(t, 2400.0, pointsMap["engine_rpm"], 0.001)
	require.InDelta(t, 85.0, pointsMap["coolant_temp"], 0.001)
	require.InDelta(t, 420.0, pointsMap["oil_pressure"], 0.001)
	require.InDelta(t, 24.50, pointsMap["battery_voltage"], 0.001)
	require.Equal(t, true, pointsMap["relay_state"])

	// 仅读取过滤点
	filtered, err := adapter.ReadTelemetry(ctx, "truck-ecu-01", []string{"engine_rpm", "oil_pressure"})
	require.NoError(t, err)
	require.Len(t, filtered.Points, 2)

	// 读取不存在的设备报错
	_, err = adapter.ReadTelemetry(ctx, "ghost-device", nil)
	require.Error(t, err)
}

func TestCANAdapterWriteCommand(t *testing.T) {
	adapter := NewCANAdapter()
	ctx := context.Background()

	config := AdapterConfig{
		DeviceID: "can-valve-01",
		Values: map[string]any{
			"interface_name": "vcan0",
			"baud_rate":      float64(500000),
			"node_id":        float64(5),
		},
	}
	require.NoError(t, adapter.Connect(ctx, config))

	// 指令 1: set_relay
	cmdRelay := Command{
		DeviceID: "can-valve-01",
		Name:     "set_relay",
		Payload:  []byte(`{"state": true}`),
	}
	require.NoError(t, adapter.WriteCommand(ctx, cmdRelay))
	txFrame := adapter.LastTransmittedFrame()
	require.NotNil(t, txFrame)
	require.Equal(t, uint32(0x18EF0001), txFrame.ID)
	require.Equal(t, uint8(2), txFrame.DLC)
	require.Equal(t, byte(0x01), txFrame.Data[0]) // Opcode
	require.Equal(t, byte(0x01), txFrame.Data[1]) // State True

	// 指令 2: emergency_stop
	cmdEstop := Command{
		DeviceID: "can-valve-01",
		Name:     "emergency_stop",
	}
	require.NoError(t, adapter.WriteCommand(ctx, cmdEstop))
	estopFrame := adapter.LastTransmittedFrame()
	require.NotNil(t, estopFrame)
	require.Equal(t, uint32(0x18EF00FF), estopFrame.ID)
	require.Equal(t, uint8(1), estopFrame.DLC)
	require.Equal(t, byte(0xFF), estopFrame.Data[0])

	// 指令 3: 超出 CAN 8 字节限制
	oversizedCmd := Command{
		DeviceID: "can-valve-01",
		Name:     "raw_stream",
		Payload:  make([]byte, 16),
	}
	require.Error(t, adapter.WriteCommand(ctx, oversizedCmd))
}
