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

// InitSSEManager 初始化全局 SSE 管理器并开始监听 Redis Pub/Sub 事件
func InitSSEManager() {
	TPSSEManager = NewSSEManager()
	TPSSEManager.ListenForEvents()
}

// SSEManager 基于 Redis Pub/Sub 的多实例 SSE 管理器，按租户维度分发事件
type SSEManager struct {
	redisClient *redis.Client
	clients     map[string]map[string]*SSEClient // map[tenantID]map[userID]*SSEClient
	mutex       sync.RWMutex
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
	return &SSEManager{
		redisClient: REDIS,
		clients:     make(map[string]map[string]*SSEClient),
	}
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

// ListenForEvents 监听 Redis Pub/Sub 的 SSE 事件并分发给本实例的客户端，使用指数退避重连
func (m *SSEManager) ListenForEvents() {
	const (
		initialBackoff = 1 * time.Second
		maxBackoff     = 30 * time.Second
	)
	backoff := initialBackoff

	for {
		pubsub := m.redisClient.PSubscribe(context.Background(), "sse:tenant:*")
		for {
			msg, err := pubsub.ReceiveMessage(context.Background())
			if err != nil {
				logrus.WithError(err).Warnf("SSE: Redis pubsub error, reconnecting in %v", backoff)
				time.Sleep(backoff)
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
		pubsub.Close()
	}
}
