// 文件用途：PROTOBUF 转换引擎（TB-19）——动态解析 proto_schema 并把二进制载荷解码为遥测键值。
// 核心逻辑：protoparse（jhump/protoreflect）解析 .proto 源 → 按 configuration.message_type
//
//	选消息类型 → 载荷自动识别 hex/base64 编码 → dynamicpb（protobuf-go）反序列化
//	（未知字段容错保留、线型不匹配按缺失跳过）→ 按 configuration.telemetry/attributes
//	的 {输出键:字段路径} 提取值（支持点路径、repeated 下标 samples[0] 与 map 键 labels.zone）。
//
// 关键注意事项：schema 缺失/解析失败/载荷不可解码一律 fail-closed（Success=false）；
//
//	映射路径缺失或线型/类型不符只记日志并跳过该键，绝不臆造值（jhump dynamic.Message
//	会把线型不匹配的字段重新解释成随机值，故解码统一走 dynamicpb——ROADMAP TB-19 指定）；
//	hex 先于 base64 尝试（两者字符集有交集，以日志声明的编码为准）。
//
// 重构建议：如需 Protobuf 下行编码（云端→设备），在此补 Encode 辅助函数并接入 downlink 管线。
package service

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	model "aetherlink-iot/backend/internal/model"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// protobufVirtualFileName 解析 proto_schema 时使用的虚拟文件名（内容来自内存，仅作标识）。
const protobufVirtualFileName = "converter.proto"

