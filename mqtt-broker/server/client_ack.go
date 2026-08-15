package server

import (
	"time"

	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// pubackHandler 完成 QoS1 出站消息确认：从队列删除消息并释放 PacketID。
// 使用注意：该函数与 queueStore/inflight 强绑定，后续拆分时要和 pubrec/pubcomp 的释放策略一起移动。
func (client *client) pubackHandler(puback *packets.Puback) *codes.Error {
	err := client.queueStore.Remove(puback.PacketID)
	if err != nil {
		return converError(err)
	}
	client.pl.release(puback.PacketID)
	if ce := zaplog.Check(zapcore.DebugLevel, "unset inflight"); ce != nil {
		ce.Write(zap.String("clientID", client.opts.ClientID),
			zap.Uint16("pid", puback.PacketID),
		)
	}
	return nil
}

// pubrelHandler 完成 QoS2 入站第二阶段，释放 unack 记录并返回 PUBCOMP。
func (client *client) pubrelHandler(pubrel *packets.Pubrel) *codes.Error {
	err := client.unackStore.Remove(pubrel.PacketID)
	if err != nil {
		return converError(err)
	}
	pubcomp := pubrel.NewPubcomp()
	client.write(pubcomp)
	return nil
}

// pubrecHandler 处理 QoS2 出站消息的第一阶段确认，成功时把队列元素替换为 PUBREL。
func (client *client) pubrecHandler(pubrec *packets.Pubrec) {
	if client.version == packets.Version5 && pubrec.Code >= codes.UnspecifiedError {
		err := client.queueStore.Remove(pubrec.PacketID)
		client.pl.release(pubrec.PacketID)
		if err != nil {
			client.setError(err)
		}
		return
	}
	pubrel := pubrec.NewPubrel()
	_, err := client.queueStore.Replace(&queue.Elem{
		At: time.Now(),
		MessageWithID: &queue.Pubrel{
			PacketID: pubrel.PacketID,
		}})
	if err != nil {
		client.setError(err)
	}
	client.write(pubrel)
}

// pubcompHandler 完成 QoS2 出站消息确认，清理队列并释放 PacketID。
func (client *client) pubcompHandler(pubcomp *packets.Pubcomp) {
	err := client.queueStore.Remove(pubcomp.PacketID)
	client.pl.release(pubcomp.PacketID)
	if err != nil {
		client.setError(err)
	}
}
