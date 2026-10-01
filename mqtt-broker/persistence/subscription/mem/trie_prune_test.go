// 文件用途：订阅树退订剪枝与共享订阅清理的回归测试。
// 覆盖：退订后中间空节点被回收、plus/hash 直连指针与 children 同步、
// UnsubscribeAll 清除客户端在所有共享组中的订阅（旧实现残留，共享组会选中已断开客户端）。
package mem

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
)

func TestTopicTrie_unsubscribePrunesEmptyAncestors(t *testing.T) {
	a := assert.New(t)
	trie := newTopicTrie()
	trie.subscribe("c1", &gmqtt.Subscription{TopicFilter: "a/b/c/d"})
	trie.subscribe("c2", &gmqtt.Subscription{TopicFilter: "a/+/#"})
	trie.unsubscribe("c1", "a/b/c/d", "")
	a.Nil(trie.walk("a/b"), "空中间节点应被剪除")
	a.NotNil(trie.find("a/+/#"))
	trie.unsubscribe("c2", "a/+/#", "")
	a.Empty(trie.children, "全部退订后根节点应无子节点")
	a.Nil(trie.plus)
	a.Nil(trie.hash)
}

func TestTopicTrie_unsubscribeKeepsNodeWithChildren(t *testing.T) {
	a := assert.New(t)
	trie := newTopicTrie()
	trie.subscribe("c1", &gmqtt.Subscription{TopicFilter: "a/b"})
	trie.subscribe("c1", &gmqtt.Subscription{TopicFilter: "a/b/#"})
	trie.unsubscribe("c1", "a/b", "")
	a.Nil(trie.find("a/b"), "无订阅的中间节点不应再被 find 命中")
	n := trie.find("a/b/#")
	a.NotNil(n)
	a.Same(n, trie.walk("a/b").hash, "hash 直连指针应指向 children[\"#\"]")
	subs := trie.getMatchedTopicFilter("a/b")
	a.Len(subs["c1"], 1, "a/b/# 仍应匹配父级 a/b")
}

func TestTrieDB_UnsubscribeAllRemovesSharedSubscriptions(t *testing.T) {
	a := assert.New(t)
	db := NewStore()
	_, _ = db.Subscribe("c1",
		&gmqtt.Subscription{ShareName: "g1", TopicFilter: "t/#"},
		&gmqtt.Subscription{ShareName: "g2", TopicFilter: "t/#"},
	)
	_, _ = db.Subscribe("c2", &gmqtt.Subscription{ShareName: "g1", TopicFilter: "t/#"})
	a.NoError(db.UnsubscribeAll("c1"))

	got := map[string]int{}
	db.Iterate(func(clientID string, sub *gmqtt.Subscription) bool {
		got[clientID]++
		return true
	}, subscription.IterationOptions{Type: subscription.TypeAll, TopicName: "t/x", MatchType: subscription.MatchFilter})
	a.Equal(map[string]int{"c2": 1}, got, "c1 的共享订阅（g1、g2）都应被清除")

	a.NoError(db.UnsubscribeAll("c2"))
	a.Empty(db.sharedTrie.children, "共享树应被完全剪空")
}
