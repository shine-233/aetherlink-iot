// 文件用途：Sparkplug B（Eclipse Sparkplug）v3.0 的话题解析与载荷解码（ROADMAP TB-10）。
//
// 核心逻辑：Sparkplug B 是 MQTT 上的工业互操作规范，由两部分构成——
//  1. 话题命名空间：`spBv1.0/<group_id>/<message_type>/<edge_node_id>[/<device_id>]`
//  2. 载荷：protobuf 编码的 `Payload { timestamp, repeated Metric metrics, seq, uuid, body }`
//
// 本包把这两层解成一个中立的 Go 结构，供 MQTT 上行链路消费；不依赖任何 MQTT 客户端，
// 也不依赖 protoc 生成代码（Sparkplug 的 Payload 是一个字段数很少的稳定消息，
// 手写 wire-format 解码比引入 protobuf 代码生成链更可控，也避免把 protoc 带进构建）。
//
// 关键注意事项（全部 fail closed，宁可报错也不产出"看起来对"的数据）：
//   - 话题不符合命名空间、或 message_type 不在白名单 → 报错。**不**猜。
//   - 未知 wire type / 截断的变长字段 / 非法 UTF-8 → 报错。
//   - `is_null` 为真的指标**不产出数值**，也不产出 0——"空值"与"零"是两种事实。
//   - 未知 datatype 的指标被**跳过**而不是强转成 float，避免把字符串/结构体塞进数值通道。
//   - 本包只做**解码**，不做任何设备身份到内部 ID 的映射（那是调用方的职责）。
//
// 与规范的已知边界（如实记录）：
//   - 只解出 Metric 的标量 oneof（字段 10~16）与公共字段；DataSet/PropertySet/MetaData
//     等嵌套结构**保留原始字节但不展开**——它们不影响遥测数值，展开会显著放大攻击面。
//   - 数组型 datatype（22~30）按"非数值"跳过。
//   - 本实现**未与真实 Sparkplug 设备联调**，验证依据是规范字段号 + 手写字节向量（见测试）。
//
// 重构建议：若后续需要 DataSet / 模板，请为它们单独建类型并复用本文件的 wire reader，
// 不要在本文件里堆嵌套解析。
package sparkplug

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// Namespace Sparkplug B 唯一合法命名空间。
const Namespace = "spBv1.0"

// 消息类型白名单（规范 v3.0）。
const (
	MessageTypeNodeBirth     = "NBIRTH"
	MessageTypeNodeDeath     = "NDEATH"
	MessageTypeDeviceBirth   = "DBIRTH"
	MessageTypeDeviceDeath   = "DDEATH"
	MessageTypeNodeData      = "NDATA"
	MessageTypeDeviceData    = "DDATA"
	MessageTypeNodeCommand   = "NCMD"
	MessageTypeDeviceCommand = "DCMD"
	MessageTypeState         = "STATE"
)

var validMessageTypes = map[string]bool{
	MessageTypeNodeBirth:     true,
	MessageTypeNodeDeath:     true,
	MessageTypeDeviceBirth:   true,
	MessageTypeDeviceDeath:   true,
	MessageTypeNodeData:      true,
	MessageTypeDeviceData:    true,
	MessageTypeNodeCommand:   true,
	MessageTypeDeviceCommand: true,
	MessageTypeState:         true,
}

// 指标数据类型（规范 v3.0 §6.4.2）。
const (
	DataTypeInt8     = 1
	DataTypeInt16    = 2
	DataTypeInt32    = 3
	DataTypeInt64    = 4
	DataTypeUInt8    = 5
	DataTypeUInt16   = 6
	DataTypeUInt32   = 7
	DataTypeUInt64   = 8
	DataTypeFloat    = 9
	DataTypeDouble   = 10
	DataTypeBoolean  = 11
	DataTypeString   = 12
	DataTypeDateTime = 13
	DataTypeText     = 14
	DataTypeUUID     = 15
	DataTypeDataSet  = 16
	DataTypeBytes    = 17
	DataTypeFile     = 18
	DataTypeTemplate = 19
)

var (
	// ErrInvalidTopic 话题不符合 Sparkplug B 命名空间或结构。
	ErrInvalidTopic = errors.New("sparkplug: invalid topic")
	// ErrUnsupportedMessageType message_type 不在规范白名单内。
	ErrUnsupportedMessageType = errors.New("sparkplug: unsupported message type")
	// ErrMalformedPayload protobuf wire-format 非法（截断、非法 wire type、非法 UTF-8）。
	ErrMalformedPayload = errors.New("sparkplug: malformed payload")
)

