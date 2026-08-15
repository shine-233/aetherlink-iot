package mqttdebug

import "time"

// Config defines the local MQTT debug runtime limits and its transport hooks.
// Zero or negative limits are replaced by bounded local defaults before use.
type Config struct {
	Broker                   string
	Username                 string
	Password                 string
	SessionTTL               time.Duration
	ConnectTimeout           time.Duration
	ActionTimeout            time.Duration
	MaxSessions              int
	MaxSessionsPerUser       int
	MaxSubscriptions         int
	MessageCapacity          int
	PayloadMaxBytes          int
	PublishMaxBytes          int
	MaxInboundPerSecond      int
	MaxInboundBytesPerSecond int
	OpenCooldown             time.Duration
	MaxCommandsPerSecond     int
	MaxPublishBytesPerSecond int
	MaxSnapshotsPerSecond    int
	TransportFactory         TransportFactory
	UplinkSource             UplinkSource
}

func withManagerDefaults(config Config) Config {
	if config.SessionTTL <= 0 {
		config.SessionTTL = 30 * time.Minute
	}
	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = 5 * time.Second
	}
	if config.ActionTimeout <= 0 {
		config.ActionTimeout = 5 * time.Second
	}
	if config.MaxSessions <= 0 {
		config.MaxSessions = 50
	}
	if config.MaxSessionsPerUser <= 0 {
		config.MaxSessionsPerUser = 3
	}
	if config.MaxSubscriptions <= 0 {
		config.MaxSubscriptions = 8
	}
	if config.MessageCapacity <= 0 {
		config.MessageCapacity = 200
	}
	if config.PayloadMaxBytes <= 0 {
		config.PayloadMaxBytes = 4096
	}
	if config.PublishMaxBytes <= 0 {
		config.PublishMaxBytes = 64 * 1024
	}
	if config.MaxInboundPerSecond <= 0 {
		config.MaxInboundPerSecond = 100
	}
	if config.MaxInboundBytesPerSecond <= 0 {
		config.MaxInboundBytesPerSecond = 256 * 1024
	}
	if config.OpenCooldown <= 0 {
		config.OpenCooldown = 2 * time.Second
	}
	if config.MaxCommandsPerSecond <= 0 {
		config.MaxCommandsPerSecond = 10
	}
	if config.MaxPublishBytesPerSecond <= 0 {
		config.MaxPublishBytesPerSecond = 256 * 1024
	}
	if config.MaxSnapshotsPerSecond <= 0 {
		config.MaxSnapshotsPerSecond = 4
	}
	return config
}
