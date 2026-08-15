package mqttadapter

import (
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

func TestConfigureMQTTClientCallbacksDefersResubscribeUntilReconnect(t *testing.T) {
	opts := mqtt.NewClientOptions()
	logger := logrus.New()
	callbackCalls := 0

	configureMQTTClientCallbacks(opts, "test-client", logger, func(mqtt.Client) {
		callbackCalls++
	})

	if opts.OnConnect == nil {
		t.Fatal("OnConnect handler was not configured")
	}

	// Paho invokes OnConnect for the initial connection too. The caller owns
	// the initial subscriptions after CreateMQTTClient returns, so this event
	// must not race those subscriptions.
	opts.OnConnect(nil)
	if callbackCalls != 0 {
		t.Fatalf("initial OnConnect callback calls = %d, want 0", callbackCalls)
	}

	opts.OnConnect(nil)
	if callbackCalls != 1 {
		t.Fatalf("first reconnect callback calls = %d, want 1", callbackCalls)
	}
}