// Topic 解析后的 Sparkplug B 话题。
type Topic struct {
	Namespace   string
	GroupID     string
	MessageType string
	EdgeNodeID  string
	// DeviceID 仅设备级消息（DBIRTH/DDATA/DDEATH/DCMD）存在；节点级消息为空。
	DeviceID string
}

// IsDeviceLevel 该消息是否针对某个具体设备（而非整个边缘节点）。
func (t Topic) IsDeviceLevel() bool { return t.DeviceID != "" }

// ParseTopic 解析 `spBv1.0/<group>/<type>/<edge>[/<device>]`。
//
// 严格要求：段数必须是 4 或 5、命名空间必须精确等于 `spBv1.0`、message_type 必须在白名单内、
// 各段不得为空。任何一项不满足都报错——**不做大小写归一、不做别名猜测**：
// 工业现场的话题是机器生成的，容错只会把"接错线"变成"看起来接通了"。
func ParseTopic(topic string) (Topic, error) {
	trimmed := strings.TrimSpace(topic)
	if trimmed == "" {
		return Topic{}, fmt.Errorf("%w: empty topic", ErrInvalidTopic)
	}
	// 刻意不用 strings.Split 后再裁掉空段：空段本身即非法，裁掉会掩盖 `a//b` 这类错误。
	segments := strings.Split(trimmed, "/")
	if len(segments) != 4 && len(segments) != 5 {
		return Topic{}, fmt.Errorf("%w: expected 4 or 5 segments, got %d", ErrInvalidTopic, len(segments))
	}
	for index, segment := range segments {
		if segment == "" {
			return Topic{}, fmt.Errorf("%w: segment %d is empty", ErrInvalidTopic, index)
		}
	}

	if segments[0] != Namespace {
		return Topic{}, fmt.Errorf("%w: namespace %q is not %q", ErrInvalidTopic, segments[0], Namespace)
	}
	if !validMessageTypes[segments[2]] {
		return Topic{}, fmt.Errorf("%w: %q", ErrUnsupportedMessageType, segments[2])
	}

	parsed := Topic{
		Namespace:   segments[0],
		GroupID:     segments[1],
		MessageType: segments[2],
		EdgeNodeID:  segments[3],
	}
	if len(segments) == 5 {
		parsed.DeviceID = segments[4]
	}
	return parsed, nil
}

// Metric 单个 Sparkplug 指标。
type Metric struct {
	Name         string
	Alias        uint64
	HasAlias     bool
	Timestamp    uint64
	HasTimestamp bool
	DataType     uint32
	IsHistorical bool
	IsTransient  bool
	IsNull       bool

	// 标量 oneof：按 DataType 决定哪个字段有效。
	IntValue       uint64
	HasIntValue    bool
	FloatValue     float64
	HasFloatValue  bool
	BoolValue      bool
	HasBoolValue   bool
	StringValue    string
	HasStringValue bool
	BytesValue     []byte
	HasBytesValue  bool
}

// NumericValue 返回该指标的数值。
//
// 只有数值型 datatype（整数族 / Float / Double / DateTime）才返回值；
// 布尔、字符串、字节、以及未识别的 datatype 一律返回 ok=false——
// **不把非数值强转成 0**，那会把"类型不对"变成"读数是 0"。
// `is_null` 为真时同样返回 ok=false。
func (m Metric) NumericValue() (float64, bool) {
	if m.IsNull {
		return 0, false
	}
	switch m.DataType {
	case DataTypeInt8, DataTypeInt16, DataTypeInt32, DataTypeInt64,
		DataTypeUInt8, DataTypeUInt16, DataTypeUInt32, DataTypeUInt64,
		DataTypeDateTime:
		if !m.HasIntValue {
			return 0, false
		}
		return float64(m.IntValue), true
	case DataTypeFloat, DataTypeDouble:
		if !m.HasFloatValue {
			return 0, false
		}
		return m.FloatValue, true
	default:
		return 0, false
	}
}

// Payload 解码后的 Sparkplug 载荷。
type Payload struct {
	Timestamp    uint64
	HasTimestamp bool
	Seq          uint64
	HasSeq       bool
	UUID         string
	Body         []byte
	Metrics      []Metric
}

