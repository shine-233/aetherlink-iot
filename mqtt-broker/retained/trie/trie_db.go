// 文件用途：实现 retained.Store 的 trie 后端，保存和查询 MQTT retained 消息。
// 核心逻辑：用读写锁保护用户主题 trie 和系统主题 trie，并对外提供增删查、遍历和过滤匹配。
// 关键注意事项：返回消息时需要复制，避免调用方修改存储中的 retained 消息。
// 重构建议：可补充并发测试和更严格的系统主题匹配用例，确认锁与 trie 分流行为稳定。
package trie

import (
	"sync"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/retained"
)

// trieDB implement the retain.Store, it use trie tree  to store retain messages .
type trieDB struct {
	sync.RWMutex
	userTrie   *topicTrie
	systemTrie *topicTrie
}

func (t *trieDB) Iterate(fn retained.IterateFn) {
	t.RLock()
	defer t.RUnlock()
	if !t.userTrie.preOrderTraverse(fn) {
		return
	}
	t.systemTrie.preOrderTraverse(fn)
}

func (t *trieDB) getTrie(topicName string) *topicTrie {
	if isSystemTopic(topicName) {
		return t.systemTrie
	}
	return t.userTrie
}

// GetRetainedMessage return the retain message of the given topic name.
// return nil if the topic name not exists
func (t *trieDB) GetRetainedMessage(topicName string) *gmqtt.Message {
	t.RLock()
	defer t.RUnlock()
	node := t.getTrie(topicName).find(topicName)
	if node != nil {
		return node.msg.Copy()
	}
	return nil
}

// ClearAll clear all retain messages.
func (t *trieDB) ClearAll() {
	t.Lock()
	defer t.Unlock()
	t.systemTrie = newTopicTrie()
	t.userTrie = newTopicTrie()
}

// AddOrReplace add or replace a retain message.
func (t *trieDB) AddOrReplace(message *gmqtt.Message) {
	t.Lock()
	defer t.Unlock()
	t.getTrie(message.Topic).addRetainMsg(message.Topic, message)
}

// remove remove the retain message of the topic name.
func (t *trieDB) Remove(topicName string) {
	t.Lock()
	defer t.Unlock()
	t.getTrie(topicName).remove(topicName)
}

// GetMatchedMessages returns all messages that match the topic filter.
func (t *trieDB) GetMatchedMessages(topicFilter string) []*gmqtt.Message {
	t.RLock()
	defer t.RUnlock()
	return t.getTrie(topicFilter).getMatchedMessages(topicFilter)
}

func NewStore() *trieDB {
	return &trieDB{
		userTrie:   newTopicTrie(),
		systemTrie: newTopicTrie(),
	}
}
