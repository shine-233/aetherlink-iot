package server

import (
	"context"
	"time"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"go.uber.org/zap"
)

func (client *client) subscribeHandler(sub *packets.Subscribe) *codes.Error {
	suback := newSubscribeAck(sub)
	now := time.Now()

	subID, err := client.validateSubscribeID(sub)
	if err != nil {
		return err
	}

	subReq := newSubscribeRequest(sub, subID)
	if client.callOnSubscribeHook(subReq, suback) {
		return nil
	}

	for k, topic := range sub.Topics {
		result := client.subscribeTopic(topic, subReq, subID)
		suback.Payload[k] = result.code
		if result.code < packets.SubscribeFailure {
			client.callOnSubscribedHook(result.sub)
			client.logSubscribeSucceeded(result.sub)
			if err := client.deliverRetainedMessages(topic, result.sub, result.subRs, now, result.isShared); err != nil {
				return err
			}
		} else {
			client.logSubscribeFailed(result.sub, result.code)
		}
	}
	client.write(suback)
	return nil
}

type subscribeTopicResult struct {
	sub      *gmqtt.Subscription
	subRs    subscription.SubscribeResult
	code     codes.Code
	isShared bool
}

func newSubscribeAck(sub *packets.Subscribe) *packets.Suback {
	return &packets.Suback{
		Version:    sub.Version,
		PacketID:   sub.PacketID,
		Properties: &packets.Properties{},
		Payload:    make([]codes.Code, len(sub.Topics)),
	}
}

func (client *client) validateSubscribeID(sub *packets.Subscribe) (uint32, *codes.Error) {
	var subID uint32
	if client.version != packets.Version5 {
		return subID, nil
	}
	if client.opts.SubIDAvailable && len(sub.Properties.SubscriptionIdentifier) != 0 {
		subID = sub.Properties.SubscriptionIdentifier[0]
	}
	if !client.config.MQTT.SubscriptionIDAvailable && subID != 0 {
		return 0, &codes.Error{
			Code: codes.SubIDNotSupported,
		}
	}
	return subID, nil
}

func newSubscribeRequest(sub *packets.Subscribe, subID uint32) *SubscribeRequest {
	subReq := &SubscribeRequest{
		Subscribe: sub,
		Subscriptions: make(map[string]*struct {
			Sub   *gmqtt.Subscription
			Error error
		}),
		ID: subID,
	}

	for _, v := range sub.Topics {
		subReq.Subscriptions[v.Name] = &struct {
			Sub   *gmqtt.Subscription
			Error error
		}{Sub: subscription.FromTopic(v, subID), Error: nil}
	}
	return subReq
}

func (client *client) callOnSubscribeHook(subReq *SubscribeRequest, suback *packets.Suback) bool {
	if client.server.hooks.OnSubscribe == nil {
		return false
	}
	err := client.server.hooks.OnSubscribe(context.Background(), client, subReq)
	if ce := converError(err); ce != nil {
		suback.Properties = getErrorProperties(client, &ce.ErrorDetails)
		for k := range suback.Payload {
			if packets.IsVersion3X(client.version) {
				suback.Payload[k] = packets.SubscribeFailure
			} else {
				suback.Payload[k] = ce.Code
			}
		}
		client.write(suback)
		return true
	}
	return false
}

func (client *client) subscribeTopic(topic packets.Topic, subReq *SubscribeRequest, subID uint32) subscribeTopicResult {
	srv := client.server
	sub := subReq.Subscriptions[topic.Name].Sub
	subErr := converError(subReq.Subscriptions[topic.Name].Error)
	code, isShared := client.subscribeReturnCode(sub, subErr, subID)
	var subRs subscription.SubscribeResult

	if code < packets.SubscribeFailure {
		var err error
		subRs, err = srv.subscriptionsDB.Subscribe(client.opts.ClientID, sub)
		if err != nil {
			zaplog.Error("failed to subscribe topic",
				zap.String("topic", topic.Name),
				zap.Uint8("qos", topic.Qos),
				zap.String("client_id", client.opts.ClientID),
				zap.String("remote_addr", client.rwc.RemoteAddr().String()),
				zap.Error(err))
			code = packets.SubscribeFailure
		}
	}

	return subscribeTopicResult{
		sub:      sub,
		subRs:    subRs,
		code:     code,
		isShared: isShared,
	}
}

