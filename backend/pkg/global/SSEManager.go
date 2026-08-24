// 文件用途：实现租户维度的 SSE 长连接管理和跨实例事件广播。
// 核心逻辑：本机用 map 保存租户客户端，通过 Redis Pub/Sub 接收事件并写入 Gin ResponseWriter。
// 关键注意事项：依赖全局 Redis，监听循环长期运行；客户端快照必须在锁内复制，网络写入不得占用全局锁。
// 重构建议：后续可加入 context 取消和 Redis 客户端注入，使监听循环可测试并支持优雅退出。
package global

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-basic/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// InitSSEManager 初始化全局 SSE 管理器并开始监听 Redis Pub/Sub 事件。
// 重复调用会先停止旧实例的监听循环再重建，避免旧监听协程成为无法回收的孤儿。
// 注意：ListenForEvents 会阻塞到停止为止，调用方需以 go 关键字拉起。
func InitSSEManager() {
	if TPSSEManager != nil {
		TPSSEManager.StopListen()
	}
	TPSSEManager = NewSSEManager()
	TPSSEManager.ListenForEvents()
}

// SSEManager 基于 Redis Pub/Sub 的多实例 SSE 管理器，按租户维度分发事件
type SSEManager struct {
	redisClient *redis.Client
	clients     map[string]map[string]*SSEClient // map[tenantID]map[userID]*SSEClient
	mutex       sync.RWMutex

	// 监听循环生命周期：listenMu 保护以下字段，listening 为 true 表示循环在运行。
	listenMu   sync.Mutex
	listening  bool
	stopListen context.CancelFunc
	listenDone chan struct{}

	// pubSubFactory 默认基于 redisClient；测试可注入假实现以验证启停语义。
	pubSubFactory wsPubSubFactory
}

// SSEClient 表示一个 SSE 长连接客户端
type SSEClient struct {
	TenantID string
	UserID   string
	Writer   gin.ResponseWriter
	ClientID string
	writeMu  sync.Mutex
}

// SSEEvent 是通过 Redis Pub/Sub 分发的 SSE 事件
type SSEEvent struct {
	Type     string `json:"type"`
	Message  any    `json:"message"`
	TenantID string `json:"tenant_id"`
}

// NewSSEManager 创建 SSE 管理器
func NewSSEManager() *SSEManager {
	manager := &SSEManager{
		redisClient: REDIS,
		clients:     make(map[string]map[string]*SSEClient),
	}
	manager.pubSubFactory = func(ctx context.Context, pattern string) wsPubSubReceiver {
		return manager.redisClient.PSubscribe(ctx, pattern)
	}
	return manager
}

// AddClient 注册一个 SSE 客户端，返回 clientID 用于后续移除
func (m *SSEManager) AddClient(tenantID, userID string, writer gin.ResponseWriter) string {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	clientID := uuid.New()

	if _, ok := m.clients[tenantID]; !ok {
		m.clients[tenantID] = make(map[string]*SSEClient)
	}
	m.clients[tenantID][clientID] = &SSEClient{
		TenantID: tenantID,
		UserID:   userID,
		ClientID: clientID,
		Writer:   writer,
	}

	return clientID
}

// RemoveClient 移除指定的 SSE 客户端，并在租户无客户端时清理租户表
func (m *SSEManager) RemoveClient(tenantID, clientID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if tenantClients, ok := m.clients[tenantID]; ok {
		delete(tenantClients, clientID)
		if len(tenantClients) == 0 {
			delete(m.clients, tenantID)
		}
	}
}

// clientsForTenant 在锁内复制客户端指针，避免网络写入期间阻塞所有连接注册和移除。
func (m *SSEManager) clientsForTenant(tenantID string) []*SSEClient {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tenantClients := m.clients[tenantID]
	clients := make([]*SSEClient, 0, len(tenantClients))
	for _, client := range tenantClients {
		clients = append(clients, client)
	}
	return clients
}

