// 文件用途：实现 GMQTT broker 的服务端运行时、监听生命周期、会话注册、离线队列、遗嘱消息和插件钩子装配。
// 核心逻辑：管理 server 状态机、TCP/WebSocket 接入、session/persistence 恢复、消息投递与停机清理。
// 使用注意：该文件位于 MQTT 协议敏感路径，任何行为改动都要保持 wire-level 兼容并补充 focused broker 测试。
// 重构建议：当前大块已明显收敛，后续优先细化 API/bootstrap 装配或继续下沉 client-service 之外的残余门面逻辑。
package server

import (
	"net"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/persistence/session"
	"github.com/DrmagicE/gmqtt/persistence/unack"

	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/retained"
)

var statusPanic = "invalid server status"

// server 运行状态，当前只区分初始化完成与已启动。
const (
	serverStatusInit = iota
	serverStatusStarted
)

var zaplog *zap.Logger

func init() {
	zaplog = zap.NewNop()
}

// LoggerWithField 派生带固定字段的 logger，插件通常用它追加插件名等上下文字段。
func LoggerWithField(fields ...zap.Field) *zap.Logger {
	return zaplog.With(fields...)
}

// server represents a mqtt server instance.
// Create a server by using New()
type server struct {
	wg       sync.WaitGroup
	initOnce sync.Once
	stopOnce sync.Once
	mu       sync.RWMutex //gard clients & offlineClients map
	status   int32        //server status
	// clients stores the  online clients
	clients map[string]*client
	// offlineClients store the expired time of all disconnected clients
	// with valid session(not expired). Key by clientID
	offlineClients  map[string]time.Time
	willMessage     map[string]*willMsg
	tcpListener     []net.Listener //tcp listeners
	websocketServer []*WsServer    //websocket serverStop
	errOnce         sync.Once
	err             error
	exitChan        chan struct{}
	exitedChan      chan struct{}

	retainedDB      retained.Store
	subscriptionsDB subscription.Store //store subscriptions

	persistence  Persistence
	queueStore   map[string]queue.Store
	unackStore   map[string]unack.Store
	sessionStore session.Store

	// guards config
	configMu             sync.RWMutex
	config               config.Config
	hooks                Hooks
	plugins              []Plugin
	statsManager         *statsManager
	publishService       Publisher
	newTopicAliasManager NewTopicAliasManager

	clientService *clientService
	apiRegistrar  *apiRegistrar
}

func (srv *server) checkStatus() {
	if srv.Status() != serverStatusInit {
		panic(statusPanic)
	}
}

func uint16P(v uint16) *uint16 {
	return &v
}
