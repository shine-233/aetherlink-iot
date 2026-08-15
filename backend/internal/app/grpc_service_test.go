package app

import (
	"testing"

	"github.com/spf13/viper"
)

func TestExternalTelemetryGRPCEnabledOnlyForExternalStores(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	for _, dbType := range []string{"", "NONE", "POSTGRESQL", "LOCAL"} {
		viper.Set("grpc.tptodb_type", dbType)
		if externalTelemetryGRPCEnabled() {
			t.Fatalf("externalTelemetryGRPCEnabled(%q) = true, want false", dbType)
		}
	}

	for _, dbType := range []string{"TSDB", "KINGBASE", "POLARDB", " tsdb "} {
		viper.Set("grpc.tptodb_type", dbType)
		if !externalTelemetryGRPCEnabled() {
			t.Fatalf("externalTelemetryGRPCEnabled(%q) = false, want true", dbType)
		}
	}
}

func TestGRPCServiceStartUsesLocalStorageWhenExternalIntegrationDisabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("grpc.tptodb_type", "NONE")
	viper.Set("grpc.tptodb_server", "")

	service := NewGRPCService()
	if err := service.Start(); err != nil {
		t.Fatalf("Start() error = %v, want local mode to start without external endpoint", err)
	}
	if service.initialized {
		t.Fatal("service initialized external client in local mode")
	}
	if err := service.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestGRPCServiceStartReturnsConfigurationErrorForEnabledExternalStore(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("grpc.tptodb_type", "TSDB")
	viper.Set("grpc.tptodb_server", "")

	service := NewGRPCService()
	if err := service.Start(); err == nil {
		t.Fatal("Start() error = nil, want missing external endpoint error")
	}
	if service.initialized {
		t.Fatal("service marked initialized after external client startup failure")
	}
}
