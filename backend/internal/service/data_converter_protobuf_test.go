// 文件用途：PROTOBUF 转换引擎（TB-19）单测——固定 proto + 手工按 protobuf wire format 预编码载荷。
// 核心逻辑：不借助被测引擎构造载荷（stdlib 二进制拼接），验证 hex/base64 解码、
//
//	telemetry/attributes 映射、未知字段容错、类型不符跳过、schema 缺失 fail-closed。
//
// 关键注意事项：预编码字节中的 tag/长度为手工推算（注释标明字段号+线型），
//
//	与 jhump/protoreflect 无耦合，避免"用库测库"的自证。
//
// 重构建议：若后续增加下行编码引擎，为 Encode 侧补充同风格往返用例。
package service

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"math"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	utils "aetherlink-iot/backend/pkg/utils"
)

// tb19TestProto 固定测试 proto 源（覆盖标量/枚举/repeated/嵌套消息/map/bytes/float32）。
const tb19TestProto = `syntax = "proto3";
package iot.tb19;

message EnvironmentReading {
  string device_label = 1;
  double temperature_c = 2;
  int64 humidity_ppm = 3;
  bool alarm_active = 4;
  Status status = 5;
  repeated double samples = 6;
  NestedMeta meta = 7;
  map<string, string> labels = 8;
  bytes payload_ref = 9;
  float battery_v = 10;

  enum Status {
    UNKNOWN = 0;
    OK = 1;
    DEGRADED = 2;
  }
  message NestedMeta {
    string zone = 1;
    int32 rssi = 2;
  }
}
`

// tb19Uvarint 标准 unsigned varint 编码（负数由调用方先转 64 位补码）。
func tb19Uvarint(v uint64) []byte {
	var out []byte
	for v >= 0x80 {
		out = append(out, byte(v)|0x80)
		v >>= 7
	}
	return append(out, byte(v))
}

// buildTb19Payload 手工按 wire format 预编码一条 EnvironmentReading：
// device_label="dev-01", temperature_c=21.5, humidity_ppm=65432, alarm_active=true,
// status=DEGRADED(2), samples=[20.5,21.0](packed), meta{zone="zone-a",rssi=-71},
// labels{"zone":"A1"}, payload_ref=0x010203, battery_v=float32(3.7)，
// 外加两个未知字段（99=varint 42、100=2 字节）用于容错断言。
func buildTb19Payload() []byte {
	var b []byte
	// field 1, wiretype 2: 0x0A + len + bytes
	b = append(b, 0x0A, 0x06)
	b = append(b, "dev-01"...)
	// field 2, wiretype 1 (double): 0x11 + 8B little-endian IEEE754
	b = append(b, 0x11)
	b = binary.LittleEndian.AppendUint64(b, math.Float64bits(21.5))
	// field 3, wiretype 0 (int64): 0x18 + varint
	b = append(b, 0x18)
	b = append(b, tb19Uvarint(65432)...)
	// field 4, wiretype 0 (bool): 0x20 + 0x01
	b = append(b, 0x20, 0x01)
	// field 5, wiretype 0 (enum): 0x28 + 0x02
	b = append(b, 0x28, 0x02)
	// field 6, wiretype 2 (packed repeated double): 0x32 + len + 2×8B
	packed := binary.LittleEndian.AppendUint64(nil, math.Float64bits(20.5))
	packed = binary.LittleEndian.AppendUint64(packed, math.Float64bits(21.0))
	b = append(b, 0x32, byte(len(packed)))
	b = append(b, packed...)
	// field 7, wiretype 2 (submessage): 0x3A + len + inner
	var rssi int64 = -71
	var meta []byte
	meta = append(meta, 0x0A, 0x06)
	meta = append(meta, "zone-a"...)
	meta = append(meta, 0x10) // inner field 2, wiretype 0 (int32, sign-extended)
	meta = append(meta, tb19Uvarint(uint64(rssi))...)
	b = append(b, 0x3A, byte(len(meta)))
	b = append(b, meta...)
	// field 8, wiretype 2 (map entry): 0x42 + len + entry(key=field1,value=field2)
	entry := []byte{0x0A, 0x04, 'z', 'o', 'n', 'e', 0x12, 0x02, 'A', '1'}
	b = append(b, 0x42, byte(len(entry)))
	b = append(b, entry...)
	// field 9, wiretype 2 (bytes): 0x4A + len + bytes
	b = append(b, 0x4A, 0x03, 0x01, 0x02, 0x03)
	// field 10, wiretype 5 (float): 0x55 + 4B little-endian
	b = append(b, 0x55)
	b = binary.LittleEndian.AppendUint32(b, math.Float32bits(3.7))
	// 未知字段 99, wiretype 0: tag varint = (99<<3)|0 = 792 → 0x98 0x06，值 42
	b = append(b, 0x98, 0x06, 0x2A)
	// 未知字段 100, wiretype 2: tag = (100<<3)|2 = 802 → 0xA2 0x06，2 字节负载
	b = append(b, 0xA2, 0x06, 0x02, 0x01, 0x02)
	return b
}

