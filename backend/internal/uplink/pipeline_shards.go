// pipeline_shards.go runs one uplink kind on a device-hash-sharded worker set.
//
// A single reader goroutine drains the bus channel and dispatches every message
// to shard = fnv32a(device_id) % N. Each shard owns a bounded queue and one
// worker, so messages of the same device are processed strictly in arrival
// order while different devices proceed in parallel. Gateway fan-out stays on
// the gateway's shard (sub-device data arrives inside the gateway message), so
// sub-device order is preserved as well.
//
// Shutdown contract (unchanged from the single-goroutine loop):
//   - channel closed: the reader closes every shard queue, workers drain what is
//     already queued, and Done closes once all workers have exited;
//   - Stop (context cancel): reader and workers exit promptly; queued messages
//     are abandoned exactly as the legacy loop abandoned the bus backlog.
package uplink

import (
	"hash/fnv"
	"runtime"
	"sync"
)

const (
	// maxUplinkShards caps the default so very wide hosts do not spawn idle workers.
	maxUplinkShards = 32
	// defaultShardQueueSize is the per-shard buffer. Back-pressure still reaches
	// the bus: when a shard queue is full the reader blocks, and the bus channel
	// fills up behind it exactly as before.
	defaultShardQueueSize = 256
)

// defaultUplinkShards is the shard count used when a kind does not configure one.
func defaultUplinkShards() int {
	n := runtime.GOMAXPROCS(0)
	if n > maxUplinkShards {
		n = maxUplinkShards
	}
	if n < 1 {
		n = 1
	}
	return n
}

// SetShards configures the number of per-device worker shards. Must be called
// before Start. n <= 0 selects the default (GOMAXPROCS, capped); n == 1 keeps
// the legacy single-consumer behavior.
func (b *uplinkBase) SetShards(n int) {
	if n <= 0 {
		n = defaultUplinkShards()
	}
	b.shards = n
}

// Shards reports the configured shard count (after defaulting).
func (b *uplinkBase) Shards() int {
	if b.shards <= 0 {
		return 1
	}
	return b.shards
}

// shardKey is the routing key for msg: the bus-level DeviceID, falling back to
// the device_id metadata that resolveDevice reads.
func shardKey(msg *DeviceMessage) string {
	if msg == nil {
		return ""
	}
	if msg.DeviceID != "" {
		return msg.DeviceID
	}
	if v, ok := msg.GetMetadata("device_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func shardIndex(key string, n int) int {
	if n <= 1 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32() % uint32(n))
}

// runSharded is the N>1 consume loop. Done closes after the reader and all
// workers have returned.
func (b *uplinkBase) runSharded(messageChan <-chan *DeviceMessage, process func(*DeviceMessage), n int) {
	queueSize := b.shardQueueSize
	if queueSize <= 0 {
		queueSize = defaultShardQueueSize
	}
	queues := make([]chan *DeviceMessage, n)
	var workers sync.WaitGroup
	for i := range queues {
		queues[i] = make(chan *DeviceMessage, queueSize)
		workers.Add(1)
		go func(q <-chan *DeviceMessage) {
			defer workers.Done()
			for {
				select {
				case msg, ok := <-q:
					if !ok {
						return
					}
					process(msg)
				case <-b.ctx.Done():
					return
				}
			}
		}(queues[i])
	}

	go func() {
		defer close(b.done)
		defer workers.Wait()
		defer func() {
			for _, q := range queues {
				close(q)
			}
		}()
		for {
			select {
			case msg, ok := <-messageChan:
				if !ok {
					b.logger.Info(b.spec.name + " message channel closed")
					return
				}
				select {
				case queues[shardIndex(shardKey(msg), n)] <- msg:
				case <-b.ctx.Done():
					b.logger.Info(b.spec.name + " stopped")
					return
				}
			case <-b.ctx.Done():
				b.logger.Info(b.spec.name + " stopped")
				return
			}
		}
	}()
}
