// 文件用途：验证 SSE 管理器的客户端注册、租户隔离、分发和失效连接清理。
// 核心逻辑：直接构造内存 clients map 和 mock writer，检查连接生命周期及事件写入边界。
// 关键注意事项：测试不启动 Redis Pub/Sub；分发测试只覆盖本地 writer 行为。
// 重构建议：监听循环支持 context 取消后，可补充 Redis 重连和退出测试。
package global

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type failingSSEWriter struct {
	gin.ResponseWriter
}

func (w *failingSSEWriter) Write([]byte) (int, error) {
	return 0, errors.New("client disconnected")
}

func TestSSEManagerAddClientRegistersTenantScopedClient(t *testing.T) {
	manager := &SSEManager{
		clients: make(map[string]map[string]*SSEClient),
	}

	clientID := manager.AddClient("tenant-a", "user-1", nil)

	if clientID == "" {
		t.Fatal("AddClient returned empty clientID")
	}
	if len(manager.clients) != 1 {
		t.Fatalf("tenant count = %d, want 1", len(manager.clients))
	}

	client := manager.clients["tenant-a"][clientID]
	if client == nil {
		t.Fatalf("client %q not registered under tenant-a", clientID)
	}
	if client.TenantID != "tenant-a" || client.UserID != "user-1" || client.ClientID != clientID {
		t.Fatalf("registered client = %+v, want tenant-a/user-1/%s", client, clientID)
	}
}

func TestSSEManagerAddClientKeepsTenantsAndClientsIsolated(t *testing.T) {
	manager := &SSEManager{
		clients: make(map[string]map[string]*SSEClient),
	}

	tenantAClient1 := manager.AddClient("tenant-a", "user-1", nil)
	tenantAClient2 := manager.AddClient("tenant-a", "user-2", nil)
	tenantBClient := manager.AddClient("tenant-b", "user-3", nil)

	if tenantAClient1 == tenantAClient2 || tenantAClient1 == tenantBClient || tenantAClient2 == tenantBClient {
		t.Fatal("AddClient generated duplicate client IDs")
	}
	if len(manager.clients["tenant-a"]) != 2 {
		t.Fatalf("tenant-a client count = %d, want 2", len(manager.clients["tenant-a"]))
	}
	if len(manager.clients["tenant-b"]) != 1 {
		t.Fatalf("tenant-b client count = %d, want 1", len(manager.clients["tenant-b"]))
	}
}

func TestSSEManagerRemoveClientDeletesOnlyTargetAndCleansEmptyTenant(t *testing.T) {
	manager := &SSEManager{
		clients: make(map[string]map[string]*SSEClient),
	}

	tenantAClient1 := manager.AddClient("tenant-a", "user-1", nil)
	tenantAClient2 := manager.AddClient("tenant-a", "user-2", nil)
	tenantBClient := manager.AddClient("tenant-b", "user-3", nil)

	manager.RemoveClient("tenant-a", tenantAClient1)

	if _, ok := manager.clients["tenant-a"][tenantAClient1]; ok {
		t.Fatal("RemoveClient left removed tenant-a client in map")
	}
	if _, ok := manager.clients["tenant-a"][tenantAClient2]; !ok {
		t.Fatal("RemoveClient removed the wrong tenant-a client")
	}
	if _, ok := manager.clients["tenant-b"][tenantBClient]; !ok {
		t.Fatal("RemoveClient affected a different tenant")
	}

	manager.RemoveClient("tenant-a", tenantAClient2)
	if _, ok := manager.clients["tenant-a"]; ok {
		t.Fatal("RemoveClient did not delete empty tenant-a map")
	}
	if _, ok := manager.clients["tenant-b"][tenantBClient]; !ok {
		t.Fatal("RemoveClient affected tenant-b while cleaning tenant-a")
	}
}

func TestSSEManagerRemoveClientIgnoresMissingTenantAndClient(t *testing.T) {
	manager := &SSEManager{
		clients: make(map[string]map[string]*SSEClient),
	}
	clientID := manager.AddClient("tenant-a", "user-1", nil)

	manager.RemoveClient("tenant-missing", "client-missing")
	manager.RemoveClient("tenant-a", "client-missing")

	if _, ok := manager.clients["tenant-a"][clientID]; !ok {
		t.Fatal("RemoveClient removed existing client when asked to remove a missing tenant/client")
	}
}

func newTestSSEWriter() (gin.ResponseWriter, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	return context.Writer, recorder
}

func TestSSEManagerDispatchEventWritesAndFlushesTenantClient(t *testing.T) {
	writer, recorder := newTestSSEWriter()
	manager := &SSEManager{clients: make(map[string]map[string]*SSEClient)}
	manager.AddClient("tenant-a", "user-1", writer)

	manager.dispatchEvent(SSEEvent{TenantID: "tenant-a", Type: "device_online", Message: `{"online":true}`})

	if got := recorder.Body.String(); !strings.Contains(got, "event: device_online\ndata: {\"online\":true}\n\n") {
		t.Fatalf("SSE body = %q, want encoded device_online event", got)
	}
	if len(manager.clients["tenant-a"]) != 1 {
		t.Fatal("dispatchEvent removed a healthy client")
	}
}

func TestSSEManagerDispatchEventRemovesNilWriter(t *testing.T) {
	manager := &SSEManager{clients: make(map[string]map[string]*SSEClient)}
	clientID := manager.AddClient("tenant-a", "user-1", nil)

	manager.dispatchEvent(SSEEvent{TenantID: "tenant-a", Type: "device_online"})

	if _, ok := manager.clients["tenant-a"][clientID]; ok {
		t.Fatal("dispatchEvent kept a client with a nil writer")
	}
}

func TestSSEManagerDispatchEventRemovesClientOnWriteFailure(t *testing.T) {
	manager := &SSEManager{clients: make(map[string]map[string]*SSEClient)}
	baseWriter, _ := newTestSSEWriter()
	writer := &failingSSEWriter{ResponseWriter: baseWriter}
	clientID := manager.AddClient("tenant-a", "user-1", writer)

	manager.dispatchEvent(SSEEvent{TenantID: "tenant-a", Type: "device_online"})

	if _, ok := manager.clients["tenant-a"][clientID]; ok {
		t.Fatal("dispatchEvent kept a client after its writer failed")
	}
}

func TestBroadcastSSEEventToTenantRejectsMissingGlobalManager(t *testing.T) {
	previous := TPSSEManager
	TPSSEManager = nil
	t.Cleanup(func() {
		TPSSEManager = previous
	})

	err := BroadcastSSEEventToTenant("tenant-a", SSEEvent{Type: "device_online"})
	if err == nil || err.Error() != "SSE manager is not initialized" {
		t.Fatalf("BroadcastSSEEventToTenant error = %v, want SSE manager is not initialized", err)
	}
}
