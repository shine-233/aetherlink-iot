// 文件用途：实现客户端 packet id 限流器，约束 inflight 消息数量不超过 receive maximum。
// 核心逻辑：用条件变量和 bitmap 分配、锁定、释放 packet id，并在关闭时唤醒等待方。
// 使用注意：该逻辑直接影响 QoS/inflight 行为，修改时要保持 packet id 分配和释放顺序稳定。
// 重构建议：后续可补充边界说明和故障场景用例，例如 limiter close、耗尽和重复释放。

package server

import (
	"sync"

	"github.com/DrmagicE/gmqtt/pkg/bitmap"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

func newPacketIDLimiter(limit uint16) *packetIDLimiter {
	return &packetIDLimiter{
		cond:      sync.NewCond(&sync.Mutex{}),
		used:      0,
		limit:     limit,
		exit:      false,
		freePid:   1,
		lockedPid: bitmap.New(packets.MaxPacketID),
	}
}

// packetIDLimiter limit the generation of packet id to keep the number of inflight messages
// always less or equal than receive maximum setting of the client.
type packetIDLimiter struct {
	cond      *sync.Cond
	used      uint16
	limit     uint16
	exit      bool
	lockedPid *bitmap.Bitmap   // packet id in-use
	freePid   packets.PacketID // next available id
}

func (p *packetIDLimiter) close() {
	p.cond.L.Lock()
	p.exit = true
	p.cond.L.Unlock()
	p.cond.Signal()
}

// pollPacketIDs returns at most max number of unused packetID and marks them as used for a client.
// If there is no available id, the call will be blocked until at least one packet id is available or the limiter has been closed.
// return 0 means the limiter is closed.
// the return number = min(max, i.used).
func (p *packetIDLimiter) pollPacketIDs(max uint16) (id []packets.PacketID) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	for p.used >= p.limit && !p.exit {
		p.cond.Wait()
	}
	if p.exit {
		return nil
	}
	n := max
	if remain := p.limit - p.used; remain < max {
		n = remain
	}
	for j := uint16(0); j < n; j++ {
		for p.lockedPid.Get(p.freePid) == 1 {
			if p.freePid == packets.MaxPacketID {
				p.freePid = packets.MinPacketID
			} else {
				p.freePid++
			}
		}
		id = append(id, p.freePid)
		p.used++
		p.lockedPid.Set(p.freePid, 1)
		if p.freePid == packets.MaxPacketID {
			p.freePid = packets.MinPacketID
		} else {
			p.freePid++
		}
	}
	return id
}

// release marks the given id list as unused
func (p *packetIDLimiter) release(id packets.PacketID) {
	p.cond.L.Lock()
	p.releaseLocked(id)
	p.cond.L.Unlock()
	p.cond.Signal()

}
func (p *packetIDLimiter) releaseLocked(id packets.PacketID) {
	if p.lockedPid.Get(id) == 1 {
		p.lockedPid.Set(id, 0)
		p.used--
	}
}

func (p *packetIDLimiter) batchRelease(id []packets.PacketID) {
	p.cond.L.Lock()
	for _, v := range id {
		p.releaseLocked(v)
	}
	p.cond.L.Unlock()
	p.cond.Signal()

}

// markInUsed marks the given id as used.
func (p *packetIDLimiter) markUsedLocked(id packets.PacketID) {
	p.used++
	p.lockedPid.Set(id, 1)
}

func (p *packetIDLimiter) lock() {
	p.cond.L.Lock()
}
func (p *packetIDLimiter) unlock() {
	p.cond.L.Unlock()
}
func (p *packetIDLimiter) unlockAndSignal() {
	p.cond.L.Unlock()
	p.cond.Signal()
}
