package service

import (
	"testing"

	"github.com/spf13/viper"
)

func TestOptionalIntegrationCapabilityStatesStayExplicit(t *testing.T) {
	checks := healthyDeploymentChecks()
	tests := []struct {
		name  string
		state runtimeDeploymentCapabilityState
		want  DeploymentCapabilityStatus
	}{
		{
			name: "disabled by default",
			want: DeploymentCapabilityDisabled,
		},
		{
			name: "enabled but not configured",
			state: runtimeDeploymentCapabilityState{
				thingsVisEnabled: true,
			},
			want: DeploymentCapabilityConfigurationRequired,
		},
		{
			name: "configured but not runtime probed",
			state: runtimeDeploymentCapabilityState{
				thingsVisEnabled:    true,
				thingsVisConfigured: true,
			},
			want: DeploymentCapabilityExternalBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := capabilityByID(t, buildRuntimeDeploymentCapabilities(tt.state, checks), "thingsvis")
			if got.Status != tt.want {
				t.Fatalf("thingsvis capability = %#v, want status %q", got, tt.want)
			}
		})
	}
}

func TestOptionalIntegrationRuntimeConfigCollection(t *testing.T) {
	oldSettings := viper.AllSettings()
	t.Cleanup(func() {
		viper.Reset()
		if err := viper.MergeConfigMap(oldSettings); err != nil {
			t.Errorf("restore viper settings: %v", err)
		}
	})

	viper.Reset()
	state := collectRuntimeDeploymentCapabilityState()
	if state.thingsVisEnabled || state.thingsVisConfigured || state.httpAdapterEnabled || state.httpAdapterConfigured {
		t.Fatalf("default optional integration state = %#v, want disabled", state)
	}

	viper.Set("integrations.thingsvis.enabled", true)
	viper.Set("integrations.thingsvis.configured", true)
	viper.Set("integrations.http_adapter.enabled", true)
	viper.Set("integrations.http_adapter.configured", true)
	state = collectRuntimeDeploymentCapabilityState()
	if !state.thingsVisEnabled || !state.thingsVisConfigured || !state.httpAdapterEnabled || !state.httpAdapterConfigured {
		t.Fatalf("configured optional integration state = %#v", state)
	}
}
