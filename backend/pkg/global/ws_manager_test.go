// 文件用途：验证 WebSocket 管理器的字段过滤、统计和非阻塞推送行为。
// 核心逻辑：构造内存订阅者和发送队列，断言过滤字段、系统时间、统计数量和满缓冲区处理。
// 关键注意事项：测试不连接真实 WebSocket 或 Redis，只覆盖本地数据结构和推送分支。
// 重构建议：后续可增加订阅/取消订阅的 Redis mock 测试和连接关闭回收测试。
package global

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestFilterDataByKeysKeepsSystimeAndRequestedTelemetryFields(t *testing.T) {
	now := time.Date(2026, 6, 27, 16, 0, 0, 0, time.UTC)
	input := map[string]interface{}{
		"systime":     now,
		"temperature": 26.5,
		"humidity":    60,
		"secret":      "not-for-this-client",
	}

	filtered := filterDataByKeys(input, []string{"temperature", "missing"})

	if len(filtered) != 2 {
		t.Fatalf("filtered field count = %d, want 2: %+v", len(filtered), filtered)
	}
	if filtered["systime"] != now {
		t.Fatalf("filtered systime = %v, want %v", filtered["systime"], now)
	}
	if filtered["temperature"] != 26.5 {
		t.Fatalf("filtered temperature = %v, want 26.5", filtered["temperature"])
	}
	if _, ok := filtered["humidity"]; ok {
		t.Fatal("filtered data unexpectedly kept humidity")
	}
	if _, ok := filtered["secret"]; ok {
		t.Fatal("filtered data unexpectedly kept secret")
	}
}

func TestFilterDataByKeysReturnsOnlyRequestedFieldsWhenSystimeMissing(t *testing.T) {
	filtered := filterDataByKeys(
		map[string]interface{}{
			"status":  "online",
			"voltage": 220,
		},
		[]string{"status"},
	)

	if len(filtered) != 1 || filtered["status"] != "online" {
		t.Fatalf("filtered = %+v, want only status", filtered)
	}
}

func TestWSClientTryEnqueueConcurrentWithCloseSendNeverPanics(t *testing.T) {
	// 回归防线：PushToDevice 与 UnsubscribeDevice 并发时，旧实现在锁外向已 close 的
	// Send 发送会触发 send on closed channel panic；守卫后的 TryEnqueue 必须安全。
	for iter := 0; iter < 200; iter++ {
		client := &WSClient{Send: make(chan []byte, 1)}
		var wg sync.WaitGroup
		stop := make(chan struct{})
		for worker := 0; worker < 4; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						client.TryEnqueue([]byte("payload"))
					}
				}
			}()
		}

		client.CloseSend()
		client.CloseSend() // 必须幂等，不允许 double-close panic
		close(stop)
		wg.Wait()

		if client.TryEnqueue([]byte("after-close")) {
			t.Fatalf("iteration %d: enqueue succeeded after CloseSend", iter)
		}
	}
}

func TestWSManagerPushToDeviceSkipsClientRemovedFromManager(t *testing.T) {
	// 回归防线：PushToDevice 必须基于读锁内的订阅快照遍历，
	// 且对已移除并关闭写队列的客户端安全跳过（不 panic、不投递）。
	ch := make(chan []byte, 1)
	client := &WSClient{DeviceID: "device-a", ConnID: "conn-a", Mu: &sync.Mutex{}, Send: ch}
	manager := &WSManager{
		deviceClients: map[string]map[string]*WSClient{
			"device-a": {"conn-a": client},
		},
	}

	manager.PushToDevice("device-a", map[string]interface{}{"temperature": 1})
	select {
	case <-ch:
	default:
		t.Fatal("expected first push to reach active subscriber")
	}

	// 模拟 UnsubscribeDevice 的本地副作用（单测环境无 Redis，不直接调用它）：
	// 从索引移除并关闭写队列。
	delete(manager.deviceClients["device-a"], "conn-a")
	client.CloseSend()

	manager.PushToDevice("device-a", map[string]interface{}{"temperature": 2})
	if len(ch) != 0 {
		t.Fatalf("removed/closed client received %d extra payload(s)", len(ch))
	}
}

