package prometheus

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/server"
)

type packetMetricValue struct {
	label string
	value *uint64
}

func collectPacketsStats(stats *server.PacketStats, metrics chan<- prometheus.Metric) {
	collectPacketValues(metricPrefix+"packets_received_bytes_total", &stats.BytesReceived, metrics)
	collectPacketValues(metricPrefix+"packets_sent_bytes_total", &stats.BytesSent, metrics)
	collectPacketValues(metricPrefix+"packets_received_total", &stats.ReceivedTotal, metrics)
	collectPacketValues(metricPrefix+"packets_sent_total", &stats.SentTotal, metrics)
}

func collectPacketValues(metricName string, values *server.PacketBytes, metrics chan<- prometheus.Metric) {
	for _, item := range packetMetricValues(values) {
		metrics <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(metricName, "", []string{"type"}, nil),
			prometheus.CounterValue,
			float64(atomic.LoadUint64(item.value)),
			item.label,
		)
	}
}

func packetMetricValues(values *server.PacketBytes) []packetMetricValue {
	return []packetMetricValue{
		{label: "AUTH", value: &values.Auth},
		{label: "CONNECT", value: &values.Connect},
		{label: "CONNACK", value: &values.Connack},
		{label: "DISCONNECT", value: &values.Disconnect},
		{label: "PINGREQ", value: &values.Pingreq},
		{label: "PINGRESP", value: &values.Pingresp},
		{label: "PUBACK", value: &values.Puback},
		{label: "PUBCOMP", value: &values.Pubcomp},
		{label: "PUBLISH", value: &values.Publish},
		{label: "PUBREC", value: &values.Pubrec},
		{label: "PUBREL", value: &values.Pubrel},
		{label: "SUBACK", value: &values.Suback},
		{label: "SUBSCRIBE", value: &values.Subscribe},
		{label: "UNSUBACK", value: &values.Unsuback},
		{label: "UNSUBSCRIBE", value: &values.Unsubscribe},
	}
}

func collectClientStats(stats *server.ConnectionStats, metrics chan<- prometheus.Metric) {
	emitMetric(metricPrefix+"clients_connected_total", prometheus.CounterValue, atomic.LoadUint64(&stats.ConnectedTotal), metrics)
	emitMetric(metricPrefix+"sessions_created_total", prometheus.CounterValue, atomic.LoadUint64(&stats.SessionCreatedTotal), metrics)
	emitLabeledMetric(metricPrefix+"sessions_terminated_total", "reason", "expired", atomic.LoadUint64(&stats.SessionTerminated.Expired), metrics)
	emitLabeledMetric(metricPrefix+"sessions_terminated_total", "reason", "taken_over", atomic.LoadUint64(&stats.SessionTerminated.TakenOver), metrics)
	emitLabeledMetric(metricPrefix+"sessions_terminated_total", "reason", "normal", atomic.LoadUint64(&stats.SessionTerminated.Normal), metrics)
	emitMetric(metricPrefix+"sessions_active_current", prometheus.GaugeValue, atomic.LoadUint64(&stats.ActiveCurrent), metrics)
	emitMetric(metricPrefix+"sessions_inactive_current", prometheus.GaugeValue, atomic.LoadUint64(&stats.InactiveCurrent), metrics)
	emitMetric(metricPrefix+"clients_disconnected_total", prometheus.CounterValue, atomic.LoadUint64(&stats.DisconnectedTotal), metrics)
}

func collectMessageStats(stats *server.MessageStats, metrics chan<- prometheus.Metric) {
	collectMessageStatsDropped(stats, metrics)
	collectMessageStatsPending(stats, metrics)
	collectMessageStatsReceived(stats, metrics)
	collectMessageStatsSent(stats, metrics)
}

