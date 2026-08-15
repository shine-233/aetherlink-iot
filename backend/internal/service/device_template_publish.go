// device_template_publish.go 负责把本地设备配置与物模型拼装成市场发布请求。
// 核心链路：
// 1. 从 device_config_id 反查设备配置与关联物模型。
// 2. 提取协议配置、凭证类型、图表配置与物模型定义。
// 3. 组装市场发布请求，并根据 market token 解析用户身份。
// 4. 调用市场 API 执行发布，再把远端错误统一映射为系统错误。
// 静态审查建议：
// 1. 当前文件里既做数据回读、默认值兜底，也做远端错误翻译，后续可继续拆成“本地装配”和“远端调用适配”两层。
// 2. 多处默认值仍直接写死，后续若要对外发布规范化，建议集中到常量或配置源。
// 3. parseJSON 对非法 JSON 直接吞错返回 nil，适合当前容错，但会掩盖脏数据来源；后续可考虑加监控或调试日志。
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/golang-jwt/jwt/v4"
)

// marketPublishClient 抽象市场发布调用，便于单测替换真实客户端。
type marketPublishClient interface {
	PublishTemplate(ctx context.Context, token string, userID string, req *model.PublishTemplateReq) (*model.MarketPublishApiResponse, error)
}

var (
	marketPublishGetDeviceConfigByID      = dal.GetDeviceConfigForTenant
	marketPublishGetDeviceTemplateByID    = dal.GetDeviceTemplateForTenant
	marketPublishGetTelemetryDataList     = dal.GetDeviceModelTelemetryDataForTenant
	marketPublishGetAttributeDataList     = dal.GetDeviceModelAttributeDataForTenant
	marketPublishGetEventDataList         = dal.GetDeviceModelEventDataForTenant
	marketPublishGetCommandDataList       = dal.GetDeviceModelCommandDataForTenant
	marketPublishGetProtocolPluginVersion = func(serviceIdentifier string) string {
		pluginMsg, _ := query.ServicePlugin.WithContext(context.Background()).
			Where(query.ServicePlugin.ServiceIdentifier.Eq(serviceIdentifier)).
			First()
		if pluginMsg != nil && pluginMsg.Version != nil {
			return *pluginMsg.Version
		}
		return ""
	}
	newMarketPublishClient = func() marketPublishClient {
		return NewMarketClient()
	}
)

// ptrStr 安全解引用字符串指针；nil 时统一回退为空串，便于市场请求拼装。
func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ptrInt16 安全解引用 int16 指针；当前文件里主要用于兼容可能为空的数值字段。
func ptrInt16(i *int16) int16 {
	if i == nil {
		return 0
	}
	return *i
}

// parseJSON 将 JSON 字符串解码成 map；空串或非法 JSON 都统一返回 nil。
func parseJSON(data string) map[string]interface{} {
	if data == "" {
		return nil
	}
	var result map[string]interface{}
	_ = json.Unmarshal([]byte(data), &result)
	return result
}

// PublishToMarket 以 device_config_id 为入口，把当前租户的 DeviceConfig 与物模型发布到市场。
// Market 请求没有显式跨租户范围参数，因此即使是 SYS_ADMIN 也必须提供有效的当前租户。
func (*DeviceTemplate) PublishToMarket(req model.PublishToMarketReq, claims *utils.UserClaims) (*model.MarketPublishApiResponse, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "Market publishing requires a non-empty tenant claim",
		})
	}

	dc, tpl, tplID, err := loadMarketPublishSource(req.DeviceConfigID, claims.TenantID)
	if err != nil {
		return nil, err
	}

	marketReq := &model.PublishTemplateReq{
		Name:               marketPublishName(req, tpl),
		Brand:              marketPublishBrand(req, tpl),
		Model:              marketPublishModel(req, tpl),
		Category:           marketPublishCategory(req),
		Author:             marketPublishAuthor(req, tpl),
		Version:            marketPublishVersion(req, tpl),
		Description:        marketPublishDescription(req, tpl),
		DeviceConfig:       buildMarketDeviceConfigPayload(dc),
		TemplateDefinition: buildMarketTemplateDefinition(tpl, tplID, claims.TenantID),
		PluginDependencies: getPluginDependenciesFromProtocol(dc),
	}

	apiResp, err := newMarketPublishClient().PublishTemplate(context.Background(), req.MarketToken, extractMarketUserID(req.MarketToken), marketReq)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "Market service unreachable or request failed: " + err.Error(),
		})
	}
	return validateMarketPublishResponse(apiResp)
}

