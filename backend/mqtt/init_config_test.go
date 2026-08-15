package mqtt

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func TestLoadConfigRequiresCredentials(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
		MqttConfig = Config{}
	})

	t.Run("missing user", func(t *testing.T) {
		viper.Reset()
		MqttConfig = Config{}
		viper.Set("mqtt.pass", "secret")

		err := loadConfig()
		if err == nil || !strings.Contains(err.Error(), "mqtt user is required") {
			t.Fatalf("loadConfig() error = %v, want mqtt user is required", err)
		}
	})

	t.Run("missing pass", func(t *testing.T) {
		viper.Reset()
		MqttConfig = Config{}
		viper.Set("mqtt.user", "device")

		err := loadConfig()
		if err == nil || !strings.Contains(err.Error(), "mqtt password is required") {
			t.Fatalf("loadConfig() error = %v, want mqtt password is required", err)
		}
	})
}

func TestLoadConfigDoesNotLogCredentials(t *testing.T) {
	viper.Reset()
	MqttConfig = Config{}
	var logs bytes.Buffer
	originalOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&logs)
	t.Cleanup(func() {
		viper.Reset()
		MqttConfig = Config{}
		logrus.SetOutput(originalOutput)
	})

	const (
		user = "sensitive-user"
		pass = "sensitive-pass"
	)
	viper.Set("mqtt.user", user)
	viper.Set("mqtt.pass", pass)

	if err := loadConfig(); err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if MqttConfig.User != user || MqttConfig.Pass != pass {
		t.Fatalf("credentials not loaded: user = %q, pass = %q", MqttConfig.User, MqttConfig.Pass)
	}
	if strings.Contains(logs.String(), user) || strings.Contains(logs.String(), pass) {
		t.Fatalf("loadConfig() logged credentials: %q", logs.String())
	}
}

func TestLoadConfigTelemetryBatchSize(t *testing.T) {
	tests := []struct {
		name      string
		batchSize *int
		want      int
	}{
		{name: "configured", batchSize: func() *int { value := 37; return &value }(), want: 37},
		{name: "default", want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			MqttConfig = Config{}
			t.Cleanup(func() {
				viper.Reset()
				MqttConfig = Config{}
			})

			viper.Set("mqtt.user", "device")
			viper.Set("mqtt.pass", "secret")
			if tt.batchSize != nil {
				viper.Set("mqtt.telemetry.batch_size", *tt.batchSize)
			}

			if err := loadConfig(); err != nil {
				t.Fatalf("loadConfig() error = %v", err)
			}
			if MqttConfig.Telemetry.BatchSize != tt.want {
				t.Fatalf("MqttConfig.Telemetry.BatchSize = %d, want %d", MqttConfig.Telemetry.BatchSize, tt.want)
			}
		})
	}
}
