// 文件用途：维护 plugin\admin\publish.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package admin

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

type publisher struct {
	a *Admin
}

func (p *publisher) mustEmbedUnimplementedPublishServiceServer() {
	return
}

// Publish publishes a message into broker.
func (p *publisher) Publish(ctx context.Context, req *PublishRequest) (resp *empty.Empty, err error) {
	if !packets.ValidTopicName(false, []byte(req.TopicName)) {
		return nil, ErrInvalidArgument("topic_name", "")
	}
	if req.Qos > uint32(packets.Qos2) {
		return nil, ErrInvalidArgument("qos", "")
	}
	if req.PayloadFormat != 0 && req.PayloadFormat != 1 {
		return nil, ErrInvalidArgument("payload_format", "")
	}
	if req.ResponseTopic != "" && !packets.ValidV5Topic([]byte(req.ResponseTopic)) {
		return nil, ErrInvalidArgument("response_topic", "")
	}
	var userPpt []packets.UserProperty
	for _, v := range req.UserProperties {
		userPpt = append(userPpt, packets.UserProperty{
			K: v.K,
			V: v.V,
		})
	}

	p.a.publisher.Publish(&gmqtt.Message{
		Dup:             false,
		QoS:             byte(req.Qos),
		Retained:        req.Retained,
		Topic:           req.TopicName,
		Payload:         []byte(req.Payload),
		ContentType:     req.ContentType,
		CorrelationData: []byte(req.CorrelationData),
		MessageExpiry:   req.MessageExpiry,
		PayloadFormat:   packets.PayloadFormat(req.PayloadFormat),
		ResponseTopic:   req.ResponseTopic,
		UserProperties:  userPpt,
	})
	return &empty.Empty{}, nil
}
