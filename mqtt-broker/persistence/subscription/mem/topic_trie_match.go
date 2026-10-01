// 文件用途：订阅树的零分配匹配遍历（每条 PUBLISH 投递的热路径）。
// 核心逻辑：按 '/' 逐层切片主题（子串，不分配），沿 "#"、"+"、精确层级三路递归，
// 命中节点时直接回调 IterateFn，不再先构造 ClientSubscriptions 中间 map 与按客户端的切片。
// 关键注意事项：遍历顺序/命中集合与旧实现 matchTopic+setRs 完全一致（含同一订阅可被
// 多条路径命中的情形）；仅回调的先后顺序不同——旧实现本就依赖 map 随机序，调用方不得依赖顺序。
// 调用方必须持有 TrieDB 读锁，回调内不得修改订阅树。
package mem

import (
	"strings"

	"github.com/DrmagicE/gmqtt/persistence/subscription"
)

// matchEmitter 承载一次匹配遍历的回调与可选 clientID 过滤；值类型放在调用方栈上。
type matchEmitter struct {
	fn       subscription.IterateFn
	clientID string // 非空时只回调该客户端的订阅
}

// emit 把节点上的普通订阅与共享订阅交给回调；返回 false 表示回调要求中止遍历。
func (e *matchEmitter) emit(node *topicNode) bool {
	if e.clientID != "" {
		if sub, ok := node.clients[e.clientID]; ok {
			if !e.fn(e.clientID, sub) {
				return false
			}
		}
		for _, c := range node.shared {
			if sub, ok := c[e.clientID]; ok {
				if !e.fn(e.clientID, sub) {
					return false
				}
			}
		}
		return true
	}
	for cid, sub := range node.clients {
		if !e.fn(cid, sub) {
			return false
		}
	}
	for _, c := range node.shared {
		for cid, sub := range c {
			if !e.fn(cid, sub) {
				return false
			}
		}
	}
	return true
}

// matchWalk 对剩余主题 topic 在当前节点下做匹配，等价于旧的 matchTopic(strings.Split(topic,"/"))。
func (t *topicNode) matchWalk(topic string, e *matchEmitter) bool {
	level, rest := topic, ""
	last := true
	if pos := strings.IndexByte(topic, '/'); pos >= 0 {
		level, rest, last = topic[:pos], topic[pos+1:], false
	}
	if c := t.children["#"]; c != nil {
		if !e.emit(c) {
			return false
		}
	}
	if c := t.children["+"]; c != nil {
		if !c.matchStep(last, rest, e) {
			return false
		}
	}
	if c := t.children[level]; c != nil {
		if !c.matchStep(last, rest, e) {
			return false
		}
	}
	return true
}

// matchStep 处理已命中当前层的子节点：末层则收集自身与其 "#" 子节点，否则继续下钻。
func (t *topicNode) matchStep(last bool, rest string, e *matchEmitter) bool {
	if !last {
		return t.matchWalk(rest, e)
	}
	if !e.emit(t) {
		return false
	}
	if n := t.children["#"]; n != nil {
		return e.emit(n)
	}
	return true
}

// walkLevels 按 '/' 逐层迭代主题过滤器的各层（子串），fn 返回 false 时提前结束。
// 与 strings.Split 的切分语义一致（空串、首尾 '/' 均产生空层），但不分配切片。
func walkLevels(topic string, fn func(level string) bool) {
	for {
		pos := strings.IndexByte(topic, '/')
		if pos < 0 {
			fn(topic)
			return
		}
		if !fn(topic[:pos]) {
			return
		}
		topic = topic[pos+1:]
	}
}

// lastLevel 返回主题过滤器的最后一层，等价于 strings.Split 结果的最后一个元素。
func lastLevel(topic string) string {
	return topic[strings.LastIndexByte(topic, '/')+1:]
}
