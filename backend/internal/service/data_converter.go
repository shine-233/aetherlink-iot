// 文件用途：实现数据转换器服务与解析执行引擎（ThingsBoard Data Converter 载荷解析）。
// 核心逻辑：提供 UPLINK/DOWNLINK 转换器的 CRUD、仿真测试（Hex/JSONPath/Lua）与运行期载荷清洗。
package service

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

type DataConverterService struct{}

// CreateDataConverter 创建转换器
func (*DataConverterService) CreateDataConverter(ctx context.Context, req *model.CreateDataConverterReq, claims *utils.UserClaims) (*model.DataConverter, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}

	configStr := "{}"
	if req.Configuration != nil && strings.TrimSpace(*req.Configuration) != "" {
		configStr = *req.Configuration
	}

	now := time.Now().UTC()
	converter := &model.DataConverter{
		ID:            uuid.New(),
		Name:          req.Name,
		Type:          req.Type,
		ConverterMode: req.ConverterMode,
		DebugMode:     req.DebugMode,
		TenantID:      claims.TenantID,
		Configuration: configStr,
		Script:        req.Script,
		Description:   req.Description,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}

	if err := dal.CreateDataConverter(converter); err != nil {
		logrus.Errorf("failed to create data converter: %v", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return converter, nil
}

// UpdateDataConverter 更新转换器
func (*DataConverterService) UpdateDataConverter(ctx context.Context, req *model.UpdateDataConverterReq, claims *utils.UserClaims) (*model.DataConverter, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}

	record, err := dal.GetDataConverterByID(req.ID, claims.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "data converter not found")
	}

	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		record.Name = *req.Name
	}
	if req.Type != nil {
		record.Type = *req.Type
	}
	if req.ConverterMode != nil {
		record.ConverterMode = *req.ConverterMode
	}
	if req.DebugMode != nil {
		record.DebugMode = *req.DebugMode
	}
	if req.Configuration != nil {
		record.Configuration = *req.Configuration
	}
	if req.Script != nil {
		record.Script = req.Script
	}
	if req.Description != nil {
		record.Description = req.Description
	}

	now := time.Now().UTC()
	record.UpdatedAt = &now

	if err := dal.UpdateDataConverter(record); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return record, nil
}

// GetDataConverterByID 查询单个转换器详情
func (*DataConverterService) GetDataConverterByID(ctx context.Context, id string, claims *utils.UserClaims) (*model.DataConverter, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	record, err := dal.GetDataConverterByID(id, claims.TenantID)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "data converter not found")
	}
	return record, nil
}

// DeleteDataConverter 删除转换器
func (*DataConverterService) DeleteDataConverter(ctx context.Context, id string, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	if _, err := dal.GetDataConverterByID(id, claims.TenantID); err != nil {
		return errcode.NewWithMessage(errcode.CodeNotFound, "data converter not found")
	}
	return dal.DeleteDataConverter(id, claims.TenantID)
}

// ListDataConverters 分页查询列表
func (*DataConverterService) ListDataConverters(ctx context.Context, req *model.GetDataConverterListReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}
	total, list, err := dal.ListDataConverters(req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	res := make(map[string]interface{})
	res["total"] = total
	res["list"] = list
	return res, nil
}

// TestDataConverter 运行仿真测试
func (*DataConverterService) TestDataConverter(ctx context.Context, req *model.TestDataConverterReq, claims *utils.UserClaims) (*model.TestDataConverterResp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	}

	if req.ConverterID != nil && *req.ConverterID != "" {
		conv, err := dal.GetDataConverterByID(*req.ConverterID, claims.TenantID)
		if err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "data converter not found")
		}
		if req.ConverterMode == "" {
			req.ConverterMode = conv.ConverterMode
		}
		if req.Type == "" {
			req.Type = conv.Type
		}
		if req.Configuration == nil || *req.Configuration == "" {
			req.Configuration = &conv.Configuration
		}
		if req.Script == nil || *req.Script == "" {
			req.Script = conv.Script
		}
	}

	if req.Type == "" {
		req.Type = "UPLINK"
	}

	switch req.ConverterMode {
	case "HEX_BINARY":
		return executeHexBinaryConverter(req)
	case "JSON_PATH":
		return executeJsonPathConverter(req)
	case "SCRIPT":
		return executeScriptConverter(req)
	default:
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "unsupported converter mode: "+req.ConverterMode)
	}
}

// ---- Hex 进制偏移量解码引擎 ----
type HexFieldRule struct {
	Key          string  `json:"key"`
	Offset       int     `json:"offset"`
	Length       int     `json:"length"`
	Type         string  `json:"type"` // uint8, int8, uint16, int16, uint32, int32, float32, float64, bool, string
	Scale        float64 `json:"scale,omitempty"`
	LittleEndian bool    `json:"little_endian,omitempty"`
}

