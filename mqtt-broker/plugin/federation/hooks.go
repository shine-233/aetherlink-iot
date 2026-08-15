// 文件用途：维护 plugin\federation\hooks.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package federation

import (
	"context"
	"sort"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/server"
)

func (f *Federation) HookWrapper() server.HookWrapper {
	return server.HookWrapper{
		OnSubscribedWrapper:        f.OnSubscribedWrapper,
		OnUnsubscribedWrapper:      f.OnUnsubscribedWrapper,
		OnMsgArrivedWrapper:        f.OnMsgArrivedWrapper,
		OnSessionTerminatedWrapper: f.OnSessionTerminatedWrapper,
		OnWillPublishWrapper:       f.OnWillPublishWrapper,
	}
}

func (f *Federation) OnSubscribedWrapper(pre server.OnSubscribed) server.OnSubscribed {
	return func(ctx context.Context, client server.Client, subscription *gmqtt.Subscription) {
		pre(ctx, client, subscription)
		if subscription != nil {
			if !f.localSubStore.subscribe(client.ClientOptions().ClientID, subscription.GetFullTopicName()) {
				return
			}
			// only send new subscription
			f.memberMu.Lock()
			defer f.memberMu.Unlock()
			for _, v := range f.peers {
				sub := &Subscribe{
					ShareName:   subscription.ShareName,
					TopicFilter: subscription.TopicFilter,
				}
				v.queue.add(&Event{
					Event: &Event_Subscribe{
						Subscribe: sub,
					}})
			}
		}
	}
}

func (f *Federation) OnUnsubscribedWrapper(pre server.OnUnsubscribed) server.OnUnsubscribed {
	return func(ctx context.Context, client server.Client, topicName string) {
		pre(ctx, client, topicName)
		if !f.localSubStore.unsubscribe(client.ClientOptions().ClientID, topicName) {
			return
		}
		// only unsubscribe topic if there is no local subscriber anymore.
		f.memberMu.Lock()
		defer f.memberMu.Unlock()
		for _, v := range f.peers {
			unsub := &Unsubscribe{
				TopicName: topicName,
			}
			v.queue.add(&Event{
				Event: &Event_Unsubscribe{
					Unsubscribe: unsub,
				}})
		}
	}
}

func sendSharedMsg(fs *fedSubStore, sharedList map[string][]string, send func(nodeName string, topicName string)) {
	// shared subscription
	fs.sharedMu.Lock()
	defer fs.sharedMu.Unlock()
	for topicName, v := range sharedList {
		sort.Strings(v)
		mod := fs.sharedSent[topicName] % (uint64(len(v)))
		fs.sharedSent[topicName]++
		send(v[mod], topicName)
	}
}

