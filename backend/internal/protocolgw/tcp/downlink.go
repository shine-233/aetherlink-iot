// 文件用途：TCP 下行命令断网缓冲（TP-03）——设备离线时命令入环形缓冲，重连注册后按 FIFO 续传。
// 核心逻辑：与 internal/edgeforward 同款模式：满则丢最旧并计数；在线直投、失败转缓冲；
// 注册冲刷（flush）逐条投递，写失败立即停止并把剩余按原序塞回队首（保留顺序语义）。
// 关键注意事项：缓冲按设备号隔离（一个坏连接不拖垮其他设备）；requeue 溢出时丢"最新"保"最旧"，
// 与 enqueue 的丢最旧方向相反——续传顺序优先级高于新命令。所有环形缓冲操作统一在
// downlinkBufferer.mu 下进行（含建/删 ring），杜绝"命令写进已回收 ring"的孤儿丢失竞态；
// 全内存态，进程重启缓冲即失（与 edgeforward 断网缓冲同口径，持久化属后续迭代）。
// 重构建议：需要跨重启续传时，把 commandRing 换成 spool 文件-backed 队列（参照 telemetry spool）。
package tcp

import (
	"errors"
	"sync"
)

// ErrUnknownDevice 目标设备不是本网关的 TCP 设备（从未注册过会话）——
// 调用方（下行发布回退层）据此回退 MQTT 通道，绝不静默吞掉。
var ErrUnknownDevice = errors.New("tcp: device has no tcp session history")

// commandRing 单设备下行命令环形缓冲（FIFO；满则丢最旧）。
// 非并发安全：全部操作由 downlinkBufferer.mu 串行化（见文件头）。
type commandRing struct {
	limit   int
	items   [][]byte
	dropped int // 满队丢最旧计数
}

func newCommandRing(limit int) *commandRing {
	if limit <= 0 {
		limit = DefaultCommandBufferLimit
	}
	return &commandRing{limit: limit}
}

// enqueueLocked 入队（拷贝 payload 防调用方复用底层数组）；满则丢最旧并计数。
func (r *commandRing) enqueueLocked(payload []byte) {
	cp := make([]byte, len(payload))
	copy(cp, payload)
	if len(r.items) >= r.limit {
		r.items = r.items[1:]
		r.dropped++
	}
	r.items = append(r.items, cp)
}

// requeueFrontLocked 把续传失败剩余的命令按原序塞回队首（保序重试）；溢出丢尾部（最新）。
func (r *commandRing) requeueFrontLocked(items [][]byte) {
	if len(items) == 0 {
		return
	}
	merged := make([][]byte, 0, len(items)+len(r.items))
	merged = append(merged, items...)
	merged = append(merged, r.items...)
	if len(merged) > r.limit {
		r.dropped += len(merged) - r.limit
		merged = merged[:r.limit]
	}
	r.items = merged
}

// drainLocked 取走全部缓冲命令（FIFO；调用后缓冲为空）。
func (r *commandRing) drainLocked() [][]byte {
	items := r.items
	r.items = nil
	return items
}

// downlinkBufferer 下行命令缓冲面：device_number → 环形缓冲。
// 单一互斥锁覆盖 ring 的建/删/读写，保证缓冲操作与回收不会互相孤儿化。
type downlinkBufferer struct {
	mu    sync.Mutex
	limit int
	rings map[string]*commandRing
}

// newDownlinkBufferer 构造；limit 为单设备缓冲上限（<=0 取默认）。
func newDownlinkBufferer(limit int) *downlinkBufferer {
	if limit <= 0 {
		limit = DefaultCommandBufferLimit
	}
	return &downlinkBufferer{limit: limit, rings: map[string]*commandRing{}}
}

// ringForLocked 取（惰性建）指定设备的环形缓冲（调用方持有 b.mu）。
func (b *downlinkBufferer) ringForLocked(number string) *commandRing {
	r, ok := b.rings[number]
	if !ok {
		r = newCommandRing(b.limit)
		b.rings[number] = r
	}
	return r
}

// enqueue 离线命令入指定设备缓冲。
func (b *downlinkBufferer) enqueue(number string, payload []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ringForLocked(number).enqueueLocked(payload)
}

// drain 取走指定设备全部缓冲命令（无缓冲返回 nil）。
func (b *downlinkBufferer) drain(number string) [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.rings[number]
	if !ok {
		return nil
	}
	return r.drainLocked()
}

// requeueFront 把续传失败剩余的命令按原序塞回指定设备队首。
func (b *downlinkBufferer) requeueFront(number string, items [][]byte) {
	if len(items) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ringForLocked(number).requeueFrontLocked(items)
}

// removeIfEmpty 设备缓冲已空时回收条目（防 map 无界增长）；非空则保留。
func (b *downlinkBufferer) removeIfEmpty(number string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if r, ok := b.rings[number]; ok && len(r.items) == 0 {
		delete(b.rings, number)
	}
}

// totals 汇总：当前缓冲命令总条数与累计溢出丢弃数（诊断面）。
func (b *downlinkBufferer) totals() (pending, dropped int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range b.rings {
		pending += len(r.items)
		dropped += r.dropped
	}
	return pending, dropped
}
