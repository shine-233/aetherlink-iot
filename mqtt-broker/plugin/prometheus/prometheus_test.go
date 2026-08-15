// This file verifies exporter lifecycle behavior and the externally consumed metrics contract.

package prometheus

import (
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	promclient "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt/server"
)

func TestLoadReturnsErrorForDuplicateRegistration(t *testing.T) {
	reg := promclient.NewRegistry()
	first := newTestPrometheus(reg, "127.0.0.1:0")
	if err := first.Load(newTestServer(t)); err != nil {
		t.Fatalf("first Load() error = %v", err)
	}
	defer func() {
		if err := first.Unload(); err != nil {
			t.Fatalf("first Unload() error = %v", err)
		}
	}()

	second := newTestPrometheus(reg, "127.0.0.1:0")
	err := second.Load(newTestServer(t))
	if err == nil {
		t.Fatal("second Load() error = nil, want duplicate registration error")
	}
	if !strings.Contains(err.Error(), "register prometheus collector") {
		t.Fatalf("second Load() error = %v, want registration context", err)
	}
}

func TestLoadReturnsListenErrorAndRollsBackRegistration(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	addr := occupied.Addr().String()
	defer occupied.Close()

	reg := promclient.NewRegistry()
	p := newTestPrometheus(reg, addr)
	err = p.Load(newTestServer(t))
	if err == nil {
		t.Fatal("Load() error = nil, want listen error")
	}
	if !strings.Contains(err.Error(), "listen prometheus exporter") {
		t.Fatalf("Load() error = %v, want listen context", err)
	}

	if err := occupied.Close(); err != nil {
		t.Fatalf("release occupied port: %v", err)
	}

	reload := newTestPrometheus(reg, "127.0.0.1:0")
	if err := reload.Load(newTestServer(t)); err != nil {
		t.Fatalf("Load() after listen failure error = %v", err)
	}
	if err := reload.Unload(); err != nil {
		t.Fatalf("Unload() after listen failure error = %v", err)
	}
}

func TestLoadUnloadAllowsReload(t *testing.T) {
	reg := promclient.NewRegistry()
	first := newTestPrometheus(reg, "127.0.0.1:0")
	if err := first.Load(newTestServer(t)); err != nil {
		t.Fatalf("first Load() error = %v", err)
	}
	if err := first.Unload(); err != nil {
		t.Fatalf("first Unload() error = %v", err)
	}

	second := newTestPrometheus(reg, "127.0.0.1:0")
	if err := second.Load(newTestServer(t)); err != nil {
		t.Fatalf("second Load() error = %v", err)
	}
	if err := second.Unload(); err != nil {
		t.Fatalf("second Unload() error = %v", err)
	}
}

func TestCollectorExportsBrokerDiagnosticMetrics(t *testing.T) {
	log = zap.NewNop()
	ctrl := gomock.NewController(t)
	statsReader := server.NewMockStatsReader(ctrl)
	statsReader.EXPECT().GetGlobalStats().Return(server.GlobalStats{
		ConnectionStats: server.ConnectionStats{ConnectedTotal: 5},
		PacketStats: server.PacketStats{
			BytesReceived: server.PacketBytes{Auth: 17},
			ReceivedTotal: server.PacketCount{Auth: 3},
		},
		MessageStats: server.MessageStats{
			InflightCurrent: 4,
			Qos1: server.MessageQosStats{
				DroppedTotal: server.DroppedTotal{InflightExpired: 2},
			},
		},
	}).AnyTimes()

	reg := promclient.NewRegistry()
	reg.MustRegister(&Prometheus{statsManager: statsReader})
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	assertMetricValue(t, families, "gmqtt_clients_connected_total", nil, 5)
	assertMetricValue(t, families, "gmqtt_packets_received_bytes_total", map[string]string{"type": "AUTH"}, 17)
	assertMetricValue(t, families, "gmqtt_packets_received_total", map[string]string{"type": "AUTH"}, 3)
	assertMetricValue(t, families, "gmqtt_messages_inflight_current", nil, 4)
	assertMetricValue(t, families, "gmqtt_messages_dropped_total", map[string]string{"qos": "1", "type": "inflight_expired"}, 2)
}

func assertMetricValue(t *testing.T, families []*dto.MetricFamily, name string, labels map[string]string, want float64) {
	t.Helper()
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			if !metricHasLabels(metric, labels) {
				continue
			}
			var got float64
			switch family.GetType() {
			case dto.MetricType_COUNTER:
				got = metric.GetCounter().GetValue()
			case dto.MetricType_GAUGE:
				got = metric.GetGauge().GetValue()
			default:
				t.Fatalf("metric %s type = %s, want counter or gauge", name, family.GetType())
			}
			if got != want {
				t.Fatalf("metric %s labels %v = %v, want %v", name, labels, got, want)
			}
			return
		}
	}
	t.Fatalf("metric %s labels %v not found", name, labels)
}

func metricHasLabels(metric *dto.Metric, want map[string]string) bool {
	if len(metric.Label) != len(want) {
		return false
	}
	for _, pair := range metric.Label {
		if want[pair.GetName()] != pair.GetValue() {
			return false
		}
	}
	return true
}

func newTestPrometheus(reg *promclient.Registry, addr string) *Prometheus {
	return &Prometheus{
		httpServer: &http.Server{Addr: addr},
		path:       "/metrics",
		registerer: reg,
		gatherer:   reg,
	}
}

func newTestServer(t *testing.T) server.Server {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockServer := server.NewMockServer(ctrl)
	mockStats := server.NewMockStatsReader(ctrl)
	mockServer.EXPECT().StatsManager().Return(mockStats)
	mockStats.EXPECT().GetGlobalStats().Return(server.GlobalStats{}).AnyTimes()
	return mockServer
}
