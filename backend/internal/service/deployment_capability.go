package service

// DeploymentCapabilityCategory describes how a capability participates in a deployment.
type DeploymentCapabilityCategory string

const (
	DeploymentCapabilityCore             DeploymentCapabilityCategory = "core"
	DeploymentCapabilityLocalOptional    DeploymentCapabilityCategory = "local-optional"
	DeploymentCapabilityExternalOptional DeploymentCapabilityCategory = "external-optional"
)

// DeploymentCapabilityStatus makes disabled, blocked, and available capabilities distinguishable
// without exposing endpoints or credentials.
type DeploymentCapabilityStatus string

const (
	DeploymentCapabilityDisabled              DeploymentCapabilityStatus = "disabled"
	DeploymentCapabilityConfigurationRequired DeploymentCapabilityStatus = "configuration-required"
	DeploymentCapabilityBlocked               DeploymentCapabilityStatus = "blocked"
	DeploymentCapabilityExternalBlocked       DeploymentCapabilityStatus = "external-blocked"
	DeploymentCapabilityAvailable             DeploymentCapabilityStatus = "available"
)

// DeploymentCapabilityDefinition is the stable, non-secret identity of a deployable capability.
type DeploymentCapabilityDefinition struct {
	ID       string                       `json:"id"`
	Category DeploymentCapabilityCategory `json:"category"`
}

// DeploymentCapabilityState contains booleans only. Configuration values and credentials must not be added here.
type DeploymentCapabilityState struct {
	Enabled    bool `json:"enabled"`
	Configured bool `json:"configured"`
	Healthy    bool `json:"healthy"`
}

// DeploymentCapability is safe to serialize because it contains identity and normalized public state only.
type DeploymentCapability struct {
	ID         string                       `json:"id"`
	Category   DeploymentCapabilityCategory `json:"category"`
	Status     DeploymentCapabilityStatus   `json:"status"`
	Enabled    bool                         `json:"enabled"`
	Configured bool                         `json:"configured"`
	Healthy    bool                         `json:"healthy"`
}

var deploymentCapabilityCatalog = []DeploymentCapabilityDefinition{
	{ID: "postgres", Category: DeploymentCapabilityCore},
	{ID: "redis", Category: DeploymentCapabilityCore},
	{ID: "mqtt-broker", Category: DeploymentCapabilityCore},
	{ID: "native-visualization", Category: DeploymentCapabilityCore},
	// The repository keeps compatibility adapters for ThingsVis and the HTTP adapter,
	// but their production runtimes are external images and are never core dependencies.
	{ID: "thingsvis", Category: DeploymentCapabilityExternalOptional},
	{ID: "http-adapter", Category: DeploymentCapabilityExternalOptional},
	{ID: "market", Category: DeploymentCapabilityExternalOptional},
	{ID: "smtp", Category: DeploymentCapabilityExternalOptional},
	{ID: "map-provider", Category: DeploymentCapabilityExternalOptional},
	// Local PostgreSQL telemetry remains the default. This capability represents
	// only explicitly selected TSDB/KINGBASE/POLARDB gRPC compatibility runtimes.
	{ID: "external-telemetry-store", Category: DeploymentCapabilityExternalOptional},
	// Anonymous product usage telemetry is opt-in and uses an external PostHog runtime.
	// Disabled telemetry performs no outbound request and never blocks the core stack.
	{ID: "usage-telemetry", Category: DeploymentCapabilityExternalOptional},
}

// DeploymentCapabilityCatalog returns a copy so callers cannot mutate the built-in catalog.
func DeploymentCapabilityCatalog() []DeploymentCapabilityDefinition {
	return append([]DeploymentCapabilityDefinition(nil), deploymentCapabilityCatalog...)
}

// NewDeploymentCapability normalizes impossible states instead of reporting misleading health.
func NewDeploymentCapability(definition DeploymentCapabilityDefinition, state DeploymentCapabilityState) DeploymentCapability {
	status := DeploymentCapabilityAvailable
	if !state.Enabled {
		state.Configured = false
		state.Healthy = false
		status = DeploymentCapabilityDisabled
	} else if !state.Configured {
		state.Healthy = false
		status = DeploymentCapabilityConfigurationRequired
	} else if !state.Healthy {
		status = DeploymentCapabilityBlocked
		if definition.Category == DeploymentCapabilityExternalOptional {
			status = DeploymentCapabilityExternalBlocked
		}
	}

	return DeploymentCapability{
		ID:         definition.ID,
		Category:   definition.Category,
		Status:     status,
		Enabled:    state.Enabled,
		Configured: state.Configured,
		Healthy:    state.Healthy,
	}
}

// BuildDeploymentCapabilities builds a stable snapshot. Missing states are disabled; unknown IDs are ignored.
func BuildDeploymentCapabilities(states map[string]DeploymentCapabilityState) []DeploymentCapability {
	catalog := DeploymentCapabilityCatalog()
	capabilities := make([]DeploymentCapability, 0, len(catalog))
	for _, definition := range catalog {
		capabilities = append(capabilities, NewDeploymentCapability(definition, states[definition.ID]))
	}
	return capabilities
}
