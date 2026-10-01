// 文件用途：验证按需解码 decodePayloadSubset 与全量 decodeFlatPayload 在 wanted 键上逐键等价，
// 以及规则集批量写回 / 门控语义与原逐条实现一致。
package calcfield

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/uplink"
)

func wantedSet(keys ...string) map[string]string {
	m := make(map[string]string, len(keys))
	for _, k := range keys {
		m[k] = k
	}
	return m
}

func referenceSubset(raw []byte, wanted map[string]string) map[string]interface{} {
	flat := decodeFlatPayload(raw)
	if flat == nil {
		return nil
	}
	out := map[string]interface{}{}
	for k, v := range flat {
		if _, ok := wanted[k]; ok {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func TestDecodePayloadSubsetMatchesFullDecode(t *testing.T) {
	wanted := wantedSet("a", "b", "c", "温度", "", "x y")
	cases := []string{
		`{"a":1,"b":2.5,"c":true}`,
		`  { "a" : -1.5e3 , "b" : false , "z" : "str" }  `,
		`{"a":"text","b":null,"c":[1,2,{"a":3}]}`,
		`{"a":{"nested":{"b":1}},"b":2}`,
		`{"a":1,"a":"override"}`,
		`{"a":"first","a":2}`,
		`{"a":1,"a":3}`,
		`{"温度":36.6,"湿度":40}`,
		`{"":7,"x y":8}`,
		`{"\u0061":5,"b":1}`,
		`{"a\"q":1,"b":2}`,
		`{"s":"with \"escaped\" quote and \\ backslash","a":4}`,
		`{"a":1e400}`,
		`{"z":1e400,"a":1}`,
		`{"z":[1,2,1e999],"a":1}`,
		`{"z":-1e-400,"a":1}`,
		`{"z":"1e400","a":1}`,
		`{"a":0,"b":-0,"c":0.000001}`,
		`{"a":12345678901234567890123}`,
		`{"z":` + strings.Repeat("9", 320) + `,"a":1}`,
		`{}`,
		`[]`,
		`[{"a":1}]`,
		`null`,
		`"a"`,
		`12`,
		`{"a":1`,
		`{"a":1,}`,
		`not json`,
		``,
		`{"a":tru}`,
		"{\"a\":1,\"b\xff\":2}",
	}
	for _, raw := range cases {
		got := decodePayloadSubset([]byte(raw), wanted)
		want := referenceSubset([]byte(raw), wanted)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("payload %q:\n got  %#v\n want %#v", raw, got, want)
		}
	}
}

func TestDecodePayloadSubsetDoesNotRetainInputBuffer(t *testing.T) {
	raw := []byte(`{"a":1}`)
	got := decodePayloadSubset(raw, wantedSet("a"))
	raw[2] = 'X' // 改写消息缓冲区后结果 map 的键必须不变
	if _, ok := got["a"]; !ok || len(got) != 1 {
		t.Fatalf("result map keys must not alias the input buffer: %#v", got)
	}
}

func TestProcessMessageBatchesSimpleRules(t *testing.T) {
	storage := &stubStorage{accept: true}
	engine := newTestEngine(storage, stubSource{
		templateID: "tpl-1",
		fields: []FieldRule{
			{ID: "f-1", OutputKey: "power_w", Expression: "voltage * current"},
			{ID: "f-2", OutputKey: "double_v", Expression: "voltage * 2"},
			{ID: "f-3", OutputKey: "missing", Expression: "voltage * absent"},
			{ID: "f-4", OutputKey: "hot", Expression: "voltage > 10"},
		},
	})
	msg := telemetryMessage("dev-1", map[string]interface{}{"voltage": 12.5, "current": 2.0, "label": "x"}, nil)
	engine.processMessage(msg)

	if len(storage.messages) != 1 {
		t.Fatalf("simple rule outputs must be batched into one message, got %d", len(storage.messages))
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(storage.messages[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"power_w": 25.0, "double_v": 25.0, "hot": true}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("batched payload = %#v, want %#v", payload, want)
	}
	if flag, ok := storage.messages[0].GetMetadata(MetadataGeneratedFlag); !ok || flag != true {
		t.Fatalf("batched message must carry generated flag")
	}
}

func TestProcessMessageBatchDropCountsPerKey(t *testing.T) {
	storage := &stubStorage{accept: false}
	engine := newTestEngine(storage, stubSource{
		templateID: "tpl-1",
		fields: []FieldRule{
			{ID: "f-1", OutputKey: "a2", Expression: "a * 2"},
			{ID: "f-2", OutputKey: "a3", Expression: "a * 3"},
		},
	})
	engine.processMessage(telemetryMessage("dev-1", map[string]interface{}{"a": 1.0}, nil))
	if engine.DroppedCount() != 2 {
		t.Fatalf("dropped = %d, want 2 (one per output key)", engine.DroppedCount())
	}
}

// 零变量表达式受"payload 至少有一个数值/布尔"门控：全是字符串时不产出，有数值时产出。
func TestProcessMessageConstantRuleKeepsPayloadGate(t *testing.T) {
	storage := &stubStorage{accept: true}
	engine := newTestEngine(storage, stubSource{
		templateID: "tpl-1",
		fields:     []FieldRule{{ID: "f-1", OutputKey: "const", Expression: "1 + 1"}},
	})
	engine.processMessage(telemetryMessage("dev-1", map[string]interface{}{"label": "x"}, nil))
	if len(storage.messages) != 0 {
		t.Fatalf("string-only payload must not trigger constant rule, got %d", len(storage.messages))
	}
	engine.processMessage(telemetryMessage("dev-1", map[string]interface{}{"other": 1.0}, nil))
	if len(storage.messages) != 1 {
		t.Fatalf("numeric payload must trigger constant rule, got %d", len(storage.messages))
	}
}

func TestProcessMessageSkipsNonTelemetryAndEmpty(t *testing.T) {
	storage := &stubStorage{accept: true}
	engine := newTestEngine(storage, stubSource{
		templateID: "tpl-1",
		fields:     []FieldRule{{ID: "f-1", OutputKey: "o", Expression: "a + 1"}},
	})
	engine.processMessage(&uplink.DeviceMessage{Type: uplink.MessageTypeTelemetry, DeviceID: "d", Payload: nil})
	engine.processMessage(&uplink.DeviceMessage{Type: "event", DeviceID: "d", Payload: []byte(`{"a":1}`)})
	if len(storage.messages) != 0 {
		t.Fatalf("got %d writes", len(storage.messages))
	}
}