// loadMarketPublishSource 统一读取当前租户发布所需的设备配置、物模型和物模型 ID。
// 如果设备配置没有绑定物模型，会直接返回参数错误，阻止后续空物模型发布。
func loadMarketPublishSource(deviceConfigID, tenantID string) (*model.DeviceConfig, *model.DeviceTemplate, string, error) {
	dc, err := marketPublishGetDeviceConfigByID(deviceConfigID, tenantID)
	if err != nil {
		return nil, nil, "", errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": fmt.Sprintf("failed to find device config: %s", err.Error()),
		})
	}
	if dc.DeviceTemplateID == nil || *dc.DeviceTemplateID == "" {
		return nil, nil, "", errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "device config is not bound to a thing model",
		})
	}

	tplID := *dc.DeviceTemplateID
	tpl, err := marketPublishGetDeviceTemplateByID(tplID, tenantID)
	if err != nil {
		return nil, nil, "", errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": fmt.Sprintf("failed to find thing model: %s", err.Error()),
		})
	}
	return dc, tpl, tplID, nil
}

// buildMarketDeviceConfigPayload 把本地设备配置转换成市场侧可接受的轻量配置载荷。
func buildMarketDeviceConfigPayload(dc *model.DeviceConfig) *model.DeviceConfigPayload {
	return &model.DeviceConfigPayload{
		Name:           dc.Name,
		DeviceType:     dc.DeviceType,
		ProtocolType:   ptrStr(dc.ProtocolType),
		VoucherType:    ptrStr(dc.VoucherType),
		ProtocolConfig: parseJSON(ptrStr(dc.ProtocolConfig)),
		DeviceConnType: ptrStr(dc.DeviceConnType),
		OtherConfig:    parseJSON(ptrStr(dc.OtherConfig)),
		AdditionalInfo: parseJSON(ptrStr(dc.AdditionalInfo)),
		AutoRegister:   dc.AutoRegister,
	}
}

// buildMarketTemplateDefinition 汇总图表配置与物模型定义，作为市场物模型主体内容。
func buildMarketTemplateDefinition(tpl *model.DeviceTemplate, tplID, tenantID string) map[string]interface{} {
	deviceModel := loadMarketDeviceModel(tplID, tenantID)
	return map[string]interface{}{
		"web_chart_config": parseJSON(ptrStr(tpl.WebChartConfig)),
		"app_chart_config": parseJSON(ptrStr(tpl.AppChartConfig)),
		"telemetry":        deviceModel["telemetry"],
		"attributes":       deviceModel["attributes"],
		"commands":         deviceModel["commands"],
		"events":           deviceModel["events"],
	}
}

// loadMarketDeviceModel 分别拉取遥测、属性、事件和命令定义，兼容单类查询失败时其余结构仍可继续装配。
func loadMarketDeviceModel(tplID, tenantID string) map[string]interface{} {
	deviceModel := map[string]interface{}{
		"telemetry":  []interface{}{},
		"attributes": []interface{}{},
		"commands":   []interface{}{},
		"events":     []interface{}{},
	}
	if ts, err := marketPublishGetTelemetryDataList(tplID, tenantID); err == nil && ts != nil {
		deviceModel["telemetry"] = ts
	}
	if attrs, err := marketPublishGetAttributeDataList(tplID, tenantID); err == nil && attrs != nil {
		deviceModel["attributes"] = attrs
	}
	if evts, err := marketPublishGetEventDataList(tplID, tenantID); err == nil && evts != nil {
		deviceModel["events"] = evts
	}
	if cmds, err := marketPublishGetCommandDataList(tplID, tenantID); err == nil && cmds != nil {
		deviceModel["commands"] = cmds
	}
	return deviceModel
}

