package server

import (
	"bytes"

	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

func (client *client) dispatchIncomingPacket(packet packets.Packet) *codes.Error {
	switch pkt := packet.(type) {
	case *packets.Subscribe:
		return client.subscribeHandler(pkt)
	case *packets.Publish:
		return client.publishHandler(pkt)
	case *packets.Puback:
		return client.pubackHandler(pkt)
	case *packets.Pubrel:
		return client.pubrelHandler(pkt)
	case *packets.Pubrec:
		client.pubrecHandler(pkt)
	case *packets.Pubcomp:
		client.pubcompHandler(pkt)
	case *packets.Pingreq:
		client.pingreqHandler(pkt)
	case *packets.Unsubscribe:
		client.unsubscribeHandler(pkt)
	case *packets.Disconnect:
		return client.disconnectHandler(pkt)
	case *packets.Auth:
		return client.dispatchAuthPacket(pkt)
	default:
		return codes.ErrProtocol
	}
	return nil
}

func (client *client) dispatchAuthPacket(auth *packets.Auth) *codes.Error {
	if client.version != packets.Version5 {
		return codes.ErrProtocol
	}
	if !bytes.Equal(client.opts.AuthMethod, auth.Properties.AuthMethod) {
		return codes.ErrProtocol
	}
	return client.reAuthHandler(auth)
}
