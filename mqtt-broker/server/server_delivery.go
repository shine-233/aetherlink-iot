// 文件用途：承载 server 的消息投递选择器与共享订阅分发策略。
// 核心逻辑：根据 DeliveryMode 处理普通订阅、共享订阅、overlap 和 onlyOnce 投递，并写入客户端队列。
// 使用注意：deliverMessage 调用方必须持有 srv.mu；本文件会访问 queueStore，不能随意改成异步投递。
// 重构建议：后续可围绕共享订阅随机/TopicHash 策略、OnlyOnce Subscription Identifier 合并补 focused broker 测试。
package server

import (
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

// addMsgToQueueLocked 为订阅者生成 src 的投递副本并入队；src 不会被修改。
// 先判跳过再拷贝：离线 QoS0 被丢弃时不再白做一次消息拷贝。
func (srv *server) addMsgToQueueLocked(now time.Time, clientID string, src *gmqtt.Message, sub *gmqtt.Subscription, ids []uint32, q queue.Store) {
	mqttCfg := srv.config.MQTT
	if srv.shouldSkipQueueingMessageLocked(clientID, src, mqttCfg.QueueQos0Msg) {
		return
	}
	msg := fanoutCopy(src, ids)
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

// fanoutCopy 为单个订阅者生成投递副本：标量字段与 SubscriptionIdentifier 独立，
// Payload/CorrelationData/UserProperties 与源消息共享底层数组（broker 内投递链路只读这些字节，
// 旧实现对每个订阅者深拷贝 payload，扇出 N 时占投递路径约 70% 的分配字节）。
// 共享切片均裁剪 cap，任何 append 都会重新分配而不会写到其它订阅者可见的数组。
// 契约：OnDelivered/OnMsgDropped 等 hook 不得原地改写 payload 字节（替换切片是安全的）。
func fanoutCopy(src *gmqtt.Message, ids []uint32) *gmqtt.Message {
	m := *src
	m.Payload = src.Payload[:len(src.Payload):len(src.Payload)]
	m.CorrelationData = src.CorrelationData[:len(src.CorrelationData):len(src.CorrelationData)]
	m.UserProperties = src.UserProperties[:len(src.UserProperties):len(src.UserProperties)]
	m.SubscriptionIdentifier = nil
	if n := len(src.SubscriptionIdentifier) + nonZeroCount(ids); n > 0 {
		m.SubscriptionIdentifier = make([]uint32, len(src.SubscriptionIdentifier), n)
		copy(m.SubscriptionIdentifier, src.SubscriptionIdentifier)
	}
	return &m
}

func nonZeroCount(ids []uint32) int {
	n := 0
	for _, id := range ids {
		if id != 0 {
			n++
		}
	}
	return n
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

// queuedPublish 把 Elem 与其 Publish 合并为一次分配（每个订阅者每条消息省一次堆分配）。
type queuedPublish struct {
	elem queue.Elem
	pub  queue.Publish
}

func newQueueElem(now time.Time, msg *gmqtt.Message, configuredExpiry time.Duration) *queue.Elem {
	qp := &queuedPublish{pub: queue.Publish{Message: msg}}
	qp.elem.At = now
	qp.elem.Expiry = queuedMessageExpiry(now, msg.MessageExpiry, configuredExpiry)
	qp.elem.MessageWithID = &qp.pub
	return &qp.elem
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

// sharedKey 标识一个共享订阅组（ShareName + TopicFilter）；用结构体作键，
// 免去每次命中都拼接 "$share/<name>/<filter>" 字符串的分配。
type sharedKey struct {
	shareName   string
	topicFilter string
}

type sharedSubscriber struct {
	clientID string
	sub      *gmqtt.Subscription
}

// sharedList 按共享订阅组记录候选客户端列表。
type sharedList map[sharedKey][]sharedSubscriber

type nonSharedMatch struct {
	sub    *gmqtt.Subscription
	subIDs []uint32
}

// maxQos 记录非共享订阅在 onlyOnce 模式下每个客户端命中的最高 QoS 订阅（值存储，免逐客户端分配）。
type maxQos map[string]nonSharedMatch

// deliverHandler 根据 DeliveryMode 统一处理普通订阅、共享订阅、overlap 与 onlyOnce 投递策略。
// 审查建议：该结构体已经比较独立，后续可围绕共享订阅均衡策略补 focused 测试。
// 性能说明：sl/mq 为惰性分配——deliverMessage 是每条消息的热路径，普通单订阅投递
// （overlap 模式）两者都用不到，按需初始化可省去每条消息两次 map 分配。
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
	if d.sl == nil {
		d.sl = make(sharedList)
	}
	k := sharedKey{shareName: sub.ShareName, topicFilter: sub.TopicFilter}
	d.sl[k] = append(d.sl[k], sharedSubscriber{clientID: clientID, sub: sub})
}

func (d *deliverHandler) deliverOverlap(clientID string, sub *gmqtt.Subscription) bool {
	d.enqueueOne(clientID, sub)
	return true
}

func (d *deliverHandler) recordOnlyOnce(clientID string, sub *gmqtt.Subscription) bool {
	// onlyOnce 模式下，同一客户端多次命中时使用最高 QoS，并合并 Subscription Identifier。
	if d.mq == nil {
		d.mq = make(maxQos)
	}
	m, ok := d.mq[clientID]
	if !ok {
		m.sub = sub
	} else if m.sub.QoS < sub.QoS {
		m.sub = sub
	}
	m.subIDs = append(m.subIDs, sub.ID)
	d.mq[clientID] = m
	return true
}

func (d *deliverHandler) flush() {
	d.flushSharedSubscriptions()
	d.flushOnlyOnceSubscriptions()
}

func (d *deliverHandler) flushSharedSubscriptions() {
	for _, v := range d.sl {
		rs := d.selectSharedSubscriber(v)
		d.enqueueOne(rs.clientID, rs.sub)
	}
}

func (d *deliverHandler) flushOnlyOnceSubscriptions() {
	for clientID, v := range d.mq {
		d.enqueue(clientID, v.sub, v.subIDs)
	}
}

func (d *deliverHandler) enqueue(clientID string, sub *gmqtt.Subscription, ids []uint32) {
	if qs := d.srv.queueStore[clientID]; qs != nil {
		d.srv.addMsgToQueueLocked(d.now, clientID, d.msg, sub, ids, qs)
	}
}

// enqueueOne 投递单个订阅命中；subscription identifier 以单元素数组在栈上传递，免切片分配。
func (d *deliverHandler) enqueueOne(clientID string, sub *gmqtt.Subscription) {
	if qs := d.srv.queueStore[clientID]; qs != nil {
		ids := [1]uint32{sub.ID}
		d.srv.addMsgToQueueLocked(d.now, clientID, d.msg, sub, ids[:], qs)
	}
}

// fnv32a 与 hash/fnv.New32a 结果一致，直接作用于 string，免 hasher 与 []byte 转换分配。
func fnv32a(s string) uint32 {
	const offset32, prime32 = 2166136261, 16777619
	h := uint32(offset32)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime32
	}
	return h
}

func (d *deliverHandler) selectSharedSubscriber(subscribers []sharedSubscriber) sharedSubscriber {
	if len(subscribers) == 1 {
		return subscribers[0]
	}
	if d.strategy == SharedSubBalanceTopicHash {
		return subscribers[int(fnv32a(d.msg.Topic))%len(subscribers)]
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
