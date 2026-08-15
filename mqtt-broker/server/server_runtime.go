package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var (
	// ErrInvalWsMsgType [MQTT-6.0.0-1]
	ErrInvalWsMsgType = errors.New("invalid websocket message type")
)

// eventLoop 维护 broker 后台定时任务，目前主要负责离线 session 过期检查。
func (srv *server) eventLoop() {
	sessionExpireTimer := time.NewTicker(time.Second * 20)
	defer func() {
		sessionExpireTimer.Stop()
		srv.wg.Done()
	}()
	for {
		select {
		case <-srv.exitChan:
			return
		case <-sessionExpireTimer.C:
			srv.sessionExpireCheck()
		}

	}
}

// WsServer is used to build websocket server
type WsServer struct {
	Server   *http.Server
	Path     string // Url path
	CertFile string //TLS configration
	KeyFile  string //TLS configration
}

func (srv *server) serveTCP(l net.Listener) {
	defer func() {
		l.Close()
	}()
	var tempDelay time.Duration
	for {
		rw, err := acceptTCPConn(l, &tempDelay)
		if err != nil {
			return
		}
		if rw == nil {
			continue
		}

		// 禁用 TCP Keep-Alive（解决 15 秒自动发送 Keep-Alive 包的问题）
		if tcpConn, ok := rw.(*net.TCPConn); ok {
			// 禁用 TCP 层的 Keep-Alive，使用 MQTT 协议层的 Keep-Alive 即可
			_ = tcpConn.SetKeepAlive(false)
		}

		if !srv.allowAcceptedConn(rw) {
			continue
		}
		if err := srv.startAcceptedClient(rw); err != nil {
			return
		}
	}
}

func acceptTCPConn(l net.Listener, tempDelay *time.Duration) (net.Conn, error) {
	rw, err := l.Accept()
	if err == nil {
		*tempDelay = 0
		return rw, nil
	}
	if ne, ok := err.(net.Error); ok && ne.Temporary() {
		*tempDelay = nextTemporaryAcceptDelay(*tempDelay)
		time.Sleep(*tempDelay)
		return nil, nil
	}
	return nil, err
}

func nextTemporaryAcceptDelay(current time.Duration) time.Duration {
	if current == 0 {
		return 5 * time.Millisecond
	}
	current *= 2
	if current > time.Second {
		return time.Second
	}
	return current
}

func (srv *server) allowAcceptedConn(rw net.Conn) bool {
	if srv.hooks.OnAccept == nil {
		return true
	}
	if srv.hooks.OnAccept(context.Background(), rw) {
		return true
	}
	_ = rw.Close()
	return false
}

func (srv *server) startAcceptedClient(rw net.Conn) error {
	client, err := srv.newClient(rw)
	if err != nil {
		zaplog.Error("new client fail", zap.Error(err))
		return err
	}
	go client.serve()
	return nil
}

var defaultUpgrader = &websocket.Upgrader{
	ReadBufferSize:  readBufferSize,
	WriteBufferSize: writeBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	Subprotocols: []string{"mqtt"},
}

// 实现io.ReadWriter接口
// wsConn implements the io.readWriter
type wsConn struct {
	net.Conn
	c   *websocket.Conn
	buf []byte
	r   int // buf copy positions
}

func (ws *wsConn) Close() error {
	return ws.Conn.Close()
}

func (ws *wsConn) Read(p []byte) (n int, err error) {
	if ws.buf == nil {
		msgType, buf, err := ws.c.ReadMessage()
		if err != nil {
			return 0, err
		}
		if msgType != websocket.BinaryMessage {
			return 0, ErrInvalWsMsgType
		}
		ws.buf = buf
	}
	n = copy(p, ws.buf[ws.r:])
	ws.r += n
	// reset reader buffer
	if ws.r+1 >= len(ws.buf) {
		ws.buf = nil
		ws.r = 0
	}
	return
}

func (ws *wsConn) Write(p []byte) (n int, err error) {
	err = ws.c.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), err
}

func (srv *server) serveWebSocket(ws *WsServer) {
	var err error
	if ws.CertFile != "" && ws.KeyFile != "" {
		err = ws.Server.ListenAndServeTLS(ws.CertFile, ws.KeyFile)
	} else {
		err = ws.Server.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		srv.setError(fmt.Errorf("serveWebSocket error: %s", err.Error()))
	}
}

