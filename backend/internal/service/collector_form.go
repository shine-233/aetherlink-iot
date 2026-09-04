// 文件用途：内置采集器协议（SNMP/OPC UA）的动态配置表单与点表保存校验（ROADMAP C6 收尾）。
// 核心逻辑：
//   1. builtinCollectorConfigForm 按前端动态表单契约（frontend/src/views/device/config-detail/
//      modules/form.vue：input/select/table 三类元素，键 type/dataKey/label/validate/options/array）
//      静态定义 config_form——内置协议无插件进程，无插件 HTTP 表单源；
//   2. validateCollectorProtocolConfig 在 device_config 创建/更新时校验点表结构，
//      复用 collector 包解析器（保存通过的点表采集器必然可解析，契约单一来源）。
// 关键注意事项：
//   - dataKey 是前后端契约：必须与 collector 包 SnmpConfig/OpcuaConfig 的 JSON 键一致
//     （target/community/timeout_ms/points[{key,oid}]、endpoint/security_mode/username/password/points[{key,node}]），
//     任何一侧改名即断链，改动需两侧同批提交；
//   - 标签用协议术语原文（语言中立），不引入前端 i18n 依赖。
package service

import (
	"strings"

	"aetherlink-iot/backend/internal/collector/pointconfig"
	"aetherlink-iot/backend/pkg/errcode"
)

// 内置采集器协议标识（与 device_configs.protocol_type、collector.Poller.ConfigType 同口径）。
const (
	builtinProtocolSNMP  = "SNMP"
	builtinProtocolOPCUA = "OPCUA"
)

// isBuiltinCollectorProtocol 判断协议类型是否为内置采集器协议（大小写不敏感）。
func isBuiltinCollectorProtocol(protocolType string) bool {
	switch strings.ToUpper(strings.TrimSpace(protocolType)) {
	case builtinProtocolSNMP, builtinProtocolOPCUA:
		return true
	default:
		return false
	}
}

// requiredRule 生成必填校验规则（naive-ui FormItemRule 形状）。
func requiredRule(message string) map[string]interface{} {
	return map[string]interface{}{"required": true, "message": message, "trigger": "blur"}
}

// inputElement 构造 input 元素。
func inputElement(dataKey, label, placeholder string, validate map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":        "input",
		"dataKey":     dataKey,
		"label":       label,
		"placeholder": placeholder,
		"validate":    validate,
	}
}

// selectElement 构造 select 元素。
func selectElement(dataKey, label string, options []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":    "select",
		"dataKey": dataKey,
		"label":   label,
		"options": options,
	}
}

// pointTableElement 构造 points 表格元素（行内子字段由协议决定）。
func pointTableElement(subElements []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":    "table",
		"dataKey": "points",
		"label":   "points",
		"array":   subElements,
	}
}

// builtinCollectorConfigForm 返回内置协议的配置表单；非内置协议返回 nil（走插件表单链路）。
func builtinCollectorConfigForm(protocolType string) interface{} {
	switch strings.ToUpper(strings.TrimSpace(protocolType)) {
	case builtinProtocolSNMP:
		return []map[string]interface{}{
			inputElement("target", "target", "host:port (e.g. 10.0.0.5:161)", requiredRule("target is required")),
			inputElement("community", "community", "public", requiredRule("community is required")),
			inputElement("timeout_ms", "timeout_ms", "1500", map[string]interface{}{"type": "number"}),
			pointTableElement([]map[string]interface{}{
				inputElement("key", "key", "temperature", requiredRule("key is required")),
				inputElement("oid", "oid", "1.3.6.1.2.1.1.3.0", requiredRule("oid is required")),
			}),
		}
	case builtinProtocolOPCUA:
		return []map[string]interface{}{
			inputElement("endpoint", "endpoint", "opc.tcp://host:port", requiredRule("endpoint is required")),
			selectElement("security_mode", "security_mode", []map[string]interface{}{
				{"label": "None", "value": "None"},
				{"label": "Sign", "value": "Sign"},
				{"label": "SignAndEncrypt", "value": "SignAndEncrypt"},
			}),
			inputElement("username", "username", "", nil),
			inputElement("password", "password", "", nil),
			pointTableElement([]map[string]interface{}{
				inputElement("key", "key", "temperature", requiredRule("key is required")),
				inputElement("node", "node", "ns=2;s=Demo.Temp", requiredRule("node is required")),
			}),
		}
	default:
		return nil
	}
}

// validateCollectorProtocolConfig 校验内置采集器协议的 protocol_config 点表结构。
// 非内置协议或未提供点表时直接放行（沿用既有 JSON 合法性校验）。
// 传入"生效值"语义：创建链路传请求字段；更新链路传"请求值优先、旧值兜底"的合成结果，
// 保证只改 protocol_type 不带新点表时也能发现跨字段不一致。
func validateCollectorProtocolConfig(protocolType, protocolConfig *string) error {
	if protocolType == nil || !isBuiltinCollectorProtocol(*protocolType) {
		return nil
	}
	if protocolConfig == nil || strings.TrimSpace(*protocolConfig) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError,
			"protocol_config is required for "+strings.ToUpper(*protocolType)+" (collector point table)")
	}
	var err error
	switch strings.ToUpper(strings.TrimSpace(*protocolType)) {
	case builtinProtocolSNMP:
		err = pointconfig.ValidateSnmpConfig(*protocolConfig)
	case builtinProtocolOPCUA:
		err = pointconfig.ValidateOpcuaConfig(*protocolConfig)
	}
	if err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "protocol_config invalid: "+err.Error())
	}
	return nil
}
