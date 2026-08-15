// 文件用途：维护 broker 全局与客户端级统计，包括连接、会话、消息、packet 和队列指标。
// 核心逻辑：在收发包、连接断开、订阅变化、消息投递/丢弃时更新统计快照。
// 使用注意：统计值会被管理接口和插件读取，修改字段含义时要同步文档和兼容说明。
// 重构建议：后续可把 packet/message/session 统计拆成小协作者，降低 statsManager 维护压力。

package server

import (
	"sync"
	"sync/atomic"

	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

type statsManager struct {
	subStatsReader subscription.StatsReader
	totalStats     *GlobalStats
	clientMu       sync.RWMutex
	clientStats    map[string]*ClientStats
}

func (s *statsManager) getClientStats(clientID string) (stats *ClientStats) {
	s.clientMu.RLock()
	stats = s.clientStats[clientID]
	s.clientMu.RUnlock()
	if stats != nil {
		return stats
	}

	subStats, _ := s.subStatsReader.GetClientStats(clientID)

	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	if stats = s.clientStats[clientID]; stats != nil {
		return stats
	}
	stats = &ClientStats{
		SubscriptionStats: subStats,
	}
	s.clientStats[clientID] = stats
	return stats
}

func (s *statsManager) getOrCreateClientStatsLocked(clientID string) (stats *ClientStats) {
	if stats = s.clientStats[clientID]; stats != nil {
		return stats
	}
	subStats, _ := s.subStatsReader.GetClientStats(clientID)
	stats = &ClientStats{
		SubscriptionStats: subStats,
	}
	s.clientStats[clientID] = stats
	return stats
}

func (s *statsManager) updateClientStats(clientID string, update func(*ClientStats)) {
	s.clientMu.RLock()
	stats := s.clientStats[clientID]
	if stats != nil {
		update(stats)
		s.clientMu.RUnlock()
		return
	}
	s.clientMu.RUnlock()

	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	update(s.getOrCreateClientStatsLocked(clientID))
}

func (s *statsManager) getExistingClientStats(clientID string) (*ClientStats, bool) {
	s.clientMu.RLock()
	defer s.clientMu.RUnlock()
	stats := s.clientStats[clientID]
	return stats, stats != nil
}

func (s *statsManager) packetReceived(packet packets.Packet, clientID string) {
	s.totalStats.PacketStats.add(packet, true)
	s.updateClientStats(clientID, func(stats *ClientStats) {
		stats.PacketStats.add(packet, true)
	})
}
func (s *statsManager) packetSent(packet packets.Packet, clientID string) {
	s.totalStats.PacketStats.add(packet, false)
	s.updateClientStats(clientID, func(stats *ClientStats) {
		stats.PacketStats.add(packet, false)
	})
}
func (s *statsManager) clientPacketReceived(packet packets.Packet, clientID string) {
	s.updateClientStats(clientID, func(stats *ClientStats) {
		stats.PacketStats.add(packet, true)
	})
}
func (s *statsManager) clientPacketSent(packet packets.Packet, clientID string) {
	s.updateClientStats(clientID, func(stats *ClientStats) {
		stats.PacketStats.add(packet, false)
	})
}

func (s *statsManager) clientConnected(clientID string) {
	atomic.AddUint64(&s.totalStats.ConnectionStats.ConnectedTotal, 1)
}

func (s *statsManager) clientDisconnected(clientID string) {
	atomic.AddUint64(&s.totalStats.ConnectionStats.DisconnectedTotal, 1)
	s.sessionInActive()
}

func (s *statsManager) sessionActive(create bool) {
	if create {
		atomic.AddUint64(&s.totalStats.ConnectionStats.SessionCreatedTotal, 1)
	} else {
		atomic.AddUint64(&s.totalStats.ConnectionStats.InactiveCurrent, ^uint64(0))
	}
	atomic.AddUint64(&s.totalStats.ConnectionStats.ActiveCurrent, 1)
}

func (s *statsManager) sessionInActive() {
	atomic.AddUint64(&s.totalStats.ConnectionStats.ActiveCurrent, ^uint64(0))
	atomic.AddUint64(&s.totalStats.ConnectionStats.InactiveCurrent, 1)
}

func (s *statsManager) sessionTerminated(clientID string, reason SessionTerminatedReason) {
	var i *uint64
	switch reason {
	case NormalTermination:
		i = &s.totalStats.ConnectionStats.SessionTerminated.Normal
	case ExpiredTermination:
		i = &s.totalStats.ConnectionStats.SessionTerminated.Expired
	case TakenOverTermination:
		i = &s.totalStats.ConnectionStats.SessionTerminated.TakenOver
	}
	atomic.AddUint64(i, 1)
	atomic.AddUint64(&s.totalStats.ConnectionStats.InactiveCurrent, ^uint64(0))
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	delete(s.clientStats, clientID)
}

// StatsReader interface provides the ability to access the statistics of the server
type StatsReader interface {
	// GetGlobalStats returns the server statistics.
	GetGlobalStats() GlobalStats
	// GetClientStats returns the client statistics for the given client id
	GetClientStats(clientID string) (sts ClientStats, exist bool)
}

// ConnectionStats provides the statistics of client connections.
type ConnectionStats struct {
	ConnectedTotal      uint64
	DisconnectedTotal   uint64
	SessionCreatedTotal uint64
	SessionTerminated   struct {
		TakenOver uint64
		Expired   uint64
		Normal    uint64
	}
	// ActiveCurrent is the number of used active session.
	ActiveCurrent uint64
	// InactiveCurrent is the number of used inactive session.
	InactiveCurrent uint64
}

func (c *ConnectionStats) copy() *ConnectionStats {
	return &ConnectionStats{
		ConnectedTotal:      atomic.LoadUint64(&c.ConnectedTotal),
		DisconnectedTotal:   atomic.LoadUint64(&c.DisconnectedTotal),
		SessionCreatedTotal: atomic.LoadUint64(&c.SessionCreatedTotal),
		SessionTerminated: struct {
			TakenOver uint64
			Expired   uint64
			Normal    uint64
		}{
			TakenOver: atomic.LoadUint64(&c.SessionTerminated.TakenOver),
			Expired:   atomic.LoadUint64(&c.SessionTerminated.Expired),
			Normal:    atomic.LoadUint64(&c.SessionTerminated.Normal),
		},
		ActiveCurrent:   atomic.LoadUint64(&c.ActiveCurrent),
		InactiveCurrent: atomic.LoadUint64(&c.InactiveCurrent),
	}
}

type DroppedTotal struct {
	Internal             uint64
	ExceedsMaxPacketSize uint64
	QueueFull            uint64
	Expired              uint64
	InflightExpired      uint64
}

type MessageQosStats struct {
	DroppedTotal  DroppedTotal
	ReceivedTotal uint64
	SentTotal     uint64
}

func (m *MessageQosStats) GetDroppedTotal() uint64 {
	return m.DroppedTotal.Internal + m.DroppedTotal.Expired + m.DroppedTotal.ExceedsMaxPacketSize + m.DroppedTotal.QueueFull + m.DroppedTotal.InflightExpired
}

// MessageStats represents the statistics of PUBLISH in, separated by QOS.
type MessageStats struct {
	Qos0            MessageQosStats
	Qos1            MessageQosStats
	Qos2            MessageQosStats
	InflightCurrent uint64
	QueuedCurrent   uint64
}

func (m *MessageStats) GetDroppedTotal() uint64 {
	return m.Qos0.GetDroppedTotal() + m.Qos1.GetDroppedTotal() + m.Qos2.GetDroppedTotal()
}

func (m *MessageStats) copy() *MessageStats {
	return &MessageStats{
		Qos0: MessageQosStats{
			DroppedTotal: DroppedTotal{
				Internal:             atomic.LoadUint64(&m.Qos0.DroppedTotal.Internal),
				ExceedsMaxPacketSize: atomic.LoadUint64(&m.Qos0.DroppedTotal.ExceedsMaxPacketSize),
				QueueFull:            atomic.LoadUint64(&m.Qos0.DroppedTotal.QueueFull),
				Expired:              atomic.LoadUint64(&m.Qos0.DroppedTotal.Expired),
				InflightExpired:      atomic.LoadUint64(&m.Qos0.DroppedTotal.InflightExpired),
			},
			ReceivedTotal: atomic.LoadUint64(&m.Qos0.ReceivedTotal),
			SentTotal:     atomic.LoadUint64(&m.Qos0.SentTotal),
		},
		Qos1: MessageQosStats{
			DroppedTotal: DroppedTotal{
				Internal:             atomic.LoadUint64(&m.Qos1.DroppedTotal.Internal),
				ExceedsMaxPacketSize: atomic.LoadUint64(&m.Qos1.DroppedTotal.ExceedsMaxPacketSize),
				QueueFull:            atomic.LoadUint64(&m.Qos1.DroppedTotal.QueueFull),
				Expired:              atomic.LoadUint64(&m.Qos1.DroppedTotal.Expired),
				InflightExpired:      atomic.LoadUint64(&m.Qos1.DroppedTotal.InflightExpired),
			},
			ReceivedTotal: atomic.LoadUint64(&m.Qos1.ReceivedTotal),
			SentTotal:     atomic.LoadUint64(&m.Qos1.SentTotal),
		},
		Qos2: MessageQosStats{
			DroppedTotal: DroppedTotal{
				Internal:             atomic.LoadUint64(&m.Qos2.DroppedTotal.Internal),
				ExceedsMaxPacketSize: atomic.LoadUint64(&m.Qos2.DroppedTotal.ExceedsMaxPacketSize),
				QueueFull:            atomic.LoadUint64(&m.Qos2.DroppedTotal.QueueFull),
				Expired:              atomic.LoadUint64(&m.Qos2.DroppedTotal.Expired),
				InflightExpired:      atomic.LoadUint64(&m.Qos2.DroppedTotal.InflightExpired),
			},
			ReceivedTotal: atomic.LoadUint64(&m.Qos2.ReceivedTotal),
			SentTotal:     atomic.LoadUint64(&m.Qos2.SentTotal),
		},
		InflightCurrent: atomic.LoadUint64(&m.InflightCurrent),
		QueuedCurrent:   atomic.LoadUint64(&m.QueuedCurrent),
	}
}

// GlobalStats is the collection of global statistics.
type GlobalStats struct {
	ConnectionStats   ConnectionStats
	PacketStats       PacketStats
	MessageStats      MessageStats
	SubscriptionStats subscription.Stats
}

// ClientStats is the statistic information of one client.
type ClientStats struct {
	PacketStats       PacketStats
	MessageStats      MessageStats
	SubscriptionStats subscription.Stats
}

func (c ClientStats) GetDroppedTotal() uint64 {
	return c.MessageStats.Qos0.GetDroppedTotal() + c.MessageStats.Qos1.GetDroppedTotal() + c.MessageStats.Qos2.GetDroppedTotal()
}

// GetGlobalStats returns the GlobalStats
func (s *statsManager) GetGlobalStats() GlobalStats {
	return GlobalStats{
		PacketStats:       *s.totalStats.PacketStats.copy(),
		ConnectionStats:   *s.totalStats.ConnectionStats.copy(),
		MessageStats:      *s.totalStats.MessageStats.copy(),
		SubscriptionStats: s.subStatsReader.GetStats(),
	}
}

// GetClientStats returns the client statistic information for given client id.
func (s *statsManager) GetClientStats(clientID string) (ClientStats, bool) {
	if stats, ok := s.getExistingClientStats(clientID); !ok {
		return ClientStats{}, false
	} else {
		subStats, _ := s.subStatsReader.GetClientStats(clientID)
		return ClientStats{
			PacketStats:       *stats.PacketStats.copy(),
			MessageStats:      *stats.MessageStats.copy(),
			SubscriptionStats: subStats,
		}, true
	}

}

func newStatsManager(subStatsReader subscription.StatsReader) *statsManager {
	return &statsManager{
		subStatsReader: subStatsReader,
		totalStats:     &GlobalStats{},
		clientMu:       sync.RWMutex{},
		clientStats:    make(map[string]*ClientStats),
	}
}
