// 文件用途：deliverMessage 投递扇出微基准（不走网络），隔离订阅匹配 + 投递选择 + 入队构造的开销。
// 方法学：内存订阅树 + 丢弃型 queue.Store；覆盖 overlap 扇出 1/100、onlyOnce 多重命中、
// 共享订阅（random / topicHash）。每 op 一条消息，统计 ns/op、B/op、allocs/op。
package server

import (
	"hash/fnv"
	"strconv"
	"testing"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/persistence/subscription/mem"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

// discardQueue 只计数的 queue.Store，用于把入队成本排除在被测路径之外。
type discardQueue struct{ n int }

func (q *discardQueue) Close() error                                   { return nil }
func (q *discardQueue) Init(*queue.InitOptions) error                  { return nil }
func (q *discardQueue) Clean() error                                   { return nil }
func (q *discardQueue) Add(*queue.Elem) error                          { q.n++; return nil }
func (q *discardQueue) Replace(*queue.Elem) (bool, error)              { return false, nil }
func (q *discardQueue) Read([]packets.PacketID) ([]*queue.Elem, error) { return nil, nil }
func (q *discardQueue) ReadInflight(uint) ([]*queue.Elem, error)       { return nil, nil }
func (q *discardQueue) Remove(packets.PacketID) error                  { return nil }

func newDeliverBenchServer(mode, strategy string) *server {
	sub := mem.NewStore()
	cfg := config.DefaultConfig()
	cfg.MQTT.DeliveryMode = mode
	cfg.MQTT.SharedSubBalanceStrategy = strategy
	cfg.MQTT.QueueQos0Msg = true
	return &server{
		subscriptionsDB: sub,
		queueStore:      make(map[string]queue.Store),
		clients:         make(map[string]*client),
		config:          cfg,
		statsManager:    newStatsManager(sub),
	}
}

func (srv *server) benchSubscribe(cid string, subs ...*gmqtt.Subscription) {
	srv.queueStore[cid] = &discardQueue{}
	_, _ = srv.subscriptionsDB.Subscribe(cid, subs...)
}

func runDeliverBench(b *testing.B, srv *server, topic string) {
	msg := &gmqtt.Message{Topic: topic, QoS: 1, Payload: make([]byte, 64)}
	opts := defaultIterateOptions(topic)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !srv.deliverMessage("pub", msg, opts) {
			b.Fatal("no match")
		}
	}
}

func BenchmarkDeliverOverlap(b *testing.B) {
	for _, fanout := range []int{1, 100} {
		b.Run("fanout="+strconv.Itoa(fanout), func(b *testing.B) {
			srv := newDeliverBenchServer(Overlap, SharedSubBalanceRandom)
			for i := 0; i < fanout; i++ {
				srv.benchSubscribe("sub-"+strconv.Itoa(i), &gmqtt.Subscription{TopicFilter: "devices/+/telemetry", QoS: 1, ID: 7})
			}
			runDeliverBench(b, srv, "devices/d1/telemetry")
		})
	}
}

func BenchmarkDeliverOnlyOnceMultiMatch(b *testing.B) {
	srv := newDeliverBenchServer(OnlyOnce, SharedSubBalanceRandom)
	for i := 0; i < 10; i++ {
		srv.benchSubscribe("sub-"+strconv.Itoa(i),
			&gmqtt.Subscription{TopicFilter: "devices/+/telemetry", QoS: 1, ID: 1},
			&gmqtt.Subscription{TopicFilter: "devices/#", QoS: 0, ID: 2},
		)
	}
	runDeliverBench(b, srv, "devices/d1/telemetry")
}

func BenchmarkDeliverShared(b *testing.B) {
	for _, strategy := range []string{SharedSubBalanceRandom, SharedSubBalanceTopicHash} {
		b.Run(strategy, func(b *testing.B) {
			srv := newDeliverBenchServer(Overlap, strategy)
			for i := 0; i < 4; i++ {
				srv.benchSubscribe("worker-"+strconv.Itoa(i), &gmqtt.Subscription{ShareName: "g", TopicFilter: "devices/#", QoS: 1})
			}
			runDeliverBench(b, srv, "devices/d1/telemetry")
		})
	}
}

// TestFnv32aMatchesHashFnv 保证内联 FNV-1a 与标准库结果一致（topicHash 共享订阅分配不得因优化漂移）。
func TestFnv32aMatchesHashFnv(t *testing.T) {
	for _, s := range []string{"", "/abc", "devices/d1/telemetry", "中文/主题/#"} {
		h := fnv.New32a()
		_, _ = h.Write([]byte(s))
		if got, want := fnv32a(s), h.Sum32(); got != want {
			t.Fatalf("fnv32a(%q)=%d want %d", s, got, want)
		}
	}
}
