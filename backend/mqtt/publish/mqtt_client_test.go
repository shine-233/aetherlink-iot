package publish

import (
	"errors"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func TestConfigurePublisherClientCallbacksLeavesReconnectToPaho(t *testing.T) {
	oldClient := mqttClient
	mqttClient = nil
	t.Cleanup(func() { mqttClient = oldClient })

	opts := mqtt.NewClientOptions()
	configurePublisherClientCallbacks(opts)

	if opts.OnConnectionLost == nil {
		t.Fatal("OnConnectionLost handler was not configured")
	}

	// The handler must remain safe even when there is no shared client. A
	// manual Disconnect/Connect loop would dereference mqttClient here and
	// race Paho's AutoReconnect in production.
	opts.OnConnectionLost(nil, errors.New("test connection loss"))
}
