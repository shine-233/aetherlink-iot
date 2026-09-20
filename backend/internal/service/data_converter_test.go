package service

import (
	"context"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/assert"
)

func TestHexBinaryConverterEngine(t *testing.T) {
	// Modbus RTU 格式帧: 01 03 04 00 64 00 c8
	// 01: 地址 1
	// 03: 功能码 03
	// 04: 字节数 4
	// 00 64: 温度 100 (x 0.1 -> 10.0)
	// 00 c8: 湿度 200 (x 0.1 -> 20.0)
	configStr := `[
		{"key": "addr", "offset": 0, "length": 1, "type": "uint8"},
		{"key": "temp", "offset": 3, "length": 2, "type": "int16", "scale": 0.1},
		{"key": "hum", "offset": 5, "length": 2, "type": "uint16", "scale": 0.1}
	]`

	req := &model.TestDataConverterReq{
		Type:          "UPLINK",
		ConverterMode: "HEX_BINARY",
		Payload:       "010304006400c8",
		Configuration: &configStr,
	}

	resp, err := executeHexBinaryConverter(req)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, uint64(1), resp.Telemetry["addr"])
	assert.InDelta(t, 10.0, resp.Telemetry["temp"], 0.001)
	assert.InDelta(t, 20.0, resp.Telemetry["hum"], 0.001)
}

func TestJsonPathConverterEngine(t *testing.T) {
	payload := `{
		"dev": "sensor-alpha",
		"metrics": {
			"temperature": 27.5,
			"humidity": 65.2,
			"rssi": -72
		},
		"metadata": {
			"firmware": "v2.1.0"
		}
	}`

	configStr := `{
		"device_name": "dev",
		"telemetry": {
			"temp": "metrics.temperature",
			"hum": "metrics.humidity",
			"rssi": "metrics.rssi"
		},
		"attributes": {
			"fw": "metadata.firmware"
		}
	}`

	req := &model.TestDataConverterReq{
		Type:          "UPLINK",
		ConverterMode: "JSON_PATH",
		Payload:       payload,
		Configuration: &configStr,
	}

	resp, err := executeJsonPathConverter(req)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "sensor-alpha", resp.DeviceName)
	assert.Equal(t, 27.5, resp.Telemetry["temp"])
	assert.Equal(t, 65.2, resp.Telemetry["hum"])
	assert.Equal(t, float64(-72), resp.Telemetry["rssi"])
	assert.Equal(t, "v2.1.0", resp.Attributes["fw"])
}

func TestDataConverterServiceValidation(t *testing.T) {
	svc := &DataConverterService{}
	_, err := svc.TestDataConverter(context.Background(), &model.TestDataConverterReq{
		Type:          "UPLINK",
		ConverterMode: "UNKNOWN_MODE",
		Payload:       "{}",
	}, &utils.UserClaims{})
	assert.Error(t, err)
}
