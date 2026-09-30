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

// mobilePage 解析分页参数。夹紧逻辑在 service 层，这里只负责取值。
func mobilePage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return page, pageSize
}

// mobileTenant 由 claims 推导租户，规则与 SCADA 侧一致。
func mobileTenant(claims *utils.UserClaims, requested string) (string, error) {
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
	HandlePublicNoBody(c, func() (interface{}, error) {
		svc := mobileService()
		if svc == nil {
			return service.MobileCapabilityMatrix{}, nil
		}
		return svc.Capabilities(c), nil
	})
}

func (*MobileApi) SubscribePush(c *gin.Context) {
	Handle(c, func(req *SubscribePushReq, claims *utils.UserClaims) (interface{}, error) {
		svc := mobileService()
		if svc == nil {
			return nil, mobileNotWired()
		}
		tenantID, err := mobileTenant(claims, req.TenantID)
		if err != nil {
			return nil, err
		}
		provider := strings.TrimSpace(req.Provider)
		if provider == "" {
			provider = "fcm"
		}
		return svc.SubscribePush(c, tenantID, claims.ID, req.Platform, req.Token, provider)
	})
}

func (*MobileApi) UnsubscribePush(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, claims *utils.UserClaims) error {
		svc := mobileService()
		if svc == nil {
			return mobileNotWired()
		}
		tenantID, err := mobileTenant(claims, c.Query("tenant_id"))
		if err != nil {
			return err
		}
		return svc.UnsubscribePush(c, tenantID, id)
	})
}

// SendMobileCommand 幂等下发命令。命中已完成幂等键时返回原收据，不再下发。
func (*MobileApi) SendMobileCommand(c *gin.Context) {
	Handle(c, func(req *MobileCommandReq, claims *utils.UserClaims) (interface{}, error) {
		svc := mobileService()
		if svc == nil {
			return nil, mobileNotWired()
		}
		tenantID, err := mobileTenant(claims, req.TenantID)
		if err != nil {
			return nil, err
		}

		key := strings.TrimSpace(c.GetHeader(idempotencyKeyHeader))
		if key == "" {
			key = strings.TrimSpace(req.IdempotencyKey)
		}
		if key == "" {
			// 没有幂等键的命令在弱网下必然重复下发，直接拒绝而不是"先发了再说"。
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "Idempotency-Key header is required")
		}

		return svc.SendCommand(c, tenantID, claims.ID, key, service.ControlExecution{
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
	})
}

// ---------------------------------------------------------------------------
// 设备 / 告警 / 影子
// ---------------------------------------------------------------------------

// ListMobileDevices 列出调用者可见的设备。
// 归属过滤在 service 层按 claims.Authority 完成，接口层不传 tenant_id——
// 允许前端指定租户等于把过滤规则交给调用方决定。
func (*MobileApi) ListMobileDevices(c *gin.Context) {
	HandleNoBody(c, func(claims *utils.UserClaims) (interface{}, error) {
		svc := mobileService()
		if svc == nil {
			return nil, mobileNotWired()
		}
		page, pageSize := mobilePage(c)
		list, total, err := svc.ListDevices(c, claims, strings.TrimSpace(c.Query("search")), page, pageSize)
		if err != nil {
			return nil, err
		}
		return gin.H{"total": total, "list": list}, nil
	})
}

// ListMobileAlarms 列出调用者可见的告警。
func (*MobileApi) ListMobileAlarms(c *gin.Context) {
	HandleNoBody(c, func(claims *utils.UserClaims) (interface{}, error) {
		svc := mobileService()
		if svc == nil {
			return nil, mobileNotWired()
		}
		page, pageSize := mobilePage(c)
		data, err := svc.ListAlarms(c, claims, page, pageSize)
		if err != nil {
			return nil, err
		}
		return gin.H{"total": data.Total, "list": data.List}, nil
	})
}

// AcknowledgeMobileAlarm 确认一条告警。
func (*MobileApi) AcknowledgeMobileAlarm(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, claims *utils.UserClaims) error {
		svc := mobileService()
		if svc == nil {
			return mobileNotWired()
		}
		return svc.AcknowledgeAlarm(c, claims, id)
	})
}

// GetMobileShadow 读取设备影子。
func (*MobileApi) GetMobileShadow(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		svc := mobileService()
		if svc == nil {
			return nil, mobileNotWired()
		}
		data, err := svc.GetShadow(c, claims, id)
		if err != nil {
			return nil, err
		}
		return gin.H{"shadow": data}, nil
	})
}

// UpdateMobileShadowReq 更新设备影子请求。
type UpdateMobileShadowReq struct {
	// Payload 命令载荷的 JSON 字符串，形如 {"method":"set_temp","params":{...}}。
	Payload string `json:"payload" binding:"required"`
}

// UpdateMobileShadow 更新设备影子（在线即下发，离线入队）。
func (*MobileApi) UpdateMobileShadow(c *gin.Context) {
	HandlePathBodyAction(c, "id", func(id string, req *UpdateMobileShadowReq, claims *utils.UserClaims) error {
		svc := mobileService()
		if svc == nil {
			return mobileNotWired()
		}
		return svc.UpdateShadow(c, claims, id, req.Payload)
	})
}

// GetMobileOTAStatus 读取单台设备的 OTA 升级状态。
func (*MobileApi) GetMobileOTAStatus(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		svc := mobileService()
		if svc == nil {
			return nil, mobileNotWired()
		}
		status, err := svc.OTAStatus(c, claims, id)
		if err != nil {
			return nil, err
		}
		return gin.H{"status": status}, nil
	})
}

// ListMobileDashboards 列出调用者可查看的看板。
func (*MobileApi) ListMobileDashboards(c *gin.Context) {
	HandleNoBody(c, func(claims *utils.UserClaims) (interface{}, error) {
		svc := mobileService()
		if svc == nil {
			return nil, mobileNotWired()
		}
		list, err := svc.ListDashboards(c, claims)
		if err != nil {
			return nil, err
		}
		return gin.H{"list": list}, nil
	})
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
