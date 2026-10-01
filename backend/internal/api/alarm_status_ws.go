// alarm_status_ws.go 实现租户级告警状态实时 WebSocket（TB-30，对标 ThingsBoard 告警实时订阅）。
// 握手后首帧携带 token 完成认证（与遥测/设备在线 WS 一致），随后服务端按租户推送：
//   {"type":"snapshot","items":[...]}           订阅建立时的初始快照
//   {"type":"trigger"|"recovery"|"status",...}   告警生命周期实时事件
package api

import (
	"context"
	"encoding/json"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// alarmStatusWSSnapshotLimit 初始快照条数上限，与 dal 侧收敛口径一致。
const alarmStatusWSSnapshotLimit = 20

// ServeAlarmStatusWS streams tenant-scoped alarm lifecycle events.
// All writes after the handshake go through WSClient.Send so Redis forwarding
// and ping handling do not write concurrently to the same connection.
// @Router       /api/v1/alarm/status/ws [get]
func (*AlarmApi) ServeAlarmStatusWS(c *gin.Context) {
	conn, closeConn, msgType, claims, ok := prepareAlarmStatusWS(c)
	if closeConn != nil {
		defer closeConn()
	}
	if !ok {
		return
	}

	localClient := newTelemetryWSClient(conn, msgType, "alarm:"+claims.TenantID, claims, nil)
	startTelemetryWSWriter(localClient, false)
	defer closeTelemetryWSClientSend(localClient)

	sendAlarmStatusWSSnapshot(localClient, claims.TenantID)

	// Background 是有意为之：该 ctx 的生命周期=WS 连接本身（连接级订阅），
	// 不能绑 HTTP 请求上下文——升级后请求对象语义不可靠。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closeSubscription := startAlarmStatusWSSubscription(ctx, localClient, claims.TenantID)
	defer func() {
		if closeSubscription != nil {
			closeSubscription()
		}
	}()

	runAlarmStatusWSMessageLoop(conn, localClient, closeSubscription, cancel)
}

// prepareAlarmStatusWS 升级连接并完成首帧认证握手。
// 返回的 closeConn 是幂等的连接级关闭入口，升级成功后必非 nil；
// 握手失败时也返回 conn/closeConn，调用方仍可写出错误帧后再统一关闭。
func prepareAlarmStatusWS(c *gin.Context) (*websocket.Conn, func(), int, *utils.UserClaims, bool) {
	conn, closeConn, ok := upgradeTelemetryWSSession(c, "alarm status websocket connected")
	if !ok {
		return nil, nil, 0, nil, false
	}

	msgType, msg, ok := readInitialWSMessage(conn)
	if !ok {
		return conn, closeConn, 0, nil, false
	}

	initMap, err := parseTelemetryWSMessage(msg)
	if err != nil {
		logrus.Error("invalid alarm status websocket JSON")
		writeAlarmStatusWSError(conn, msgType, "Invalid initial message format")
		return conn, closeConn, msgType, nil, false
	}

	addDeviceStatusWSHeaderCredentials(c, initMap)

	claims, err := validateAuth(initMap)
	if err != nil {
		logrus.Error("alarm status websocket authentication failed")
		writeAlarmStatusWSError(conn, msgType, err.Error())
		return conn, closeConn, msgType, nil, false
	}
	if claims.TenantID == "" {
		writeAlarmStatusWSError(conn, msgType, "tenant scope is required")
		return conn, closeConn, msgType, nil, false
	}

	return conn, closeConn, msgType, claims, true
}

// sendAlarmStatusWSSnapshot 推送订阅建立时的初始快照（最近 20 条告警历史摘要）。
func sendAlarmStatusWSSnapshot(localClient *global.WSClient, tenantID string) {
	items, err := service.GroupApp.Alarm.RecentAlarmSnapshot(tenantID, alarmStatusWSSnapshotLimit)
	if err != nil {
		logrus.Warn("query alarm status websocket snapshot failed")
		return
	}
	payload, err := json.Marshal(map[string]interface{}{
		"type":  "snapshot",
		"items": items,
	})
	if err != nil {
		logrus.Error("marshal alarm status websocket snapshot failed")
		return
	}
	queueTelemetryWSMessage(localClient, payload, "alarm status snapshot send buffer full, dropping snapshot")
}

// startAlarmStatusWSSubscription 订阅租户告警频道，把 Redis 事件原样转发给本连接。
func startAlarmStatusWSSubscription(ctx context.Context, localClient *global.WSClient, tenantID string) func() {
	channels := []string{service.AlarmStatusChannel(tenantID)}
	return startRedisWSPubSubForwarder(
		ctx,
		channels,
		logrus.Fields{"tenant_id": tenantID, "channels": channels},
		"alarm status websocket",
		func(_ string, payload string) {
			queueTelemetryWSMessage(localClient, []byte(payload), "alarm status send buffer full, dropping update")
		},
	)
}

func runAlarmStatusWSMessageLoop(
	conn *websocket.Conn,
	localClient *global.WSClient,
	closeSubscription func(),
	cancel context.CancelFunc,
) {
	for {
		_, wsMsg, err := conn.ReadMessage()
		if err != nil {
			logrus.Info("alarm status websocket closed")
			writeTelemetryWSClose(localClient, websocket.CloseNormalClosure, "connection closed")
			if closeSubscription != nil {
				closeSubscription()
			}
			cancel()
			return
		}
		if string(wsMsg) == "ping" {
			if !localClient.TryEnqueue([]byte("pong")) {
				writeTelemetryWSPongControl(localClient)
			}
			continue
		}
		// 订阅绑定在 JWT 的租户上，客户端帧不参与订阅选择，忽略即可。
	}
}

func writeAlarmStatusWSError(conn *websocket.Conn, msgType int, message string) {
	if err := conn.WriteMessage(msgType, []byte(message)); err != nil {
		// 写失败通常说明客户端已经断开，只记录日志便于排查，不改变握手失败的处理流程。
		logrus.Warn("alarm status websocket error frame write failed: ", err)
	}
}