// tb19ProtoSchemaPtr 返回 schema 指针的便捷封装。
func tb19ProtoSchemaPtr() *string {
	s := tb19TestProto
	return &s
}

// tb19FullConfig 常规映射配置：遥测 10 键 + 缺失属性 1 键。
func tb19FullConfig() *string {
	conf := `{
		"message_type": "EnvironmentReading",
		"device_name": "device_label",
		"telemetry": {
			"temperature": "temperature_c",
			"humidity": "humidity_ppm",
			"alarm": "alarm_active",
			"status": "status",
			"sample0": "samples[0]",
			"samples": "samples",
			"zone": "meta.zone",
			"rssi": "meta.rssi",
			"label_zone": "labels.zone",
			"ref": "payload_ref",
			"battery": "battery_v"
		},
		"attributes": {"missing_attr": "labels.fw"}
	}`
	return &conf
}

// TestProtobufConverterDecodesFixedPayload 主干用例：固定 proto + 预编码载荷 → 期望遥测。
func TestProtobufConverterDecodesFixedPayload(t *testing.T) {
	raw := buildTb19Payload()

	req := &model.TestDataConverterReq{
		ConverterMode: model.ConverterModeProtoBuf,
		Payload:       hex.EncodeToString(raw),
		ProtoSchema:   tb19ProtoSchemaPtr(),
		Configuration: tb19FullConfig(),
	}
	resp, err := executeProtobufConverter(req)
	if err != nil {
		t.Fatalf("executeProtobufConverter 返回 error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("期望解码成功，实际 Success=false, error=%q, logs=%v", resp.Error, resp.Logs)
	}

	wantTelemetry := map[string]interface{}{
		"temperature": 21.5,
		"humidity":    int64(65432),
		"alarm":       true,
		"status":      "DEGRADED", // 枚举解析为符号名
		"sample0":     20.5,
		"samples":     []interface{}{20.5, 21.0},
		"zone":        "zone-a",
		"rssi":        int64(-71),
		"label_zone":  "A1",
		"ref":         "010203", // bytes 转 hex 字符串
		"battery":     float64(float32(3.7)),
	}
	if len(resp.Telemetry) != len(wantTelemetry) {
		t.Fatalf("遥测键数不符：want %d got %d (%v)", len(wantTelemetry), len(resp.Telemetry), resp.Telemetry)
	}
	for key, want := range wantTelemetry {
		got, ok := resp.Telemetry[key]
		if !ok {
			t.Fatalf("缺少遥测键 %s；实际 %v", key, resp.Telemetry)
		}
		if !protobufValueEqual(got, want) {
			t.Errorf("遥测 %s = %#v，期望 %#v", key, got, want)
		}
	}

	if resp.DeviceName != "dev-01" {
		t.Errorf("DeviceName = %q，期望 dev-01", resp.DeviceName)
	}
	// 缺失的属性路径被跳过且不臆造。
	if len(resp.Attributes) != 0 {
		t.Errorf("缺失路径不应产出属性，实际 %v", resp.Attributes)
	}
	// 未知字段容错日志。
	protobufAssertLogContains(t, resp, "unknown/unmatched field data")
	// 编码识别日志。
	protobufAssertLogContains(t, resp, "from hex payload")
}

// TestProtobufConverterBase64Payload 同一载荷走 base64 通道，结果与 hex 通道一致。
func TestProtobufConverterBase64Payload(t *testing.T) {
	raw := buildTb19Payload()

	req := &model.TestDataConverterReq{
		ConverterMode: model.ConverterModeProtoBuf,
		Payload:       base64.StdEncoding.EncodeToString(raw),
		ProtoSchema:   tb19ProtoSchemaPtr(),
		Configuration: tb19FullConfig(),
	}
	resp, err := executeProtobufConverter(req)
	if err != nil {
		t.Fatalf("executeProtobufConverter 返回 error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("base64 载荷应解码成功，error=%q", resp.Error)
	}
	if resp.Telemetry["temperature"] != 21.5 || resp.Telemetry["humidity"] != int64(65432) {
		t.Fatalf("base64 通道遥测不符：%v", resp.Telemetry)
	}
	protobufAssertLogContains(t, resp, "from base64")
}

// TestProtobufConverterMessageTypeSelection 消息类型选择：全名/短名/缺省回退/未知类型。
func TestProtobufConverterMessageTypeSelection(t *testing.T) {
	raw := buildTb19Payload()

	cases := []struct {
		name        string
		messageType string
		wantSuccess bool
		wantLog     string
	}{
		{name: "包限定全名", messageType: "iot.tb19.EnvironmentReading", wantSuccess: true},
		{name: "短名经包名解析", messageType: "EnvironmentReading", wantSuccess: true},
		{name: "缺省回退首个 message", messageType: "", wantSuccess: true, wantLog: "using first message"},
		{name: "未知类型 fail-closed", messageType: "NoSuchMessage", wantSuccess: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			conf := `{"telemetry": {"temperature": "temperature_c"}}`
			if testCase.messageType != "" {
				conf = `{"message_type": "` + testCase.messageType + `", "telemetry": {"temperature": "temperature_c"}}`
			}
			req := &model.TestDataConverterReq{
				ConverterMode: model.ConverterModeProtoBuf,
				Payload:       "0x" + hex.EncodeToString(raw),
				ProtoSchema:   tb19ProtoSchemaPtr(),
				Configuration: &conf,
			}
			resp, err := executeProtobufConverter(req)
			if err != nil {
				t.Fatalf("引擎不应返回 error，实际 %v", err)
			}
			if resp.Success != testCase.wantSuccess {
				t.Fatalf("Success = %v（error=%q, logs=%v）", resp.Success, resp.Error, resp.Logs)
			}
			if testCase.wantSuccess && resp.Telemetry["temperature"] != 21.5 {
				t.Errorf("temperature = %v，期望 21.5", resp.Telemetry["temperature"])
			}
			if testCase.wantLog != "" {
				protobufAssertLogContains(t, resp, testCase.wantLog)
			}
		})
	}
}

// TestProtobufConverterFailClosed 失败面：schema 缺失/非法 proto/坏载荷/截断二进制。
func TestProtobufConverterFailClosed(t *testing.T) {
	cases := []struct {
		name        string
		payload     string
		schema      *string
		wantErrPart string
	}{
		{
			name:        "schema 缺失",
			payload:     "0A0600",
			schema:      nil,
			wantErrPart: "proto_schema is empty",
		},
		{
			name:        "schema 非法语法",
			payload:     "0A0600",
			schema:      strPtr("message Broken {"),
			wantErrPart: "invalid proto schema",
		},
		{
			name:        "载荷既非 hex 也非 base64",
			payload:     "!!!not-encodable!!!",
			schema:      tb19ProtoSchemaPtr(),
			wantErrPart: "invalid protobuf payload",
		},
		{
			name:        "二进制截断（长度声明越界）",
			payload:     hex.EncodeToString([]byte{0x0A, 0xFF}),
			schema:      tb19ProtoSchemaPtr(),
			wantErrPart: "protobuf decode failed",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			req := &model.TestDataConverterReq{
				ConverterMode: model.ConverterModeProtoBuf,
				Payload:       testCase.payload,
				ProtoSchema:   testCase.schema,
			}
			resp, err := executeProtobufConverter(req)
			if err != nil {
				t.Fatalf("引擎失败面不应返回 error（应写入 resp.Error），实际 %v", err)
			}
			if resp.Success {
				t.Fatalf("期望 fail-closed（Success=false），实际成功：%v", resp.Telemetry)
			}
			if !strings.Contains(resp.Error, testCase.wantErrPart) {
				t.Fatalf("error=%q 未包含 %q", resp.Error, testCase.wantErrPart)
			}
		})
	}
}

