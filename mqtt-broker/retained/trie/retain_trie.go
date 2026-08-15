// 文件用途：实现 retained 消息存储使用的主题前缀树节点和匹配算法。
// 核心逻辑：按主题层级建立 trie，支持精确查找、通配符匹配、添加、删除和前序遍历。
// 关键注意事项：`#` 和 `+` 通配符遵循 MQTT 主题过滤语义，系统主题由独立 trie 管理。
// 重构建议：可补充更多 MQTT 边界用例，并把节点清理逻辑扩展为递归回收空父节点。
package trie

import (
	"strings"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/retained"
)

// topicTrie
type topicTrie = topicNode

// children
type children = map[string]*topicNode

// topicNode
type topicNode struct {
	children  children
	msg       *gmqtt.Message
	parent    *topicNode // pointer of parent node
	topicName string
}

// newTopicTrie create a new trie tree
func newTopicTrie() *topicTrie {
	return newNode()
}

// newNode create a new trie node
func newNode() *topicNode {
	return &topicNode{
		children: children{},
	}
}

// newChild create a child node of t
func (t *topicNode) newChild() *topicNode {
	return &topicNode{
		children: children{},
		parent:   t,
	}
}

// find walk through the tire and return the node that represent the topicName
// return nil if not found
func (t *topicTrie) find(topicName string) *topicNode {
	topicSlice := strings.Split(topicName, "/")
	var pNode = t
	for _, lv := range topicSlice {
		if _, ok := pNode.children[lv]; ok {
			pNode = pNode.children[lv]
		} else {
			return nil
		}
	}
	if pNode.msg != nil {
		return pNode
	}
	return nil
}

// matchTopic walk through the tire and call the fn callback for each message witch match the topic filter.
func (t *topicTrie) matchTopic(topicSlice []string, fn retained.IterateFn) {
	endFlag := len(topicSlice) == 1
	switch topicSlice[0] {
	case "#":
		t.preOrderTraverse(fn)
	case "+":
		// 当前层的所有子节点都可能匹配 +。
		for _, v := range t.children {
			if endFlag {
				if v.msg != nil {
					fn(v.msg)
				}
			} else {
				v.matchTopic(topicSlice[1:], fn)
			}
		}
	default:
		if n := t.children[topicSlice[0]]; n != nil {
			if endFlag {
				if n.msg != nil {
					fn(n.msg)
				}
			} else {
				n.matchTopic(topicSlice[1:], fn)
			}
		}
	}
}

func (t *topicTrie) getMatchedMessages(topicFilter string) []*gmqtt.Message {
	topicLv := strings.Split(topicFilter, "/")
	var rs []*gmqtt.Message
	t.matchTopic(topicLv, func(message *gmqtt.Message) bool {
		rs = append(rs, message.Copy())
		return true
	})
	return rs
}

func isSystemTopic(topicName string) bool {
	return len(topicName) >= 1 && topicName[0] == '$'
}

// addRetainMsg add a retain message
func (t *topicTrie) addRetainMsg(topicName string, message *gmqtt.Message) {
	topicSlice := strings.Split(topicName, "/")
	var pNode = t
	for _, lv := range topicSlice {
		if _, ok := pNode.children[lv]; !ok {
			pNode.children[lv] = pNode.newChild()
		}
		pNode = pNode.children[lv]
	}
	pNode.msg = message
	pNode.topicName = topicName
}

func (t *topicTrie) remove(topicName string) {
	topicSlice := strings.Split(topicName, "/")
	l := len(topicSlice)
	var pNode = t
	for _, lv := range topicSlice {
		if _, ok := pNode.children[lv]; ok {
			pNode = pNode.children[lv]
		} else {
			return
		}
	}
	pNode.msg = nil
	if len(pNode.children) == 0 {
		delete(pNode.parent.children, topicSlice[l-1])
	}
}

func (t *topicTrie) preOrderTraverse(fn retained.IterateFn) bool {
	if t == nil {
		return false
	}
	if t.msg != nil {
		if !fn(t.msg) {
			return false
		}
	}
	for _, c := range t.children {
		if !c.preOrderTraverse(fn) {
			return false
		}
	}
	return true
}
