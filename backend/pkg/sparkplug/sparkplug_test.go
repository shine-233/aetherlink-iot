// 文件用途：Sparkplug B 话题解析与载荷解码的契约测试（ROADMAP TB-10）。
//
// 核心逻辑：分五组——话题解析、**手算字节向量锚点**、往返、数据类型判定、fail-closed 负向对照。
//
// 关键注意事项：本包最容易出的错不是"抛异常"，而是**静默解错**：
// 字段号写错、把字符串当数值、把 is_null 当 0。因此这里刻意做了两件事：
//  1. `TestSpecAnchorVector` 用**手工按规范推出的字节**断言编码结果。
//     如果编码器与解码器共享同一个错误字段号，往返测试会全绿而锚点会红——
//     这是本文件唯一能打破"自洽但错误"的防线。
//  2. 字段号另有官方定义比对：Eclipse Tahu `sparkplug_b/sparkplug_b.proto`
//     （Payload 1~5、Metric 1~19、DataType 1~19），已逐条核对。
//
// 未覆盖（如实记录）：本实现**未与真实 Sparkplug 设备联调**，验证依据是规范字段号 +
// 字节向量。DataSet / Template / PropertySet 等嵌套结构按设计只跳过不展开，故不在此断言其内容。
package sparkplug

import (
	"errors"
	"math"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 测试用编码器：按官方 proto 的字段号构造字节。仅用于构造测试向量，不参与生产路径。
// ---------------------------------------------------------------------------

func encVarint(value uint64) []byte {
	out := make([]byte, 0, 10)
	for {
		current := byte(value & 0x7F)
		value >>= 7
		if value != 0 {
			out = append(out, current|0x80)
			continue
		}
		out = append(out, current)
		return out
	}
}

func encKey(field int, wireType int) []byte {
	return encVarint(uint64(field)<<3 | uint64(wireType))
}

func encVarintField(field int, value uint64) []byte {
	return append(encKey(field, wireVarint), encVarint(value)...)
}

func encStringField(field int, value string) []byte {
	body := []byte(value)
	return append(append(encKey(field, wireBytes), encVarint(uint64(len(body)))...), body...)
}

func encBytesField(field int, body []byte) []byte {
	return append(append(encKey(field, wireBytes), encVarint(uint64(len(body)))...), body...)
}

func encFixed32Field(field int, value float32) []byte {
	raw := math.Float32bits(value)
	return append(encKey(field, wireFixed32),
		byte(raw), byte(raw>>8), byte(raw>>16), byte(raw>>24))
}

func encFixed64Field(field int, value float64) []byte {
	raw := math.Float64bits(value)
	out := encKey(field, wireFixed64)
	for index := 0; index < 8; index++ {
		out = append(out, byte(raw>>(8*index)))
	}
	return out
}

func encMessageField(field int, body []byte) []byte {
	return append(append(encKey(field, wireBytes), encVarint(uint64(len(body)))...), body...)
}

func concat(parts ...[]byte) []byte {
	out := []byte{}
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

// ---------------------------------------------------------------------------
// 话题解析
// ---------------------------------------------------------------------------

func TestParseTopic(t *testing.T) {
	cases := []struct {
		name       string
		topic      string
		wantGroup  string
		wantType   string
		wantEdge   string
		wantDevice string
		wantErr    error
	}{
		{name: "节点级 NDATA", topic: "spBv1.0/Factory-A/NDATA/edge-1", wantGroup: "Factory-A", wantType: MessageTypeNodeData, wantEdge: "edge-1"},
		{name: "设备级 DDATA", topic: "spBv1.0/Factory-A/DDATA/edge-1/plc-7", wantGroup: "Factory-A", wantType: MessageTypeDeviceData, wantEdge: "edge-1", wantDevice: "plc-7"},
		{name: "设备级 DBIRTH", topic: "spBv1.0/g/DBIRTH/e/d", wantGroup: "g", wantType: MessageTypeDeviceBirth, wantEdge: "e", wantDevice: "d"},
		{name: "STATE", topic: "spBv1.0/g/STATE/e", wantGroup: "g", wantType: MessageTypeState, wantEdge: "e"},
		{name: "首尾空白被裁剪", topic: "  spBv1.0/g/NDATA/e  ", wantGroup: "g", wantType: MessageTypeNodeData, wantEdge: "e"},
		{name: "命名空间错误", topic: "spBv2.0/g/NDATA/e", wantErr: ErrInvalidTopic},
		{name: "段数过少", topic: "spBv1.0/g/NDATA", wantErr: ErrInvalidTopic},
		{name: "段数过多", topic: "spBv1.0/g/NDATA/e/d/extra", wantErr: ErrInvalidTopic},
		{name: "空段必须拒绝", topic: "spBv1.0//NDATA/e", wantErr: ErrInvalidTopic},
		{name: "空话题", topic: "", wantErr: ErrInvalidTopic},
		{name: "仅空白", topic: "   ", wantErr: ErrInvalidTopic},
		{name: "未知消息类型", topic: "spBv1.0/g/NFOO/e", wantErr: ErrUnsupportedMessageType},
		{name: "小写消息类型不做归一", topic: "spBv1.0/g/ndata/e", wantErr: ErrUnsupportedMessageType},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			parsed, err := ParseTopic(testCase.topic)
			if testCase.wantErr != nil {
				if !errors.Is(err, testCase.wantErr) {
					t.Fatalf("期望 %v，实际 %v", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("意外报错：%v", err)
			}
			if parsed.GroupID != testCase.wantGroup || parsed.MessageType != testCase.wantType ||
				parsed.EdgeNodeID != testCase.wantEdge || parsed.DeviceID != testCase.wantDevice {
				t.Fatalf("解析结果不符：%+v", parsed)
			}
			if parsed.Namespace != Namespace {
				t.Fatalf("namespace = %q", parsed.Namespace)
			}
			if parsed.IsDeviceLevel() != (testCase.wantDevice != "") {
				t.Fatalf("IsDeviceLevel = %v", parsed.IsDeviceLevel())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 规范锚点：手工按字段号推出的字节，用来打破"编码器与解码器同样写错"的自洽陷阱
// ---------------------------------------------------------------------------

// TestSpecAnchorVector 断言一段**手工推导**的最小载荷字节。
//
// 推导（字段号取自官方 sparkplug_b.proto）：
//
//	Payload.timestamp = 1   → key 0x08, value 0x01
//	Payload.metrics   = 2   → key 0x12, len, <Metric>
//	  Metric.name      = 1  → key 0x0A, len 1, 'a'(0x61)
//	  Metric.datatype  = 4  → key 0x20, value 3 (Int32)
//	  Metric.int_value = 10 → key 0x50, value 7
//
// Metric 体长 7 字节，故整体为：08 01 12 07 0A 01 61 20 03 50 07
func TestSpecAnchorVector(t *testing.T) {
	metric := concat(
		encStringField(1, "a"),
		encVarintField(4, DataTypeInt32),
		encVarintField(10, 7),
	)
	encoded := concat(encVarintField(1, 1), encMessageField(2, metric))

	want := []byte{0x08, 0x01, 0x12, 0x07, 0x0A, 0x01, 0x61, 0x20, 0x03, 0x50, 0x07}
	if len(encoded) != len(want) {
		t.Fatalf("字节长度 = %d，期望 %d：% X", len(encoded), len(want), encoded)
	}
	for index := range want {
		if encoded[index] != want[index] {
			t.Fatalf("第 %d 字节 = 0x%02X，期望 0x%02X（完整：% X）", index, encoded[index], want[index], encoded)
		}
	}

	// 同一个向量必须能被解回原值——锚点同时约束编码与解码两侧。
	decoded, err := DecodePayload(want)
	if err != nil {
		t.Fatalf("锚点向量解码失败：%v", err)
	}
	if !decoded.HasTimestamp || decoded.Timestamp != 1 {
		t.Fatalf("timestamp = %d (has=%v)", decoded.Timestamp, decoded.HasTimestamp)
	}
	if len(decoded.Metrics) != 1 {
		t.Fatalf("metrics 数 = %d", len(decoded.Metrics))
	}
	anchor := decoded.Metrics[0]
	if anchor.Name != "a" || anchor.DataType != DataTypeInt32 {
		t.Fatalf("metric = %+v", anchor)
	}
	if value, ok := anchor.NumericValue(); !ok || value != 7 {
		t.Fatalf("NumericValue = %v ok=%v，期望 7", value, ok)
	}
}

// ---------------------------------------------------------------------------
// 往返：覆盖全部标量 oneof 与公共字段
// ---------------------------------------------------------------------------

func TestDecodePayloadRoundTrip(t *testing.T) {
	doubleMetric := concat(
		encStringField(1, "Temperature"),
		encVarintField(2, 42),            // alias
		encVarintField(3, 1700000000000), // timestamp
		encVarintField(4, DataTypeDouble),
		encVarintField(5, 1), // is_historical
		encVarintField(6, 1), // is_transient
		encVarintField(7, 0), // is_null
		encFixed64Field(13, 21.5),
	)
	intMetric := concat(
		encStringField(1, "Pressure"),
		encVarintField(4, DataTypeInt32),
		encVarintField(10, 1013),
	)
	floatMetric := concat(
		encStringField(1, "Flow"),
		encVarintField(4, DataTypeFloat),
		encFixed32Field(12, 3.5),
	)
	boolMetric := concat(
		encStringField(1, "Running"),
		encVarintField(4, DataTypeBoolean),
		encVarintField(14, 1),
	)
	stringMetric := concat(
		encStringField(1, "Serial"),
		encVarintField(4, DataTypeString),
		encStringField(15, "SN-001"),
	)
	bytesMetric := concat(
		encStringField(1, "Blob"),
		encVarintField(4, DataTypeBytes),
		encBytesField(16, []byte{0xDE, 0xAD}),
	)

	encoded := concat(
		encVarintField(1, 1700000000001),
		encMessageField(2, doubleMetric),
		encMessageField(2, intMetric),
		encMessageField(2, floatMetric),
		encMessageField(2, boolMetric),
		encMessageField(2, stringMetric),
		encMessageField(2, bytesMetric),
		encVarintField(3, 99),
		encStringField(4, "uuid-abc"),
		encBytesField(5, []byte{0x01, 0x02}),
	)

	payload, err := DecodePayload(encoded)
	if err != nil {
		t.Fatalf("解码失败：%v", err)
	}
	if payload.Timestamp != 1700000000001 || !payload.HasTimestamp {
		t.Fatalf("timestamp = %d", payload.Timestamp)
	}
	if payload.Seq != 99 || !payload.HasSeq {
		t.Fatalf("seq = %d", payload.Seq)
	}
	if payload.UUID != "uuid-abc" {
		t.Fatalf("uuid = %q", payload.UUID)
	}
	if len(payload.Body) != 2 || payload.Body[0] != 0x01 || payload.Body[1] != 0x02 {
		t.Fatalf("body = % X", payload.Body)
	}
	if len(payload.Metrics) != 6 {
		t.Fatalf("metrics 数 = %d", len(payload.Metrics))
	}

	double := payload.Metrics[0]
	if double.Name != "Temperature" || !double.HasAlias || double.Alias != 42 ||
		!double.HasTimestamp || double.Timestamp != 1700000000000 ||
		double.DataType != DataTypeDouble || !double.IsHistorical || !double.IsTransient || double.IsNull {
		t.Fatalf("double metric = %+v", double)
	}
	if value, ok := double.NumericValue(); !ok || value != 21.5 {
		t.Fatalf("double 值 = %v ok=%v", value, ok)
	}

	if value, ok := payload.Metrics[1].NumericValue(); !ok || value != 1013 {
		t.Fatalf("int32 值 = %v ok=%v", value, ok)
	}
	if value, ok := payload.Metrics[2].NumericValue(); !ok || value != 3.5 {
		t.Fatalf("float 值 = %v ok=%v", value, ok)
	}
	if payload.Metrics[3].StringValue != "" || payload.Metrics[3].HasStringValue {
		t.Fatal("布尔指标的 string_value 不应被置位")
	}
	if !payload.Metrics[3].HasBoolValue || !payload.Metrics[3].BoolValue {
		t.Fatal("布尔值未解出")
	}
	if payload.Metrics[4].StringValue != "SN-001" {
		t.Fatalf("string = %q", payload.Metrics[4].StringValue)
	}
	if len(payload.Metrics[5].BytesValue) != 2 || payload.Metrics[5].BytesValue[0] != 0xDE {
		t.Fatalf("bytes = % X", payload.Metrics[5].BytesValue)
	}
}

// ---------------------------------------------------------------------------
// 数据类型判定：非数值绝不能被强转成 0
// ---------------------------------------------------------------------------

func TestNumericValueByDataType(t *testing.T) {
	numericTypes := []uint32{
		DataTypeInt8, DataTypeInt16, DataTypeInt32, DataTypeInt64,
		DataTypeUInt8, DataTypeUInt16, DataTypeUInt32, DataTypeUInt64,
		DataTypeDateTime,
	}
	for _, dataType := range numericTypes {
		metric := Metric{DataType: dataType, IntValue: 5, HasIntValue: true}
		if value, ok := metric.NumericValue(); !ok || value != 5 {
			t.Fatalf("datatype %d 应为数值 5，实际 %v ok=%v", dataType, value, ok)
		}
	}

	for _, dataType := range []uint32{DataTypeFloat, DataTypeDouble} {
		metric := Metric{DataType: dataType, FloatValue: 1.25, HasFloatValue: true}
		if value, ok := metric.NumericValue(); !ok || value != 1.25 {
			t.Fatalf("datatype %d 应为 1.25，实际 %v ok=%v", dataType, value, ok)
		}
	}

	// 非数值类型必须返回 ok=false，而不是 0。
	nonNumeric := []struct {
		name   string
		metric Metric
	}{
		{name: "布尔", metric: Metric{DataType: DataTypeBoolean, BoolValue: true, HasBoolValue: true}},
		{name: "字符串", metric: Metric{DataType: DataTypeString, StringValue: "12", HasStringValue: true}},
		{name: "字节", metric: Metric{DataType: DataTypeBytes, BytesValue: []byte{1}, HasBytesValue: true}},
		{name: "DataSet", metric: Metric{DataType: DataTypeDataSet}},
		{name: "Template", metric: Metric{DataType: DataTypeTemplate}},
		{name: "未知类型 0", metric: Metric{DataType: 0}},
		{name: "数组类型 22", metric: Metric{DataType: 22, IntValue: 1, HasIntValue: true}},
		{name: "数值类型但缺值", metric: Metric{DataType: DataTypeInt32}},
		{name: "is_null 显式空值", metric: Metric{DataType: DataTypeInt32, IntValue: 5, HasIntValue: true, IsNull: true}},
	}
	for _, testCase := range nonNumeric {
		t.Run(testCase.name, func(t *testing.T) {
			if value, ok := testCase.metric.NumericValue(); ok {
				t.Fatalf("必须返回 ok=false，实际 %v（把非数值当 0 会让下游拿到假读数）", value)
			}
		})
	}
}

func TestNumericTelemetrySkipsNonNumericAndNameless(t *testing.T) {
	payload := Payload{Metrics: []Metric{
		{Name: "good", DataType: DataTypeInt32, IntValue: 3, HasIntValue: true},
		{Name: "  spaced  ", DataType: DataTypeDouble, FloatValue: 1.5, HasFloatValue: true},
		{Name: "text", DataType: DataTypeString, StringValue: "x", HasStringValue: true},
		{Name: "nulled", DataType: DataTypeInt32, IntValue: 9, HasIntValue: true, IsNull: true},
		{Name: "", DataType: DataTypeInt32, IntValue: 4, HasIntValue: true},
		{Name: "   ", DataType: DataTypeInt32, IntValue: 4, HasIntValue: true},
		{Alias: 7, DataType: DataTypeInt32, IntValue: 4, HasIntValue: true},
		{Name: "good", DataType: DataTypeInt32, IntValue: 8, HasIntValue: true},
	}}

	telemetry := payload.NumericTelemetry()

	want := map[string]float64{"good": 8, "spaced": 1.5}
	if len(telemetry) != len(want) {
		t.Fatalf("键数 = %d（%v），期望 %d", len(telemetry), telemetry, len(want))
	}
	for key, value := range want {
		if telemetry[key] != value {
			t.Fatalf("%s = %v，期望 %v", key, telemetry[key], value)
		}
	}
	if _, exists := telemetry["text"]; exists {
		t.Fatal("字符串指标不得进入数值通道")
	}
}

// ---------------------------------------------------------------------------
// 负向对照：畸形载荷必须报错，不能解出半截结果
// ---------------------------------------------------------------------------

func TestDecodePayloadRejectsMalformedInput(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{name: "空载荷", data: []byte{}},
		{name: "截断的 key", data: []byte{0x80}},
		{name: "截断的 varint 值", data: []byte{0x08, 0x80}},
		{name: "metrics 用了 varint 而非 length-delimited", data: []byte{0x10, 0x01}},
		{name: "metrics 长度超出剩余字节", data: []byte{0x12, 0x7F, 0x0A}},
		{name: "非法 wire type 3", data: []byte{0x0B}},
		{name: "非法 wire type 4", data: []byte{0x0C}},
		{name: "非法 wire type 6", data: []byte{0x0E}},
		{name: "float_value 用了 varint", data: concat(encMessageField(2, concat(encVarintField(4, DataTypeFloat), encVarintField(12, 1))))},
		{name: "double_value 用了 32 位", data: concat(encMessageField(2, concat(encVarintField(4, DataTypeDouble), encFixed32Field(13, 1))))},
		{name: "非法 UTF-8 的名称", data: concat(encMessageField(2, encBytesField(1, []byte{0xFF, 0xFE})))},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := DecodePayload(testCase.data); err == nil {
				t.Fatal("畸形载荷必须报错，不得解出半截结果")
			} else if !errors.Is(err, ErrMalformedPayload) {
				t.Fatalf("期望 ErrMalformedPayload，实际 %v", err)
			}
		})
	}
}

// 前向兼容：规范未来新增字段不应让整包解码失败（未知字段按 wire type 跳过）。
func TestDecodePayloadSkipsUnknownFields(t *testing.T) {
	encoded := concat(
		encVarintField(1, 5),
		encVarintField(6, 123),      // Payload 的扩展位（规范保留 6 to max）
		encStringField(7, "future"), // 未知的 length-delimited 字段
		encMessageField(2, concat(encStringField(1, "k"), encVarintField(4, DataTypeInt32), encVarintField(10, 1))),
		encFixed64Field(8, 1.5), // 未知的 64 位字段
		encFixed32Field(9, 1.5), // 未知的 32 位字段
	)

	payload, err := DecodePayload(encoded)
	if err != nil {
		t.Fatalf("未知字段不应导致解码失败：%v", err)
	}
	if payload.Timestamp != 5 {
		t.Fatalf("timestamp = %d", payload.Timestamp)
	}
	if len(payload.Metrics) != 1 || payload.Metrics[0].Name != "k" {
		t.Fatalf("metrics = %+v", payload.Metrics)
	}
}

// 深层嵌套/超长输入不应导致 panic（模糊测试式的粗粒度防护）。
func TestDecodePayloadNeverPanicsOnGarbage(t *testing.T) {
	inputs := [][]byte{
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		[]byte(strings.Repeat("\x12\x02\x0a\x00", 64)),
		[]byte(strings.Repeat("\x0a", 256)),
	}
	for index, input := range inputs {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("输入 %d 触发 panic：%v", index, recovered)
				}
			}()
			_, _ = DecodePayload(input)
		}()
	}
}