// TestProtobufConverterUnknownFieldTolerance 未知字段不阻断解码，已知字段照常提取。
func TestProtobufConverterUnknownFieldTolerance(t *testing.T) {
	raw := buildTb19Payload()
	conf := `{"telemetry": {"temperature": "temperature_c"}}`

	req := &model.TestDataConverterReq{
		ConverterMode: model.ConverterModeProtoBuf,
		Payload:       hex.EncodeToString(raw),
		ProtoSchema:   tb19ProtoSchemaPtr(),
		Configuration: &conf,
	}
	resp, err := executeProtobufConverter(req)
	if err != nil {
		t.Fatalf("executeProtobufConverter 返回 error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("未知字段应被容错，实际 Success=false error=%q", resp.Error)
	}
	if resp.Telemetry["temperature"] != 21.5 {
		t.Errorf("已知字段提取不符：%v", resp.Telemetry)
	}
	protobufAssertLogContains(t, resp, "unknown/unmatched field data")
}

// TestProtobufConverterTypeMismatchSkipSkipsMappedKey 类型不符 fail-closed 探针：
// 字段 2（声明 double/wiretype1）以 varint（wiretype0）线型写入 → 值落入未知区，
// 映射键必须跳过，不允许臆造值。
func TestProtobufConverterTypeMismatchSkipsMappedKey(t *testing.T) {
	var payload []byte
	// field 2 以错误线型 wiretype 0 写入：tag = (2<<3)|0 = 0x10
	payload = append(payload, 0x10, 0x2A)

	conf := `{"telemetry": {"temperature": "temperature_c"}}`
	req := &model.TestDataConverterReq{
		ConverterMode: model.ConverterModeProtoBuf,
		Payload:       hex.EncodeToString(payload),
		ProtoSchema:   tb19ProtoSchemaPtr(),
		Configuration: &conf,
	}
	resp, err := executeProtobufConverter(req)
	if err != nil {
		t.Fatalf("executeProtobufConverter 返回 error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("线型不匹配应容错处理，实际 Success=false error=%q", resp.Error)
	}
	if _, exists := resp.Telemetry["temperature"]; exists {
		t.Fatalf("线型不匹配的字段不得产出遥测值（fail-closed），实际 %v", resp.Telemetry["temperature"])
	}
	protobufAssertLogContains(t, resp, "unknown/unmatched field data")
	protobufAssertLogContains(t, resp, "Skipped telemetry")
}

// TestProtobufConverterAbsentPathSkipped proto3 未出现字段（零值等价）按缺失处理。
func TestProtobufConverterAbsentPathSkipped(t *testing.T) {
	conf := `{"telemetry": {"temperature": "temperature_c", "zone": "meta.zone"}}`
	req := &model.TestDataConverterReq{
		ConverterMode: model.ConverterModeProtoBuf,
		Payload:       hex.EncodeToString([]byte{0x0A, 0x00}), // 仅空 device_label
		ProtoSchema:   tb19ProtoSchemaPtr(),
		Configuration: &conf,
	}
	resp, err := executeProtobufConverter(req)
	if err != nil {
		t.Fatalf("executeProtobufConverter 返回 error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("空消息应成功解码（空遥测），error=%q", resp.Error)
	}
	if len(resp.Telemetry) != 0 {
		t.Errorf("未出现字段不应产出遥测，实际 %v", resp.Telemetry)
	}
	protobufAssertLogContains(t, resp, "Skipped telemetry")
}

// TestProtobufConverterEmptyMapping 无映射配置时成功返回空遥测（与其它引擎语义一致）。
func TestProtobufConverterEmptyMapping(t *testing.T) {
	req := &model.TestDataConverterReq{
		ConverterMode: model.ConverterModeProtoBuf,
		Payload:       hex.EncodeToString(buildTb19Payload()),
		ProtoSchema:   tb19ProtoSchemaPtr(),
	}
	resp, err := executeProtobufConverter(req)
	if err != nil {
		t.Fatalf("executeProtobufConverter 返回 error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("无映射配置应成功，error=%q", resp.Error)
	}
	if len(resp.Telemetry) != 0 || len(resp.Attributes) != 0 {
		t.Errorf("无映射配置不应产出键值：%v %v", resp.Telemetry, resp.Attributes)
	}
}

// TestExecuteUplinkPipelineProtobuf 管线接线：PROTOBUF 转换器经 ExecuteUplinkDataConverter 执行。
func TestExecuteUplinkPipelineProtobuf(t *testing.T) {
	conf := `{"message_type": "EnvironmentReading", "telemetry": {"temperature": "temperature_c"}}`
	conv := &model.DataConverter{
		ID:            "conv-proto",
		ConverterMode: model.ConverterModeProtoBuf,
		Configuration: conf,
		ProtoSchema:   tb19ProtoSchemaPtr(),
	}
	out, err := ExecuteUplinkDataConverter(conv, map[string]interface{}{
		"payload": hex.EncodeToString(buildTb19Payload()),
	}, map[string]string{"deviceName": "dev-01"})
	if err != nil {
		t.Fatalf("管线执行失败: %v", err)
	}
	if out["temperature"] != 21.5 {
		t.Fatalf("管线输出 temperature = %v，期望 21.5", out["temperature"])
	}

	// 模式为 PROTOBUF 但缺 schema：管线 fail-fast。
	broken := &model.DataConverter{
		ID:            "conv-proto-noschema",
		ConverterMode: model.ConverterModeProtoBuf,
		Configuration: conf,
	}
	if _, err := ExecuteUplinkDataConverter(broken, map[string]interface{}{"payload": "0A00"}, nil); err == nil {
		t.Fatal("缺 proto_schema 的 PROTOBUF 转换器应返回 error")
	}
}

// TestTestDataConverterDispatchProtobuf 服务分派点：ConverterMode=PROTOBUF 走新引擎。
func TestTestDataConverterDispatchProtobuf(t *testing.T) {
	svc := &DataConverterService{}
	resp, err := svc.TestDataConverter(nil, &model.TestDataConverterReq{
		ConverterMode: model.ConverterModeProtoBuf,
		Payload:       hex.EncodeToString(buildTb19Payload()),
		ProtoSchema:   tb19ProtoSchemaPtr(),
		Configuration: tb19FullConfig(),
	}, &utils.UserClaims{TenantID: "t1"})
	if err != nil {
		t.Fatalf("TestDataConverter 返回 error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("分派失败 error=%q", resp.Error)
	}
	if resp.Telemetry["temperature"] != 21.5 {
		t.Errorf("分派后遥测不符：%v", resp.Telemetry)
	}
}

// ---- 断言辅助 ----

// protobufValueEqual 数值通道归一后比较（float64 直接比，切片逐元素比）。
func protobufValueEqual(got, want interface{}) bool {
	switch wantVal := want.(type) {
	case float64:
		gotVal, ok := got.(float64)
		return ok && gotVal == wantVal
	case int64:
		gotVal, ok := got.(int64)
		return ok && gotVal == wantVal
	case []interface{}:
		gotSlice, ok := got.([]interface{})
		if !ok || len(gotSlice) != len(wantVal) {
			return false
		}
		for i := range wantVal {
			if !protobufValueEqual(gotSlice[i], wantVal[i]) {
				return false
			}
		}
		return true
	default:
		return got == want
	}
}

// protobufAssertLogContains 断言日志行中存在包含片段的行。
func protobufAssertLogContains(t *testing.T, resp *model.TestDataConverterResp, fragment string) {
	t.Helper()
	for _, line := range resp.Logs {
		if strings.Contains(line, fragment) {
			return
		}
	}
	t.Errorf("日志未包含 %q；日志=%v", fragment, resp.Logs)
}
