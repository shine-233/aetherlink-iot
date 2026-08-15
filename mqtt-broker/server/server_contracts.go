package server

import (
	"context"

	"github.com/DrmagicE/gmqtt/config"
)

// ServerRuntimeAccess groups runtime configuration, statistics, and lifecycle controls.
type ServerRuntimeAccess interface {
	// GetConfig returns the current broker config.
	GetConfig() config.Config
	// StatsManager returns the stats reader.
	StatsManager() StatsReader
	// Stop gracefully stops the broker.
	Stop(ctx context.Context) error
	// ApplyConfig replaces the runtime config.
	ApplyConfig(config config.Config)
}

// ServerBrokerServices exposes broker service facades used by plugins and admin APIs.
type ServerBrokerServices interface {
	// Publisher returns the internal publish service.
	Publisher() Publisher
	ClientService() ClientService
	SubscriptionService() SubscriptionService
	RetainedService() RetainedService
}

// ServerPluginRegistry exposes plugin and API registration state.
type ServerPluginRegistry interface {
	// Plugins returns enabled plugin instances.
	Plugins() []Plugin
	APIRegistrar() APIRegistrar
}

// Server exposes the public broker service contract.
type Server interface {
	ServerRuntimeAccess
	ServerBrokerServices
	ServerPluginRegistry
}