// 以下一组 helper 负责“请求显式值优先，本地物模型字段次之，最后用默认值兜底”。
func marketPublishName(req model.PublishToMarketReq, tpl *model.DeviceTemplate) string {
	if req.MarketName != "" {
		return req.MarketName
	}
	return tpl.Name
}

func marketPublishBrand(req model.PublishToMarketReq, tpl *model.DeviceTemplate) string {
	if req.Brand != "" {
		return req.Brand
	}
	if brand := ptrStr(tpl.Brand); brand != "" {
		return brand
	}
	return "AetherLink"
}

func marketPublishModel(req model.PublishToMarketReq, tpl *model.DeviceTemplate) string {
	if req.Model != "" {
		return req.Model
	}
	if modelNumber := ptrStr(tpl.ModelNumber); modelNumber != "" {
		return modelNumber
	}
	return "IoT-Device"
}

func marketPublishCategory(req model.PublishToMarketReq) string {
	if req.Category != "" {
		return req.Category
	}
	return "default"
}

func marketPublishVersion(req model.PublishToMarketReq, tpl *model.DeviceTemplate) string {
	if req.Version != "" {
		return req.Version
	}
	return ptrStr(tpl.Version)
}

func marketPublishAuthor(req model.PublishToMarketReq, tpl *model.DeviceTemplate) string {
	if req.Author != "" {
		return req.Author
	}
	return ptrStr(tpl.Author)
}

func marketPublishDescription(req model.PublishToMarketReq, tpl *model.DeviceTemplate) string {
	if req.Description != "" {
		return req.Description
	}
	return ptrStr(tpl.Description)
}

// extractMarketUserID 从市场 token 中读取 sub 字段，作为远端发布接口的用户标识。
// 当前仅做无验证解析，适合透传场景，但不应用于安全决策。
func extractMarketUserID(marketToken string) string {
	token, _, _ := new(jwt.Parser).ParseUnverified(marketToken, jwt.MapClaims{})
	if token == nil {
		return ""
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	sub, _ := claims["sub"].(string)
	return sub
}

// validateMarketPublishResponse 把市场接口业务错误映射成统一的系统错误语义。
// 目前对版本冲突做了专门分支，其他非 0 code 都按通用系统错误处理。
func validateMarketPublishResponse(apiResp *model.MarketPublishApiResponse) (*model.MarketPublishApiResponse, error) {
	if apiResp.Code == 4009 {
		return apiResp, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error":   "Version conflict: this thing model version already exists in the market",
			"message": apiResp.Message,
		})
	}
	if apiResp.Code != 0 {
		return apiResp, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": apiResp.Message,
		})
	}
	return apiResp, nil
}

// getPluginDependenciesFromProtocol 从 DeviceConfig 的 protocol_type 提取插件依赖。
// 这里会反查协议插件版本，帮助市场侧提前识别安装物模型所需的协议插件前置条件。
func getPluginDependenciesFromProtocol(dc *model.DeviceConfig) []model.PluginDependency {
	pt := ptrStr(dc.ProtocolType)
	if pt == "" {
		return []model.PluginDependency{}
	}

	// 查询协议插件当前版本，作为最低依赖版本写入市场元数据。
	version := marketPublishGetProtocolPluginVersion(pt)

	return []model.PluginDependency{
		{
			PluginName: pt,
			PluginType: "protocol",
			MinVersion: version,
			Required:   true,
		},
	}
}