// dispatchEvent 将事件写入本实例的租户客户端；无效 writer 或写失败的客户端会从管理器中移除。
func (m *SSEManager) dispatchEvent(event SSEEvent) {
	for _, client := range m.clientsForTenant(event.TenantID) {
		client.writeMu.Lock()
		flusher, ok := client.Writer.(http.Flusher)
		if !ok {
			client.writeMu.Unlock()
			m.RemoveClient(client.TenantID, client.ClientID)
			continue
		}
		_, err := fmt.Fprintf(client.Writer, "event: %s\ndata: %s\n\n", event.Type, event.Message)
		if err == nil {
			flusher.Flush()
		}
		client.writeMu.Unlock()
		if err != nil {
			m.RemoveClient(client.TenantID, client.ClientID)
		}
	}
}

// BroadcastEventToTenant 通过 Redis Pub/Sub 向指定租户的所有实例广播 SSE 事件
func (m *SSEManager) BroadcastEventToTenant(tenantID string, event SSEEvent) error {
	event.TenantID = tenantID
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}
	logrus.Debugf("发送SSE事件: %v", event)
	return m.redisClient.Publish(context.Background(), "sse:tenant:"+tenantID, string(eventJSON)).Err()
}

// BroadcastSSEEventToTenant safely publishes an SSE event through the global manager.
func BroadcastSSEEventToTenant(tenantID string, event SSEEvent) error {
	if TPSSEManager == nil {
		return fmt.Errorf("SSE manager is not initialized")
	}
	if TPSSEManager.redisClient == nil {
		return fmt.Errorf("SSE Redis client is not initialized")
	}
	return TPSSEManager.BroadcastEventToTenant(tenantID, event)
}

// ListenForEvents 阻塞运行 Redis Pub/Sub 的 SSE 事件监听循环并分发给本实例客户端，
// 使用指数退避重连。幂等守卫：已在运行时直接返回，避免重复初始化叠加第二个监听协程。
func (m *SSEManager) ListenForEvents() {
	m.listenMu.Lock()
	if m.listening {
		m.listenMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	m.listening = true
	m.stopListen = cancel
	m.listenDone = done
	factory := m.pubSubFactory
	m.listenMu.Unlock()

	defer func() {
		cancel()
		m.listenMu.Lock()
		m.listening = false
		m.stopListen = nil
		m.listenDone = nil
		m.listenMu.Unlock()
		close(done)
	}()

	if factory == nil && m.redisClient != nil {
		factory = func(ctx context.Context, pattern string) wsPubSubReceiver {
			return m.redisClient.PSubscribe(ctx, pattern)
		}
	}
	if factory == nil {
		logrus.Error("SSE manager cannot listen: redis client is not initialized")
		return
	}

	logrus.Info("SSE manager started listening for Redis Pub/Sub events")
	const (
		initialBackoff = 1 * time.Second
		maxBackoff     = 30 * time.Second
	)
	backoff := initialBackoff

	for {
		pubsub := factory(ctx, "sse:tenant:*")
		for {
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				_ = pubsub.Close()
				if ctx.Err() != nil {
					return
				}
				logrus.WithError(err).Warnf("SSE: Redis pubsub error, reconnecting in %v", backoff)
				if !waitPubSubBackoff(ctx, backoff) {
					return
				}
				backoff = min(backoff*2, maxBackoff)
				break
			}
			backoff = initialBackoff

			var event SSEEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				logrus.Errorf("Failed to unmarshal event: %v", err)
				continue
			}

			m.dispatchEvent(event)
		}
	}
}

// StopListen 取消监听循环并等待其退出；未启动或已停止时为幂等空操作。
// 有界等待防止异常阻塞拖垮停机序列（与既有 worker 的停止语义一致）。
func (m *SSEManager) StopListen() {
	m.listenMu.Lock()
	cancel := m.stopListen
	done := m.listenDone
	m.stopListen = nil
	m.listenDone = nil
	m.listening = false
	m.listenMu.Unlock()

	if cancel == nil || done == nil {
		return
	}
	cancel()

	timer := time.NewTimer(managerListenStopTimeout)
	defer timer.Stop()
	select {
	case <-done:
		logrus.Info("SSE manager listener stopped")
	case <-timer.C:
		logrus.Warn("SSE manager listener stop timed out")
	}
}

// StopSSEManagerListener 停止全局 SSE 管理器的监听循环；未初始化时空操作。
// 供应用优雅停机序列调用：该监听协程不经 ServiceManager 托管，
// 必须在关闭 Redis 客户端之前显式停止。
func StopSSEManagerListener() {
	if TPSSEManager != nil {
		TPSSEManager.StopListen()
	}
}