// sendMessage sends messages to cluster nodes.
// For retained message, broadcasts the message to all nodes to update their local retained store.
// For none retained message , send it to the nodes which have matched topics.
// For shared subscription, we should either only send the message to local subscriber or only send the message to one node.
// If drop is true, the local node will drop the message.
// If options is not nil, the local node will apply the options to topic matching process.
func (f *Federation) sendMessage(msg *gmqtt.Message) (drop bool, options *subscription.IterationOptions) {
	f.memberMu.Lock()
	defer f.memberMu.Unlock()

	if msg.Retained {
		eventMsg := messageToEvent(msg)
		for _, v := range f.peers {
			v.queue.add(&Event{
				Event: &Event_Message{
					Message: eventMsg,
				}})
		}
		return
	}

	// shared topic => []nodeName.
	sharedList := make(map[string][]string)
	// append local shared subscription
	f.localSubStore.localStore.Iterate(func(clientID string, sub *gmqtt.Subscription) bool {
		fullTopic := sub.GetFullTopicName()
		sharedList[fullTopic] = append(sharedList[fullTopic], f.nodeName)
		return true
	}, subscription.IterationOptions{
		Type:      subscription.TypeShared,
		TopicName: msg.Topic,
		MatchType: subscription.MatchFilter,
	})

	// store non-shared topic, key by nodeName
	nonShared := make(map[string]struct{})

	f.fedSubStore.Iterate(func(nodeName string, sub *gmqtt.Subscription) bool {
		if sub.ShareName != "" {
			fullTopic := sub.GetFullTopicName()
			sharedList[fullTopic] = append(sharedList[fullTopic], nodeName)
			return true
		}
		nonShared[nodeName] = struct{}{}
		return true
	}, subscription.IterationOptions{
		Type:      subscription.TypeAll,
		TopicName: msg.Topic,
		MatchType: subscription.MatchFilter,
	})

	sent := make(map[string]struct{})
	// shared subscription
	sendSharedMsg(f.fedSubStore, sharedList, func(nodeName string, topicName string) {
		// Do nothing if it is the local node.
		if nodeName == f.nodeName {
			return
		}
		if _, ok := sent[nodeName]; ok {
			return
		}
		sent[nodeName] = struct{}{}
		if p, ok := f.peers[nodeName]; ok {
			eventMsg := messageToEvent(msg)
			p.queue.add(&Event{
				Event: &Event_Message{
					Message: eventMsg,
				}})
			drop = true
			nonSharedOpts := subscription.IterationOptions{
				Type:      subscription.TypeAll ^ subscription.TypeShared,
				TopicName: msg.Topic,
				MatchType: subscription.MatchFilter,
			}
			f.localSubStore.localStore.Iterate(func(clientID string, sub *gmqtt.Subscription) bool {
				// If the message also matches non-shared subscription in local node, it can not be dropped.
				// But the broker must not match any local shared subscriptions for this message,
				// so we modify the iterationOptions to ignore shared subscriptions.
				drop = false
				options = &nonSharedOpts
				return false
			}, nonSharedOpts)
		}
	})
	// non-shared subscription
	for nodeName := range nonShared {
		if _, ok := sent[nodeName]; ok {
			continue
		}
		if p, ok := f.peers[nodeName]; ok {
			eventMsg := messageToEvent(msg)
			p.queue.add(&Event{
				Event: &Event_Message{
					Message: eventMsg,
				}})
		}
	}
	return
}
func (f *Federation) OnMsgArrivedWrapper(pre server.OnMsgArrived) server.OnMsgArrived {
	return func(ctx context.Context, client server.Client, req *server.MsgArrivedRequest) error {
		err := pre(ctx, client, req)
		if err != nil {
			return err
		}
		if req.Message != nil {
			drop, opts := f.sendMessage(req.Message)
			if drop {
				req.Drop()
			}
			if opts != nil {
				req.IterationOptions = *opts
			}
		}
		return nil
	}
}

func (f *Federation) OnSessionTerminatedWrapper(pre server.OnSessionTerminated) server.OnSessionTerminated {
	return func(ctx context.Context, clientID string, reason server.SessionTerminatedReason) {
		pre(ctx, clientID, reason)
		if unsubs := f.localSubStore.unsubscribeAll(clientID); len(unsubs) != 0 {
			f.memberMu.Lock()
			defer f.memberMu.Unlock()
			for _, v := range f.peers {
				for _, topicName := range unsubs {
					unsub := &Unsubscribe{
						TopicName: topicName,
					}
					v.queue.add(&Event{
						Event: &Event_Unsubscribe{
							Unsubscribe: unsub,
						}})
				}
			}
		}
	}

}

func (f *Federation) OnWillPublishWrapper(pre server.OnWillPublish) server.OnWillPublish {
	return func(ctx context.Context, clientID string, req *server.WillMsgRequest) {
		pre(ctx, clientID, req)
		if req.Message != nil {
			drop, opts := f.sendMessage(req.Message)
			if drop {
				req.Drop()
			}
			if opts != nil {
				req.IterationOptions = *opts
			}
		}
	}
}
