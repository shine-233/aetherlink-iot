// telemetry_ws_stream.go owns shared telemetry WebSocket stream helpers.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	redisWSPubSubInitialBackoff = 500 * time.Millisecond
	redisWSPubSubMaxBackoff     = 15 * time.Second
)

func newTelemetryWSClient(conn *websocket.Conn, msgType int, deviceID string, claims *utils.UserClaims, keys []string) *global.WSClient {
	var mu sync.Mutex
	return &global.WSClient{
		DeviceID: deviceID,
		TenantID: claims.TenantID,
		UserID:   claims.ID,
		Conn:     conn,
		ConnID:   fmt.Sprintf("%s-%d", conn.RemoteAddr().String(), time.Now().UnixNano()),
		MsgType:  msgType,
		Mu:       &mu,
		Keys:     keys,
		Send:     make(chan []byte, 64),
	}
}

func startTelemetryWSWriter(wsClient *global.WSClient, unsubscribeOnError bool) {
	// All telemetry WS paths write through this queue so Redis/session fan-out
	// never writes to the websocket connection concurrently with the read loop.
	go func(c *global.WSClient) {
		defer func() {
			if r := recover(); r != nil {
				logrus.Warn("telemetry websocket writer recovered")
			}
		}()
		for b := range c.Send {
			c.Mu.Lock()
			c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			err := c.Conn.WriteMessage(c.MsgType, b)
			c.Mu.Unlock()
			if err != nil {
				logrus.Error("telemetry websocket writer failed")
				if unsubscribeOnError {
					_ = global.TPWSManager.UnsubscribeDevice(c.DeviceID, c.ConnID)
				}
				return
			}
		}
	}(wsClient)
}

func readInitialWSMessage(conn *websocket.Conn) (int, []byte, bool) {
	msgType, msg, err := conn.ReadMessage()
	if err != nil {
		logrus.Error("read initial websocket message failed")
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Failed to read message"))
		return 0, nil, false
	}
	return msgType, msg, true
}

func parseTelemetryWSMessage(msg []byte) (map[string]interface{}, error) {
	var msgMap map[string]interface{}
	if err := json.Unmarshal(msg, &msgMap); err != nil {
		return nil, err
	}
	if msgMap == nil {
		return nil, errors.New("message body must be a JSON object")
	}
	return msgMap, nil
}

func telemetryWSDeviceID(msgMap map[string]interface{}) (string, error) {
	deviceID, ok := msgMap["device_id"].(string)
	if !ok || strings.TrimSpace(deviceID) == "" {
		return "", errors.New("device_id must be a non-empty string")
	}
	return deviceID, nil
}

func telemetryWSKeys(msgMap map[string]interface{}) ([]string, error) {
	rawKeys, ok := msgMap["keys"].([]interface{})
	if !ok {
		return nil, errors.New("keys must be array")
	}
	if len(rawKeys) == 0 {
		return nil, errors.New("keys array cannot be empty")
	}
	keys := make([]string, 0, len(rawKeys))
	for _, key := range rawKeys {
		strKey, ok := key.(string)
		if !ok || strings.TrimSpace(strKey) == "" {
			return nil, errors.New("keys must be non-empty strings")
		}
		keys = append(keys, strKey)
	}
	return keys, nil
}

func queueTelemetryWSMessage(wsClient *global.WSClient, payload []byte, dropLog string) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Warn("telemetry websocket send queue recovered")
		}
	}()
	select {
	case wsClient.Send <- payload:
	default:
		logrus.Warn("telemetry websocket send queue full; dropping message")
	}
}

func closeTelemetryWSClientSend(wsClient *global.WSClient) {
	if wsClient == nil || wsClient.Send == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logrus.Warn("telemetry websocket close recovered")
		}
	}()
	close(wsClient.Send)
}

func writeTelemetryWSControl(wsClient *global.WSClient, msgType int, payload []byte, deadline time.Time) error {
	// Keep data frames and control frames behind the WSClient write owner.
	wsClient.Mu.Lock()
	defer wsClient.Mu.Unlock()
	return wsClient.Conn.WriteControl(msgType, payload, deadline)
}

func writeTelemetryWSClose(wsClient *global.WSClient, code int, text string) {
	deadline := time.Now().Add(time.Second)
	_ = writeTelemetryWSControl(wsClient, websocket.CloseMessage, websocket.FormatCloseMessage(code, text), deadline)
}

func writeTelemetryWSPongControl(wsClient *global.WSClient) {
	_ = writeTelemetryWSControl(wsClient, websocket.PongMessage, []byte{}, time.Now().Add(2*time.Second))
}

func queueTelemetryWSPong(wsClient *global.WSClient) {
	select {
	case wsClient.Send <- []byte("pong"):
	default:
		writeTelemetryWSPongControl(wsClient)
	}
}