func executeHexBinaryConverter(req *model.TestDataConverterReq) (*model.TestDataConverterResp, error) {
	resp := &model.TestDataConverterResp{
		Success:    true,
		Telemetry:  make(map[string]interface{}),
		Attributes: make(map[string]interface{}),
		Logs:       []string{},
	}

	cleanHex := strings.TrimSpace(req.Payload)
	cleanHex = strings.TrimPrefix(cleanHex, "0x")
	cleanHex = strings.TrimPrefix(cleanHex, "0X")
	cleanHex = strings.ReplaceAll(cleanHex, " ", "")

	rawBytes, err := hex.DecodeString(cleanHex)
	if err != nil {
		resp.Success = false
		resp.Error = "invalid hex payload: " + err.Error()
		return resp, nil
	}
	resp.Logs = append(resp.Logs, fmt.Sprintf("Decoded %d bytes from hex string", len(rawBytes)))

	configStr := "{}"
	if req.Configuration != nil {
		configStr = *req.Configuration
	}

	var rules []HexFieldRule
	if err := json.Unmarshal([]byte(configStr), &rules); err != nil {
		// 允许包裹在 {"fields": [...]} 或 {"mappings": [...]} 或 {"rules": [...]}
		var wrapped struct {
			Fields   []HexFieldRule `json:"fields"`
			Mappings []HexFieldRule `json:"mappings"`
			Rules    []HexFieldRule `json:"rules"`
		}
		if wrapErr := json.Unmarshal([]byte(configStr), &wrapped); wrapErr == nil {
			if len(wrapped.Fields) > 0 {
				rules = wrapped.Fields
			} else if len(wrapped.Mappings) > 0 {
				rules = wrapped.Mappings
			} else if len(wrapped.Rules) > 0 {
				rules = wrapped.Rules
			}
		}
		if len(rules) == 0 {
			resp.Success = false
			resp.Error = "invalid hex rules configuration: " + err.Error()
			return resp, nil
		}
	}

	for _, rule := range rules {
		if rule.Offset < 0 || rule.Offset+rule.Length > len(rawBytes) {
			resp.Logs = append(resp.Logs, fmt.Sprintf("Skipping %s: offset %d length %d exceeds byte length %d", rule.Key, rule.Offset, rule.Length, len(rawBytes)))
			continue
		}
		slice := rawBytes[rule.Offset : rule.Offset+rule.Length]
		val, parseErr := decodeByteSlice(slice, rule.Type, rule.LittleEndian)
		if parseErr != nil {
			resp.Logs = append(resp.Logs, fmt.Sprintf("Error decoding %s: %v", rule.Key, parseErr))
			continue
		}

		if rule.Scale != 0 {
			if num, ok := toFloatNum(val); ok {
				scaled := num * rule.Scale
				val = roundFloat(scaled, 4)
			}
		}

		resp.Telemetry[rule.Key] = val
		resp.Logs = append(resp.Logs, fmt.Sprintf("Parsed %s = %v (type=%s)", rule.Key, val, rule.Type))
	}

	if req.Metadata != nil {
		if name, ok := req.Metadata["deviceName"]; ok {
			resp.DeviceName = name
		}
		if devType, ok := req.Metadata["deviceType"]; ok {
			resp.DeviceType = devType
		}
	}

	return resp, nil
}

func decodeByteSlice(b []byte, typeName string, littleEndian bool) (interface{}, error) {
	var bo binary.ByteOrder = binary.BigEndian
	if littleEndian {
		bo = binary.LittleEndian
	}

	switch strings.ToLower(typeName) {
	case "uint8":
		if len(b) < 1 {
			return nil, fmt.Errorf("need 1 byte")
		}
		return uint64(b[0]), nil
	case "int8":
		if len(b) < 1 {
			return nil, fmt.Errorf("need 1 byte")
		}
		return int64(int8(b[0])), nil
	case "uint16":
		if len(b) < 2 {
			return nil, fmt.Errorf("need 2 bytes")
		}
		return uint64(bo.Uint16(b)), nil
	case "int16":
		if len(b) < 2 {
			return nil, fmt.Errorf("need 2 bytes")
		}
		return int64(int16(bo.Uint16(b))), nil
	case "uint32":
		if len(b) < 4 {
			return nil, fmt.Errorf("need 4 bytes")
		}
		return uint64(bo.Uint32(b)), nil
	case "int32":
		if len(b) < 4 {
			return nil, fmt.Errorf("need 4 bytes")
		}
		return int64(int32(bo.Uint32(b))), nil
	case "float32":
		if len(b) < 4 {
			return nil, fmt.Errorf("need 4 bytes")
		}
		bits := bo.Uint32(b)
		return float64(math.Float32frombits(bits)), nil
	case "float64":
		if len(b) < 8 {
			return nil, fmt.Errorf("need 8 bytes")
		}
		bits := bo.Uint64(b)
		return math.Float64frombits(bits), nil
	case "bool":
		if len(b) < 1 {
			return nil, fmt.Errorf("need 1 byte")
		}
		return b[0] != 0, nil
	case "string":
		return string(b), nil
	default:
		return hex.EncodeToString(b), nil
	}
}

