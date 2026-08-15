// 文件用途：实现 FIFO 策略的 MQTT topic alias 管理器。
// 核心逻辑：为每个客户端维护 topic 到 alias 的映射，alias 达到上限后复用最早写入的编号。
// 关键注意事项：该管理器按单客户端状态设计，调用方需要保证同一客户端上下文中的使用边界。
// 重构建议：可补充 maxAlias 为 0 的边界处理和并发访问约束说明，避免配置异常导致运行时问题。
package fifo

import (
	"container/list"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"github.com/DrmagicE/gmqtt/server"
)

var _ server.TopicAliasManager = (*Queue)(nil)

func init() {
	server.RegisterTopicAliasMgrFactory("fifo", New)
}

// New is the constructor of Queue.
func New(config config.Config, maxAlias uint16, clientID string) server.TopicAliasManager {
	return &Queue{
		clientID: clientID,
		topicAlias: &topicAlias{
			max:   int(maxAlias),
			alias: list.New(),
			index: make(map[string]uint16),
		},
	}
}

// Queue is the fifo queue which store all topic alias for one client
type Queue struct {
	clientID   string
	topicAlias *topicAlias
}
type topicAlias struct {
	max   int
	alias *list.List
	// topic name => alias
	index map[string]uint16
}
type aliasElem struct {
	topic string
	alias uint16
}

func (q *Queue) Check(publish *packets.Publish) (alias uint16, exist bool) {
	topicName := string(publish.TopicName)
	// alias exist
	if a, ok := q.topicAlias.index[topicName]; ok {
		return a, true
	}
	l := q.topicAlias.alias.Len()
	// alias has been exhausted
	if l == q.topicAlias.max {
		first := q.topicAlias.alias.Front()
		elem := first.Value.(*aliasElem)
		q.topicAlias.alias.Remove(first)
		delete(q.topicAlias.index, elem.topic)
		alias = elem.alias
	} else {
		alias = uint16(l + 1)
	}
	q.topicAlias.alias.PushBack(&aliasElem{
		topic: topicName,
		alias: alias,
	})
	q.topicAlias.index[topicName] = alias
	return
}