// ProtoBufConfig PROTOBUF 转换模式配置（configuration 列的 JSON 结构）。
// Telemetry/Attributes 为 {输出遥测键: 消息内字段路径}；字段路径支持点分嵌套、
// repeated 下标（samples[0]）与 map 键（labels.zone）。
type ProtoBufConfig struct {
	MessageType string            `json:"message_type,omitempty"` // 消息类型名（支持短名/包限定名）；空则取文件首个 message
	DeviceName  string            `json:"device_name,omitempty"`  // 设备名字段路径；空则回退 metadata.deviceName
	DeviceType  string            `json:"device_type,omitempty"`  // 设备类型字段路径；空则回退 metadata.deviceType
	Telemetry   map[string]string `json:"telemetry,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

// executeProtobufConverter Dry-Run/管线共用的 PROTOBUF 解码引擎。
// 与 HEX_BINARY/JSON_PATH 引擎同构：失败写 resp.Success=false + resp.Error，不返回 error。
func executeProtobufConverter(req *model.TestDataConverterReq) (*model.TestDataConverterResp, error) {
	resp := &model.TestDataConverterResp{
		Success:    true,
		Telemetry:  make(map[string]interface{}),
		Attributes: make(map[string]interface{}),
		Logs:       []string{},
	}

	// 1. proto_schema 必须非空（fail-closed：不猜测默认 schema）。
	schema := ""
	if req.ProtoSchema != nil {
		schema = *req.ProtoSchema
	}
	if strings.TrimSpace(schema) == "" {
		resp.Success = false
		resp.Error = "proto_schema is empty: PROTOBUF converter mode requires a .proto source"
		return resp, nil
	}

	// 2. 解析 .proto 源为文件描述符（jhump/protoreflect 动态解析）。
	fileDesc, err := parseProtoSchema(schema)
	if err != nil {
		resp.Success = false
		resp.Error = "invalid proto schema: " + err.Error()
		return resp, nil
	}

	// 3. 解析 configuration（缺省视为无映射配置）。
	var conf ProtoBufConfig
	configStr := "{}"
	if req.Configuration != nil {
		configStr = *req.Configuration
	}
	if strings.TrimSpace(configStr) != "" {
		if jsonErr := json.Unmarshal([]byte(configStr), &conf); jsonErr != nil {
			resp.Success = false
			resp.Error = "invalid protobuf configuration: " + jsonErr.Error()
			return resp, nil
		}
	}

	// 4. 选择消息类型：显式 message_type 优先（容忍短名/带包名/前导点，找不到即 fail-closed），
	//    未配置时回退文件中第一个 message 并记日志。
	msgDesc, logLines := selectProtoMessageDescriptor(fileDesc, conf.MessageType)
	resp.Logs = append(resp.Logs, logLines...)
	if msgDesc == nil {
		resp.Success = false
		resp.Error = fmt.Sprintf("message type %q not found in proto schema", conf.MessageType)
		return resp, nil
	}
	msgFQN := msgDesc.GetFullyQualifiedName()
	resp.Logs = append(resp.Logs, "Using message type: "+msgFQN)

	// 5. 载荷解码：hex/base64 自动识别 → 二进制。
	rawBytes, encoding, decodeErr := decodeProtoPayload(req.Payload)
	if decodeErr != nil {
		resp.Success = false
		resp.Error = decodeErr.Error()
		return resp, nil
	}
	resp.Logs = append(resp.Logs, fmt.Sprintf("Decoded %d bytes from %s payload", len(rawBytes), encoding))

	// 6. dynamicpb 反序列化：未知字段/线型不匹配数据由 protobuf-go 保留进 unknown 区，
	//    不会失败；字段缺席由路径解析层按"缺失"跳过（fail-closed，不臆造值）。
	msgDescV2 := msgDesc.UnwrapMessage()
	msg := dynamicpb.NewMessage(msgDescV2)
	if unmarshalErr := proto.Unmarshal(rawBytes, msg); unmarshalErr != nil {
		resp.Success = false
		resp.Error = "protobuf decode failed: " + unmarshalErr.Error()
		return resp, nil
	}
	reflectMsg := msg.ProtoReflect()
	if len(reflectMsg.GetUnknown()) > 0 {
		resp.Logs = append(resp.Logs, fmt.Sprintf("Payload contains %d byte(s) of unknown/unmatched field data, tolerated and preserved",
			len(reflectMsg.GetUnknown())))
	}

	// 7. 设备名/设备类型：配置路径优先，回退 metadata（与 JSON_PATH 引擎语义一致）。
	if conf.DeviceName != "" {
		if v, _, ok := resolveProtoFieldPath(reflectMsg, conf.DeviceName); ok {
			resp.DeviceName = fmt.Sprintf("%v", convertProtoFieldValue(nil, v))
		}
	} else if req.Metadata != nil {
		resp.DeviceName = req.Metadata["deviceName"]
	}
	if conf.DeviceType != "" {
		if v, _, ok := resolveProtoFieldPath(reflectMsg, conf.DeviceType); ok {
			resp.DeviceType = fmt.Sprintf("%v", convertProtoFieldValue(nil, v))
		}
	} else if req.Metadata != nil {
		resp.DeviceType = req.Metadata["deviceType"]
	}

	// 8. telemetry/attributes 映射提取：路径缺失仅记日志跳过（不臆造值）。
	for targetKey, path := range conf.Telemetry {
		if v, fd, ok := resolveProtoFieldPath(reflectMsg, path); ok {
			resp.Telemetry[targetKey] = convertProtoFieldValue(fd, v)
			resp.Logs = append(resp.Logs, fmt.Sprintf("Mapped telemetry: %s <- %s = %v", targetKey, path, resp.Telemetry[targetKey]))
		} else {
			resp.Logs = append(resp.Logs, fmt.Sprintf("Skipped telemetry %s: field path %q absent or type mismatch", targetKey, path))
		}
	}
	for targetKey, path := range conf.Attributes {
		if v, fd, ok := resolveProtoFieldPath(reflectMsg, path); ok {
			resp.Attributes[targetKey] = convertProtoFieldValue(fd, v)
			resp.Logs = append(resp.Logs, fmt.Sprintf("Mapped attribute: %s <- %s = %v", targetKey, path, resp.Attributes[targetKey]))
		} else {
			resp.Logs = append(resp.Logs, fmt.Sprintf("Skipped attribute %s: field path %q absent or type mismatch", targetKey, path))
		}
	}

	return resp, nil
}

// parseProtoSchema 用 protoparse 在内存中解析 .proto 源文件全文。
func parseProtoSchema(schema string) (*desc.FileDescriptor, error) {
	parser := &protoparse.Parser{
		Accessor: protoparse.FileContentsFromMap(map[string]string{protobufVirtualFileName: schema}),
	}
	files, err := parser.ParseFiles(protobufVirtualFileName)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("proto schema parsed to no file descriptors")
	}
	return files[0], nil
}

// selectProtoMessageDescriptor 选择消息类型：显式名按多形态候选解析（找不到返回 nil，
// 由调用方 fail-closed）；未配置回退首个 message。第二个值是需要追加到日志的提示行。
func selectProtoMessageDescriptor(fileDesc *desc.FileDescriptor, messageTypeName string) (*desc.MessageDescriptor, []string) {
	var logs []string
	if strings.TrimSpace(messageTypeName) != "" {
		candidates := []string{messageTypeName, "." + messageTypeName}
		if pkg := fileDesc.GetPackage(); pkg != "" {
			candidates = append(candidates, pkg+"."+messageTypeName, "."+pkg+"."+messageTypeName)
		}
		seen := make(map[string]bool, len(candidates))
		for _, candidate := range candidates {
			if seen[candidate] {
				continue
			}
			seen[candidate] = true
			if md := fileDesc.FindMessage(candidate); md != nil {
				return md, logs
			}
		}
		return nil, logs
	}

	msgTypes := fileDesc.GetMessageTypes()
	if len(msgTypes) == 0 {
		return nil, logs
	}
	logs = append(logs, fmt.Sprintf("No message_type configured, using first message: %s", msgTypes[0].GetFullyQualifiedName()))
	return msgTypes[0], logs
}

// decodeProtoPayload 识别载荷编码并解码为二进制。
// 识别顺序：hex（允许 0x/0X 前缀与空白，偶数长度十六进制）→ base64（Std/RawStd/URL/RawURL）。
// 两者字符集有交集，hex 优先；返回所用编码名供日志声明。
func decodeProtoPayload(payload string) ([]byte, string, error) {
	cleaned := strings.TrimSpace(payload)
	if cleaned == "" {
		return nil, "", fmt.Errorf("invalid protobuf payload: payload is empty")
	}
	cleaned = strings.TrimPrefix(cleaned, "0x")
	cleaned = strings.TrimPrefix(cleaned, "0X")
	cleaned = strings.ReplaceAll(cleaned, " ", "")

	if isHexString(cleaned) {
		if raw, hexErr := hex.DecodeString(cleaned); hexErr == nil {
			return raw, "hex", nil
		}
	}
	encodings := []struct {
		name string
		enc  *base64.Encoding
	}{
		{"base64", base64.StdEncoding},
		{"base64(raw)", base64.RawStdEncoding},
		{"base64url", base64.URLEncoding},
		{"base64url(raw)", base64.RawURLEncoding},
	}
	for _, candidate := range encodings {
		if raw, b64Err := candidate.enc.DecodeString(cleaned); b64Err == nil && len(raw) > 0 {
			return raw, candidate.name, nil
		}
	}
	return nil, "", fmt.Errorf("invalid protobuf payload: not a valid hex or base64 encoded binary")
}

// isHexString 判断字符串是否为偶数长度的纯十六进制字符。
func isHexString(s string) bool {
	if len(s) == 0 || len(s)%2 != 0 {
		return false
	}
	for _, ch := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", ch) {
			return false
		}
	}
	return true
}

// lookupProtoFieldDescriptor 按字段名（proto 名或 JSON 名）在消息描述符上查找字段。
func lookupProtoFieldDescriptor(md protoreflect.MessageDescriptor, name string) protoreflect.FieldDescriptor {
	if fd := md.Fields().ByName(protoreflect.Name(name)); fd != nil {
		return fd
	}
	return md.Fields().ByJSONName(name)
}

// resolveProtoFieldPath 在动态消息上解析字段路径。
// 支持：点分嵌套（meta.zone）、repeated 下标（samples[2]）、map 键（labels.zone）。
// 返回值携带产出该值的字段描述符（map 键取值为 nil），供枚举名等类型感知转换；
// 第三个返回值为 false 表示路径缺失/越界/线型不匹配（调用方应跳过而非报错）。
func resolveProtoFieldPath(root protoreflect.Message, path string) (protoreflect.Value, protoreflect.FieldDescriptor, bool) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	currentNode := root
	var mapNode protoreflect.Map // 非空表示下一层按 map 键解析
	var current protoreflect.Value
	var currentFD protoreflect.FieldDescriptor

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return protoreflect.Value{}, nil, false
		}
		fieldName, index, hasIndex := splitProtoIndexSuffix(part)

		// 上一层落在 map 字段上：本段是键查找（遍历比较键的字符串形态，避免按 Kind 构造 MapKey）。
		if mapNode != nil {
			if hasIndex {
				return protoreflect.Value{}, nil, false
			}
			value, found := lookupProtoMapValue(mapNode, fieldName)
			if !found {
				return protoreflect.Value{}, nil, false
			}
			current, currentFD, mapNode = value, nil, nil
			continue
		}

		fd := lookupProtoFieldDescriptor(currentNode.Descriptor(), fieldName)
		if fd == nil {
			return protoreflect.Value{}, nil, false
		}
		// Has 覆盖三类缺失：字段未出现在 wire 数据、proto3 零值、
		// 线型不匹配被归入 unknown（fail-closed：不臆造重解释值）。
		if !currentNode.Has(fd) {
			return protoreflect.Value{}, nil, false
		}
		value := currentNode.Get(fd)
		current, currentFD = value, fd

		switch {
		case fd.IsMap():
			if hasIndex {
				return protoreflect.Value{}, nil, false
			}
			mapNode = value.Map()
		case fd.Cardinality() == protoreflect.Repeated:
			if hasIndex {
				list := value.List()
				if index < 0 || index >= list.Len() {
					return protoreflect.Value{}, nil, false
				}
				current = list.Get(index)
				if kind := fd.Kind(); kind == protoreflect.MessageKind || kind == protoreflect.GroupKind {
					currentNode = current.Message()
				}
			}
		case fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind:
			if hasIndex {
				return protoreflect.Value{}, nil, false
			}
			currentNode = value.Message()
		default:
			if hasIndex {
				return protoreflect.Value{}, nil, false
			}
		}
	}
	return current, currentFD, true
}

// lookupProtoMapValue 在 map 字段中按键的字符串形态查找值。
func lookupProtoMapValue(m protoreflect.Map, key string) (protoreflect.Value, bool) {
	var found protoreflect.Value
	hit := false
	m.Range(func(mapKey protoreflect.MapKey, val protoreflect.Value) bool {
		if fmt.Sprintf("%v", mapKey.Value().Interface()) == key {
			found, hit = val, true
			return false
		}
		return true
	})
	return found, hit
}

// splitProtoIndexSuffix 拆分形如 samples[2] 的路径段为字段名与下标。
func splitProtoIndexSuffix(part string) (string, int, bool) {
	open := strings.IndexByte(part, '[')
	if open < 0 {
		return part, 0, false
	}
	if !strings.HasSuffix(part, "]") {
		return part, 0, false
	}
	indexStr := part[open+1 : len(part)-1]
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return part, 0, false
	}
	return part[:open], index, true
}

// convertProtoFieldValue 把 protoreflect 值规范化为 JSON 友好的 Go 值。
// 枚举转符号名（查不到则保留数值）；bytes 转 hex 字符串；float32 归一为 float64；
// 嵌套消息递归为 map；fd 为 nil 时（如设备名路径取标量）跳过枚举解析。
func convertProtoFieldValue(fd protoreflect.FieldDescriptor, value protoreflect.Value) interface{} {
	if fd != nil && fd.Kind() == protoreflect.EnumKind {
		if enumNumber, ok := value.Interface().(protoreflect.EnumNumber); ok {
			if enumValue := fd.Enum().Values().ByNumber(enumNumber); enumValue != nil {
				return string(enumValue.Name())
			}
			return int64(enumNumber)
		}
	}

	switch typed := value.Interface().(type) {
	case protoreflect.Message:
		return protoreflectMessageToMap(typed)
	case protoreflect.List:
		out := make([]interface{}, typed.Len())
		for i := 0; i < typed.Len(); i++ {
			out[i] = convertProtoFieldValue(fd, typed.Get(i))
		}
		return out
	case protoreflect.Map:
		out := make(map[string]interface{}, typed.Len())
		valueFD := fd.MapValue()
		typed.Range(func(key protoreflect.MapKey, val protoreflect.Value) bool {
			out[fmt.Sprintf("%v", key.Value().Interface())] = convertProtoFieldValue(valueFD, val)
			return true
		})
		return out
	case int32:
		return int64(typed)
	case uint32:
		return uint64(typed)
	case float32:
		return float64(typed)
	case []byte:
		return hex.EncodeToString(typed)
	default:
		return value.Interface()
	}
}

// protoreflectMessageToMap 把已出现字段递归转为 map（未出现字段不产出键）。
func protoreflectMessageToMap(message protoreflect.Message) map[string]interface{} {
	out := make(map[string]interface{})
	fields := message.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if !message.Has(fd) {
			continue
		}
		out[string(fd.Name())] = convertProtoFieldValue(fd, message.Get(fd))
	}
	return out
}