func collectMessageStatsDropped(stats *server.MessageStats, metrics chan<- prometheus.Metric) {
	metricName := metricPrefix + "messages_dropped_total"
	collectQoSDropped(metricName, "0", &stats.Qos0, metrics)
	collectQoSDropped(metricName, "1", &stats.Qos1, metrics)
	collectQoSDropped(metricName, "2", &stats.Qos2, metrics)
}

func collectQoSDropped(metricName string, qos string, stats *server.MessageQosStats, metrics chan<- prometheus.Metric) {
	emitQosDrop(metricName, qos, "internal", atomic.LoadUint64(&stats.DroppedTotal.Internal), metrics)
	emitQosDrop(metricName, qos, "expired", atomic.LoadUint64(&stats.DroppedTotal.Expired), metrics)
	emitQosDrop(metricName, qos, "inflight_expired", atomic.LoadUint64(&stats.DroppedTotal.InflightExpired), metrics)
	emitQosDrop(metricName, qos, "queue_full", atomic.LoadUint64(&stats.DroppedTotal.QueueFull), metrics)
	emitQosDrop(metricName, qos, "exceeds_max_size", atomic.LoadUint64(&stats.DroppedTotal.ExceedsMaxPacketSize), metrics)
}

func collectMessageStatsPending(stats *server.MessageStats, metrics chan<- prometheus.Metric) {
	emitMetric(metricPrefix+"messages_inflight_current", prometheus.GaugeValue, atomic.LoadUint64(&stats.InflightCurrent), metrics)
	emitMetric(metricPrefix+"messages_queued_current", prometheus.GaugeValue, atomic.LoadUint64(&stats.QueuedCurrent), metrics)
}

func collectMessageStatsReceived(stats *server.MessageStats, metrics chan<- prometheus.Metric) {
	metricName := metricPrefix + "messages_received_total"
	emitLabeledMetric(metricName, "qos", "0", atomic.LoadUint64(&stats.Qos0.ReceivedTotal), metrics)
	emitLabeledMetric(metricName, "qos", "1", atomic.LoadUint64(&stats.Qos1.ReceivedTotal), metrics)
	emitLabeledMetric(metricName, "qos", "2", atomic.LoadUint64(&stats.Qos2.ReceivedTotal), metrics)
}

func collectMessageStatsSent(stats *server.MessageStats, metrics chan<- prometheus.Metric) {
	metricName := metricPrefix + "messages_sent_total"
	emitLabeledMetric(metricName, "qos", "0", atomic.LoadUint64(&stats.Qos0.SentTotal), metrics)
	emitLabeledMetric(metricName, "qos", "1", atomic.LoadUint64(&stats.Qos1.SentTotal), metrics)
	emitLabeledMetric(metricName, "qos", "2", atomic.LoadUint64(&stats.Qos2.SentTotal), metrics)
}

func collectSubscriptionStats(stats *subscription.Stats, metrics chan<- prometheus.Metric) {
	emitMetric(metricPrefix+"subscriptions_total", prometheus.CounterValue, atomic.LoadUint64(&stats.SubscriptionsTotal), metrics)
	emitMetric(metricPrefix+"subscriptions_current", prometheus.GaugeValue, atomic.LoadUint64(&stats.SubscriptionsCurrent), metrics)
}

func emitMetric(name string, valueType prometheus.ValueType, value uint64, metrics chan<- prometheus.Metric) {
	metrics <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(name, "", nil, nil),
		valueType,
		float64(value),
	)
}

func emitLabeledMetric(name string, labelName string, labelValue string, value uint64, metrics chan<- prometheus.Metric) {
	metrics <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(name, "", []string{labelName}, nil),
		prometheus.CounterValue,
		float64(value),
		labelValue,
	)
}

func emitQosDrop(name string, qos string, dropType string, value uint64, metrics chan<- prometheus.Metric) {
	metrics <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(name, "", []string{"qos", "type"}, nil),
		prometheus.CounterValue,
		float64(value),
		qos,
		dropType,
	)
}
