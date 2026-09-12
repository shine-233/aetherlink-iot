// 文件用途：移动端 HTTP 接口（ROADMAP P1.4）。
// 核心逻辑：能力矩阵、推送令牌登记/撤销、幂等命令下发。
// 关键注意事项：
//  1. 服务未接线时所有接口 fail closed，不返回"成功"。移动端最容易出现的假成功就是
//     按钮点了、返回 200、其实什么都没发生。
//  2. 命令必须带幂等键，优先取 `Idempotency-Key` 头，其次取请求体字段。
//     缺键直接拒绝：弱网下没有幂等键的重试必然重复下发。
package api

import (
	"encoding/json"
	"strconv"
	"strings"

	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type MobileApi struct{}

// 幂等键请求头。与常见网关约定一致，便于移动端统一处理重试。
const idempotencyKeyHeader = "Idempotency-Key"

type (
	SubscribePushReq struct {
		TenantID string `json:"tenant_id"`
		Platform string `json:"platform" binding:"required"`
		Token    string `json:"token" binding:"required"`
		Provider string `json:"provider"`
	}
	MobileCommandReq struct {
		TenantID       string                 `json:"tenant_id"`
		IdempotencyKey string                 `json:"idempotency_key"`
		DeviceID       string                 `json:"device_id" binding:"required"`
		Command        string                 `json:"command" binding:"required"`
		WidgetType     string                 `json:"widget_type"`
		Version        string                 `json:"version"`
		Params         map[string]interface{} `json:"params"`
	}
)

// mobileService 取移动端服务；未接线返回 nil。
func mobileService() *service.MobileService {
	return service.GroupApp.Mobile
}

// mobileNotWired 统一的未接线错误。
func mobileNotWired() error {
	return errcode.NewWithMessage(errcode.CodeOpDenied, "mobile capability is not wired")
}

// mobileClaims 取当前请求的真实凭证。
// 设备列表/告警/影子的归属过滤都依赖 claims.Authority，因此这几个接口只能走凭证，
// 不接受"用请求里的 tenant_id 自己拼一个身份"。
func mobileClaims(c *gin.Context) (*utils.UserClaims, error) {
	claims, ok := c.MustGet("claims").(*utils.UserClaims)
	if !ok || claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "missing user claims")
	}
	return claims, nil
}

// mobilePage 解析分页参数。夹紧逻辑在 service 层，这里只负责取值。
func mobilePage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return page, pageSize
}

// mobileTenant 由 claims 推导租户，规则与 SCADA 侧一致。
func mobileTenant(c *gin.Context, requested string) (string, error) {
	claims, ok := c.MustGet("claims").(*utils.UserClaims)
	if !ok || claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "missing user claims")
	}
	requested = strings.TrimSpace(requested)
	if claims.Authority == "SYS_ADMIN" {
		if requested != "" {
			return requested, nil
		}
		return claims.TenantID, nil
	}
	if strings.TrimSpace(claims.TenantID) == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no tenant context")
	}
	if requested != "" && requested != claims.TenantID {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "cross-tenant operation is not allowed")
	}
	return claims.TenantID, nil
}

// GetMobileCapabilities 返回移动端**实际可用**的能力。
// 这是唯一一个服务未接线时也不报错的接口：能力矩阵本身就是"未接线"的声明，
// 让客户端据此隐藏入口。其余接口一律要求已接线。
func (*MobileApi) GetMobileCapabilities(c *gin.Context) {
	svc := mobileService()
	if svc == nil {
		c.Set("data", service.MobileCapabilityMatrix{})
		return
	}
	c.Set("data", svc.Capabilities(c))
}

