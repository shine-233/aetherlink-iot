package server

import (
	"net"
	"time"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

const (
	Connecting = iota
	Connected
)

// Client is the plugin-visible read/control contract for a live broker client.
type Client interface {
	ClientOptions() *ClientOptions
	SessionInfo() *gmqtt.Session
	Version() packets.Version
	ConnectedAt() time.Time
	Connection() net.Conn
	Close()
	Disconnect(disconnect *packets.Disconnect)
}
