// device_status_ws.go owns the multi-device online-status WebSocket endpoint.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var timeNow = time.Now

func deviceStatusWSDeviceIDs(initMap map[string]interface{}) []string {
	seen := make(map[string]struct{})
	deviceIDs := make([]string, 0)
	appendID := func(raw string) {
		for _, part := range strings.Split(raw, ",") {
			id := strings.TrimSpace(part)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			deviceIDs = append(deviceIDs, id)
		}
	}

	appendValue := func(value interface{}) {
		switch v := value.(type) {
		case []interface{}:
			for _, item := range v {
				appendID(fmt.Sprintf("%v", item))
			}
		case []string:
			for _, item := range v {
				appendID(item)
			}
		case string:
			appendID(v)
		}
	}

	for key, value := range initMap {
		if strings.EqualFold(key, "device_ids") {
			appendValue(value)
			break
		}
	}
	if len(deviceIDs) > 0 {
		return deviceIDs
	}
	for key, value := range initMap {
		if strings.EqualFold(key, "device_id") {
			appendValue(value)
			break
		}
	}
	return deviceIDs
}

// ServeDeviceOnlineStatusWS streams online status for one or more devices.
// All writes after the handshake go through WSClient.Send so Redis forwarding
// and ping handling do not write concurrently to the same connection.
// @Router       /api/v1/device/online/status/ws/batch [get]
func (*TelemetryDataApi) ServeDeviceOnlineStatusWS(c *gin.Context) {
	conn, msgType, claims, deviceIDs, ok := prepareDeviceOnlineStatusWS(c)
	if conn != nil {
		defer conn.Close()
	}
	if !ok {
		return
	}

	authorizedDeviceIDs, initialList := loadDeviceStatusWSInitialList(deviceIDs, claims)
	if len(authorizedDeviceIDs) == 0 {
		writeDeviceStatusWSError(conn, msgType, "no authorized devices")
		return
	}

	localClient := startDeviceStatusWSClient(conn, msgType, authorizedDeviceIDs, claims)
	defer closeTelemetryWSClientSend(localClient)

	sendDeviceStatusWSInitialList(localClient, initialList)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closeSubscription := startDeviceStatusWSSubscription(ctx, localClient, authorizedDeviceIDs)
	defer func() {
		if closeSubscription != nil {
			closeSubscription()
		}
	}()

	runDeviceStatusWSMessageLoop(ctx, conn, localClient, claims, &closeSubscription, cancel)
}

func prepareDeviceOnlineStatusWS(c *gin.Context) (*websocket.Conn, int, *utils.UserClaims, []string, bool) {
	conn, err := Wsupgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.Error(errcode.WithData(errcode.CodeSystemError, "WebSocket upgrade failed"))
		return nil, 0, nil, nil, false
	}

	logrus.Info("device online status websocket connected")

	msgType, msg, ok := readInitialWSMessage(conn)
	if !ok {
		return conn, 0, nil, nil, false
	}

	initMap, err := parseTelemetryWSMessage(msg)
	if err != nil {
		logrus.Error("invalid device online status websocket JSON")
		writeDeviceStatusWSError(conn, msgType, "Invalid initial message format")
		return conn, msgType, nil, nil, false
	}

	addDeviceStatusWSHeaderCredentials(c, initMap)

	logrus.Debug("WS initial message received")

	claims, err := validateAuth(initMap)
	if err != nil {
		logrus.Error("device online status websocket authentication failed")
		writeDeviceStatusWSError(conn, msgType, err.Error())
		return conn, msgType, nil, nil, false
	}

	deviceIDs := deviceStatusWSDeviceIDs(initMap)
	if len(deviceIDs) == 0 {
		writeDeviceStatusWSError(conn, msgType, "device_ids is required")
		return conn, msgType, nil, nil, false
	}

	return conn, msgType, claims, deviceIDs, true
}

func addDeviceStatusWSHeaderCredentials(c *gin.Context, initMap map[string]interface{}) {
	if telemetryWSStringValue(initMap, "token", "authorization") == "" {
		if auth := strings.TrimSpace(c.GetHeader("Authorization")); auth != "" {
			initMap["authorization"] = auth
		}
	}
	if telemetryWSStringValue(initMap, "x-api-key", "x_api_key", "xapikey", "apikey") == "" {
		if apiKey := strings.TrimSpace(c.GetHeader("X-Api-Key")); apiKey != "" {
			initMap["x-api-key"] = apiKey
		}
	}
}

func loadDeviceStatusWSInitialList(deviceIDs []string, claims *utils.UserClaims) ([]string, []map[string]interface{}) {
	initialList := make([]map[string]interface{}, 0, len(deviceIDs))
	authorizedDeviceIDs := make([]string, 0, len(deviceIDs))
	statusByDeviceID, err := service.GroupApp.Device.GetDeviceOnlineStatuses(deviceIDs, claims)
	if err != nil {
		logrus.Warn("query current device statuses failed")
		return authorizedDeviceIDs, initialList
	}

	for _, did := range deviceIDs {
		statusMap, ok := statusByDeviceID[did]
		if !ok {
			logrus.Warn("query current device status skipped")
			continue
		}
		authorizedDeviceIDs = append(authorizedDeviceIDs, did)
		isOnline := 0
		if v, ok := statusMap["is_online"]; ok {
			isOnline = v
		} else if v, ok := statusMap["device_status"]; ok {
			isOnline = v
		}
		initialList = append(initialList, map[string]interface{}{
			"device_id": did,
			"is_online": isOnline,
			"timestamp": timeNowUnixMilli(),
		})
	}
	return authorizedDeviceIDs, initialList
}

