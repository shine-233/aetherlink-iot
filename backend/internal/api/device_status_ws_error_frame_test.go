package api

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// startDeviceStatusWSErrorFrameServer 启动一个只升级连接、读取一帧后关闭的服务端。
func startDeviceStatusWSErrorFrameServer(t *testing.T) *httptest.Server {
	t.Helper()
	router := gin.New()
	router.GET("/ws", func(c *gin.Context) {
		conn, err := Wsupgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// 读取客户端发来的错误帧后立即断开，制造下一次写失败的场景。
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func dialDeviceStatusWS(t *testing.T, serverURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/ws"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil && resp != nil {
		t.Fatalf("dial %s failed with http status %d: %v", wsURL, resp.StatusCode, err)
	}
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestWriteDeviceStatusWSErrorSendsFrameThenToleratesClosedConnection(t *testing.T) {
	server := startDeviceStatusWSErrorFrameServer(t)
	conn := dialDeviceStatusWS(t, server.URL)

	// 正常路径：错误帧应成功写出（服务端 ReadMessage 收到后返回）。
	writeDeviceStatusWSError(conn, websocket.TextMessage, "invalid initial message")

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	// 连接已断开：写失败必须只记 warn 日志，不得 panic，也不得改变调用方控制流。
	require.NotPanics(t, func() {
		writeDeviceStatusWSError(conn, websocket.TextMessage, "no authorized devices")
	})
}
