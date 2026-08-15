// 文件用途：把 uplink.Bus 适配到 mqttdebug.UplinkSource 接口，供 MQTTService wire 使用。
// 迁移原因：原实现位于 internal/mqttdebug/uplink_bus_source.go，导致
// internal/service → internal/mqttdebug → internal/uplink → internal/service 的 import cycle。
// 该文件仅在 app 层做 wiring，不改变原有运行时行为。

package app

import (
	"fmt"
	"strings"
	"sync"

	"aetherlink-iot/backend/internal/mqttdebug"
	"aetherlink-iot/backend/internal/uplink"
)

const mqttDebugUplinkObserverBuffer = 1024

type busUplinkSource struct {
	bus          *uplink.Bus
	mu           sync.RWMutex
	subscription *uplink.AcceptedMessageSubscription
}

// NewBusUplinkSource observes accepted production MQTT uplinks without
// competing with the real flow consumers or opening one global MQTT
// subscription per debug session.
func NewBusUplinkSource(bus *uplink.Bus) mqttdebug.UplinkSource {
	if bus == nil {
		return nil
	}
	return &busUplinkSource{bus: bus}
}

func (source *busUplinkSource) Start(handler func(mqttdebug.TrustedUplinkMessage)) (func(), error) {
	if source == nil || source.bus == nil || handler == nil {
		return nil, fmt.Errorf("mqtt debug uplink source is unavailable")
	}
	subscription, err := source.bus.SubscribeAcceptedMessages(mqttDebugUplinkObserverBuffer)
	if err != nil {
		return nil, fmt.Errorf("subscribe accepted mqtt uplinks: %w", err)
	}
	source.mu.Lock()
	source.subscription = subscription
	source.mu.Unlock()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			case message, ok := <-subscription.Messages:
				if !ok {
					return
				}
				if trusted, ok := trustedMQTTUplink(message); ok {
					handler(trusted)
				}
			}
		}
	}()
	var stopOnce sync.Once
	return func() {
		stopOnce.Do(func() {
			close(stop)
			subscription.Close()
			<-done
			source.mu.Lock()
			if source.subscription == subscription {
				source.subscription = nil
			}
			source.mu.Unlock()
		})
	}, nil
}

func (source *busUplinkSource) DroppedMessages() uint64 {
	if source == nil {
		return 0
	}
	source.mu.RLock()
	subscription := source.subscription
	source.mu.RUnlock()
	if subscription == nil {
		return 0
	}
	return subscription.DroppedMessages()
}

func trustedMQTTUplink(message *uplink.DeviceMessage) (mqttdebug.TrustedUplinkMessage, bool) {
	if message == nil || strings.TrimSpace(message.DeviceID) == "" || strings.TrimSpace(message.TenantID) == "" {
		return mqttdebug.TrustedUplinkMessage{}, false
	}
	topic, _ := message.Metadata["topic"].(string)
	sourceProtocol, _ := message.Metadata["source_protocol"].(string)
	if strings.TrimSpace(topic) == "" || !strings.EqualFold(strings.TrimSpace(sourceProtocol), "mqtt") {
		return mqttdebug.TrustedUplinkMessage{}, false
	}
	return mqttdebug.TrustedUplinkMessage{
		DeviceID: strings.TrimSpace(message.DeviceID),
		TenantID: strings.TrimSpace(message.TenantID),
		Type:     strings.TrimSpace(message.Type),
		Topic:    strings.TrimSpace(topic),
		// The Bus observer already owns a defensive copy. The handler consumes it
		// synchronously and applies the session capture limit before stringifying.
		Payload: message.Payload,
	}, true
}