func startDeviceStatusWSClient(conn *websocket.Conn, msgType int, authorizedDeviceIDs []string, claims *utils.UserClaims) *global.WSClient {
	localClient := newTelemetryWSClient(conn, msgType, strings.Join(authorizedDeviceIDs, ","), claims, nil)
	startTelemetryWSWriter(localClient, false)
	return localClient
}

func sendDeviceStatusWSInitialList(localClient *global.WSClient, initialList []map[string]interface{}) {
	if data, err := json.Marshal(initialList); err == nil {
		queueTelemetryWSMessage(localClient, data, "device status initial send buffer full, dropping initial data")
	} else {
		logrus.Error("marshal initial device status websocket payload failed")
	}
}

func sendDeviceStatusWSQueuedError(localClient *global.WSClient, message string) {
	queueTelemetryWSMessage(localClient, []byte(message), "device status error send buffer full, dropping error")
}

func deviceStatusWSChannels(authorizedDeviceIDs []string) []string {
	channels := make([]string, 0, len(authorizedDeviceIDs))
	for _, did := range authorizedDeviceIDs {
		channels = append(channels, fmt.Sprintf("device:%s:status", did))
	}
	return channels
}

func startDeviceStatusWSSubscription(ctx context.Context, localClient *global.WSClient, authorizedDeviceIDs []string) func() {
	channels := deviceStatusWSChannels(authorizedDeviceIDs)
	return startRedisWSPubSubForwarder(
		ctx,
		channels,
		logrus.Fields{"device_ids": authorizedDeviceIDs, "channels": channels},
		"multi-device status websocket",
		func(channel string, payload string) {
			queueTelemetryWSMessage(localClient, deviceStatusWSForwardPayload(channel, payload), "device status send buffer full, dropping update")
		},
	)
}

func replaceDeviceStatusWSSubscription(
	ctx context.Context,
	localClient *global.WSClient,
	claims *utils.UserClaims,
	closeSubscription *func(),
	wsMsg []byte,
) {
	initMap, err := parseTelemetryWSMessage(wsMsg)
	if err != nil {
		logrus.Debug("ignore invalid device online status websocket message")
		return
	}

	deviceIDs := deviceStatusWSDeviceIDs(initMap)
	if len(deviceIDs) == 0 {
		sendDeviceStatusWSQueuedError(localClient, "device_ids is required")
		return
	}

	authorizedDeviceIDs, initialList := loadDeviceStatusWSInitialList(deviceIDs, claims)
	if len(authorizedDeviceIDs) == 0 {
		sendDeviceStatusWSQueuedError(localClient, "no authorized devices")
		return
	}

	nextCloseSubscription := startDeviceStatusWSSubscription(ctx, localClient, authorizedDeviceIDs)
	if closeSubscription != nil && *closeSubscription != nil {
		(*closeSubscription)()
	}
	if closeSubscription != nil {
		*closeSubscription = nextCloseSubscription
	}
	localClient.DeviceID = strings.Join(authorizedDeviceIDs, ",")
	sendDeviceStatusWSInitialList(localClient, initialList)
}

func runDeviceStatusWSMessageLoop(
	ctx context.Context,
	conn *websocket.Conn,
	localClient *global.WSClient,
	claims *utils.UserClaims,
	closeSubscription *func(),
	cancel context.CancelFunc,
) {
	for {
		_, wsMsg, err := conn.ReadMessage()
		if err != nil {
			logrus.Info("device online status websocket closed")
			writeTelemetryWSClose(localClient, websocket.CloseNormalClosure, "connection closed")
			cancel()
			return
		}
		if string(wsMsg) == "ping" {
			handleDeviceStatusWSPing(localClient)
			continue
		}
		replaceDeviceStatusWSSubscription(ctx, localClient, claims, closeSubscription, wsMsg)
	}
}

func handleDeviceStatusWSPing(localClient *global.WSClient) {
	if !localClient.TryEnqueue([]byte("pong")) {
		writeTelemetryWSPongControl(localClient)
	}
}

func writeDeviceStatusWSError(conn *websocket.Conn, msgType int, message string) {
	_ = conn.WriteMessage(msgType, []byte(message))
}

func deviceStatusWSForwardPayload(channel string, payload string) []byte {
	deviceID := ""
	parts := strings.Split(channel, ":")
	if len(parts) >= 3 {
		deviceID = parts[1]
	}

	var payloadMap map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &payloadMap); err == nil {
		payloadMap["device_id"] = deviceID
		if data, err := json.Marshal(payloadMap); err == nil {
			return data
		}
	}
	data, _ := json.Marshal(map[string]interface{}{"device_id": deviceID, "payload": payload})
	return data
}

func timeNowUnixMilli() int64 {
	return timeNow().UnixMilli()
}