func (client *client) subscribeReturnCode(sub *gmqtt.Subscription, subErr *codes.Error, subID uint32) (codes.Code, bool) {
	var isShared bool
	code := sub.QoS
	if client.version == packets.Version5 {
		if sub.ShareName != "" {
			isShared = true
			if !client.opts.SharedSubAvailable {
				code = codes.SharedSubNotSupported
			}
		}
		if !client.opts.SubIDAvailable && subID != 0 {
			code = codes.SubIDNotSupported
		}
		if !client.opts.WildcardSubAvailable {
			for _, c := range sub.TopicFilter {
				if c == '+' || c == '#' {
					code = codes.WildcardSubNotSupported
					break
				}
			}
		}
	}

	if subErr != nil {
		code = subErr.Code
		if packets.IsVersion3X(client.version) {
			code = packets.SubscribeFailure
		}
	}

	return code, isShared
}

func (client *client) callOnSubscribedHook(sub *gmqtt.Subscription) {
	if client.server.hooks.OnSubscribed != nil {
		client.server.hooks.OnSubscribed(context.Background(), client, sub)
	}
}

func (client *client) logSubscribeSucceeded(sub *gmqtt.Subscription) {
	zaplog.Info("subscribe succeeded",
		zap.String("topic", sub.TopicFilter),
		zap.Uint8("qos", sub.QoS),
		zap.Uint8("retain_handling", sub.RetainHandling),
		zap.Bool("retain_as_published", sub.RetainAsPublished),
		zap.Bool("no_local", sub.NoLocal),
		zap.Uint32("id", sub.ID),
		zap.String("client_id", client.opts.ClientID),
		zap.String("remote_addr", client.rwc.RemoteAddr().String()),
	)
}

func (client *client) logSubscribeFailed(sub *gmqtt.Subscription, code codes.Code) {
	zaplog.Info("subscribe failed",
		zap.String("topic", sub.TopicFilter),
		zap.Uint8("qos", code),
		zap.String("client_id", client.opts.ClientID),
		zap.String("remote_addr", client.rwc.RemoteAddr().String()),
	)
}

func (client *client) deliverRetainedMessages(topic packets.Topic, sub *gmqtt.Subscription, subRs subscription.SubscribeResult, now time.Time, isShared bool) *codes.Error {
	// Keep the existing mosquitto-compatible retained-message behavior for no-local subscriptions.
	if isShared || !shouldDeliverRetainedMessages(topic, subRs) {
		return nil
	}

	msgs := client.server.retainedDB.GetMatchedMessages(sub.TopicFilter)
	for _, v := range msgs {
		if v.QoS > subRs[0].Subscription.QoS {
			v.QoS = subRs[0].Subscription.QoS
		}
		v.Dup = false
		if !sub.RetainAsPublished {
			v.Retained = false
		}
		var expiry time.Time
		if v.MessageExpiry != 0 {
			expiry = now.Add(time.Second * time.Duration(v.MessageExpiry))
		}
		err := client.queueStore.Add(&queue.Elem{
			At:     now,
			Expiry: expiry,
			MessageWithID: &queue.Publish{
				Message: v,
			},
		})
		if err != nil {
			return client.handleRetainedQueueError(v, err)
		}
	}
	return nil
}

func shouldDeliverRetainedMessages(topic packets.Topic, subRs subscription.SubscribeResult) bool {
	return (!subRs[0].AlreadyExisted && topic.RetainHandling != 2) || topic.RetainHandling == 0
}

func (client *client) handleRetainedQueueError(msg *gmqtt.Message, err error) *codes.Error {
	client.queueNotifier.notifyDropped(msg, &queue.InternalError{Err: err})
	if codesErr, ok := err.(*codes.Error); ok {
		return codesErr
	}
	return &codes.Error{
		Code: codes.UnspecifiedError,
	}
}
