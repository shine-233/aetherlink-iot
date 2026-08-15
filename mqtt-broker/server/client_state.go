package server

import (
	"bufio"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/persistence/unack"
	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

// client is the broker-internal protocol state holder for one network connection.
type client struct {
	connectedAt int64
	server      *server
	wg          sync.WaitGroup

	rwc          net.Conn
	bufr         *bufio.Reader
	bufw         *bufio.Writer
	packetReader *packets.Reader
	packetWriter *packets.Writer

	in        chan packets.Packet
	out       chan packets.Packet
	close     chan struct{}
	closed    chan struct{}
	connected chan struct{}
	status    int32

	forceRemoveSession int32
	error              chan error
	errOnce            sync.Once
	err                error

	opts    *ClientOptions
	session *gmqtt.Session

	cleanWillFlag bool
	disconnect    *packets.Disconnect

	topicAliasManager TopicAliasManager
	version           packets.Version
	aliasMapper       [][]byte

	serverQuotaMu             sync.Mutex
	serverReceiveMaximumQuota uint16

	config config.Config

	queueStore    queue.Store
	unackStore    unack.Store
	pl            *packetIDLimiter
	queueNotifier *queueNotifier

	register       func(connect *packets.Connect, client *client) (sessionResume bool, err error)
	unregister     func(client *client)
	deliverMessage func(srcClientID string, msg *gmqtt.Message, options subscription.IterationOptions) (matched bool)
}

func (client *client) ClientOptions() *ClientOptions {
	return client.opts
}

func (client *client) SessionInfo() *gmqtt.Session {
	return client.session
}

func (client *client) Version() packets.Version {
	return client.version
}

func (client *client) Disconnect(disconnect *packets.Disconnect) {
	client.write(disconnect)
}

func (client *client) ConnectedAt() time.Time {
	return time.Unix(atomic.LoadInt64(&client.connectedAt), 0)
}

func (client *client) Connection() net.Conn {
	return client.rwc
}

func (client *client) setConnecting() {
	atomic.StoreInt32(&client.status, Connecting)
}

func (client *client) setConnected(connectedAt time.Time) {
	atomic.StoreInt64(&client.connectedAt, connectedAt.Unix())
	atomic.StoreInt32(&client.status, Connected)
}

func (client *client) Status() int32 {
	return atomic.LoadInt32(&client.status)
}

func (client *client) IsConnected() bool {
	return client.Status() == Connected
}

func (client *client) setError(err error) {
	client.errOnce.Do(func() {
		if err != nil && err != io.EOF {
			zaplog.Warn("client connection closed",
				zap.String("client_id", client.opts.ClientID),
				zap.String("remote_addr", client.rwc.RemoteAddr().String()),
				zap.Error(err))
			client.err = err
			if client.version == packets.Version5 {
				if code, ok := err.(*codes.Error); ok && client.IsConnected() {
					client.write(&packets.Disconnect{
						Version: packets.Version5,
						Code:    code.Code,
						Properties: &packets.Properties{
							ReasonString: code.ReasonString,
							User:         kvsToProperties(code.UserProperties),
						},
					})
				}
			}
		}
		_ = client.rwc.Close()
		close(client.close)
	})
}

func (client *client) Close() {
	if client.rwc != nil {
		_ = client.rwc.Close()
	}
}
