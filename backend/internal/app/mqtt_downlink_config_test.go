package app

import (
	"context"
	"testing"

	"aetherlink-iot/backend/internal/downlink"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func TestMQTTServiceEnabledRequiresExplicitOptIn(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	if mqttServiceEnabled() {
		t.Fatal("mqttServiceEnabled should default to false when mqtt.enabled is absent")
	}

	viper.Set("mqtt.enabled", false)
	if mqttServiceEnabled() {
		t.Fatal("mqttServiceEnabled should respect explicit false")
	}

	viper.Set("mqtt.enabled", true)
	if !mqttServiceEnabled() {
		t.Fatal("mqttServiceEnabled should respect explicit true")
	}
}

func TestMQTTServiceStartReturnsNilWhenDisabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("mqtt.enabled", false)

	service := NewMQTTService()
	if err := service.Start(); err != nil {
		t.Fatalf("Start returned error when mqtt disabled: %v", err)
	}
}

func TestDownlinkServiceStartReturnsNilWhenMQTTDisabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("mqtt.enabled", false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wrapper := &DownlinkServiceWrapper{
		bus:    downlink.NewBus(1),
		ctx:    ctx,
		cancel: cancel,
		logger: logrus.New(),
	}

	if err := wrapper.Start(); err != nil {
		t.Fatalf("Downlink Start returned error when mqtt disabled: %v", err)
	}
	if wrapper.handler != nil {
		t.Fatal("downlink handler should stay nil when mqtt is disabled")
	}
}
