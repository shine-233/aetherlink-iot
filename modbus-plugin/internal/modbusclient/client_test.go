// 文件用途：Modbus 客户端读写闭环回归测试（ROADMAP B1）。
// 核心逻辑：对内嵌从站验证 u16/i16 读取缩放、f32 写读回环与只读点拒绝写入。
// 关键注意事项：地址/字节序契约以本测试为准，改动解码逻辑需同步评审点表文档。
package modbusclient

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/config"
	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/fakemodbus"
)

func testTarget(server *fakemodbus.Server) config.TargetConfig {
	addr := server.Addr()
	idx := strings.LastIndex(addr, ":")
	return config.TargetConfig{
		Host:      addr[:idx],
		Port:      int(mustPort(addr[idx+1:])),
		UnitID:    1,
		TimeoutMs: 2000,
	}
}

func mustPort(s string) uint64 {
	var n uint64
	for i := 0; i < len(s); i++ {
		n = n*10 + uint64(s[i]-'0')
	}
	return n
}

func TestReadHoldingU16WithScaling(t *testing.T) {
	server := fakemodbus.Start(t)
	server.SetHolding(100, 255)
	client := NewClient(testTarget(server))
	register := &config.RegisterPoint{
		Key: "humidity", Type: "holding", Address: 100,
		DataType: "u16", Multiplier: 0.1,
	}
	if err := register.Normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	value, err := client.ReadPoint(context.Background(), register)
	if err != nil {
		t.Fatalf("ReadPoint: %v", err)
	}
	got := value.(float64)
	if math.Abs(got-25.5) > 1e-9 {
		t.Fatalf("value = %v, want 25.5", got)
	}
}

func TestReadInputI16(t *testing.T) {
	server := fakemodbus.Start(t)
	server.SetInput(50, 0xFFCE) // -50 as i16
	client := NewClient(testTarget(server))
	register := &config.RegisterPoint{Key: "temp", Type: "input", Address: 50, DataType: "i16"}
	if err := register.Normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	value, err := client.ReadPoint(context.Background(), register)
	if err != nil {
		t.Fatalf("ReadPoint: %v", err)
	}
	if value.(float64) != -50 {
		t.Fatalf("value = %v, want -50", value)
	}
}

func TestWriteAndReadBackF32(t *testing.T) {
	server := fakemodbus.Start(t)
	client := NewClient(testTarget(server))
	register := &config.RegisterPoint{
		Key: "setpoint", Type: "holding", Address: 200,
		DataType: "f32", Writable: true,
	}
	if err := register.Normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if err := client.WritePoint(context.Background(), register, 36.5); err != nil {
		t.Fatalf("WritePoint: %v", err)
	}
	value, err := client.ReadPoint(context.Background(), register)
	if err != nil {
		t.Fatalf("ReadPoint back: %v", err)
	}
	if math.Abs(value.(float64)-36.5) > 1e-4 {
		t.Fatalf("read-back = %v, want ~36.5", value)
	}
}

func TestWriteRejectsReadOnlyRegister(t *testing.T) {
	server := fakemodbus.Start(t)
	client := NewClient(testTarget(server))
	readOnly := &config.RegisterPoint{Key: "ro", Type: "input", Address: 10}
	if err := client.WritePoint(context.Background(), readOnly, 1); err == nil {
		t.Fatal("write to read-only register must fail")
	}
}

func TestPlanBatchesMergesContiguousAndNearbyPoints(t *testing.T) {
	points := []config.RegisterPoint{
		{Key: "a", Type: "holding", Address: 0, DataType: "u16"},
		{Key: "b", Type: "holding", Address: 1, DataType: "u16"},
		{Key: "c", Type: "holding", Address: 2, DataType: "u32"},  // 占 2 和 3
		{Key: "d", Type: "holding", Address: 8, DataType: "u16"},  // gap=4 (4..7 空洞) 应合并
		{Key: "e", Type: "holding", Address: 50, DataType: "u16"}, // gap 太大，另起一组
	}
	for i := range points {
		if err := points[i].Normalize(); err != nil {
			t.Fatalf("normalize %d: %v", i, err)
		}
	}
	batches := PlanBatches(points, -1)
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2: %+v", len(batches), batches)
	}
	first := batches[0]
	// 覆盖区间 [0,9)：a@0(1) b@1(1) c@2-3(2，u32) d@8(1)，末端地址 9。
	if first.Start != 0 || first.Count != 9 || len(first.Points) != 4 {
		t.Fatalf("first batch = %+v, want start=0 count=9 points=4", first)
	}
	second := batches[1]
	if second.Start != 50 || second.Count != 1 || len(second.Points) != 1 {
		t.Fatalf("second batch = %+v, want start=50 count=1 points=1", second)
	}
}

