// 文件用途：把 DataConverter 转换引擎暴露为上行管线执行入口（TB-45 Integration 纳管接线）。
// 核心逻辑：以采集回填的遥测键值为载荷，按转换器模式（HEX_BINARY/JSON_PATH/SCRIPT/PROTOBUF）分派
//
//	到既有引擎执行，成功时输出转换后遥测；Success=false 或空输出按失败返回 error。
//
// 关键注意事项：本函数是采集器管线与转换器引擎之间唯一的桥接面——转换失败由调用方
// （collector.IntegrationResolver）丢弃并计数，绝不向上阻断采集循环；返回 nil map 视为失败。
// 重构建议：Downlink 反向下发执行落地时，在本文件对偶补 ExecuteDownlinkDataConverter。
package service

import (
	"encoding/json"
	"fmt"

	model "aetherlink-iot/backend/internal/model"
)

// ExecuteUplinkDataConverter 以采集遥测键值为输入执行上行转换器。
// 返回转换后的遥测键值；失败（模式不支持/引擎报错/无有效输出）返回 error，由管线丢弃。
// 载荷语义：JSON_PATH/SCRIPT 收到整个遥测键值的 JSON 文档；HEX_BINARY 收到原始 hex 串、
// PROTOBUF 收到 hex/base64 编码串（两者均取自遥测中的唯一字符串值，优先键 raw/payload/data
// ——二进制解码引擎不吃 JSON 文档）；PROTOBUF 模式的 proto_schema 取转换器档案列。
func ExecuteUplinkDataConverter(conv *model.DataConverter, values map[string]interface{}, metadata map[string]string) (map[string]interface{}, error) {
	if conv == nil {
		return nil, fmt.Errorf("converter is nil")
	}

	// 二进制模式只取原始编码串，JSON 文档仅在 JSON_PATH/SCRIPT 等模式需要：
	// 按模式二选一，避免每条上行消息对二进制模式做一次随即丢弃的 json.Marshal。
	var (
		payload []byte
		err     error
	)
	if conv.ConverterMode == "HEX_BINARY" || conv.ConverterMode == model.ConverterModeProtoBuf {
		payload, err = extractRawPayload(values)
		if err != nil {
			return nil, err
		}
	} else {
		payload, err = json.Marshal(values)
		if err != nil {
			return nil, fmt.Errorf("marshal telemetry payload: %w", err)
		}
	}

	req := &model.TestDataConverterReq{
		Type:          "UPLINK",
		ConverterMode: conv.ConverterMode,
		Payload:       string(payload),
		Metadata:      metadata,
		Configuration: &conv.Configuration,
		Script:        conv.Script,
		ProtoSchema:   conv.ProtoSchema,
	}

	var resp *model.TestDataConverterResp
	switch conv.ConverterMode {
	case "HEX_BINARY":
		resp, err = executeHexBinaryConverter(req)
	case "JSON_PATH":
		resp, err = executeJsonPathConverter(req)
	case "SCRIPT":
		resp, err = executeScriptConverter(req)
	case model.ConverterModeProtoBuf:
		resp, err = executeProtobufConverter(req)
	default:
		return nil, fmt.Errorf("unsupported converter mode: %s", conv.ConverterMode)
	}
	if err != nil {
		return nil, err
	}
	if resp == nil || !resp.Success {
		msg := "converter execution failed"
		if resp != nil && resp.Error != "" {
			msg = resp.Error
		}
		return nil, fmt.Errorf("%s", msg)
	}

	// 输出语义：telemetry 为主输出；attributes 并入同一键值面（键冲突以 telemetry 为准）。
	out := make(map[string]interface{}, len(resp.Telemetry)+len(resp.Attributes))
	for k, v := range resp.Telemetry {
		out[k] = v
	}
	for k, v := range resp.Attributes {
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("converter produced no telemetry output")
	}
	return out, nil
}

// extractRawPayload 从遥测键值中提取二进制解码引擎（HEX_BINARY/PROTOBUF）所需的原始编码串：
// 优先取 raw/payload/data 键的字符串值；否则仅当恰好存在一个字符串值时取之；
// 多个或零个字符串值视为失败（歧义载荷不猜，返回 error 由管线丢弃）。
func extractRawPayload(values map[string]interface{}) ([]byte, error) {
	for _, key := range []string{"raw", "payload", "data"} {
		if s, ok := values[key].(string); ok {
			return []byte(s), nil
		}
	}
	var candidates []string
	for _, v := range values {
		if s, ok := v.(string); ok {
			candidates = append(candidates, s)
		}
	}
	if len(candidates) == 1 {
		return []byte(candidates[0]), nil
	}
	return nil, fmt.Errorf("binary converter requires exactly one raw string telemetry value (got %d)", len(candidates))
}