// NumericTelemetry 把载荷里的数值型指标转成 `名称 → 数值`。
//
// 规则：
//   - 名称为空的指标被跳过（Sparkplug 允许纯 alias 上报，但 alias→name 需要会话上下文，
//     本包不持有该上下文，因此不猜）；
//   - 非数值 / is_null / 未知 datatype 被跳过；
//   - 同名指标后者覆盖前者（DDATA 里重复名称属异常上报，取最后一个与规范"按序应用"一致）。
//
// 返回的 map 不含任何非数值键——把字符串塞进遥测通道会让下游分析拿到 NaN 却查不出原因。
func (p Payload) NumericTelemetry() map[string]float64 {
	out := make(map[string]float64, len(p.Metrics))
	for _, metric := range p.Metrics {
		name := strings.TrimSpace(metric.Name)
		if name == "" {
			continue
		}
		if value, ok := metric.NumericValue(); ok {
			out[name] = value
		}
	}
	return out
}

// ---- protobuf wire-format 解码 ----

// wireType 常量。
const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
	wireFixed32 = 5
)

type wireReader struct {
	data []byte
	pos  int
}

func (r *wireReader) done() bool { return r.pos >= len(r.data) }

func (r *wireReader) readVarint() (uint64, error) {
	var result uint64
	var shift uint
	for {
		if r.pos >= len(r.data) {
			return 0, fmt.Errorf("%w: truncated varint", ErrMalformedPayload)
		}
		byteValue := r.data[r.pos]
		r.pos++
		if shift >= 64 {
			return 0, fmt.Errorf("%w: varint overflows 64 bits", ErrMalformedPayload)
		}
		result |= uint64(byteValue&0x7F) << shift
		if byteValue&0x80 == 0 {
			return result, nil
		}
		shift += 7
	}
}

func (r *wireReader) readFixed32() (uint32, error) {
	if r.pos+4 > len(r.data) {
		return 0, fmt.Errorf("%w: truncated fixed32", ErrMalformedPayload)
	}
	value := uint32(r.data[r.pos]) | uint32(r.data[r.pos+1])<<8 |
		uint32(r.data[r.pos+2])<<16 | uint32(r.data[r.pos+3])<<24
	r.pos += 4
	return value, nil
}

func (r *wireReader) readFixed64() (uint64, error) {
	if r.pos+8 > len(r.data) {
		return 0, fmt.Errorf("%w: truncated fixed64", ErrMalformedPayload)
	}
	var value uint64
	for index := 0; index < 8; index++ {
		value |= uint64(r.data[r.pos+index]) << (8 * index)
	}
	r.pos += 8
	return value, nil
}

func (r *wireReader) readBytes() ([]byte, error) {
	length, err := r.readVarint()
	if err != nil {
		return nil, err
	}
	if length > uint64(len(r.data)-r.pos) {
		return nil, fmt.Errorf("%w: length %d exceeds remaining %d bytes", ErrMalformedPayload, length, len(r.data)-r.pos)
	}
	value := r.data[r.pos : r.pos+int(length)]
	r.pos += int(length)
	return value, nil
}

func (r *wireReader) readString() (string, error) {
	raw, err := r.readBytes()
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "", fmt.Errorf("%w: string field is not valid UTF-8", ErrMalformedPayload)
	}
	return string(raw), nil
}

// skipField 跳过未知字段，保证前向兼容（规范未来新增字段不会让整包解码失败）。
func (r *wireReader) skipField(wireType uint64) error {
	switch wireType {
	case wireVarint:
		_, err := r.readVarint()
		return err
	case wireFixed64:
		_, err := r.readFixed64()
		return err
	case wireBytes:
		_, err := r.readBytes()
		return err
	case wireFixed32:
		_, err := r.readFixed32()
		return err
	default:
		return fmt.Errorf("%w: unsupported wire type %d", ErrMalformedPayload, wireType)
	}
}

