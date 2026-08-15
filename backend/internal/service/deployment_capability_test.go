package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/global"

	"github.com/spf13/viper"
)

func TestDeploymentCapabilityCatalogHasStableIDsAndCategories(t *testing.T) {
	tests := []struct {
		id       string
		category DeploymentCapabilityCategory
	}{
		{id: "postgres", category: DeploymentCapabilityCore},
		{id: "redis", category: DeploymentCapabilityCore},
		{id: "mqtt-broker", category: DeploymentCapabilityCore},
		{id: "native-visualization", category: DeploymentCapabilityCore},
		{id: "thingsvis", category: DeploymentCapabilityExternalOptional},
		{id: "http-adapter", category: DeploymentCapabilityExternalOptional},
		{id: "market", category: DeploymentCapabilityExternalOptional},
		{id: "smtp", category: DeploymentCapabilityExternalOptional},
		{id: "map-provider", category: DeploymentCapabilityExternalOptional},
		{id: "external-telemetry-store", category: DeploymentCapabilityExternalOptional},
		{id: "usage-telemetry", category: DeploymentCapabilityExternalOptional},
	}

	catalog := DeploymentCapabilityCatalog()
	if len(catalog) != len(tests) {
		t.Fatalf("catalog length = %d, want %d", len(catalog), len(tests))
	}

	seen := make(map[string]struct{}, len(catalog))
	for index, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := catalog[index]
			if got.ID != tt.id || got.Category != tt.category {
				t.Fatalf("catalog[%d] = %#v, want id %q category %q", index, got, tt.id, tt.category)
			}
			if _, exists := seen[got.ID]; exists {
				t.Fatalf("duplicate capability ID %q", got.ID)
			}
			seen[got.ID] = struct{}{}
		})
	}
}

func TestDeploymentCapabilityCatalogReturnsDefensiveCopy(t *testing.T) {
	first := DeploymentCapabilityCatalog()
	first[0].ID = "changed"

	second := DeploymentCapabilityCatalog()
	if second[0].ID != "postgres" {
		t.Fatalf("catalog was mutated through returned slice: %#v", second[0])
	}
}

