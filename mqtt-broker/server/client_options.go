// client_options.go 定义单个 MQTT 客户端在连接协商完成后的运行时选项结构，
// 供 broker 内部状态机与插件层共享读取。
package server

import "github.com/DrmagicE/gmqtt/pkg/packets"

// ClientOptions stores negotiated runtime options for one MQTT client.
type ClientOptions struct {
	ClientID string
	Username string

	KeepAlive     uint16
	SessionExpiry uint32
	MaxInflight   uint16
	ReceiveMax    uint16

	ClientMaxPacketSize uint32
	ServerMaxPacketSize uint32

	ClientTopicAliasMax uint16
	ServerTopicAliasMax uint16

	RequestProblemInfo bool
	UserProperties     []*packets.UserProperty

	RetainAvailable      bool
	WildcardSubAvailable bool
	SubIDAvailable       bool
	SharedSubAvailable   bool

	AuthMethod []byte
}
