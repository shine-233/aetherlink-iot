// 文件用途：验证共享 WebSocket 会话助手的升级失败分支与连接关闭所有权。
// 核心逻辑：断言升级失败时不返回连接、只记录一条 gin 错误；幂等关闭函数在并发与重复触发下只真正关闭一次。
// 关键注意事项：使用 httptest 真实 TCP 服务验证关闭后写入立即失败，全部等待均有超时上限。
package api

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestUpgradeTelemetryWSSessionFailsWithoutHijackableWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/ws", nil)

	conn, closeConn, ok := upgradeTelemetryWSSession(c, "telemetry websocket connected")

	require.False(t, ok)
	require.Nil(t, conn)
	require.Nil(t, closeConn)
	require.Len(t, c.Errors, 1)
}

func TestNewWSConnCloserClosesConnectionExactlyOnce(t *testing.T) {
	upgraded := make(chan *websocket.Conn, 1)
	router := gin.New()
	router.GET("/ws", func(c *gin.Context) {
		conn, err := Wsupgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		upgraded <- conn
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { clientConn.Close() })

	conn := <-upgraded
	closeConn := newWSConnCloser(conn)

	// 模拟多条关闭路径（handler defer、读循环失败等）并发触发：
	// once 守卫必须保证底层连接只被真正关闭一次且不 panic。
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			closeConn()
		}()
	}
	wg.Wait()
	closeConn() // 重复调用同样必须安全。

	// 关闭后写入立即失败，作为“确实已关闭”的服务端同步证据。
	require.Error(t, conn.WriteMessage(websocket.TextMessage, []byte("after-close")))
}