func TestDeploymentCapabilityStateRules(t *testing.T) {
	definition := DeploymentCapabilityDefinition{ID: "fixture", Category: DeploymentCapabilityLocalOptional}
	tests := []struct {
		name  string
		state DeploymentCapabilityState
		want  DeploymentCapability
	}{
		{
			name:  "disabled clears configured and healthy",
			state: DeploymentCapabilityState{Enabled: false, Configured: true, Healthy: true},
			want: DeploymentCapability{
				ID: "fixture", Category: DeploymentCapabilityLocalOptional, Status: DeploymentCapabilityDisabled,
			},
		},
		{
			name:  "enabled unconfigured clears healthy",
			state: DeploymentCapabilityState{Enabled: true, Configured: false, Healthy: true},
			want: DeploymentCapability{
				ID: "fixture", Category: DeploymentCapabilityLocalOptional,
				Status: DeploymentCapabilityConfigurationRequired, Enabled: true,
			},
		},
		{
			name:  "configured unhealthy is explicitly blocked",
			state: DeploymentCapabilityState{Enabled: true, Configured: true, Healthy: false},
			want: DeploymentCapability{
				ID: "fixture", Category: DeploymentCapabilityLocalOptional,
				Status: DeploymentCapabilityBlocked, Enabled: true, Configured: true,
			},
		},
		{
			name:  "enabled configured healthy is available",
			state: DeploymentCapabilityState{Enabled: true, Configured: true, Healthy: true},
			want: DeploymentCapability{
				ID: "fixture", Category: DeploymentCapabilityLocalOptional,
				Status: DeploymentCapabilityAvailable, Enabled: true, Configured: true, Healthy: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewDeploymentCapability(definition, tt.state); got != tt.want {
				t.Fatalf("NewDeploymentCapability() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDeploymentCapabilitySnapshotUsesCatalogAndIgnoresUnknownStates(t *testing.T) {
	got := BuildDeploymentCapabilities(map[string]DeploymentCapabilityState{
		"postgres": {Enabled: true, Configured: true, Healthy: true},
		"unknown":  {Enabled: true, Configured: true, Healthy: true},
	})

	if len(got) != len(DeploymentCapabilityCatalog()) {
		t.Fatalf("snapshot length = %d, want catalog length", len(got))
	}
	if got[0].ID != "postgres" || !got[0].Enabled || !got[0].Configured || !got[0].Healthy {
		t.Fatalf("postgres state = %#v, want enabled configured healthy", got[0])
	}
	for _, capability := range got {
		if capability.ID == "unknown" {
			t.Fatal("unknown state ID leaked into capability snapshot")
		}
		if capability.ID == "redis" && (capability.Enabled || capability.Configured || capability.Healthy) {
			t.Fatalf("missing redis state = %#v, want all false", capability)
		}
	}
}

func TestDeploymentCapabilityJSONContainsOnlyPublicBooleanState(t *testing.T) {
	const secretCanary = "capability-secret-canary"
	capabilities := BuildDeploymentCapabilities(map[string]DeploymentCapabilityState{
		"smtp":       {Enabled: true, Configured: true, Healthy: true},
		secretCanary: {Enabled: true, Configured: true, Healthy: true},
	})

	encoded, err := json.Marshal(capabilities)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var records []map[string]any
	if err := json.Unmarshal(encoded, &records); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	wantKeys := map[string]struct{}{
		"id": {}, "category": {}, "status": {}, "enabled": {}, "configured": {}, "healthy": {},
	}
	for _, fields := range records {
		if len(fields) != len(wantKeys) {
			t.Fatalf("serialized fields = %#v, want only public capability fields", fields)
		}
		for key := range fields {
			if _, ok := wantKeys[key]; !ok {
				t.Fatalf("unexpected serialized field %q", key)
			}
		}
	}

	lowerJSON := strings.ToLower(string(encoded))
	for _, sensitive := range []string{"secret", "password", "token", "credential", "api_key", "apikey", "endpoint", secretCanary} {
		if strings.Contains(lowerJSON, strings.ToLower(sensitive)) {
			t.Fatalf("serialized capability contains sensitive field or value %q: %s", sensitive, encoded)
		}
	}
}

func TestDeploymentHealthReportContractIncludesCapabilities(t *testing.T) {
	encoded, err := json.Marshal(DeploymentHealthReport{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	wantKeys := []string{"service", "status", "version", "timestamp", "checks", "guidance", "capabilities"}
	gotKeys := make([]string, 0, len(fields))
	for _, key := range wantKeys {
		if _, ok := fields[key]; !ok {
			t.Fatalf("DeploymentHealthReport JSON missing field %q: %s", key, encoded)
		}
		gotKeys = append(gotKeys, key)
	}
	if len(fields) != len(wantKeys) || !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("DeploymentHealthReport fields = %#v, want %v", fields, wantKeys)
	}
}

func capabilityByID(t *testing.T, capabilities []DeploymentCapability, id string) DeploymentCapability {
	t.Helper()
	for _, capability := range capabilities {
		if capability.ID == id {
			return capability
		}
	}
	t.Fatalf("capability %q not found", id)
	return DeploymentCapability{}
}

func healthyDeploymentChecks() map[string]DeploymentHealthCheck {
	return map[string]DeploymentHealthCheck{
		"database":      {OK: true},
		"db_migrations": {OK: true},
		"redis":         {OK: true},
		"mqtt":          {OK: true},
	}
}

func TestBuildRuntimeDeploymentCapabilities(t *testing.T) {
	state := runtimeDeploymentCapabilityState{
		postgresConfigured: true,
		redisConfigured:    true,
		mqttEnabled:        true,
		mqttConfigured:     true,
		marketEnabled:      true,
		marketConfigured:   true,
	}

	t.Run("healthy configured runtime", func(t *testing.T) {
		got := buildRuntimeDeploymentCapabilities(state, healthyDeploymentChecks())
		catalog := DeploymentCapabilityCatalog()
		if len(got) != len(catalog) {
			t.Fatalf("capabilities length = %d, want catalog length %d", len(got), len(catalog))
		}
		for _, id := range []string{"postgres", "redis", "mqtt-broker"} {
			capability := capabilityByID(t, got, id)
			if !capability.Enabled || !capability.Configured || !capability.Healthy {
				t.Fatalf("%s = %#v, want enabled configured healthy", id, capability)
			}
		}
		native := capabilityByID(t, got, "native-visualization")
		if !native.Enabled || !native.Configured || !native.Healthy {
			t.Fatalf("native-visualization = %#v, want enabled configured healthy", native)
		}
		market := capabilityByID(t, got, "market")
		if !market.Enabled || !market.Configured || market.Healthy {
			t.Fatalf("market = %#v, want enabled configured but unprobed", market)
		}
		for _, id := range []string{"thingsvis", "http-adapter", "smtp", "map-provider", "external-telemetry-store"} {
			capability := capabilityByID(t, got, id)
			if capability.Category != DeploymentCapabilityExternalOptional {
				t.Fatalf("optional capability %s category = %q, want %q", id, capability.Category, DeploymentCapabilityExternalOptional)
			}
			if capability.Enabled || capability.Configured || capability.Healthy {
				t.Fatalf("optional capability %s = %#v, want conservative false state", id, capability)
			}
		}
	})

	t.Run("postgres migration failure", func(t *testing.T) {
		checks := healthyDeploymentChecks()
		checks["db_migrations"] = DeploymentHealthCheck{OK: false}
		if got := capabilityByID(t, buildRuntimeDeploymentCapabilities(state, checks), "postgres"); got.Healthy {
			t.Fatalf("postgres = %#v, want unhealthy when migrations fail", got)
		}
	})

	t.Run("status redis failure", func(t *testing.T) {
		checks := healthyDeploymentChecks()
		checks["status_redis"] = DeploymentHealthCheck{OK: false}
		if got := capabilityByID(t, buildRuntimeDeploymentCapabilities(state, checks), "redis"); got.Healthy {
			t.Fatalf("redis = %#v, want unhealthy when status redis fails", got)
		}
	})

	t.Run("mqtt disabled and unconfigured", func(t *testing.T) {
		disabled := state
		disabled.mqttEnabled = false
		disabled.mqttConfigured = true
		if got := capabilityByID(t, buildRuntimeDeploymentCapabilities(disabled, healthyDeploymentChecks()), "mqtt-broker"); got.Enabled || got.Configured || got.Healthy {
			t.Fatalf("disabled mqtt = %#v, want all false", got)
		}
		unconfigured := state
		unconfigured.mqttConfigured = false
		if got := capabilityByID(t, buildRuntimeDeploymentCapabilities(unconfigured, healthyDeploymentChecks()), "mqtt-broker"); !got.Enabled || got.Configured || got.Healthy {
			t.Fatalf("unconfigured mqtt = %#v, want enabled only", got)
		}
	})
}

func TestDeploymentCapabilityRuntimeCollectionAndMarketURL(t *testing.T) {
	oldSettings := viper.AllSettings()
	oldDB, oldRedis := global.DB, global.REDIS
	mqttHealthProbe.RLock()
	oldProbe := mqttHealthProbe.fn
	mqttHealthProbe.RUnlock()
	t.Cleanup(func() {
		viper.Reset()
		if err := viper.MergeConfigMap(oldSettings); err != nil {
			t.Errorf("restore viper settings: %v", err)
		}
		global.DB, global.REDIS = oldDB, oldRedis
		SetMQTTHealthProbe(oldProbe)
	})

	viper.Reset()
	global.DB, global.REDIS = nil, nil
	SetMQTTHealthProbe(nil)
	viper.Set("mqtt.broker", "tcp://broker:1883")
	viper.Set("market.base_url", marketFallbackBaseURL)
	state := collectRuntimeDeploymentCapabilityState()
	if state.mqttEnabled || state.mqttConfigured || state.marketEnabled || state.marketConfigured {
		t.Fatalf("default runtime state = %#v, want disabled optional integrations", state)
	}

	viper.Set("mqtt.enabled", true)
	state = collectRuntimeDeploymentCapabilityState()
	if !state.mqttEnabled || state.mqttConfigured {
		t.Fatalf("explicitly enabled but unprobed mqtt = %#v", state)
	}

	SetMQTTHealthProbe(func() bool { return true })
	viper.Set("market.enabled", true)
	viper.Set("market.base_url", "https://market.example.test/api")
	state = collectRuntimeDeploymentCapabilityState()
	if !state.mqttConfigured || !state.marketConfigured {
		t.Fatalf("configured runtime state = %#v", state)
	}

	for _, rawURL := range []string{"ftp://market.example.test", "https:///missing-host", "relative/path", marketFallbackBaseURL} {
		if isConfiguredMarketBaseURL(rawURL) {
			t.Fatalf("market URL %q unexpectedly configured", rawURL)
		}
	}

	viper.Set("grpc.tptodb_type", "NONE")
	viper.Set("grpc.tptodb_server", "")
	state = collectRuntimeDeploymentCapabilityState()
	localTelemetry := capabilityByID(t, buildRuntimeDeploymentCapabilities(state, healthyDeploymentChecks()), "external-telemetry-store")
	if localTelemetry.Status != DeploymentCapabilityDisabled {
		t.Fatalf("local telemetry capability = %#v, want disabled external store", localTelemetry)
	}

	viper.Set("grpc.tptodb_type", " tsdb ")
	state = collectRuntimeDeploymentCapabilityState()
	unconfiguredTelemetry := capabilityByID(t, buildRuntimeDeploymentCapabilities(state, healthyDeploymentChecks()), "external-telemetry-store")
	if unconfiguredTelemetry.Status != DeploymentCapabilityConfigurationRequired {
		t.Fatalf("external telemetry without endpoint = %#v, want configuration-required", unconfiguredTelemetry)
	}

	viper.Set("grpc.tptodb_server", "127.0.0.1:50052")
	state = collectRuntimeDeploymentCapabilityState()
	blockedTelemetry := capabilityByID(t, buildRuntimeDeploymentCapabilities(state, healthyDeploymentChecks()), "external-telemetry-store")
	if blockedTelemetry.Status != DeploymentCapabilityExternalBlocked {
		t.Fatalf("configured external telemetry = %#v, want blocked until probed", blockedTelemetry)
	}

	viper.Set("mqtt.enabled", false)
	viper.Set("market.enabled", false)
	state = collectRuntimeDeploymentCapabilityState()
	if state.mqttEnabled || state.mqttConfigured || state.marketEnabled || state.marketConfigured {
		t.Fatalf("explicitly disabled runtime state = %#v", state)
	}
}

func TestDeploymentHealthStatusStillUsesOnlyRequiredChecks(t *testing.T) {
	checks := map[string]DeploymentHealthCheck{
		"required": {Required: true, OK: true},
		"optional": {Required: false, OK: false},
	}
	if got := deploymentHealthStatus(checks); got != "ok" {
		t.Fatalf("status = %q, want ok when only optional check fails", got)
	}
	checks["required"] = DeploymentHealthCheck{Required: true, OK: false}
	if got := deploymentHealthStatus(checks); got != "down" {
		t.Fatalf("status = %q, want down when required check fails", got)
	}
}

func TestServerAddressHealthChecksFailClosedForLoopback(t *testing.T) {
	checks := map[string]DeploymentHealthCheck{}
	addServerAddressHealthChecks(checks, "http://127.0.0.1:8080", "localhost:1883")

	for _, key := range []string{"public_url", "mqtt_access_address"} {
		check, ok := checks[key]
		if !ok {
			t.Fatalf("missing server address check %q", key)
		}
		if check.OK || !check.Required {
			t.Fatalf("server address check %q = %#v, want required failure", key, check)
		}
	}
}

func TestServerAddressHealthChecksAcceptReachableAddresses(t *testing.T) {
	checks := map[string]DeploymentHealthCheck{}
	addServerAddressHealthChecks(checks, "https://console.example.test", "broker.example.test:1883")

	for _, key := range []string{"public_url", "mqtt_access_address"} {
		check, ok := checks[key]
		if !ok {
			t.Fatalf("missing server address check %q", key)
		}
		if !check.OK || !check.Required {
			t.Fatalf("server address check %q = %#v, want required success", key, check)
		}
	}
}

func TestServerAddressHealthChecksRejectPlaceholdersAndInvalidMQTTPorts(t *testing.T) {
	tests := []struct {
		name        string
		publicURL   string
		mqttAddress string
		key         string
	}{
		{name: "placeholder public url", publicURL: "https://example.com", mqttAddress: "broker.example.test:1883", key: "public_url"},
		{name: "placeholder mqtt host", publicURL: "https://console.example.test", mqttAddress: "example.com:1883", key: "mqtt_access_address"},
		{name: "non numeric mqtt port", publicURL: "https://console.example.test", mqttAddress: "broker.example.test:not-a-port", key: "mqtt_access_address"},
		{name: "out of range mqtt port", publicURL: "https://console.example.test", mqttAddress: "broker.example.test:65536", key: "mqtt_access_address"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks := map[string]DeploymentHealthCheck{}
			addServerAddressHealthChecks(checks, tt.publicURL, tt.mqttAddress)
			check := checks[tt.key]
			if check.OK || !check.Required {
				t.Fatalf("server address check %q = %#v, want required failure", tt.key, check)
			}
		})
	}
}

func TestServerModeEnabledAcceptsExplicitBooleanValues(t *testing.T) {
	for _, value := range []string{"1", "true", "yes", "on"} {
		t.Setenv("AETHERLINK_SERVER_MODE", value)
		if !serverModeEnabled() {
			t.Fatalf("serverModeEnabled() = false for %q", value)
		}
	}
	for _, value := range []string{"", "0", "false", "no", "off"} {
		t.Setenv("AETHERLINK_SERVER_MODE", value)
		if serverModeEnabled() {
			t.Fatalf("serverModeEnabled() = true for %q", value)
		}
	}
}

func TestCheckMQTTDisabledConfigurationIsNotRequired(t *testing.T) {
	oldSettings := viper.AllSettings()
	t.Cleanup(func() {
		viper.Reset()
		if err := viper.MergeConfigMap(oldSettings); err != nil {
			t.Errorf("restore viper settings: %v", err)
		}
	})

	viper.Reset()
	SetMQTTHealthProbe(nil)
	viper.Set("mqtt.enabled", false)

	got := checkMQTT()
	if !got.OK || got.Required {
		t.Fatalf("disabled mqtt health = %#v, want ok and not required", got)
	}

	viper.Set("mqtt.enabled", true)
	got = checkMQTT()
	if got.OK || !got.Required {
		t.Fatalf("enabled mqtt without probe = %#v, want required failure", got)
	}
}

func TestDeploymentHealthReportCloneIsolatesCapabilities(t *testing.T) {
	report := DeploymentHealthReport{Capabilities: BuildDeploymentCapabilities(nil)}
	clone := cloneDeploymentHealthReport(report)
	clone.Capabilities[0].ID = "mutated"
	if report.Capabilities[0].ID != "postgres" {
		t.Fatalf("source capabilities mutated through clone: %#v", report.Capabilities[0])
	}
}

func TestDeploymentHealthReportCapabilitySerializationDoesNotLeakRuntimeConfig(t *testing.T) {
	const canary = "canary-secret-config-url.example.test"
	report := DeploymentHealthReport{
		Capabilities: buildRuntimeDeploymentCapabilities(runtimeDeploymentCapabilityState{
			mqttEnabled: true, mqttConfigured: true, marketEnabled: true, marketConfigured: true,
		}, healthyDeploymentChecks()),
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded struct {
		Capabilities []map[string]any `json:"capabilities"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	wantFields := map[string]struct{}{
		"id": {}, "category": {}, "status": {}, "enabled": {}, "configured": {}, "healthy": {},
	}
	for _, fields := range decoded.Capabilities {
		if len(fields) != len(wantFields) {
			t.Fatalf("capability fields = %#v, want exactly six", fields)
		}
		for field := range fields {
			if _, ok := wantFields[field]; !ok {
				t.Fatalf("unexpected capability field %q: %#v", field, fields)
			}
		}
	}
	lowerJSON := strings.ToLower(string(encoded))
	for _, forbidden := range []string{"\"url\"", "\"host\"", "\"error\"", "\"detail\"", "\"config\"", "\"secret\"", canary} {
		if strings.Contains(lowerJSON, strings.ToLower(forbidden)) {
			t.Fatalf("serialized report capability leaks %q: %s", forbidden, encoded)
		}
	}
}
