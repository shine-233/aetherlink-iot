// 文件用途：实现设备维度 WebSocket 订阅管理、字段过滤和本实例推送。
// 核心逻辑：维护 deviceID 到连接的内存索引，在 Redis 中记录订阅计数，并通过客户端发送队列非阻塞推送数据。
// 关键注意事项：Send 缓冲区满时会丢弃消息以避免阻塞；Redis 订阅计数失败会影响多实例感知。
// 生命周期：Redis Pub/Sub 监听循环支持 context 取消与幂等启停（StopListen），由应用停机序列显式调用。
// 重构建议：后续可把 Redis 操作和连接写入抽象为接口，并补充连接关闭和订阅续期的统一生命周期管理。
package global

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

var TPWSManager *WSManager

// InitWSManager 初始化 WebSocket 管理器。
// 重复调用会先停止旧实例的监听循环再重建，避免旧监听协程成为无法回收的孤儿。
func InitWSManager() {
	if TPWSManager != nil {
		TPWSManager.StopListen()
	}
	TPWSManager = NewWSManager()
	go TPWSManager.ListenForEvents()
}

// managerListenStopTimeout 等待监听协程退出的停机预算，超时只告警不阻断停机序列。
const managerListenStopTimeout = 5 * time.Second

// wsPubSubReceiver 抽象监听循环对 Pub/Sub 的最小依赖，供生命周期测试注入假实现。
type wsPubSubReceiver interface {
	ReceiveMessage(ctx context.Context) (*redis.Message, error)
	Close() error
}

// wsPubSubFactory 按订阅模式创建一个 Pub/Sub 接收器。
type wsPubSubFactory func(ctx context.Context, pattern string) wsPubSubReceiver

// waitPubSubBackoff 等待重连退避时间；ctx 取消时提前返回 false 以便立即退出循环。
func waitPubSubBackoff(ctx context.Context, backoff time.Duration) bool {
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// WSManager WebSocket 管理器
type WSManager struct {
	redisClient *redis.Client
	// 设备订阅: map[deviceID][connID]*WSClient
	deviceClients map[string]map[string]*WSClient
	mutex         sync.RWMutex

	// 监听循环生命周期：listenMu 保护以下字段，listening 为 true 表示循环在运行。
	listenMu   sync.Mutex
	listening  bool
	stopListen context.CancelFunc
	listenDone chan struct{}

	// pubSubFactory 默认基于 redisClient；测试可注入假实现以验证启停语义。
	pubSubFactory wsPubSubFactory
}

// WSClient WebSocket 客户端
type WSClient struct {
	DeviceID string
	TenantID string
	UserID   string
	Conn     *websocket.Conn
	ConnID   string
	MsgType  int // websocket.TextMessage or websocket.BinaryMessage
	Mu       *sync.Mutex
	Keys     []string // 订阅的字段（为空表示订阅全部）
	// Send 用于写入数据的缓冲管道，避免在多个goroutine中直接写Conn导致阻塞
	Send chan []byte

	// sendMu 与 closed 保护 Send 的关闭生命周期：
	// 发送方持读锁入队，关闭方持写锁置位并 close，杜绝 send on closed channel panic。
	sendMu sync.RWMutex
	closed bool
}

// TryEnqueue 非阻塞地把 payload 写入发送队列。
// 返回 false 表示客户端已关闭或缓冲区满，调用方应丢弃或走降级路径。
func (c *WSClient) TryEnqueue(payload []byte) bool {
	if c == nil || c.Send == nil {
		return false
	}
	c.sendMu.RLock()
	defer c.sendMu.RUnlock()
	if c.closed {
		return false
	}
	select {
	case c.Send <- payload:
		return true
	default:
		return false
	}
}

// CloseSend 幂等关闭发送队列，用于唤醒写入 goroutine 退出。
func (c *WSClient) CloseSend() {
	if c == nil || c.Send == nil {
		return
	}
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.Send)
}

