// Packet I/O, bufio reuse, quota accounting, and packet size validation.
package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	readBufferSize  = 1024
	writeBufferSize = 1024
)

var (
	bufioReaderPool sync.Pool
	bufioWriterPool sync.Pool
)

func kvsToProperties(kvs []struct {
	K []byte
	V []byte
}) []packets.UserProperty {
	u := make([]packets.UserProperty, len(kvs))
	for k, v := range kvs {
		u[k].K = v.K
		u[k].V = v.V
	}
	return u
}

func newBufioReaderSize(r io.Reader, size int) *bufio.Reader {
	if v := bufioReaderPool.Get(); v != nil {
		br := v.(*bufio.Reader)
		br.Reset(r)
		return br
	}
	return bufio.NewReaderSize(r, size)
}

func putBufioReader(br *bufio.Reader) {
	br.Reset(nil)
	bufioReaderPool.Put(br)
}

func newBufioWriterSize(w io.Writer, size int) *bufio.Writer {
	if v := bufioWriterPool.Get(); v != nil {
		bw := v.(*bufio.Writer)
		bw.Reset(w)
		return bw
	}
	return bufio.NewWriterSize(w, size)
}

func putBufioWriter(bw *bufio.Writer) {
	bw.Reset(nil)
	bufioWriterPool.Put(bw)
}

// writeLoop serializes all outbound packets and preserves their wire order.
func (client *client) writeLoop() {
	var err error
	srv := client.server
	defer func() {
		if re := recover(); re != nil {
			err = errors.New(fmt.Sprint(re))
		}
		client.setError(err)
	}()
	for {
		select {
		case <-client.close:
			return
		case packet := <-client.out:
			client.prepareOutgoingPacket(packet)
			err = client.writePacket(packet)
			if err != nil {
				return
			}
			srv.statsManager.packetSent(packet, client.opts.ClientID)
			if _, ok := packet.(*packets.Disconnect); ok {
				_ = client.rwc.Close()
				return
			}
		}
	}
}

func (client *client) prepareOutgoingPacket(packet packets.Packet) {
	switch p := packet.(type) {
	case *packets.Publish:
		client.prepareOutgoingPublish(p)
	case *packets.Puback, *packets.Pubcomp:
		if client.version == packets.Version5 {
			client.addServerQuota()
		}
	case *packets.Pubrec:
		if client.version == packets.Version5 && p.Code >= codes.UnspecifiedError {
			client.addServerQuota()
		}
	}
}

func (client *client) writePacket(packet packets.Packet) error {
	if client.server.config.Log.DumpPacket {
		if ce := zaplog.Check(zapcore.DebugLevel, "sending packet"); ce != nil {
			ce.Write(
				zap.String("packet", packet.String()),
				zap.String("remote_addr", client.rwc.RemoteAddr().String()),
				zap.String("client_id", client.opts.ClientID),
			)
		}
	}

	return client.packetWriter.WriteAndFlush(packet)
}

func (client *client) addServerQuota() {
	client.serverQuotaMu.Lock()
	if client.serverReceiveMaximumQuota < client.opts.ReceiveMax {
		client.serverReceiveMaximumQuota++
	}
	client.serverQuotaMu.Unlock()
}

func (client *client) tryDecServerQuota() error {
	client.serverQuotaMu.Lock()
	defer client.serverQuotaMu.Unlock()
	if client.serverReceiveMaximumQuota == 0 {
		return codes.NewError(codes.RecvMaxExceeded)
	}
	client.serverReceiveMaximumQuota--
	return nil
}

// readLoop reads packets, refreshes keepalive deadlines, records inbound stats,
// and hands packets to readHandle after CONNECT processing opens client.connected.
func (client *client) readLoop() {
	var err error
	defer func() {
		if re := recover(); re != nil {
			err = errors.New(fmt.Sprint(re))
		}
		client.setError(err)
		close(client.in)
	}()
	for {

		var packet packets.Packet
		client.setReadDeadlineForKeepAlive()
		packet, err = client.packetReader.ReadPacket()
		if err != nil {
			if err != io.EOF && packet != nil {
				zaplog.Error("read error", zap.String("packet_type", reflect.TypeOf(packet).String()))
			}
			return
		}

		err = client.recordIncomingPublish(packet)
		if err != nil {
			return
		}
		client.in <- packet
		<-client.connected
		client.server.statsManager.packetReceived(packet, client.opts.ClientID)
		client.logReceivedPacket(packet)
	}
}

func (client *client) setReadDeadlineForKeepAlive() {
	if client.IsConnected() {
		if keepAlive := client.opts.KeepAlive; keepAlive != 0 {
			_ = client.rwc.SetReadDeadline(time.Now().Add(time.Duration(keepAlive/2+keepAlive) * time.Second))
		}
	}
}

func (client *client) recordIncomingPublish(packet packets.Packet) error {
	pub, ok := packet.(*packets.Publish)
	if !ok {
		return nil
	}
	client.server.statsManager.messageReceived(pub.Qos, client.opts.ClientID)
	if client.version == packets.Version5 && pub.Qos > packets.Qos0 {
		return client.tryDecServerQuota()
	}
	return nil
}

func (client *client) logReceivedPacket(packet packets.Packet) {
	if client.server.config.Log.DumpPacket {
		if ce := zaplog.Check(zapcore.DebugLevel, "received packet"); ce != nil {
			ce.Write(
				zap.String("packet", packet.String()),
				zap.String("remote_addr", client.rwc.RemoteAddr().String()),
				zap.String("client_id", client.opts.ClientID),
			)
		}
	}
}

func (client *client) checkMaxPacketSize(msg *gmqtt.Message) (valid bool) {
	totalBytes := msg.TotalBytes(packets.Version5)
	if client.opts.ClientMaxPacketSize != 0 && totalBytes > client.opts.ClientMaxPacketSize {
		return false
	}
	return true
}

func (client *client) write(packets packets.Packet) {
	select {
	case <-client.close:
		return
	case client.out <- packets:
	}
}

func (client *client) validateIncomingPacketSize(packet packets.Packet) *codes.Error {
	// v3 与 v5 统一执行后置校验；预分配阶段的上限已在 packetReader 中生效。
	if client.opts.ServerMaxPacketSize == 0 {
		return nil
	}
	if packets.TotalBytes(packet) <= client.opts.ServerMaxPacketSize {
		return nil
	}
	return codes.NewError(codes.PacketTooLarge)
}