func (srv *server) newClient(c net.Conn) (*client, error) {
	srv.configMu.Lock()
	cfg := srv.config
	srv.configMu.Unlock()
	client := &client{
		server:        srv,
		rwc:           c,
		bufr:          newBufioReaderSize(c, readBufferSize),
		bufw:          newBufioWriterSize(c, writeBufferSize),
		close:         make(chan struct{}),
		closed:        make(chan struct{}),
		connected:     make(chan struct{}),
		error:         make(chan error, 1),
		in:            make(chan packets.Packet, 8),
		out:           make(chan packets.Packet, 8),
		status:        Connecting,
		opts:          &ClientOptions{},
		cleanWillFlag: false,
		config:        cfg,
		register:      srv.registerClient,
		unregister:    srv.unregisterClient,
		deliverMessage: func(srcClientID string, msg *gmqtt.Message, options subscription.IterationOptions) (matched bool) {
			srv.mu.Lock()
			defer srv.mu.Unlock()
			return srv.deliverMessage(srcClientID, msg, options)
		},
	}
	client.packetReader = packets.NewReader(client.bufr)
	client.packetWriter = packets.NewWriter(client.bufw)
	client.queueNotifier = &queueNotifier{
		dropHook: srv.hooks.OnMsgDropped,
		sts:      srv.statsManager,
		cli:      client,
	}
	client.setConnecting()

	return client, nil
}

func (srv *server) wsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := defaultUpgrader.Upgrade(w, r, nil)
		if err != nil {
			zaplog.Error("websocket upgrade error", zap.String("Msg", err.Error()))
			return
		}
		defer c.Close()
		conn := &wsConn{Conn: c.UnderlyingConn(), c: c}
		client, err := srv.newClient(conn)
		if err != nil {
			zaplog.Error("new client fail", zap.Error(err))
			return
		}
		client.serve()
	}
}

func (srv *server) setError(err error) {
	srv.errOnce.Do(func() {
		srv.err = err
		srv.exit()
	})
}

// Run starts the mqtt server.
func (srv *server) Run() (err error) {
	err = srv.Init()
	if err != nil {
		return err
	}
	var tcps []string
	var ws []string
	for _, v := range srv.tcpListener {
		tcps = append(tcps, v.Addr().String())
	}
	for _, v := range srv.websocketServer {
		ws = append(ws, v.Server.Addr)
	}
	zaplog.Info("gmqtt server started", zap.Strings("tcp server listen on", tcps), zap.Strings("websocket server listen on", ws))

	atomic.StoreInt32(&srv.status, serverStatusStarted)
	srv.wg.Add(2)
	go srv.eventLoop()
	go srv.serveAPIServer()
	for _, ln := range srv.tcpListener {
		go srv.serveTCP(ln)
	}
	for _, server := range srv.websocketServer {
		mux := http.NewServeMux()
		mux.Handle(server.Path, srv.wsHandler())
		server.Server.Handler = mux
		go srv.serveWebSocket(server)
	}
	srv.wg.Wait()
	<-srv.exitedChan
	return srv.err
}

// Stop gracefully stops the mqtt server by the following steps:
//  1. Closing all opening TCP listeners and shutting down all opening websocket servers
//  2. Closing all idle connections
//  3. Waiting for all connections have been closed
//  4. Triggering OnStop()
func (srv *server) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var err error
	srv.stopOnce.Do(func() {
		zaplog.Info("stopping gmqtt server")
		defer func() {
			defer close(srv.exitedChan)
			zaplog.Info("server stopped")
		}()
		srv.exit()

		for _, l := range srv.tcpListener {
			l.Close()
		}
		for _, ws := range srv.websocketServer {
			ws.Server.Shutdown(ctx)
		}
		// Snapshot client close channels under srv.mu, then wait outside the
		// lock. Each client shutdown re-enters unregisterClient and needs srv.mu.
		srv.mu.Lock()
		chs := make([]chan struct{}, len(srv.clients))
		i := 0
		for _, c := range srv.clients {
			chs[i] = c.closed
			i++
			c.Close()
		}
		srv.mu.Unlock()

		done := make(chan struct{})
		if len(chs) != 0 {
			go func() {
				for _, v := range chs {
					<-v
				}
				close(done)
			}()
		} else {
			close(done)
		}

		select {
		case <-ctx.Done():
			zaplog.Warn("server stop timeout, force exit", zap.String("error", ctx.Err().Error()))
			err = ctx.Err()
		case <-done:
		}

		for _, v := range srv.plugins {
			zaplog.Info("unloading plugin", zap.String("name", v.Name()))
			err := v.Unload()
			if err != nil {
				zaplog.Warn("plugin unload error", zap.String("error", err.Error()))
			}
		}
		if srv.hooks.OnStop != nil {
			srv.hooks.OnStop(context.Background())
		}
	})
	return err
}