// WSEvent WebSocket 事件
type WSEvent struct {
	DeviceID  string                 `json:"device_id"`
	TenantID  string                 `json:"tenant_id"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// NewWSManager 创建 WebSocket 管理器
func NewWSManager() *WSManager {
	manager := &WSManager{
		redisClient:   REDIS,
		deviceClients: make(map[string]map[string]*WSClient),
	}
	manager.pubSubFactory = func(ctx context.Context, pattern string) wsPubSubReceiver {
		return manager.redisClient.PSubscribe(ctx, pattern)
	}
	return manager
}

// SubscribeDevice 订阅设备
func (m *WSManager) SubscribeDevice(deviceID, connID string, client *WSClient) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 注册到内存 map
	if _, ok := m.deviceClients[deviceID]; !ok {
		m.deviceClients[deviceID] = make(map[string]*WSClient)
	}
	m.deviceClients[deviceID][connID] = client

	// 更新 Redis 订阅表
	ctx := context.Background()
	if err := m.redisClient.Incr(ctx, "ws:sub:"+deviceID).Err(); err != nil {
		logrus.WithError(err).Error("Failed to increment Redis subscription counter")
		return err
	}

	// 设置过期时间（5 分钟）
	if err := m.redisClient.Expire(ctx, "ws:sub:"+deviceID, 5*time.Minute).Err(); err != nil {
		logrus.WithError(err).Error("Failed to set Redis expiration")
	}

	logrus.Info("WebSocket client subscribed to device")

	return nil
}

// UnsubscribeDevice 取消订阅设备
func (m *WSManager) UnsubscribeDevice(deviceID, connID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 从内存 map 移除
	var removedClient *WSClient
	if clients, ok := m.deviceClients[deviceID]; ok {
		if c, ok2 := clients[connID]; ok2 {
			removedClient = c
		}
		delete(clients, connID)
		if len(clients) == 0 {
			delete(m.deviceClients, deviceID)
		}
	}

	// 先关闭写队列再处理 Redis：即使 Redis Decr 失败提前返回，
	// 也不会把写入 goroutine 永久留在 range Send 上泄漏。
	if removedClient != nil {
		removedClient.CloseSend()
	}

	// 更新 Redis 订阅表
	ctx := context.Background()
	count, err := m.redisClient.Decr(ctx, "ws:sub:"+deviceID).Result()
	if err != nil {
		logrus.WithError(err).Error("Failed to decrement Redis subscription counter")
		return err
	}

	// 如果订阅数为 0，删除 key
	if count <= 0 {
		m.redisClient.Del(ctx, "ws:sub:"+deviceID)
	}

	logrus.Info("WebSocket client unsubscribed from device")

	return nil
}

// RefreshSubscription 续期订阅（心跳）
func (m *WSManager) RefreshSubscription(deviceID string) error {
	ctx := context.Background()
	if err := m.redisClient.Expire(ctx, "ws:sub:"+deviceID, 5*time.Minute).Err(); err != nil {
		logrus.Error("Failed to refresh WebSocket subscription")
		return err
	}
	return nil
}

// PushToDevice 推送消息到设备订阅者（本实例）
func (m *WSManager) PushToDevice(deviceID string, data map[string]interface{}) {
	// 在读锁内拷贝订阅者快照后再释放锁遍历：
	// 直接在锁外迭代内层 map 会与 UnsubscribeDevice 的 delete 并发，
	// 触发 runtime 的 concurrent map read/write 致命错误。
	m.mutex.RLock()
	clientsMap, ok := m.deviceClients[deviceID]
	var clients []*WSClient
	if ok {
		clients = make([]*WSClient, 0, len(clientsMap))
		for _, client := range clientsMap {
			clients = append(clients, client)
		}
	}
	m.mutex.RUnlock()

	if !ok || len(clients) == 0 {
		return // 本实例无订阅者
	}

	// 添加系统时间
	data["systime"] = time.Now().UTC()

	for _, client := range clients {
		// 过滤字段（如果指定了 keys）
		filteredData := data
		if len(client.Keys) > 0 {
			filteredData = filterDataByKeys(data, client.Keys)
		}

		// 序列化
		jsonData, err := json.Marshal(filteredData)
		if err != nil {
			logrus.Error("Failed to marshal WebSocket data")
			continue
		}
		// 推送到 WebSocket：优先通过 client.Send 非阻塞发送到写入 goroutine，
		// 避免在此处直接写 Conn 导致阻塞整个管理器或读处理循环。
		if !client.TryEnqueue(jsonData) {
			// 客户端已关闭或 send queue is full，记录并丢弃消息，避免阻塞
			logrus.Warn("WebSocket send unavailable (closed or buffer full), dropping message")
		}
	}
}

// ListenForEvents 阻塞运行 Redis Pub/Sub 监听循环，直到 StopListen 被调用或
// Redis 客户端不可用。幂等守卫：已在运行时直接返回，避免重复初始化叠加第二个
// 监听协程导致事件重复投递。
func (m *WSManager) ListenForEvents() {
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
		logrus.Error("WebSocketManager cannot listen: redis client is not initialized")
		return
	}

	logrus.Info("WebSocketManager started listening for Redis Pub/Sub events")
	m.listenLoop(ctx, factory)
}

// listenLoop 是带指数退避重连的监听主体：ctx 取消后不再重连并立即返回，
// 退避等待也从 time.Sleep 改为可取消的定时器，保证 StopListen 能及时生效。
func (m *WSManager) listenLoop(ctx context.Context, factory wsPubSubFactory) {
	const (
		initialBackoff = 1 * time.Second
		maxBackoff     = 30 * time.Second
	)
	backoff := initialBackoff

	for {
		pubsub := factory(ctx, "ws:device:*")
		for {
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				_ = pubsub.Close()
				if ctx.Err() != nil {
					return
				}
				logrus.WithError(err).Warnf("WS: Redis pubsub error, reconnecting in %v", backoff)
				if !waitPubSubBackoff(ctx, backoff) {
					return
				}
				backoff = min(backoff*2, maxBackoff)
				break
			}
			backoff = initialBackoff

			var event WSEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				logrus.WithError(err).Error("Failed to unmarshal WebSocket event")
				continue
			}

			m.PushToDevice(event.DeviceID, event.Data)
		}
	}
}

// StopListen 取消监听循环并等待其退出；未启动或已停止时为幂等空操作。
// 有界等待防止异常阻塞拖垮停机序列（与既有 worker 的停止语义一致）。
func (m *WSManager) StopListen() {
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
		logrus.Info("WebSocketManager listener stopped")
	case <-timer.C:
		logrus.Warn("WebSocketManager listener stop timed out")
	}
}

// StopWSManagerListener 停止全局 WS 管理器的监听循环；未初始化时空操作。
// 供应用优雅停机序列调用：该监听协程不经 ServiceManager 托管，
// 必须在关闭 Redis 客户端之前显式停止。
func StopWSManagerListener() {
	if TPWSManager != nil {
		TPWSManager.StopListen()
	}
}

// GetStats 获取统计信息
func (m *WSManager) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	totalClients := 0
	for _, clients := range m.deviceClients {
		totalClients += len(clients)
	}

	return map[string]interface{}{
		"device_subscriptions": len(m.deviceClients),
		"total_clients":        totalClients,
	}
}

// filterDataByKeys 过滤数据字段
func filterDataByKeys(data map[string]interface{}, keys []string) map[string]interface{} {
	filtered := make(map[string]interface{})

	// 保留 systime
	if systime, ok := data["systime"]; ok {
		filtered["systime"] = systime
	}

	// 只保留指定的 keys
	for _, key := range keys {
		if value, ok := data[key]; ok {
			filtered[key] = value
		}
	}

	return filtered
}
