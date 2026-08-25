// 文件用途：轮询采集器回归测试（ROADMAP B1）。
// 核心逻辑：对内嵌从站验证一轮采集的键值快照、i16 解码与乘法缩放。
// 关键注意事项：Reporter 以 fake 实现，只断言快照内容与调用次数。
package poller

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/config"
	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/fakemodbus"
)

type fakeReporter struct {
	mu        sync.Mutex
	snapshots []map[string]any
}

func (f *fakeReporter) PublishTelemetry(deviceNumber string, values map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	snapshot := map[string]any{}
	for k, v := range values {
		snapshot[k] = v
	}
	f.snapshots = append(f.snapshots, snapshot)
	return nil
}

func TestCollectOnceReadsAndScalesPoints(t *testing.T) {
	server := fakemodbus.Start(t)
	server.SetInput(0, 0xFFCE)  // -50 as i16
	server.SetHolding(10, 100)  // 100 * 0.5 = 50
	addr := server.Addr()
	idx := strings.LastIndex(addr, ":")
	port, _ := strconv.Atoi(addr[idx+1:])
	cfg := config.DeviceConfig{
		DeviceNumber: "plc-01",
		Target:       config.TargetConfig{Host: addr[:idx], Port: port, UnitID: 1, TimeoutMs: 2000},
		Registers: []config.RegisterPoint{
			{Key: "temperature", Type: "input", Address: 0, DataType: "i16"},
			{Key: "humidity", Type: "holding", Address: 10, DataType: "u16", Multiplier: 0.5},
		},
	}
	for i := range cfg.Registers {
		if err := cfg.Registers[i].Normalize(); err != nil {
			t.Fatalf("normalize register %d: %v", i, err)
		}
	}
	reporter := &fakeReporter{}
	p := New(cfg, reporter, nil)
	p.CollectOnce(context.Background())

	if len(reporter.snapshots) != 1 {
		t.Fatalf("snapshots = %d, want 1", len(reporter.snapshots))
	}
	snapshot := reporter.snapshots[0]
	if v, ok := snapshot["temperature"].(float64); !ok || v != -50 {
		t.Fatalf("temperature = %#v, want -50 (i16 decode)", snapshot["temperature"])
	}
	if v, ok := snapshot["humidity"].(float64); !ok || v != 50 {
		t.Fatalf("humidity = %#v, want 50 (100*0.5)", snapshot["humidity"])
	}
}

var _ Reporter = (*fakeReporter)(nil)
