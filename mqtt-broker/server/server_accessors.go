package server

import (
	"sync/atomic"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/retained"
)

func (srv *server) APIRegistrar() APIRegistrar {
	return srv.apiRegistrar
}

func (srv *server) Plugins() []Plugin {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	p := make([]Plugin, len(srv.plugins))
	copy(p, srv.plugins)
	return p
}

func (srv *server) RetainedService() RetainedService {
	return srv.retainedDB
}

func (srv *server) ClientService() ClientService {
	return srv.clientService
}

func (srv *server) ApplyConfig(config config.Config) {
	srv.configMu.Lock()
	defer srv.configMu.Unlock()
	srv.config = config
}

func (srv *server) SubscriptionService() SubscriptionService {
	return srv.subscriptionsDB
}

func (srv *server) RetainedStore() retained.Store {
	return srv.retainedDB
}

func (srv *server) Publisher() Publisher {
	return srv.publishService
}

// GetConfig returns the config of the server
func (srv *server) GetConfig() config.Config {
	srv.configMu.Lock()
	defer srv.configMu.Unlock()
	return srv.config
}

// StatsManager returns StatsReader
func (srv *server) StatsManager() StatsReader {
	return srv.statsManager
}

// Status returns the server status
func (srv *server) Status() int32 {
	return atomic.LoadInt32(&srv.status)
}

// Client returns the client for given clientID
func (srv *server) Client(clientID string) Client {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.clients[clientID]
}