func TestPlanBatchesSeparatesTypesAndRespectsRegisterCap(t *testing.T) {
	points := []config.RegisterPoint{
		{Key: "h0", Type: "holding", Address: 0, DataType: "u16"},
		{Key: "i0", Type: "input", Address: 0, DataType: "u16"},
	}
	for i := range points {
		if err := points[i].Normalize(); err != nil {
			t.Fatalf("normalize %d: %v", i, err)
		}
	}
	batches := PlanBatches(points, -1)
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2 (holding and input must not merge): %+v", len(batches), batches)
	}

	// 126 个连续 u16 点超过单次请求 125 寄存器上限，必须切成两批。
	many := make([]config.RegisterPoint, 126)
	for i := range many {
		many[i] = config.RegisterPoint{Key: fmt.Sprintf("r%d", i), Type: "holding", Address: uint16(i), DataType: "u16"}
		if err := many[i].Normalize(); err != nil {
			t.Fatalf("normalize many %d: %v", i, err)
		}
	}
	capped := PlanBatches(many, -1)
	if len(capped) != 2 {
		t.Fatalf("capped batches = %d, want 2 (125 cap split): %+v", len(capped), capped)
	}
	total := 0
	for _, b := range capped {
		if b.Count > maxRegistersPerRequest {
			t.Fatalf("batch count %d exceeds protocol max %d", b.Count, maxRegistersPerRequest)
		}
		total += len(b.Points)
	}
	if total != 126 {
		t.Fatalf("total points across capped batches = %d, want 126", total)
	}
}

func TestReadBatchesHoldingAndInputAndCoilDiscrete(t *testing.T) {
	server := fakemodbus.Start(t)
	server.SetHolding(0, 10)
	server.SetHolding(1, 20)
	server.SetInput(5, 0xFFCE) // -50 as i16

	points := []config.RegisterPoint{
		{Key: "h0", Type: "holding", Address: 0, DataType: "u16"},
		{Key: "h1", Type: "holding", Address: 1, DataType: "u16", Multiplier: 2},
		{Key: "i0", Type: "input", Address: 5, DataType: "i16"},
	}
	for i := range points {
		if err := points[i].Normalize(); err != nil {
			t.Fatalf("normalize %d: %v", i, err)
		}
	}
	batches := PlanBatches(points, -1)
	client := NewClient(testTarget(server))
	out := map[string]any{}
	if errs := client.ReadBatches(context.Background(), batches, out); len(errs) != 0 {
		t.Fatalf("ReadBatches errs = %v", errs)
	}
	if v := out["h0"].(float64); v != 10 {
		t.Fatalf("h0 = %v, want 10", v)
	}
	if v := out["h1"].(float64); v != 40 {
		t.Fatalf("h1 = %v, want 40 (20*2)", v)
	}
	if v := out["i0"].(float64); v != -50 {
		t.Fatalf("i0 = %v, want -50", v)
	}
}

func TestReadBatchesFailureReturnsErrorWithoutPanicking(t *testing.T) {
	// 指向一个未监听的端口，连接必然失败；验证批量路径优雅返回错误而不是 panic，
	// 且不会往 out 里写入任何半成品数据。
	client := NewClient(config.TargetConfig{Host: "127.0.0.1", Port: 1, UnitID: 1, TimeoutMs: 200})
	points := []config.RegisterPoint{{Key: "x", Type: "holding", Address: 0, DataType: "u16"}}
	points[0].Normalize()
	batches := PlanBatches(points, -1)
	out := map[string]any{}
	errs := client.ReadBatches(context.Background(), batches, out)
	if len(errs) == 0 {
		t.Fatal("expected error from unreachable target")
	}
	if len(out) != 0 {
		t.Fatalf("out should stay empty on failure, got %v", out)
	}
}
