// 文件用途：维护 persistence\subscription\mem\topic_trie.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package mem

import (
	"strings"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
)

// topicTrie
type topicTrie = topicNode

// children
type children = map[string]*topicNode

type clientOpts map[string]*gmqtt.Subscription

// topicNode
type topicNode struct {
	children children
	// clients store non-share subscription
	clients   clientOpts
	parent    *topicNode // pointer of parent node
	topicName string
	// shared store shared subscription, key by ShareName
	shared map[string]clientOpts
}

// newTopicTrie create a new trie tree
func newTopicTrie() *topicTrie {
	return newNode()
}

// newNode create a new trie node
func newNode() *topicNode {
	return &topicNode{
		children: children{},
		clients:  make(clientOpts),
		shared:   make(map[string]clientOpts),
	}
}

// newChild create a child node of t
func (t *topicNode) newChild() *topicNode {
	n := newNode()
	n.parent = t
	return n
}

// subscribe add a subscription and return the added node
func (t *topicTrie) subscribe(clientID string, s *gmqtt.Subscription) *topicNode {
	topicSlice := strings.Split(s.TopicFilter, "/")
	var pNode = t
	for _, lv := range topicSlice {
		if _, ok := pNode.children[lv]; !ok {
			pNode.children[lv] = pNode.newChild()
		}
		pNode = pNode.children[lv]
	}
	// shared subscription
	if s.ShareName != "" {
		if pNode.shared[s.ShareName] == nil {
			pNode.shared[s.ShareName] = make(clientOpts)
		}
		pNode.shared[s.ShareName][clientID] = s
	} else {
		// non-shared
		pNode.clients[clientID] = s
	}
	pNode.topicName = s.TopicFilter
	return pNode
}

// find walk through the tire and return the node that represent the topicFilter.
// Return nil if not found
func (t *topicTrie) find(topicFilter string) *topicNode {
	var pNode = t
	walkLevels(topicFilter, func(lv string) bool {
		pNode = pNode.children[lv]
		return pNode != nil
	})
	if pNode != nil && pNode.topicName == topicFilter {
		return pNode
	}
	return nil
}

// unsubscribe
func (t *topicTrie) unsubscribe(clientID string, topicName string, shareName string) {
	var pNode = t
	walkLevels(topicName, func(lv string) bool {
		pNode = pNode.children[lv]
		return pNode != nil
	})
	if pNode == nil {
		return
	}
	leaf := lastLevel(topicName)
	if shareName != "" {
		if c := pNode.shared[shareName]; c != nil {
			delete(c, clientID)
			if len(pNode.shared[shareName]) == 0 {
				delete(pNode.shared, shareName)
			}
			if len(pNode.shared) == 0 && len(pNode.children) == 0 {
				delete(pNode.parent.children, leaf)
			}
		}
	} else {
		delete(pNode.clients, clientID)
		if len(pNode.clients) == 0 && len(pNode.children) == 0 {
			delete(pNode.parent.children, leaf)
		}
	}

}

func isSystemTopic(topicName string) bool {
	return len(topicName) >= 1 && topicName[0] == '$'
}

func (t *topicTrie) preOrderTraverse(fn subscription.IterateFn) bool {
	if t == nil {
		return false
	}
	if t.topicName != "" {
		for clientID, subOpts := range t.clients {
			if !fn(clientID, subOpts) {
				return false
			}
		}

		for _, c := range t.shared {
			for clientID, subOpts := range c {
				if !fn(clientID, subOpts) {
					return false
				}
			}
		}
	}
	for _, c := range t.children {
		if !c.preOrderTraverse(fn) {
			return false
		}
	}
	return true
}