func toFloatNum(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

// ---- JSONPath / 映射引擎 ----
type JsonPathConfig struct {
	DeviceName string            `json:"device_name,omitempty"`
	DeviceType string            `json:"device_type,omitempty"`
	Telemetry  map[string]string `json:"telemetry,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func executeJsonPathConverter(req *model.TestDataConverterReq) (*model.TestDataConverterResp, error) {
	resp := &model.TestDataConverterResp{
		Success:    true,
		Telemetry:  make(map[string]interface{}),
		Attributes: make(map[string]interface{}),
		Logs:       []string{},
	}

	var payloadMap map[string]interface{}
	if err := json.Unmarshal([]byte(req.Payload), &payloadMap); err != nil {
		resp.Success = false
		resp.Error = "invalid JSON payload: " + err.Error()
		return resp, nil
	}

	configStr := "{}"
	if req.Configuration != nil {
		configStr = *req.Configuration
	}

	var conf JsonPathConfig
	if err := json.Unmarshal([]byte(configStr), &conf); err != nil {
		resp.Success = false
		resp.Error = "invalid JSONPath configuration: " + err.Error()
		return resp, nil
	}

	if conf.DeviceName != "" {
		if v, ok := getNestedValue(payloadMap, conf.DeviceName); ok {
			resp.DeviceName = fmt.Sprintf("%v", v)
		}
	} else if req.Metadata != nil {
		resp.DeviceName = req.Metadata["deviceName"]
	}

	if conf.DeviceType != "" {
		if v, ok := getNestedValue(payloadMap, conf.DeviceType); ok {
			resp.DeviceType = fmt.Sprintf("%v", v)
		}
	} else if req.Metadata != nil {
		resp.DeviceType = req.Metadata["deviceType"]
	}

	for targetKey, path := range conf.Telemetry {
		if v, ok := getNestedValue(payloadMap, path); ok {
			resp.Telemetry[targetKey] = v
			resp.Logs = append(resp.Logs, fmt.Sprintf("Mapped telemetry: %s -> %v", targetKey, v))
		}
	}

	for targetKey, path := range conf.Attributes {
		if v, ok := getNestedValue(payloadMap, path); ok {
			resp.Attributes[targetKey] = v
			resp.Logs = append(resp.Logs, fmt.Sprintf("Mapped attribute: %s -> %v", targetKey, v))
		}
	}

	return resp, nil
}

func getNestedValue(data map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var current interface{} = data
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if m, ok := current.(map[string]interface{}); ok {
			val, exists := m[part]
			if !exists {
				return nil, false
			}
			current = val
		} else {
			return nil, false
		}
	}
	return current, true
}

// ---- Script 动态脚本转换引擎 ----
func executeScriptConverter(req *model.TestDataConverterReq) (*model.TestDataConverterResp, error) {
	resp := &model.TestDataConverterResp{
		Success:    true,
		Telemetry:  make(map[string]interface{}),
		Attributes: make(map[string]interface{}),
		Logs:       []string{},
	}

	scriptContent := ""
	if req.Script != nil {
		scriptContent = *req.Script
	}
	if strings.TrimSpace(scriptContent) == "" {
		resp.Success = false
		resp.Error = "script is empty"
		return resp, nil
	}

	// 若未显式定义 encodeInp 函数，自动将顶层代码包裹为安全入口并注入 json / payload / metadata 上下文
	if !strings.Contains(scriptContent, "encodeInp") {
		scriptContent = fmt.Sprintf(`
local json = require("json")
function encodeInp(msg, topic)
    local payload = msg
    local metadata = {}
    if topic ~= nil and topic ~= "" then
        pcall(function() metadata = json.decode(topic) end)
    end
%s
end
`, scriptContent)
	}

	metaJSON, _ := json.Marshal(req.Metadata)
	resStr, err := utils.ScriptDeal(scriptContent, []byte(req.Payload), string(metaJSON))
	if err != nil {
		resp.Success = false
		resp.Error = "script execution error: " + err.Error()
		return resp, nil
	}

	resp.RawOutput = resStr
	resp.Logs = append(resp.Logs, "Script execution succeeded")

	// 尝试解析为标准 ThingsBoard 转换器输出
	var parsed struct {
		DeviceName string                 `json:"deviceName"`
		DeviceType string                 `json:"deviceType"`
		Telemetry  map[string]interface{} `json:"telemetry"`
		Attributes map[string]interface{} `json:"attributes"`
	}

	if jsonErr := json.Unmarshal([]byte(resStr), &parsed); jsonErr == nil {
		resp.DeviceName = parsed.DeviceName
		resp.DeviceType = parsed.DeviceType
		if parsed.Telemetry != nil {
			resp.Telemetry = parsed.Telemetry
		}
		if parsed.Attributes != nil {
			resp.Attributes = parsed.Attributes
		}
	} else {
		// 若返回的是扁平 JSON，则全部作为遥测
		var flatMap map[string]interface{}
		if flatErr := json.Unmarshal([]byte(resStr), &flatMap); flatErr == nil {
			resp.Telemetry = flatMap
		}
	}

	return resp, nil
}
