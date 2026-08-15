package server

import (
	"sync/atomic"

	"github.com/DrmagicE/gmqtt/pkg/packets"
)

// PacketStats represents the statistics of MQTT Packet.
type PacketStats struct {
	BytesReceived PacketBytes
	ReceivedTotal PacketCount
	BytesSent     PacketBytes
	SentTotal     PacketCount
}

func (p *PacketStats) add(pt packets.Packet, receive bool) {
	b := packets.TotalBytes(pt)
	var bytes *PacketBytes
	var count *PacketCount
	if receive {
		bytes = &p.BytesReceived
		count = &p.ReceivedTotal
	} else {
		bytes = &p.BytesSent
		count = &p.SentTotal
	}
	switch pt.(type) {
	case *packets.Auth:
		atomic.AddUint64(&bytes.Auth, uint64(b))
		atomic.AddUint64(&count.Auth, 1)
	case *packets.Connect:
		atomic.AddUint64(&bytes.Connect, uint64(b))
		atomic.AddUint64(&count.Connect, 1)
	case *packets.Connack:
		atomic.AddUint64(&bytes.Connack, uint64(b))
		atomic.AddUint64(&count.Connack, 1)
	case *packets.Disconnect:
		atomic.AddUint64(&bytes.Disconnect, uint64(b))
		atomic.AddUint64(&count.Disconnect, 1)
	case *packets.Pingreq:
		atomic.AddUint64(&bytes.Pingreq, uint64(b))
		atomic.AddUint64(&count.Pingreq, 1)
	case *packets.Pingresp:
		atomic.AddUint64(&bytes.Pingresp, uint64(b))
		atomic.AddUint64(&count.Pingresp, 1)
	case *packets.Puback:
		atomic.AddUint64(&bytes.Puback, uint64(b))
		atomic.AddUint64(&count.Puback, 1)
	case *packets.Pubcomp:
		atomic.AddUint64(&bytes.Pubcomp, uint64(b))
		atomic.AddUint64(&count.Pubcomp, 1)
	case *packets.Publish:
		atomic.AddUint64(&bytes.Publish, uint64(b))
		atomic.AddUint64(&count.Publish, 1)
	case *packets.Pubrec:
		atomic.AddUint64(&bytes.Pubrec, uint64(b))
		atomic.AddUint64(&count.Pubrec, 1)
	case *packets.Pubrel:
		atomic.AddUint64(&bytes.Pubrel, uint64(b))
		atomic.AddUint64(&count.Pubrel, 1)
	case *packets.Suback:
		atomic.AddUint64(&bytes.Suback, uint64(b))
		atomic.AddUint64(&count.Suback, 1)
	case *packets.Subscribe:
		atomic.AddUint64(&bytes.Subscribe, uint64(b))
		atomic.AddUint64(&count.Subscribe, 1)
	case *packets.Unsuback:
		atomic.AddUint64(&bytes.Unsuback, uint64(b))
		atomic.AddUint64(&count.Unsuback, 1)
	case *packets.Unsubscribe:
		atomic.AddUint64(&bytes.Unsubscribe, uint64(b))
		atomic.AddUint64(&count.Unsubscribe, 1)
	}
	atomic.AddUint64(&bytes.Total, uint64(b))
	atomic.AddUint64(&count.Total, 1)
}

func (p *PacketStats) copy() *PacketStats {
	return &PacketStats{
		BytesReceived: p.BytesReceived.copy(),
		ReceivedTotal: p.ReceivedTotal.copy(),
		BytesSent:     p.BytesSent.copy(),
		SentTotal:     p.SentTotal.copy(),
	}
}

// PacketBytes represents total bytes of each in type have been received or sent.
type PacketBytes struct {
	Auth        uint64
	Connect     uint64
	Connack     uint64
	Disconnect  uint64
	Pingreq     uint64
	Pingresp    uint64
	Puback      uint64
	Pubcomp     uint64
	Publish     uint64
	Pubrec      uint64
	Pubrel      uint64
	Suback      uint64
	Subscribe   uint64
	Unsuback    uint64
	Unsubscribe uint64
	Total       uint64
}

func (p *PacketBytes) copy() PacketBytes {
	return PacketBytes{
		Auth:        atomic.LoadUint64(&p.Auth),
		Connect:     atomic.LoadUint64(&p.Connect),
		Connack:     atomic.LoadUint64(&p.Connack),
		Disconnect:  atomic.LoadUint64(&p.Disconnect),
		Pingreq:     atomic.LoadUint64(&p.Pingreq),
		Pingresp:    atomic.LoadUint64(&p.Pingresp),
		Puback:      atomic.LoadUint64(&p.Puback),
		Pubcomp:     atomic.LoadUint64(&p.Pubcomp),
		Publish:     atomic.LoadUint64(&p.Publish),
		Pubrec:      atomic.LoadUint64(&p.Pubrec),
		Pubrel:      atomic.LoadUint64(&p.Pubrel),
		Suback:      atomic.LoadUint64(&p.Suback),
		Subscribe:   atomic.LoadUint64(&p.Subscribe),
		Unsuback:    atomic.LoadUint64(&p.Unsuback),
		Unsubscribe: atomic.LoadUint64(&p.Unsubscribe),
		Total:       atomic.LoadUint64(&p.Total),
	}
}

// PacketCount represents total number of each in type have been received or sent.
type PacketCount = PacketBytes
