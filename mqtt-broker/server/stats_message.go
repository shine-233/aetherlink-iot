package server

import (
	"sync/atomic"

	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

func (s *statsManager) messageDropped(qos uint8, clientID string, err error) {
	switch qos {
	case packets.Qos0:
		s.totalStats.MessageStats.Qos0.DroppedTotal.messageDropped(err)
		s.updateClientStats(clientID, func(stats *ClientStats) {
			stats.MessageStats.Qos0.DroppedTotal.messageDropped(err)
		})
	case packets.Qos1:
		s.totalStats.MessageStats.Qos1.DroppedTotal.messageDropped(err)
		s.updateClientStats(clientID, func(stats *ClientStats) {
			stats.MessageStats.Qos1.DroppedTotal.messageDropped(err)
		})
	case packets.Qos2:
		s.totalStats.MessageStats.Qos2.DroppedTotal.messageDropped(err)
		s.updateClientStats(clientID, func(stats *ClientStats) {
			stats.MessageStats.Qos2.DroppedTotal.messageDropped(err)
		})
	}
}

func (d *DroppedTotal) messageDropped(err error) {
	switch err {
	case queue.ErrDropExceedsMaxPacketSize:
		atomic.AddUint64(&d.ExceedsMaxPacketSize, 1)
	case queue.ErrDropQueueFull:
		atomic.AddUint64(&d.QueueFull, 1)
	case queue.ErrDropExpired:
		atomic.AddUint64(&d.Expired, 1)
	case queue.ErrDropExpiredInflight:
		atomic.AddUint64(&d.InflightExpired, 1)
	default:
		atomic.AddUint64(&d.Internal, 1)
	}
}

func (s *statsManager) messageReceived(qos uint8, clientID string) {
	switch qos {
	case packets.Qos0:
		atomic.AddUint64(&s.totalStats.MessageStats.Qos0.ReceivedTotal, 1)
		s.updateClientStats(clientID, func(stats *ClientStats) {
			atomic.AddUint64(&stats.MessageStats.Qos0.ReceivedTotal, 1)
		})
	case packets.Qos1:
		atomic.AddUint64(&s.totalStats.MessageStats.Qos1.ReceivedTotal, 1)
		s.updateClientStats(clientID, func(stats *ClientStats) {
			atomic.AddUint64(&stats.MessageStats.Qos1.ReceivedTotal, 1)
		})
	case packets.Qos2:
		atomic.AddUint64(&s.totalStats.MessageStats.Qos2.ReceivedTotal, 1)
		s.updateClientStats(clientID, func(stats *ClientStats) {
			atomic.AddUint64(&stats.MessageStats.Qos2.ReceivedTotal, 1)
		})
	}
}

func (s *statsManager) messageSent(qos uint8, clientID string) {
	switch qos {
	case packets.Qos0:
		atomic.AddUint64(&s.totalStats.MessageStats.Qos0.SentTotal, 1)
		s.updateClientStats(clientID, func(stats *ClientStats) {
			atomic.AddUint64(&stats.MessageStats.Qos0.SentTotal, 1)
		})
	case packets.Qos1:
		atomic.AddUint64(&s.totalStats.MessageStats.Qos1.SentTotal, 1)
		s.updateClientStats(clientID, func(stats *ClientStats) {
			atomic.AddUint64(&stats.MessageStats.Qos1.SentTotal, 1)
		})
	case packets.Qos2:
		atomic.AddUint64(&s.totalStats.MessageStats.Qos2.SentTotal, 1)
		s.updateClientStats(clientID, func(stats *ClientStats) {
			atomic.AddUint64(&stats.MessageStats.Qos2.SentTotal, 1)
		})
	}
}

func (s *statsManager) addInflight(clientID string, delta uint64) {
	s.updateClientStats(clientID, func(stats *ClientStats) {
		atomic.AddUint64(&stats.MessageStats.InflightCurrent, delta)
	})
	atomic.AddUint64(&s.totalStats.MessageStats.InflightCurrent, 1)
}

func (s *statsManager) decInflight(clientID string, delta uint64) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	sts := s.getOrCreateClientStatsLocked(clientID)
	// Avoid the counter to be negative.
	// This could happen if the broker is start with persistence data loaded and send messages from the persistent queue.
	// Because the statistic data is not persistent, the init value is always 0.
	if atomic.LoadUint64(&sts.MessageStats.InflightCurrent) == 0 {
		return
	}
	atomic.AddUint64(&sts.MessageStats.InflightCurrent, ^uint64(delta-1))
	atomic.AddUint64(&s.totalStats.MessageStats.InflightCurrent, ^uint64(delta-1))
}

func (s *statsManager) addQueueLen(clientID string, delta uint64) {
	s.updateClientStats(clientID, func(stats *ClientStats) {
		atomic.AddUint64(&stats.MessageStats.QueuedCurrent, delta)
	})
	atomic.AddUint64(&s.totalStats.MessageStats.QueuedCurrent, delta)
}

func (s *statsManager) decQueueLen(clientID string, delta uint64) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	sts := s.getOrCreateClientStatsLocked(clientID)
	// Avoid the counter to be negative.
	// This could happen if the broker is start with persistence data loaded and send messages from the persistent queue.
	// Because the statistic data is not persistent, the init value is always 0.
	if atomic.LoadUint64(&sts.MessageStats.QueuedCurrent) == 0 {
		return
	}
	atomic.AddUint64(&sts.MessageStats.QueuedCurrent, ^uint64(delta-1))
	atomic.AddUint64(&s.totalStats.MessageStats.QueuedCurrent, ^uint64(delta-1))
}
