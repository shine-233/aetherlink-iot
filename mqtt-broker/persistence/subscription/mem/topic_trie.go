// 文件用途：内存订阅树（TrieDB 的底层结构）的节点定义与订阅/退订/遍历。
// 核心逻辑：按 '/' 分层建树；"+"、"#" 子节点除登记在 children 外，另以 plus/hash 指针直连，
// 让每条 PUBLISH 的匹配遍历每层少两次 map 查找；clients/shared 惰性分配，节点只付实际用到的内存。
// 关键注意事项：退订后自叶向根剪除空节点（旧实现只删叶子，设备 ID 频繁变化时中间节点永久泄漏）；
// plus/hash 必须与 children 同步，只能经 addChild/removeChild 修改 children。
package mem

import (
	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
)

// topicTrie 根节点类型别名。
type topicTrie = topicNode

type clientOpts map[string]*gmqtt.Subscription

// topicNode 订阅树节点。
type topicNode struct {
	children map[string]*topicNode // 含 "+"、"#"，供 find/遍历使用
	plus     *topicNode            // == children["+"]，匹配热路径直连
	hash     *topicNode            // == children["#"]，匹配热路径直连
	// clients 普通订阅，惰性分配
	clients clientOpts
	// shared 共享订阅，key 为 ShareName，惰性分配
	shared    map[string]clientOpts
	parent    *topicNode
	level     string // 本节点在 parent.children 中的键
	topicName string // 节点上存在订阅时为其主题过滤器
}

func newTopicTrie() *topicTrie { return &topicNode{} }

// child 返回层级 lv 对应的子节点；"+"/"#" 走直连指针。
func (t *topicNode) child(lv string) *topicNode {
	switch lv {
	case "+":
		return t.plus
	case "#":
		return t.hash
	}
	return t.children[lv]
}

// addChild 取得或创建层级 lv 的子节点。
func (t *topicNode) addChild(lv string) *topicNode {
	if c := t.child(lv); c != nil {
		return c
	}
	c := &topicNode{parent: t, level: lv}
	if t.children == nil {
		t.children = make(map[string]*topicNode, 1)
	}
	t.children[lv] = c
	switch lv {
	case "+":
		t.plus = c
	case "#":
		t.hash = c
	}
	return c
}

// removeChild 从本节点摘除子节点 c，并同步直连指针。
func (t *topicNode) removeChild(c *topicNode) {
	delete(t.children, c.level)
	switch c {
	case t.plus:
		t.plus = nil
	case t.hash:
		t.hash = nil
	}
}

// empty 节点既无订阅也无子节点时可被剪除。
func (t *topicNode) empty() bool {
	return len(t.clients) == 0 && len(t.shared) == 0 && len(t.children) == 0
}

// prune 自 t 向根剪除空节点；根节点（parent == nil）保留。
func (t *topicNode) prune() {
	for n := t; n.parent != nil; n = n.parent {
		if len(n.clients) == 0 && len(n.shared) == 0 {
			n.topicName = ""
		}
		if !n.empty() {
			return
		}
		n.parent.removeChild(n)
	}
}

// subscribe 添加订阅并返回所在节点；逐层切子串，不分配层级切片。
func (t *topicTrie) subscribe(clientID string, s *gmqtt.Subscription) *topicNode {
	pNode := t
	walkLevels(s.TopicFilter, func(lv string) bool {
		pNode = pNode.addChild(lv)
		return true
	})
	if s.ShareName != "" {
		if pNode.shared == nil {
			pNode.shared = make(map[string]clientOpts, 1)
		}
		group := pNode.shared[s.ShareName]
		if group == nil {
			group = make(clientOpts, 1)
			pNode.shared[s.ShareName] = group
		}
		group[clientID] = s
	} else {
		if pNode.clients == nil {
			pNode.clients = make(clientOpts, 1)
		}
		pNode.clients[clientID] = s
	}
	pNode.topicName = s.TopicFilter
	return pNode
}

// walk 返回主题过滤器对应的节点（不校验 topicName），不存在返回 nil。
func (t *topicTrie) walk(topicFilter string) *topicNode {
	pNode := t
	walkLevels(topicFilter, func(lv string) bool {
		pNode = pNode.child(lv)
		return pNode != nil
	})
	return pNode
}

// find 返回代表 topicFilter 的订阅节点，不存在返回 nil。
func (t *topicTrie) find(topicFilter string) *topicNode {
	if n := t.walk(topicFilter); n != nil && n.topicName == topicFilter {
		return n
	}
	return nil
}

// unsubscribe 删除 clientID 在 topicName（及可选 shareName）上的订阅并剪除空节点。
func (t *topicTrie) unsubscribe(clientID string, topicName string, shareName string) {
	pNode := t.walk(topicName)
	if pNode == nil || pNode == t {
		return
	}
	if shareName != "" {
		pNode.removeShared(shareName, clientID)
	} else {
		delete(pNode.clients, clientID)
	}
	pNode.prune()
}

// removeShared 从共享组 shareName 中删除 clientID，组空则删组。
func (t *topicNode) removeShared(shareName, clientID string) {
	if c := t.shared[shareName]; c != nil {
		delete(c, clientID)
		if len(c) == 0 {
			delete(t.shared, shareName)
		}
	}
}

// removeClient 删除 clientID 在本节点上的全部订阅（普通 + 所有共享组）并剪除空节点。
func (t *topicNode) removeClient(clientID string) {
	delete(t.clients, clientID)
	for shareName := range t.shared {
		t.removeShared(shareName, clientID)
	}
	t.prune()
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
