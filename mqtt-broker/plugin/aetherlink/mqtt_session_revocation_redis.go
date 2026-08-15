// 文件用途：把 Redis Pub/Sub 的设备会话撤销命令与处理 ACK 适配到 broker 内部 monitor。
// 核心逻辑：订阅固定控制 channel，并把结构化 processing ACK 发布到专用 :ack channel。
// 关键注意事项：go-redis v5 的 ReceiveMessage 会自动处理网络重连/重订阅；其它错误会记录并退避重试，channel 修改必须两侧同步。
package aetherlink

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/redis.v5"
)

const (
	mqttDeviceSessionRevocationChannel    = "aetherlink:mqtt:device-session:terminate"
	mqttDeviceSessionRevocationAckChannel = mqttDeviceSessionRevocationChannel + ":ack"
)

func publishRedisMQTTSessionRevocationAck(ack mqttSessionRevocationAck) error {
	if redisCache == nil {
		return fmt.Errorf("redis is not initialized for mqtt session revocation acknowledgement")
	}
	payload, err := json.Marshal(ack)
	if err != nil {
		return fmt.Errorf("encode mqtt session revocation acknowledgement: %w", err)
	}
	subscriberCount, err := redisCache.Publish(mqttDeviceSessionRevocationAckChannel, string(payload)).Result()
	if err != nil {
		return fmt.Errorf("publish mqtt session revocation acknowledgement: %w", err)
	}
	if subscriberCount == 0 {
		return fmt.Errorf("publish mqtt session revocation acknowledgement: no backend subscriber")
	}
	return nil
}

type redisMQTTSessionRevocationSubscription struct {
	pubsub    *redis.PubSub
	messages  chan string
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

func subscribeRedisMQTTSessionRevocations() (mqttSessionRevocationSubscription, error) {
	if redisCache == nil {
		return nil, fmt.Errorf("redis is not initialized for mqtt session revocation")
	}
	pubsub, err := redisCache.Subscribe(mqttDeviceSessionRevocationChannel)
	if err != nil {
		return nil, fmt.Errorf("subscribe mqtt session revocation channel: %w", err)
	}
	confirmation, err := pubsub.ReceiveTimeout(3 * time.Second)
	if err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("confirm mqtt session revocation subscription: %w", err)
	}
	subscribed, ok := confirmation.(*redis.Subscription)
	if !ok || subscribed.Kind != "subscribe" || subscribed.Channel != mqttDeviceSessionRevocationChannel {
		_ = pubsub.Close()
		return nil, fmt.Errorf("unexpected mqtt session revocation subscription confirmation: %T", confirmation)
	}

	subscription := &redisMQTTSessionRevocationSubscription{
		pubsub:   pubsub,
		messages: make(chan string),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go subscription.forward()
	return subscription, nil
}

func (s *redisMQTTSessionRevocationSubscription) Messages() <-chan string {
	if s == nil {
		return nil
	}
	return s.messages
}

func (s *redisMQTTSessionRevocationSubscription) forward() {
	defer close(s.done)
	defer close(s.messages)
	for {
		select {
		case <-s.stop:
			return
		default:
		}
		message, err := s.pubsub.ReceiveMessage()
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
			}
			if Log != nil {
				Log.Warn("mqtt session revocation subscription receive failed", zap.Error(err))
			}
			retry := time.NewTimer(time.Second)
			select {
			case <-s.stop:
				if !retry.Stop() {
					select {
					case <-retry.C:
					default:
					}
				}
				return
			case <-retry.C:
				continue
			}
		}
		if message == nil {
			continue
		}
		select {
		case s.messages <- message.Payload:
		case <-s.stop:
			return
		}
	}
}

func (s *redisMQTTSessionRevocationSubscription) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		close(s.stop)
		s.closeErr = s.pubsub.Close()
		<-s.done
	})
	return s.closeErr
}
