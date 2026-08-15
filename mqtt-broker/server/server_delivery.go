// 文件用途：承载 server 的消息投递选择器与共享订阅分发策略。
// 核心逻辑：根据 DeliveryMode 处理普通订阅、共享订阅、overlap 和 onlyOnce 投递，并写入客户端队列。
// 使用注意：deliverMessage 调用方必须持有 srv.mu；本文件会访问 queueStore，不能随意改成异步投递。
// 重构建议：后续可围绕共享订阅随机/TopicHash 策略、OnlyOnce Subscription Identifier 合并补 focused broker 测试。
package server

import (
	"hash/fnv"
	"math/rand"
	"time"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

func defaultIterateOptions(topicName string) subscription.IterationOptions {
	return subscription.IterationOptions{
		Type:      subscription.TypeAll,
		TopicName: topicName,
		MatchType: subscription.MatchFilter,
	}
}

type DeliveryMode = string

const (
	Overlap  DeliveryMode = "overlap"
	OnlyOnce DeliveryMode = "onlyonce"
)

type SharedSubBalanceStrategy = string

const (
	SharedSubBalanceRandom    SharedSubBalanceStrategy = config.SharedSubBalanceRandom
	SharedSubBalanceTopicHash SharedSubBalanceStrategy = config.SharedSubBalanceTopicHash
)

func (srv *server) addMsgToQueueLocked(now time.Time, clientID string, msg *gmqtt.Message, sub *gmqtt.Subscription, ids []uint32, q queue.Store) {
	mqttCfg := srv.config.MQTT
	if srv.shouldSkipQueueingMessageLocked(clientID, msg, mqttCfg.QueueQos0Msg) {
		return
	}
	prepareQueuedMessage(msg, sub, ids)
	err := q.Add(newQueueElem(now, msg, mqttCfg.MessageExpiry))
	if err != nil {
		srv.clients[clientID].queueNotifier.notifyDropped(msg, &queue.InternalError{Err: err})
		return
	}
}

func (srv *server) shouldSkipQueueingMessageLocked(clientID string, msg *gmqtt.Message, queueQos0Msg bool) bool {
	if queueQos0Msg {
		return false
	}
	// 未连接客户端默认跳过 QoS0 离线消息，避免离线队列被无确认消息压满。
	c := srv.clients[clientID]
	return c == nil && msg.QoS == packets.Qos0
}

func prepareQueuedMessage(msg *gmqtt.Message, sub *gmqtt.Subscription, ids []uint32) {
	if msg.QoS > sub.QoS {
		msg.QoS = sub.QoS
	}
	appendSubscriptionIdentifiers(msg, ids)
	msg.Dup = false
	applyRetainAsPublished(msg, sub)
}

func appendSubscriptionIdentifiers(msg *gmqtt.Message, ids []uint32) {
	for _, id := range ids {
		if id != 0 {
			msg.SubscriptionIdentifier = append(msg.SubscriptionIdentifier, id)
		}
	}
}

func applyRetainAsPublished(msg *gmqtt.Message, sub *gmqtt.Subscription) {
	if !sub.RetainAsPublished {
		msg.Retained = false
	}
}

func newQueueElem(now time.Time, msg *gmqtt.Message, configuredExpiry time.Duration) *queue.Elem {
	return &queue.Elem{
		At:     now,
		Expiry: queuedMessageExpiry(now, msg.MessageExpiry, configuredExpiry),
		MessageWithID: &queue.Publish{
			Message: msg,
		},
	}
}

func queuedMessageExpiry(now time.Time, messageExpiry uint32, configuredExpiry time.Duration) time.Time {
	expiryInterval := queuedMessageExpiryInterval(messageExpiry, configuredExpiry)
	if expiryInterval != 0 {
		return now.Add(expiryInterval)
	}
	return time.Time{}
}

func queuedMessageExpiryInterval(messageExpiry uint32, configuredExpiry time.Duration) time.Duration {
	if configuredExpiry != 0 {
		if messageExpiry != 0 && int(messageExpiry) <= int(configuredExpiry) {
			return time.Duration(messageExpiry) * time.Second
		}
		return configuredExpiry
	}
	if messageExpiry != 0 {
		return time.Duration(messageExpiry) * time.Second
	}
	return 0
}

// sharedList 按完整共享订阅主题记录候选客户端列表。
type sharedList map[string][]struct {
	clientID string
	sub      *gmqtt.Subscription
}

type nonSharedMatch struct {
	sub    *gmqtt.Subscription
	subIDs []uint32
}

// maxQos 记录非共享订阅在 onlyOnce 模式下每个客户端命中的最高 QoS 订阅。
type maxQos map[string]*nonSharedMatch

// deliverHandler 根据 DeliveryMode 统一处理普通订阅、共享订阅、overlap 与 onlyOnce 投递策略。
// 审查建议：该结构体已经比较独立，后续可围绕共享订阅均衡策略补 focused 测试。
type deliverHandler struct {
	fn       subscription.IterateFn
	sl       sharedList
	mq       maxQos
	matched  bool
	now      time.Time
	msg      *gmqtt.Message
	srv      *server
	strategy SharedSubBalanceStrategy
}

func newDeliverHandler(mode string, strategy string, srcClientID string, msg *gmqtt.Message, now time.Time, srv *server) *deliverHandler {
	d := &deliverHandler{
		sl:       make(sharedList),
		mq:       make(maxQos),
		msg:      msg,
		srv:      srv,
		now:      now,
		strategy: strategy,
	}
	if mode == Overlap {
		d.fn = d.iterateSubscriptions(srcClientID, d.deliverOverlap)
	} else {
		d.fn = d.iterateSubscriptions(srcClientID, d.recordOnlyOnce)
	}
	return d
}

func (d *deliverHandler) iterateSubscriptions(srcClientID string, nonShared subscription.IterateFn) subscription.IterateFn {
	return func(clientID string, sub *gmqtt.Subscription) bool {
		if sub.NoLocal && clientID == srcClientID {
			return true
		}
		d.matched = true
		if sub.ShareName != "" {
			d.addSharedSubscriber(clientID, sub)
			return true
		}
		return nonShared(clientID, sub)
	}
}

func (d *deliverHandler) addSharedSubscriber(clientID string, sub *gmqtt.Subscription) {
	fullTopic := sub.GetFullTopicName()
	d.sl[fullTopic] = append(d.sl[fullTopic], struct {
		clientID string
		sub      *gmqtt.Subscription
	}{clientID: clientID, sub: sub})
}

func (d *deliverHandler) deliverOverlap(clientID string, sub *gmqtt.Subscription) bool {
	if qs := d.srv.queueStore[clientID]; qs != nil {
		d.srv.addMsgToQueueLocked(d.now, clientID, d.msg.Copy(), sub, []uint32{sub.ID}, qs)
	}
	return true
}

func (d *deliverHandler) recordOnlyOnce(clientID string, sub *gmqtt.Subscription) bool {
	// onlyOnce 模式下，同一客户端多次命中时使用最高 QoS，并合并 Subscription Identifier。
	if d.mq[clientID] == nil {
		d.mq[clientID] = &nonSharedMatch{sub: sub, subIDs: []uint32{sub.ID}}
		return true
	}
	if d.mq[clientID].sub.QoS < sub.QoS {
		d.mq[clientID].sub = sub
	}
	d.mq[clientID].subIDs = append(d.mq[clientID].subIDs, sub.ID)
	return true
}

func (d *deliverHandler) flush() {
	d.flushSharedSubscriptions()
	d.flushOnlyOnceSubscriptions()
}

func (d *deliverHandler) flushSharedSubscriptions() {
	for _, v := range d.sl {
		rs := d.selectSharedSubscriber(v)
		d.enqueue(rs.clientID, rs.sub, []uint32{rs.sub.ID})
	}
}

func (d *deliverHandler) flushOnlyOnceSubscriptions() {
	for clientID, v := range d.mq {
		d.enqueue(clientID, v.sub, v.subIDs)
	}
}

func (d *deliverHandler) enqueue(clientID string, sub *gmqtt.Subscription, ids []uint32) {
	if qs := d.srv.queueStore[clientID]; qs != nil {
		d.srv.addMsgToQueueLocked(d.now, clientID, d.msg.Copy(), sub, ids, qs)
	}
}

func (d *deliverHandler) selectSharedSubscriber(subscribers []struct {
	clientID string
	sub      *gmqtt.Subscription
}) struct {
	clientID string
	sub      *gmqtt.Subscription
} {
	if len(subscribers) == 1 {
		return subscribers[0]
	}
	if d.strategy == SharedSubBalanceTopicHash {
		h := fnv.New32a()
		_, _ = h.Write([]byte(d.msg.Topic))
		return subscribers[int(h.Sum32())%len(subscribers)]
	}
	return subscribers[rand.Intn(len(subscribers))]
}

// deliverMessage 将消息投递给匹配订阅者，调用方必须持有 srv.mu。
// 使用注意：该函数会写入各客户端 queueStore，不能在未理解锁和队列 store 线程安全前改为异步。
func (srv *server) deliverMessage(srcClientID string, msg *gmqtt.Message, options subscription.IterationOptions) (matched bool) {
	now := time.Now()
	d := newDeliverHandler(srv.config.MQTT.DeliveryMode, srv.config.MQTT.SharedSubBalanceStrategy, srcClientID, msg, now, srv)
	srv.subscriptionsDB.Iterate(d.fn, options)
	d.flush()
	return d.matched
}
