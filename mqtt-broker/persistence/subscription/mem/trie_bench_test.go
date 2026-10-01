// 文件用途：订阅树发布匹配热路径基准（每条 PUBLISH 都会以 MatchFilter 方式 Iterate 一次）。
// 方法学：构造 N 个设备客户端（精确订阅 + 单层通配 + 多层通配混合）与少量共享订阅，
// 对典型设备上行主题执行 Iterate(TypeAll, MatchFilter)，统计 ns/op、B/op、allocs/op。
package mem

import (
	"fmt"
	"testing"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
)

func newBenchTrieDB(clients int) *TrieDB {
	db := NewStore()
	for i := 0; i < clients; i++ {
		cid := fmt.Sprintf("dev-%d", i)
		db.Subscribe(cid,
			&gmqtt.Subscription{TopicFilter: fmt.Sprintf("devices/telemetry/control/%d", i), QoS: 1},
			&gmqtt.Subscription{TopicFilter: fmt.Sprintf("devices/attributes/set/%d/+", i), QoS: 1},
		)
	}
	// 后端服务：通配订阅 + 共享订阅组
	db.Subscribe("backend-1",
		&gmqtt.Subscription{TopicFilter: "devices/telemetry/#", QoS: 1},
		&gmqtt.Subscription{TopicFilter: "devices/+/control/+", QoS: 0},
		&gmqtt.Subscription{ShareName: "g1", TopicFilter: "devices/telemetry/#", QoS: 1},
	)
	db.Subscribe("backend-2",
		&gmqtt.Subscription{ShareName: "g1", TopicFilter: "devices/telemetry/#", QoS: 1},
	)
	db.Subscribe("sys-monitor", &gmqtt.Subscription{TopicFilter: "$SYS/#", QoS: 0})
	return db
}

func BenchmarkTrieIterateMatchFilter(b *testing.B) {
	for _, n := range []int{10, 1000} {
		db := newBenchTrieDB(n)
		topic := fmt.Sprintf("devices/telemetry/control/%d", n/2)
		opts := subscription.IterationOptions{Type: subscription.TypeAll, TopicName: topic, MatchType: subscription.MatchFilter}
		b.Run(fmt.Sprintf("clients=%d", n), func(b *testing.B) {
			var hits int
			fn := func(clientID string, sub *gmqtt.Subscription) bool { hits++; return true }
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				db.Iterate(fn, opts)
			}
			if hits == 0 {
				b.Fatal("no matches")
			}
		})
	}
}

func BenchmarkTrieIterateMatchFilterNoMatch(b *testing.B) {
	db := newBenchTrieDB(1000)
	opts := subscription.IterationOptions{Type: subscription.TypeAll, TopicName: "other/room/1/temp", MatchType: subscription.MatchFilter}
	fn := func(clientID string, sub *gmqtt.Subscription) bool { return true }
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Iterate(fn, opts)
	}
}

func BenchmarkTrieSubscribeUnsubscribe(b *testing.B) {
	db := newBenchTrieDB(1000)
	sub := &gmqtt.Subscription{TopicFilter: "devices/telemetry/control/bench/x", QoS: 1}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Subscribe("bench-client", sub)
		db.Unsubscribe("bench-client", sub.TopicFilter)
	}
}