func (*MobileApi) SubscribePush(c *gin.Context) {
	var req SubscribePushReq
	if !BindAndValidate(c, &req) {
		return
	}
	svc := mobileService()
	if svc == nil {
		c.Error(mobileNotWired())
		return
	}
	tenantID, err := mobileTenant(c, req.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	claims, _ := c.MustGet("claims").(*utils.UserClaims)
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = "fcm"
	}
	data, err := svc.SubscribePush(c, tenantID, claims.ID, req.Platform, req.Token, provider)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*MobileApi) UnsubscribePush(c *gin.Context) {
	svc := mobileService()
	if svc == nil {
		c.Error(mobileNotWired())
		return
	}
	tenantID, err := mobileTenant(c, c.Query("tenant_id"))
	if err != nil {
		c.Error(err)
		return
	}
	if err := svc.UnsubscribePush(c, tenantID, c.Param("id")); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// SendMobileCommand 幂等下发命令。命中已完成幂等键时返回原收据，不再下发。
func (*MobileApi) SendMobileCommand(c *gin.Context) {
	var req MobileCommandReq
	if !BindAndValidate(c, &req) {
		return
	}
	svc := mobileService()
	if svc == nil {
		c.Error(mobileNotWired())
		return
	}
	tenantID, err := mobileTenant(c, req.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	claims, _ := c.MustGet("claims").(*utils.UserClaims)

	key := strings.TrimSpace(c.GetHeader(idempotencyKeyHeader))
	if key == "" {
		key = strings.TrimSpace(req.IdempotencyKey)
	}
	if key == "" {
		// 没有幂等键的命令在弱网下必然重复下发，直接拒绝而不是"先发了再说"。
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "Idempotency-Key header is required"))
		return
	}

	receipt, err := svc.SendCommand(c, tenantID, claims.ID, key, service.ControlExecution{
		TenantID:    tenantID,
		DeviceID:    req.DeviceID,
		WidgetType:  req.WidgetType,
		Version:     req.Version,
		Command:     req.Command,
		Params:      marshalAPIParams(req.Params),
		ActorUserID: claims.ID,
		// 与 SCADA 侧同一条规则：下发通道的写权限校验依赖真实凭证。
		ActorClaims: claims,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", receipt)
}

// ---------------------------------------------------------------------------
// 设备 / 告警 / 影子
// ---------------------------------------------------------------------------

// ListMobileDevices 列出调用者可见的设备。
// 归属过滤在 service 层按 claims.Authority 完成，接口层不传 tenant_id——
// 允许前端指定租户等于把过滤规则交给调用方决定。
func (*MobileApi) ListMobileDevices(c *gin.Context) {
	svc := mobileService()
	if svc == nil {
		c.Error(mobileNotWired())
		return
	}
	claims, err := mobileClaims(c)
	if err != nil {
		c.Error(err)
		return
	}
	page, pageSize := mobilePage(c)
	list, total, err := svc.ListDevices(c, claims, strings.TrimSpace(c.Query("search")), page, pageSize)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"total": total, "list": list})
}

// ListMobileAlarms 列出调用者可见的告警。
func (*MobileApi) ListMobileAlarms(c *gin.Context) {
	svc := mobileService()
	if svc == nil {
		c.Error(mobileNotWired())
		return
	}
	claims, err := mobileClaims(c)
	if err != nil {
		c.Error(err)
		return
	}
	page, pageSize := mobilePage(c)
	data, err := svc.ListAlarms(c, claims, page, pageSize)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"total": data.Total, "list": data.List})
}

// AcknowledgeMobileAlarm 确认一条告警。
func (*MobileApi) AcknowledgeMobileAlarm(c *gin.Context) {
	svc := mobileService()
	if svc == nil {
		c.Error(mobileNotWired())
		return
	}
	claims, err := mobileClaims(c)
	if err != nil {
		c.Error(err)
		return
	}
	if err := svc.AcknowledgeAlarm(c, claims, c.Param("id")); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// GetMobileShadow 读取设备影子。
func (*MobileApi) GetMobileShadow(c *gin.Context) {
	svc := mobileService()
	if svc == nil {
		c.Error(mobileNotWired())
		return
	}
	claims, err := mobileClaims(c)
	if err != nil {
		c.Error(err)
		return
	}
	data, err := svc.GetShadow(c, claims, c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"shadow": data})
}

// UpdateMobileShadowReq 更新设备影子请求。
type UpdateMobileShadowReq struct {
	// Payload 命令载荷的 JSON 字符串，形如 {"method":"set_temp","params":{...}}。
	Payload string `json:"payload" binding:"required"`
}

// UpdateMobileShadow 更新设备影子（在线即下发，离线入队）。
func (*MobileApi) UpdateMobileShadow(c *gin.Context) {
	var req UpdateMobileShadowReq
	if !BindAndValidate(c, &req) {
		return
	}
	svc := mobileService()
	if svc == nil {
		c.Error(mobileNotWired())
		return
	}
	claims, err := mobileClaims(c)
	if err != nil {
		c.Error(err)
		return
	}
	if err := svc.UpdateShadow(c, claims, c.Param("id"), req.Payload); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// GetMobileOTAStatus 读取单台设备的 OTA 升级状态。
func (*MobileApi) GetMobileOTAStatus(c *gin.Context) {
	svc := mobileService()
	if svc == nil {
		c.Error(mobileNotWired())
		return
	}
	claims, err := mobileClaims(c)
	if err != nil {
		c.Error(err)
		return
	}
	status, err := svc.OTAStatus(c, claims, c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"status": status})
}

// ListMobileDashboards 列出调用者可查看的看板。
func (*MobileApi) ListMobileDashboards(c *gin.Context) {
	svc := mobileService()
	if svc == nil {
		c.Error(mobileNotWired())
		return
	}
	claims, err := mobileClaims(c)
	if err != nil {
		c.Error(err)
		return
	}
	list, err := svc.ListDashboards(c, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"list": list})
}

// marshalAPIParams 将请求参数序列化为字符串指纹。
// 与 service.marshalParams 保持一致的空对象回退，避免 nil/失败时产生空串指纹，
// 那样会让"带参数"与"不带参数"的命令指纹相同。
func marshalAPIParams(params map[string]interface{}) string {
	if len(params) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(params)
	if err != nil || string(raw) == "" {
		return "{}"
	}
	return string(raw)
}