func runTelemetryWSHeartbeatLoop(wsClient *global.WSClient, deviceID string, refreshSubscription bool) {
	lastPingTime := time.Now()
	heartbeatTimeout := 15 * time.Second

	for {
		if time.Since(lastPingTime) > heartbeatTimeout {
			writeTelemetryWSClose(wsClient, websocket.CloseGoingAway, "connection closed due to heartbeat timeout")
			return
		}

		_ = wsClient.Conn.SetReadDeadline(time.Now().Add(heartbeatTimeout + 5*time.Second))
		_, msg, err := wsClient.Conn.ReadMessage()
		if err != nil {
			if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
				continue
			}

			logrus.Error("telemetry websocket read failed")
			writeTelemetryWSClose(wsClient, websocket.CloseInternalServerErr, "connection closed due to error")
			return
		}

		if string(msg) != "ping" {
			logrus.Debug("received non-ping websocket message")
			continue
		}

		lastPingTime = time.Now()
		if refreshSubscription {
			if err := global.TPWSManager.RefreshSubscription(deviceID); err != nil {
				logrus.Error("refresh telemetry websocket subscription failed")
			}
		}

		queueTelemetryWSPong(wsClient)
	}
}

type telemetryWSHandshake struct {
	msgType  int
	deviceID string
	keys     []string
	claims   *utils.UserClaims
}

func readTelemetryWSHandshake(conn *websocket.Conn, logContext string, requireKeys bool) (telemetryWSHandshake, bool) {
	msgType, msg, ok := readInitialWSMessage(conn)
	if !ok {
		return telemetryWSHandshake{}, false
	}

	msgMap, err := parseTelemetryWSMessage(msg)
	if err != nil {
		logrus.Error("invalid telemetry websocket JSON")
		_ = conn.WriteMessage(msgType, []byte("Invalid message format"))
		return telemetryWSHandshake{}, false
	}

	deviceID, err := telemetryWSDeviceID(msgMap)
	if err != nil {
		_ = conn.WriteMessage(msgType, []byte(err.Error()))
		return telemetryWSHandshake{}, false
	}

	var stringKeys []string
	if requireKeys {
		stringKeys, err = telemetryWSKeys(msgMap)
		if err != nil {
			_ = conn.WriteMessage(msgType, []byte(err.Error()))
			return telemetryWSHandshake{}, false
		}
	}

	claims, err := validateAuth(msgMap)
	if err != nil {
		logrus.Error("telemetry websocket authentication failed")
		_ = conn.WriteMessage(msgType, []byte(err.Error()))
		return telemetryWSHandshake{}, false
	}

	return telemetryWSHandshake{
		msgType:  msgType,
		deviceID: deviceID,
		keys:     stringKeys,
		claims:   claims,
	}, true
}

func subscribeTelemetryWSStream(conn *websocket.Conn, handshake telemetryWSHandshake, logContext string) (*global.WSClient, func(), bool) {
	wsClient := newTelemetryWSClient(conn, handshake.msgType, handshake.deviceID, handshake.claims, handshake.keys)
	connID := wsClient.ConnID

	if err := global.TPWSManager.SubscribeDevice(handshake.deviceID, connID, wsClient); err != nil {
		logrus.Error("telemetry websocket subscription failed")
		_ = conn.WriteMessage(handshake.msgType, []byte("Failed to subscribe to device"))
		return nil, nil, false
	}

	startTelemetryWSWriter(wsClient, true)
	return wsClient, func() {
		_ = global.TPWSManager.UnsubscribeDevice(handshake.deviceID, connID)
	}, true
}

func queueInitialTelemetryData(wsClient *global.WSClient, deviceID string, data interface{}, contextName string) bool {
	if data == nil {
		return true
	}

	dataByte, err := json.Marshal(data)
	if err != nil {
		logrus.Error("marshal initial telemetry websocket payload failed")
		queueTelemetryWSMessage(wsClient, []byte("Failed to process telemetry data"), fmt.Sprintf("%s marshal error send buffer full", contextName))
		return false
	}

	queueTelemetryWSMessage(wsClient, dataByte, fmt.Sprintf("initial %s send buffer full, dropping initial data", contextName))
	return true
}

func queueInitialCurrentTelemetryData(wsClient *global.WSClient, deviceID string, claims *utils.UserClaims) bool {
	data, err := service.GroupApp.TelemetryData.GetCurrentTelemetrDataForWs(deviceID, claims)
	if err != nil {
		logrus.Error("get current telemetry for websocket failed")
		queueTelemetryWSMessage(wsClient, []byte("Failed to get telemetry data"), "telemetry error send buffer full")
		return false
	}

	return queueInitialTelemetryData(wsClient, deviceID, data, "telemetry")
}

func readTelemetryCurrentWSHandshake(conn *websocket.Conn) (telemetryWSHandshake, bool) {
	return readTelemetryWSHandshake(conn, "telemetry websocket", false)
}

