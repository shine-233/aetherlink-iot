// 文件用途：设备综合健康度评估（Device Health Score）算法单元测试。
// 核心逻辑：验证多维扣分模型、时间窗口衰减、严重度加权、分值边界截断与等级分类。
package service

import (
	model "aetherlink-iot/backend/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestComputeDeviceHealth_Healthy(t *testing.T) {
	devName := "Smart Power Meter 01"
	device := &model.Device{
		ID:           "dev-test-001",
		Name:         &devName,
		DeviceNumber: "SN-PM-001",
		IsOnline:     1,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
	}

	scoreEntity, detail := ComputeDeviceHealth(device, []*model.AlarmHistory{})

	assert.Equal(t, 100.0, scoreEntity.Score)
	assert.Equal(t, model.HealthStatusHealthy, scoreEntity.HealthStatus)
	assert.Equal(t, 0.0, scoreEntity.AlarmPenalty)
	assert.Equal(t, 0.0, scoreEntity.OfflinePenalty)
	assert.Equal(t, 0.0, scoreEntity.AnomalyPenalty)
	assert.True(t, detail.IsOnline)
	assert.Equal(t, 0, detail.ActiveAlarmCount)
	assert.Contains(t, detail.Suggestions[0], "稳定基线")
}

func TestComputeDeviceHealth_Alarms(t *testing.T) {
	devName := "Boiler Temp Sensor"
	device := &model.Device{
		ID:           "dev-test-002",
		Name:         &devName,
		DeviceNumber: "SN-BOILER-002",
		IsOnline:     1,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
	}

	alarms := []*model.AlarmHistory{
		{
			ID:          "alarm-01",
			Name:        "High Temp Alarm",
			AlarmStatus: "H", // -30
			CreateAt:    time.Now().Add(-1 * time.Hour),
		},
		{
			ID:          "alarm-02",
			Name:        "Pressure Warning",
			AlarmStatus: "M", // -15
			CreateAt:    time.Now().Add(-30 * time.Minute),
		},
	}

	scoreEntity, detail := ComputeDeviceHealth(device, alarms)

	assert.Equal(t, 45.0, scoreEntity.AlarmPenalty)
	assert.Equal(t, 55.0, scoreEntity.Score)
	assert.Equal(t, model.HealthStatusWarning, scoreEntity.HealthStatus)
	assert.Equal(t, 2, detail.ActiveAlarmCount)
	assert.Equal(t, 2, len(detail.ActiveAlarms))
}

func TestComputeDeviceHealth_OfflineDecay(t *testing.T) {
	devName := "Gateway Edge 03"
	offlineTime := time.Now().Add(-30 * time.Hour) // > 24 hours, < 7 days -> -35
	device := &model.Device{
		ID:              "dev-test-003",
		Name:            &devName,
		DeviceNumber:    "SN-GW-003",
		IsOnline:        0,
		LastOfflineTime: &offlineTime,
		IsEnabled:       "enabled",
		ActivateFlag:    "active",
	}

	scoreEntity, detail := ComputeDeviceHealth(device, []*model.AlarmHistory{})

	assert.Equal(t, 35.0, scoreEntity.OfflinePenalty)
	assert.Equal(t, 65.0, scoreEntity.Score)
	assert.Equal(t, model.HealthStatusWarning, scoreEntity.HealthStatus)
	assert.False(t, detail.IsOnline)
	assert.GreaterOrEqual(t, detail.OfflineDurationSeconds, int64(30*3600))
}

func TestComputeDeviceHealth_CriticalAccumulationAndClamp(t *testing.T) {
	devName := "Faulty Transformer"
	longOffline := time.Now().Add(-10 * 24 * time.Hour) // > 7 days -> -50
	device := &model.Device{
		ID:              "dev-test-004",
		Name:            &devName,
		DeviceNumber:    "SN-TF-004",
		IsOnline:        0,
		LastOfflineTime: &longOffline,
		IsEnabled:       "disabled", // -15
		ActivateFlag:    "inactive", // -10
	}

	alarms := []*model.AlarmHistory{
		{ID: "a1", AlarmStatus: "H", CreateAt: time.Now()}, // -30
		{ID: "a2", AlarmStatus: "H", CreateAt: time.Now()}, // -30
		{ID: "a3", AlarmStatus: "M", CreateAt: time.Now()}, // -15
	} // alarm penalty capped at 70

	scoreEntity, detail := ComputeDeviceHealth(device, alarms)

	assert.Equal(t, 70.0, scoreEntity.AlarmPenalty)
	assert.Equal(t, 50.0, scoreEntity.OfflinePenalty)
	assert.Equal(t, 25.0, scoreEntity.AnomalyPenalty)
	// 100 - 70 - 50 - 25 = -45 -> clamped to 0.0
	assert.Equal(t, 0.0, scoreEntity.Score)
	assert.Equal(t, model.HealthStatusCritical, scoreEntity.HealthStatus)
	assert.Equal(t, 0.0, detail.Score)
}
