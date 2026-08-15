// 文件用途：承载客户端连接成功后的 inflight 消息重放和离线队列轮询。
// 核心逻辑：恢复 QoS1/QoS2 inflight 包、计算 MQTT v5 Message Expiry 剩余时间、分配 packet id 并投递新队列消息。
// 使用注意：`pollInflights` 在 packet id limiter 锁内写出重放包，移动或重构时不能顺手调整锁范围。
// 重构建议：后续如优化重放性能，应先补 focused broker 用例锁定 Dup 标志、Subscription Identifier 清理和未使用 packet id 释放语义。
package server

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/pkg/bitmap"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

func (client *client) newPacketIDLimiter(limit uint16) {
	client.pl = &packetIDLimiter{
		cond:      sync.NewCond(&sync.Mutex{}),
		used:      0,
		limit:     limit,
		exit:      false,
		freePid:   1,
		lockedPid: bitmap.New(packets.MaxPacketID),
	}
}

func (client *client) pollInflights() (cont bool, err error) {
	var elems []*queue.Elem
	elems, err = client.queueStore.ReadInflight(uint(client.opts.MaxInflight))
	if err != nil || len(elems) == 0 {
		return false, err
	}
	client.pl.lock()
	defer client.pl.unlock()
	now := time.Now()
	for _, v := range elems {
		id := v.MessageWithID.ID()
		switch m := v.MessageWithID.(type) {
		case *queue.Publish:
			m.Dup = true
			// https://docs.oasis-open.org/mqtt/mqtt/v5.0/os/mqtt-v5.0-os.html#_Subscription_Options
			// The Server need not use the same set of Subscription Identifiers in the retransmitted PUBLISH packet.
			m.SubscriptionIdentifier = nil
			client.pl.markUsedLocked(id)
			client.write(gmqtt.MessageToPublish(messageForDelivery(m.Message, client.version, v.At, now), client.version))
		case *queue.Pubrel:
			client.write(&packets.Pubrel{PacketID: id})
		}
	}

	return true, nil
}

func messageForDelivery(msg *gmqtt.Message, version packets.Version, queuedAt, now time.Time) *gmqtt.Message {
	if version != packets.Version5 || msg.MessageExpiry == 0 || queuedAt.IsZero() || !now.After(queuedAt) {
		return msg
	}
	elapsed := uint32(now.Sub(queuedAt) / time.Second)
	if elapsed == 0 {
		return msg
	}
	deliverMsg := msg.Copy()
	if elapsed >= msg.MessageExpiry {
		deliverMsg.MessageExpiry = 1
	} else {
		deliverMsg.MessageExpiry -= elapsed
	}
	return deliverMsg
}

func (client *client) pollNewMessages(ids []packets.PacketID) (unused []packets.PacketID, err error) {
	now := time.Now()
	var elems []*queue.Elem
	elems, err = client.queueStore.Read(ids)
	if err != nil {
		return nil, err
	}
	for _, v := range elems {
		switch m := v.MessageWithID.(type) {
		case *queue.Publish:
			if m.QoS != packets.Qos0 {
				ids = ids[1:]
			}
			client.write(gmqtt.MessageToPublish(messageForDelivery(m.Message, client.version, v.At, now), client.version))
		case *queue.Pubrel:
		}
	}
	return ids, err
}

func (client *client) pollMessageHandler() {
	var err error
	defer func() {
		if re := recover(); re != nil {
			err = errors.New(fmt.Sprint(re))
		}
		client.setError(err)
	}()
	// drain all inflight messages
	cont := true
	for cont {
		cont, err = client.pollInflights()
		if err != nil {
			return
		}
	}
	var ids []packets.PacketID
	for {
		max := uint16(100)
		if client.opts.MaxInflight < max {
			max = client.opts.MaxInflight
		}
		ids = client.pl.pollPacketIDs(max)
		if ids == nil {
			return
		}
		ids, err = client.pollNewMessages(ids)
		if err != nil {
			return
		}
		client.pl.batchRelease(ids)
	}
}