func TestWSManagerGetStatsCountsDeviceSubscriptionsAndClients(t *testing.T) {
	manager := &WSManager{
		deviceClients: map[string]map[string]*WSClient{
			"device-a": {
				"conn-a": {DeviceID: "device-a"},
				"conn-b": {DeviceID: "device-a"},
			},
			"device-b": {
				"conn-c": {DeviceID: "device-b"},
			},
		},
	}

	stats := manager.GetStats()

	if stats["device_subscriptions"] != 2 {
		t.Fatalf("device_subscriptions = %v, want 2", stats["device_subscriptions"])
	}
	if stats["total_clients"] != 3 {
		t.Fatalf("total_clients = %v, want 3", stats["total_clients"])
	}
}

func TestWSManagerPushToDeviceSendsFilteredPayloadsPerClient(t *testing.T) {
	allFields := make(chan []byte, 1)
	temperatureOnly := make(chan []byte, 1)
	manager := &WSManager{
		deviceClients: map[string]map[string]*WSClient{
			"device-a": {
				"all": {
					DeviceID: "device-a",
					ConnID:   "all",
					Mu:       &sync.Mutex{},
					Send:     allFields,
				},
				"temperature": {
					DeviceID: "device-a",
					ConnID:   "temperature",
					Keys:     []string{"temperature"},
					Mu:       &sync.Mutex{},
					Send:     temperatureOnly,
				},
			},
		},
	}

	manager.PushToDevice("device-a", map[string]interface{}{
		"temperature": 26.5,
		"humidity":    60,
	})

	var allPayload map[string]interface{}
	if err := json.Unmarshal(<-allFields, &allPayload); err != nil {
		t.Fatalf("unmarshal all payload: %v", err)
	}
	if allPayload["temperature"] != 26.5 || allPayload["humidity"] != float64(60) {
		t.Fatalf("all payload = %+v, want temperature and humidity", allPayload)
	}
	if _, ok := allPayload["systime"]; !ok {
		t.Fatal("all payload missing systime")
	}

	var filteredPayload map[string]interface{}
	if err := json.Unmarshal(<-temperatureOnly, &filteredPayload); err != nil {
		t.Fatalf("unmarshal filtered payload: %v", err)
	}
	if filteredPayload["temperature"] != 26.5 {
		t.Fatalf("filtered payload temperature = %v, want 26.5", filteredPayload["temperature"])
	}
	if _, ok := filteredPayload["humidity"]; ok {
		t.Fatalf("filtered payload unexpectedly includes humidity: %+v", filteredPayload)
	}
	if _, ok := filteredPayload["systime"]; !ok {
		t.Fatal("filtered payload missing systime")
	}
}

func TestWSManagerPushToDeviceIgnoresMissingSubscribersAndFullSendBuffers(t *testing.T) {
	fullBuffer := make(chan []byte, 1)
	fullBuffer <- []byte(`{"old":true}`)
	manager := &WSManager{
		deviceClients: map[string]map[string]*WSClient{
			"device-a": {
				"full": {
					DeviceID: "device-a",
					ConnID:   "full",
					Send:     fullBuffer,
					Mu:       &sync.Mutex{},
				},
			},
		},
	}

	manager.PushToDevice("missing-device", map[string]interface{}{"temperature": 1})
	manager.PushToDevice("device-a", map[string]interface{}{"temperature": 2})

	if len(fullBuffer) != 1 {
		t.Fatalf("full send buffer length = %d, want unchanged 1", len(fullBuffer))
	}
	if got := string(<-fullBuffer); got != `{"old":true}` {
		t.Fatalf("full send buffer payload = %s, want old payload unchanged", got)
	}
}
