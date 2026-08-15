package server

import (
	"context"

	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"go.uber.org/zap"
)

// unsubscribeHandler 处理 UNSUBSCRIBE：调用 hook、删除订阅、触发 OnUnsubscribed 并回写 UNSUBACK。
// 审查建议：MQTT v5 的每主题返回码与 hook 错误传播需要补充 focused 测试后再继续拆分。
func (client *client) unsubscribeHandler(unSub *packets.Unsubscribe) {
	unSuback := newUnsubscribeAck(unSub)
	cs := make([]codes.Code, len(unSub.Topics))
	defer func() {
		if client.version == packets.Version5 {
			unSuback.Payload = cs
		}
		client.write(unSuback)
	}()

	req := newUnsubscribeRequest(unSub)
	if client.callOnUnsubscribeHook(req, unSuback, cs) {
		return
	}

	for k, topic := range unSub.Topics {
		code := client.unsubscribeTopic(req, topic)
		client.logUnsubscribeResult(req.Unsubs[topic].TopicName, code)
		cs[k] = code
	}
}

func newUnsubscribeAck(unSub *packets.Unsubscribe) *packets.Unsuback {
	return &packets.Unsuback{
		Version:    unSub.Version,
		PacketID:   unSub.PacketID,
		Properties: &packets.Properties{},
	}
}

func newUnsubscribeRequest(unSub *packets.Unsubscribe) *UnsubscribeRequest {
	req := &UnsubscribeRequest{
		Unsubscribe: unSub,
		Unsubs: make(map[string]*struct {
			TopicName string
			Error     error
		}),
	}

	for _, v := range unSub.Topics {
		req.Unsubs[v] = &struct {
			TopicName string
			Error     error
		}{TopicName: v}
	}
	return req
}

func (client *client) callOnUnsubscribeHook(req *UnsubscribeRequest, unSuback *packets.Unsuback, ackCodes []codes.Code) bool {
	if client.server.hooks.OnUnsubscribe == nil {
		return false
	}

	err := client.server.hooks.OnUnsubscribe(context.Background(), client, req)
	if ce := converError(err); ce != nil {
		unSuback.Properties = getErrorProperties(client, &ce.ErrorDetails)
		for k := range ackCodes {
			ackCodes[k] = ce.Code
		}
		return true
	}
	return false
}

func (client *client) unsubscribeTopic(req *UnsubscribeRequest, requestedTopic string) codes.Code {
	code := codes.Success
	topicName := req.Unsubs[requestedTopic].TopicName
	if ce := converError(req.Unsubs[requestedTopic].Error); ce != nil {
		code = ce.Code
	}
	if code != codes.Success {
		return code
	}

	err := client.server.subscriptionsDB.Unsubscribe(client.opts.ClientID, topicName)
	if ce := converError(err); ce != nil {
		return ce.Code
	}

	client.callOnUnsubscribedHook(topicName)
	return codes.Success
}

func (client *client) callOnUnsubscribedHook(topicName string) {
	if client.server.hooks.OnUnsubscribed != nil {
		client.server.hooks.OnUnsubscribed(context.Background(), client, topicName)
	}
}

func (client *client) logUnsubscribeResult(topicName string, code codes.Code) {
	if code == codes.Success {
		zaplog.Info("unsubscribed succeed",
			zap.String("topic", topicName),
			zap.String("client_id", client.opts.ClientID),
			zap.String("remote_addr", client.rwc.RemoteAddr().String()),
		)
		return
	}

	zaplog.Info("unsubscribed failed",
		zap.String("topic", topicName),
		zap.String("client_id", client.opts.ClientID),
		zap.String("remote_addr", client.rwc.RemoteAddr().String()),
		zap.Uint8("code", code))
}
