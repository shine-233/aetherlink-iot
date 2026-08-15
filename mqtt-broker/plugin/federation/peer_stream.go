package federation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func outgoingPeerMetadata(nodeName string, peerSecret string) metadata.MD {
	pairs := []string{metadataNodeNameKey, nodeName}
	if peerSecret != "" {
		pairs = append(pairs, metadataPeerSecretKey, peerSecret)
	}
	return metadata.Pairs(pairs...)
}

func (p *peer) serveEventStream() {
	timer := time.NewTimer(0)
	var reconnectCount int
	for {
		select {
		case <-p.exit:
			return
		case <-timer.C:
			err := p.serveStream(reconnectCount, timer)
			select {
			case <-p.exit:
				return
			default:
			}
			if err != nil {
				log.Error("stream broken, reconnecting", zap.Error(err),
					zap.Int("reconnect_count", reconnectCount))
				reconnectCount++
				continue
			}
			return
		}
	}
}

func (p *peer) initStream(client FederationClient, conn *grpc.ClientConn) (s *stream, err error) {
	if err := p.ensureStreamStartable(); err != nil {
		return nil, err
	}

	rpcCtx, cancel := p.streamOpenContext()
	defer cancel()

	sh, err := p.handshake(client, rpcCtx)
	if err != nil {
		return nil, fmt.Errorf("handshake error: %s", err.Error())
	}
	log.Info("handshake succeed", zap.String("remote_node", p.member.Name), zap.Bool("clean_start", sh.CleanStart))
	p.applyServerHello(sh)
	c, err := p.openClientStream(client, rpcCtx)
	if err != nil {
		return nil, err
	}
	s = newStream(p.queue, conn, c)

	if err := p.activateStream(s); err != nil {
		s.closeConn()
		return nil, err
	}
	return s, nil
}

func (p *peer) ensureStreamStartable() error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.ensureStreamStartableLocked()
}

func (p *peer) handshake(client FederationClient, rpcCtx context.Context) (*ServerHello, error) {
	return client.Hello(p.outgoingStreamContext(rpcCtx), &ClientHello{
		SessionId: p.sessionID,
	})
}

func (p *peer) openClientStream(client FederationClient, rpcCtx context.Context) (Federation_EventStreamClient, error) {
	return client.EventStream(p.outgoingStreamContext(rpcCtx))
}

func (p *peer) activateStream(s *stream) error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	if err := p.ensureStreamStartableLocked(); err != nil {
		return err
	}
	p.queue.open()
	p.state = peerStateStreaming
	p.stream = s
	return nil
}

func (p *peer) streamOpenContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), peerStreamOpenTimeout)
	if p.exit == nil {
		return ctx, cancel
	}
	go func() {
		select {
		case <-p.exit:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func (p *peer) outgoingStreamContext(rpcCtx context.Context) context.Context {
	md := outgoingPeerMetadata(p.localName, p.fed.peerSecret())
	return metadata.NewOutgoingContext(rpcCtx, md)
}

func (p *peer) ensureStreamStartableLocked() error {
	if p.state == peerStateStopped {
		return errors.New("peer has been stopped")
	}
	return nil
}

func (p *peer) serveStream(reconnectCount int, backoff *time.Timer) (err error) {
	defer func() {
		if err != nil {
			backoff.Reset(reconnectDelay(reconnectCount))
		}
	}()
	conn, client, err := p.connectClient()
	if err != nil {
		return err
	}
	// The federation wire contract is plaintext gRPC unless the plugin config
	// grows an explicit TLS option. Do not replace this with default TLS without
	// a config migration for existing clusters.
	defer conn.Close()
	s, err := p.initStream(client, conn)
	if err != nil {
		return err
	}
	return s.serve()
}

func (p *peer) connectClient() (*grpc.ClientConn, FederationClient, error) {
	addr, err := p.serverAddr()
	if err != nil {
		return nil, nil, err
	}
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return conn, NewFederationClient(conn), nil
}

func reconnectDelay(reconnectCount int) time.Duration {
	if reconnectCount == 0 {
		return 0
	}
	du := time.Duration(reconnectCount) * 500 * time.Millisecond
	if max := 2 * time.Second; du > max {
		return max
	}
	return du
}

func (p *peer) serverAddr() (string, error) {
	addr := p.member.Tags["fed_addr"]
	if addr == "" {
		return "", fmt.Errorf("federation peer %s has empty fed_addr tag", p.member.Name)
	}
	return addr, nil
}

func newStream(queue queue, conn *grpc.ClientConn, client Federation_EventStreamClient) *stream {
	return &stream{
		queue:  queue,
		conn:   conn,
		client: client,
		close:  make(chan struct{}),
	}
}

func (s *stream) serve() error {
	s.wg.Add(2)
	go s.runLoop(s.readLoop)
	go s.runLoop(s.sendEvents)
	s.wg.Wait()
	return s.err
}

func (s *stream) setError(err error) {
	s.errOnce.Do(func() {
		s.shutdown()
		if err != nil && err != io.EOF {
			log.Error("stream error", zap.Error(err))
			s.err = err
		}
	})
}

func (s *stream) shutdown() {
	if s.queue != nil {
		s.queue.close()
	}
	s.closeConn()
	if s.close != nil {
		close(s.close)
	}
}

func (s *stream) closeConn() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *stream) runLoop(run func() error) {
	var err error
	defer func() {
		if re := recover(); re != nil {
			err = errors.New(fmt.Sprint(re))
		}
		s.setError(err)
		s.wg.Done()
	}()
	err = run()
}

func (s *stream) readLoop() error {
	var resp *Ack
	for {
		select {
		case <-s.close:
			return nil
		default:
			var err error
			resp, err = s.client.Recv()
			if err != nil {
				return err
			}
			s.queue.ack(resp.EventId)
			if ce := log.Check(zapcore.DebugLevel, "event acked"); ce != nil {
				ce.Write(zap.Uint64("id", resp.EventId))
			}
		}
	}
}

func (s *stream) sendEvents() error {
	for {
		events := s.queue.fetchEvents()
		// stream has been closed
		if events == nil {
			return nil
		}
		for _, v := range events {
			err := s.client.Send(v)
			if err != nil {
				return err
			}
			if ce := log.Check(zapcore.DebugLevel, "event sent"); ce != nil {
				ce.Write(zap.String("event", v.String()))
			}
		}
	}
}
