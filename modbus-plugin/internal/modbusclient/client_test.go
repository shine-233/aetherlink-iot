// 文件用途：Modbus 客户端读写闭环回归测试（ROADMAP B1）。
// 核心逻辑：对内嵌从站验证 u16/i16 读取缩放、f32 写读回环与只读点拒绝写入。
// 关键注意事项：地址/字节序契约以本测试为准，改动解码逻辑需同步评审点表文档。
package modbusclient

import (
	"context"
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
