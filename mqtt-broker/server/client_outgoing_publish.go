// 文件用途：集中处理客户端出站 PUBLISH 写出前的协议后处理。
// 核心逻辑：按旧顺序执行 MQTT v5 topic alias、OnDelivered hook 和消息发送统计。
// 使用注意：该顺序属于 broker 投递兼容边界，重构时不要把 hook 或统计提前到 alias 处理之前。
// 重构建议：后续如扩展出站 publish 观测能力，可在本文件追加小 helper，避免重新塞回 client.go 或 client_packet_io.go。
package server

import (
	"context"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

// prepareOutgoingPublish 保持旧有顺序：先处理 MQTT v5 topic alias，再触发 OnDelivered hook，最后记录消息发送统计。
func (client *client) prepareOutgoingPublish(p *packets.Publish) {
	if client.version == packets.Version5 {
		client.applyOutgoingTopicAlias(p)
	}
	client.notifyPublishDelivered(p)
	client.recordOutgoingPublish(p)
}

func (client *client) applyOutgoingTopicAlias(p *packets.Publish) {
	if client.opts.ClientTopicAliasMax == 0 {
		return
	}
	if alias, ok := client.topicAliasManager.Check(p); ok {
		p.TopicName = []byte{}
		p.Properties.TopicAlias = &alias
	} else if alias != 0 {
		p.Properties.TopicAlias = &alias
	}
}

func (client *client) notifyPublishDelivered(p *packets.Publish) {
	if client.server.hooks.OnDelivered != nil {
		client.server.hooks.OnDelivered(context.Background(), client, gmqtt.MessageFromPublish(p))
	}
}

func (client *client) recordOutgoingPublish(p *packets.Publish) {
	client.server.statsManager.messageSent(p.Qos, client.opts.ClientID)
}
