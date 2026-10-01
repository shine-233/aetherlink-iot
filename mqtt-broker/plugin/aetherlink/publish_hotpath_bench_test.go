package aetherlink

import (
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"gopkg.in/redis.v5"
)

// installBenchRedis 为基准测试挂一个 miniredis，调试配置缺省（即"调试关闭"的生产常态）。
func installBenchRedis(b *testing.B) {
	b.Helper()
	s := miniredis.RunT(b)
	prev := redisCache
	redisCache = redis.NewClient(&redis.Options{Addr: s.Addr()})
	b.Cleanup(func() {
		_ = redisCache.Close()
		redisCache = prev
	})
}

var benchUplinkRaw = []byte(`{"temperature":23.5,"humidity":61,"switch":true,"ts":1727654400000}`)

// BenchmarkRecordMQTTDiagnosticDebugOff 覆盖每条上行都会走的诊断记录：
// 设备未开调试时，应当在构建 meta map 之前就短路。
func BenchmarkRecordMQTTDiagnosticDebugOff(b *testing.B) {
	installBenchRedis(b)
	ev := mqttDiagnosticEvent{
		deviceID: "6f1d2a7e-1111-4c3b-9d7e-0a1b2c3d4e5f", clientID: "client-1", username: "u",
		action: "publish", direction: "up", outcome: "ok", code: "publish_ok",
		topic: "devices/telemetry", payload: benchUplinkRaw,
	}
	recordMQTTDiagnosticEvent(ev) // 预热本地负缓存
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recordMQTTDiagnosticEvent(ev)
	}
}

func BenchmarkBuildMQTTUplinkPayload(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = buildMQTTUplinkPayload("6f1d2a7e-1111-4c3b-9d7e-0a1b2c3d4e5f", benchUplinkRaw)
	}
}

// TestBuildMQTTUplinkPayloadMatchesJSONMarshal 锁定线上契约：手写编码必须与
// json.Marshal(map{"device_id","values"}) 逐字节一致（含 nil/空 payload 与需转义的 device_id）。
func TestBuildMQTTUplinkPayloadMatchesJSONMarshal(t *testing.T) {
	ids := []string{"6f1d2a7e-1111-4c3b-9d7e-0a1b2c3d4e5f", "", "a\"b\\c", "<script>&", "中文设备", "x y", "ctl\x01", "bad\xffutf8"}
	payloads := [][]byte{nil, {}, benchUplinkRaw, []byte{0, 1, 2, 0xff}, []byte("a"), []byte("ab"), []byte("abc")}
	for _, id := range ids {
		for _, p := range payloads {
			want, _ := json.Marshal(map[string]interface{}{"device_id": id, "values": p})
			got := buildMQTTUplinkPayload(id, p)
			if string(got) != string(want) {
				t.Fatalf("id=%q payload=%v\n got=%s\nwant=%s", id, p, got, want)
			}
		}
	}
}