// DecodePayload 解码 Sparkplug B 的 `Payload` protobuf 消息。
//
// 字段号（规范 v3.0）：1 timestamp(uint64) / 2 metrics(repeated Metric) / 3 seq(uint64) /
// 4 uuid(string) / 5 body(bytes)。未知字段按 wire type 跳过。
func DecodePayload(data []byte) (*Payload, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty payload", ErrMalformedPayload)
	}
	reader := &wireReader{data: data}
	payload := &Payload{}

	for !reader.done() {
		key, err := reader.readVarint()
		if err != nil {
			return nil, err
		}
		fieldNumber := key >> 3
		wireType := key & 0x7

		switch fieldNumber {
		case 1: // timestamp
			value, err := reader.readVarint()
			if err != nil {
				return nil, err
			}
			payload.Timestamp = value
			payload.HasTimestamp = true
		case 2: // metrics
			if wireType != wireBytes {
				return nil, fmt.Errorf("%w: metrics must be length-delimited", ErrMalformedPayload)
			}
			raw, err := reader.readBytes()
			if err != nil {
				return nil, err
			}
			metric, err := decodeMetric(raw)
			if err != nil {
				return nil, err
			}
			payload.Metrics = append(payload.Metrics, *metric)
		case 3: // seq
			value, err := reader.readVarint()
			if err != nil {
				return nil, err
			}
			payload.Seq = value
			payload.HasSeq = true
		case 4: // uuid
			value, err := reader.readString()
			if err != nil {
				return nil, err
			}
			payload.UUID = value
		case 5: // body
			value, err := reader.readBytes()
			if err != nil {
				return nil, err
			}
			payload.Body = append([]byte(nil), value...)
		default:
			if err := reader.skipField(wireType); err != nil {
				return nil, err
			}
		}
	}
	return payload, nil
}

// decodeMetric 解码单个 Metric。
//
// 字段号（规范 v3.0）：1 name / 2 alias / 3 timestamp / 4 datatype / 5 is_historical /
// 6 is_transient / 7 is_null / 8 metadata / 9 properties /
// 10 int_value / 11 long_value / 12 float_value / 13 double_value /
// 14 boolean_value / 15 string_value / 16 bytes_value。
//
// 8 / 9 与 17+ 的嵌套结构**只跳过不展开**：它们不承载遥测数值，展开只会放大攻击面。
func decodeMetric(data []byte) (*Metric, error) {
	reader := &wireReader{data: data}
	metric := &Metric{}

	for !reader.done() {
		key, err := reader.readVarint()
		if err != nil {
			return nil, err
		}
		fieldNumber := key >> 3
		wireType := key & 0x7

		switch fieldNumber {
		case 1:
			value, err := reader.readString()
			if err != nil {
				return nil, err
			}
			metric.Name = value
		case 2:
			value, err := reader.readVarint()
			if err != nil {
				return nil, err
			}
			metric.Alias = value
			metric.HasAlias = true
		case 3:
			value, err := reader.readVarint()
			if err != nil {
				return nil, err
			}
			metric.Timestamp = value
			metric.HasTimestamp = true
		case 4:
			value, err := reader.readVarint()
			if err != nil {
				return nil, err
			}
			metric.DataType = uint32(value)
		case 5:
			value, err := reader.readVarint()
			if err != nil {
				return nil, err
			}
			metric.IsHistorical = value != 0
		case 6:
			value, err := reader.readVarint()
			if err != nil {
				return nil, err
			}
			metric.IsTransient = value != 0
		case 7:
			value, err := reader.readVarint()
			if err != nil {
				return nil, err
			}
			metric.IsNull = value != 0
		case 10, 11: // int_value / long_value
			value, err := reader.readVarint()
			if err != nil {
				return nil, err
			}
			metric.IntValue = value
			metric.HasIntValue = true
		case 12: // float_value（32 位）
			if wireType != wireFixed32 {
				return nil, fmt.Errorf("%w: float_value must be 32-bit", ErrMalformedPayload)
			}
			raw, err := reader.readFixed32()
			if err != nil {
				return nil, err
			}
			metric.FloatValue = float64(math.Float32frombits(raw))
			metric.HasFloatValue = true
		case 13: // double_value（64 位）
			if wireType != wireFixed64 {
				return nil, fmt.Errorf("%w: double_value must be 64-bit", ErrMalformedPayload)
			}
			raw, err := reader.readFixed64()
			if err != nil {
				return nil, err
			}
			metric.FloatValue = math.Float64frombits(raw)
			metric.HasFloatValue = true
		case 14: // boolean_value
			value, err := reader.readVarint()
			if err != nil {
				return nil, err
			}
			metric.BoolValue = value != 0
			metric.HasBoolValue = true
		case 15: // string_value
			value, err := reader.readString()
			if err != nil {
				return nil, err
			}
			metric.StringValue = value
			metric.HasStringValue = true
		case 16: // bytes_value
			value, err := reader.readBytes()
			if err != nil {
				return nil, err
			}
			metric.BytesValue = append([]byte(nil), value...)
			metric.HasBytesValue = true
		default:
			if err := reader.skipField(wireType); err != nil {
				return nil, err
			}
		}
	}
	return metric, nil
}
