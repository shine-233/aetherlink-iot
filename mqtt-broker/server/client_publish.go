package server

import (
	"context"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

// publishHandler 处理客户端入站 PUBLISH：能力校验、topic alias 解析、QoS2 去重、retained 存储、订阅投递和 PUBACK/PUBREC。
// 审查建议：这是 client.go 中风险最高的拆分点之一；后续应只先搬移纯 helper，并保留 ack code 与 hook 顺序。
func (client *client) publishHandler(pub *packets.Publish) *codes.Error {
	msg, codeErr := client.validatePublish(pub)
	if codeErr != nil {
		return codeErr
	}

	dup, codeErr := client.trackQoS2Publish(pub)
	if codeErr != nil {
		return codeErr
	}

	client.storeRetainedPublish(pub, msg)

	topicMatched, hookErr := client.deliverPublish(pub, msg, dup)
	return client.writePublishAck(pub, topicMatched, hookErr)
}

func (client *client) validatePublish(pub *packets.Publish) (*gmqtt.Message, *codes.Error) {
	if !client.opts.RetainAvailable && pub.Retain {
		return nil, &codes.Error{
			Code: codes.RetainNotSupported,
		}
	}

	msg := gmqtt.MessageFromPublish(pub)
	if codeErr := client.applyPublishTopicAlias(pub, msg); codeErr != nil {
		return nil, codeErr
	}
	return msg, nil
}

func (client *client) applyPublishTopicAlias(pub *packets.Publish, msg *gmqtt.Message) *codes.Error {
	if client.version != packets.Version5 || pub.Properties.TopicAlias == nil {
		return nil
	}

	topicAlias := *pub.Properties.TopicAlias
	// Topic Alias 为 0 是无效值，见 MQTT v5 规范 [MQTT-3.3.2-7]
	// Topic Alias Maximum 是服务端接受的最高值（含），见 [MQTT-3.1.2-26]
	if topicAlias == 0 || topicAlias > client.opts.ServerTopicAliasMax {
		return &codes.Error{
			Code: codes.TopicAliasInvalid,
		}
	}
	// 边界检查：确保 topicAlias 不会导致 aliasMapper 索引越界
	if int(topicAlias) >= len(client.aliasMapper) {
		return &codes.Error{
			Code: codes.TopicAliasInvalid,
		}
	}

	name := client.aliasMapper[int(topicAlias)]
	if len(pub.TopicName) == 0 {
		if len(name) == 0 {
			return &codes.Error{
				Code: codes.TopicAliasInvalid,
			}
		}
		msg.Topic = string(name)
		return nil
	}

	client.aliasMapper[int(topicAlias)] = pub.TopicName
	return nil
}

// trackQoS2Publish 记录 QoS2 PacketID；重复包不会再次投递业务消息，只继续完成协议确认链。
func (client *client) trackQoS2Publish(pub *packets.Publish) (bool, *codes.Error) {
	if pub.Qos != packets.Qos2 {
		return false, nil
	}

	exist, err := client.unackStore.Set(pub.PacketID)
	if err != nil {
		return false, converError(err)
	}
	return exist, nil
}

func (client *client) storeRetainedPublish(pub *packets.Publish, msg *gmqtt.Message) {
	if !pub.Retain {
		return
	}

	if len(pub.Payload) == 0 {
		client.server.retainedDB.Remove(string(pub.TopicName))
		return
	}
	client.server.retainedDB.AddOrReplace(msg.Copy())
}

// deliverPublish 调用 OnMsgArrived hook 后执行订阅匹配投递。
// 使用注意：hook 可以修改消息和迭代选项，也可以返回 nil 消息来丢弃本次投递。
func (client *client) deliverPublish(pub *packets.Publish, msg *gmqtt.Message, dup bool) (bool, error) {
	if dup {
		return false, nil
	}

	opts := defaultIterateOptions(msg.Topic)
	var err error
	if client.server.hooks.OnMsgArrived != nil {
		msg, opts, err = client.callOnMsgArrivedHook(pub, msg, opts)
	}
	if msg != nil && err == nil {
		return client.deliverMessage(client.opts.ClientID, msg, opts), nil
	}
	return false, err
}

func (client *client) callOnMsgArrivedHook(pub *packets.Publish, msg *gmqtt.Message, opts subscription.IterationOptions) (*gmqtt.Message, subscription.IterationOptions, error) {
	req := &MsgArrivedRequest{
		Publish:          pub,
		Message:          msg,
		IterationOptions: opts,
	}
	err := client.server.hooks.OnMsgArrived(context.Background(), client, req)
	return req.Message, req.IterationOptions, err
}

func (client *client) writePublishAck(pub *packets.Publish, topicMatched bool, err error) *codes.Error {
	code, ppt := client.publishAckCode(topicMatched, err)
	var ack packets.Packet
	if pub.Qos == packets.Qos1 {
		ack = pub.NewPuback(code, ppt)
	}
	if pub.Qos == packets.Qos2 {
		ack = pub.NewPubrec(code, ppt)
		if code >= codes.UnspecifiedError {
			// QoS2 在 PUBREC 阶段失败时需要释放 unack 记录，避免 PacketID 长期占用。
			err = client.unackStore.Remove(pub.PacketID)
			if err != nil {
				return converError(err)
			}
		}
	}
	if ack != nil {
		client.write(ack)
	}
	return nil
}

func (client *client) publishAckCode(topicMatched bool, err error) (codes.Code, *packets.Properties) {
	code := codes.Success
	var ppt *packets.Properties
	if client.version == packets.Version5 {
		if !topicMatched && err == nil {
			code = codes.NotMatchingSubscribers
		}
		if codeErr := converError(err); codeErr != nil {
			ppt = getErrorProperties(client, &codeErr.ErrorDetails)
			code = codeErr.Code
		}
	}
	return code, ppt
}
