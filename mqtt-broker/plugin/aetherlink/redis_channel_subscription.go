// 文件用途：backend→broker Redis Pub/Sub 控制通道的共享订阅骨架（会话撤销、凭证缓存失效共用）。
// 核心逻辑：subscribeRedisChannel 订阅并确认单个 channel，后台把消息负载转发到无缓冲 chan；
//
//	subscriptionLifecycle 统一 monitor 的 started/stop/done 状态与幂等 Close。
//
// 关键注意事项：go-redis v5 的 ReceiveMessage 自动处理网络重连/重订阅；其它错误记录后退避 1s 重试。
// purpose 仅用于错误/日志文案，保持各调用方原有的可读描述。
package aetherlink

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/redis.v5"
)

const redisChannelSubscribeConfirmTimeout = 3 * time.Second

// redisChannelSubscription 把单个 Redis channel 的消息负载转发到 Messages()。
type redisChannelSubscription struct {
	purpose   string
	pubsub    *redis.PubSub
	messages  chan string
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

// subscribeRedisChannel 订阅 channel 并等待 subscribe 确认，成功后启动后台转发。
func subscribeRedisChannel(channel, purpose string) (*redisChannelSubscription, error) {
	if redisCache == nil {
		return nil, fmt.Errorf("redis is not initialized for %s", purpose)
	}
	pubsub, err := redisCache.Subscribe(channel)
	if err != nil {
		return nil, fmt.Errorf("subscribe %s channel: %w", purpose, err)
	}
	confirmation, err := pubsub.ReceiveTimeout(redisChannelSubscribeConfirmTimeout)
	if err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("confirm %s subscription: %w", purpose, err)
	}
	subscribed, ok := confirmation.(*redis.Subscription)
	if !ok || subscribed.Kind != "subscribe" || subscribed.Channel != channel {
		_ = pubsub.Close()
		return nil, fmt.Errorf("unexpected %s subscription confirmation: %T", purpose, confirmation)
	}

	subscription := &redisChannelSubscription{
		purpose:  purpose,
		pubsub:   pubsub,
		messages: make(chan string),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go subscription.forward()
	return subscription, nil
}

func (s *redisChannelSubscription) Messages() <-chan string {
	if s == nil {
		return nil
	}
	return s.messages
}

func (s *redisChannelSubscription) forward() {
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
				Log.Warn(s.purpose+" subscription receive failed", zap.Error(err))
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

func (s *redisChannelSubscription) Close() error {
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

// subscriptionLifecycle 是 Pub/Sub monitor 共用的启停状态；嵌入后 mu 同时保护 monitor 自身配置。
type subscriptionLifecycle struct {
	mu           sync.Mutex
	subscription interface{ Close() error }
	stop         chan struct{}
	done         chan struct{}
	started      bool
}

// beginLocked 记录已建立的订阅并返回 run goroutine 使用的 stop/done；调用方必须持有 mu。
func (l *subscriptionLifecycle) beginLocked(subscription interface{ Close() error }) (<-chan struct{}, chan<- struct{}) {
	l.subscription = subscription
	l.stop = make(chan struct{})
	l.done = make(chan struct{})
	l.started = true
	return l.stop, l.done
}

// shutdown 幂等地停止 run goroutine、关闭订阅并等待 run 退出。
func (l *subscriptionLifecycle) shutdown() error {
	l.mu.Lock()
	if !l.started {
		l.mu.Unlock()
		return nil
	}
	subscription := l.subscription
	stop := l.stop
	done := l.done
	l.subscription = nil
	l.stop = nil
	l.done = nil
	l.started = false
	close(stop)
	l.mu.Unlock()

	err := subscription.Close()
	<-done
	return err
}