func readTelemetryKeyCurrentWSHandshake(conn *websocket.Conn) (telemetryWSHandshake, bool) {
	return readTelemetryWSHandshake(conn, "telemetry key websocket", true)
}

func readDeviceStatusWSHandshake(conn *websocket.Conn) (int, string, *utils.UserClaims, bool) {
	handshake, ok := readTelemetryWSHandshake(conn, "device status websocket", false)
	if !ok {
		return 0, "", nil, false
	}

	return handshake.msgType, handshake.deviceID, handshake.claims, true
}

func queryDeviceStatusInitialState(conn *websocket.Conn, msgType int, deviceID string, claims *utils.UserClaims) (int, bool) {
	currentStatusMap, err := service.GroupApp.Device.GetDeviceOnlineStatus(deviceID, claims)
	if err != nil {
		logrus.Error("query current device status failed")
		_ = conn.WriteMessage(msgType, []byte("Failed to query device status"))
		return 0, false
	}

	isOnline := 0
	if status, ok := currentStatusMap["is_online"]; ok {
		isOnline = status
	}

	return isOnline, true
}

func queueInitialDeviceStatus(localClient *global.WSClient, deviceID string, isOnline int) {
	if data, err := json.Marshal(map[string]interface{}{"is_online": isOnline}); err == nil {
		queueTelemetryWSMessage(localClient, data, "status initial send buffer full, dropping initial data")
	} else {
		logrus.Error("marshal initial status websocket payload failed")
	}
}

func nextRedisWSPubSubBackoff(current time.Duration) time.Duration {
	if current <= 0 {
		return redisWSPubSubInitialBackoff
	}
	next := current * 2
	if next > redisWSPubSubMaxBackoff {
		return redisWSPubSubMaxBackoff
	}
	return next
}

func logRedisWSPubSubReconnect(logFields logrus.Fields, contextName string, backoff time.Duration) {
	if backoff <= redisWSPubSubInitialBackoff {
		logrus.Warn("Redis WebSocket channel closed; resubscribing")
		return
	}
	logrus.Debug("Redis WebSocket channel still unavailable; resubscribing")
}

func waitRedisWSPubSubBackoff(ctx context.Context, backoff time.Duration) bool {
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func startRedisWSPubSubForwarder(
	ctx context.Context,
	channels []string,
	logFields logrus.Fields,
	contextName string,
	forward func(channel string, payload string),
) func() {
	forwarderCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Warn("Redis WebSocket forwarder recovered")
			}
		}()

		backoff := redisWSPubSubInitialBackoff
		for {
			if forwarderCtx.Err() != nil {
				return
			}

			pubsub := global.REDIS.Subscribe(forwarderCtx, channels...)
			ch := pubsub.Channel()
			shouldRetry := false

			for {
				select {
				case <-forwarderCtx.Done():
					_ = pubsub.Close()
					return
				case redisMsg, ok := <-ch:
					if !ok {
						shouldRetry = true
						break
					}
					backoff = redisWSPubSubInitialBackoff
					forward(redisMsg.Channel, redisMsg.Payload)
				}
				if shouldRetry {
					break
				}
			}

			_ = pubsub.Close()
			if forwarderCtx.Err() != nil {
				return
			}

			logRedisWSPubSubReconnect(logFields, contextName, backoff)
			if !waitRedisWSPubSubBackoff(forwarderCtx, backoff) {
				return
			}
			backoff = nextRedisWSPubSubBackoff(backoff)
		}
	}()

	return cancel
}

func startDeviceStatusRedisForwarder(ctx context.Context, deviceID string, localClient *global.WSClient) func() {
	channel := fmt.Sprintf("device:%s:status", deviceID)
	return startRedisWSPubSubForwarder(
		ctx,
		[]string{channel},
		logrus.Fields{"device_id": deviceID, "channel": channel},
		"device status websocket",
		func(_ string, payload string) {
			queueTelemetryWSMessage(localClient, []byte(payload), "status send buffer full, dropping update")
		},
	)
}

func runDeviceStatusWSReadLoop(conn *websocket.Conn, localClient *global.WSClient, cancel context.CancelFunc) {
	for {
		_, wsMsg, err := conn.ReadMessage()
		if err != nil {
			logrus.Info("device status websocket closed")
			writeTelemetryWSClose(localClient, websocket.CloseNormalClosure, "connection closed")
			cancel()
			return
		}

		if string(wsMsg) == "ping" {
			queueTelemetryWSPong(localClient)
		}
	}
}

func fetchInitialTelemetryKeyData(conn *websocket.Conn, handshake telemetryWSHandshake) (interface{}, bool) {
	data, err := service.GroupApp.TelemetryData.GetCurrentTelemetrDataKeysForWs(handshake.deviceID, handshake.keys, handshake.claims)
	if err != nil {
		logrus.Error("get current telemetry keys for websocket failed")
		_ = conn.WriteMessage(handshake.msgType, []byte("Failed to get telemetry data"))
		return nil, false
	}

	return data, true
}
