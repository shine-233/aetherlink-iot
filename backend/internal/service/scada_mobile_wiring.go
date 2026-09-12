// 文件用途：P1.3（SCADA 实时控制）与 P1.4（移动端）的启动装配与真实依赖适配。
// 核心逻辑：把「内置 Widget 注册表 / 二次确认签发器 / 命令下发执行器」装成
// ScadaControlService，把「命令下发执行器 / 幂等存储」装成 MobileService。
//
// 关键注意事项（本文件是"接口存在"与"能力可用"之间的那道线，别让它变成装饰）：
//  1. **命令下发必须带真实 claims**。`CommandPutMessageWithTracking` 在没有 claims
//     参数时会跳过设备写权限校验（command_data.go: ensureCommandWriteAccess 首行即 return nil）。
//     因此执行器在 ActorClaims 为空时直接拒绝执行，绝不"补个空 claims 先发了"。
//  2. **未接线的能力就让它报 false**。推送（FCM/APNs）没有真实 Provider 凭据，
//     这里就不构造 PushService——构造一个空 Provider 注册表的 PushService 会让
//     能力矩阵报 Push=true，而每次 Enqueue 都失败，这正是最典型的假成功。
//  3. 内置 Widget 与前端 scada-editor 的 WIDGET_REGISTRY 逐项一致。两边不一致时，
//     前端能画出来的控件后端判定"未注册"、或后端放行的命令前端根本不存在，
//     任一种都是画布与执行脱节。改任一侧都必须同步另一侧（见 widget_parity 测试）。
package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
)

// ErrControlActorClaimsMissing 执行器缺少请求者凭证。
// 与"执行器未接线"区分开：后者是装配问题，前者是调用链丢了凭证。
var ErrControlActorClaimsMissing = errors.New("control command is missing actor claims; refusing to dispatch")

// ---------------------------------------------------------------------------
// 内置 Widget 注册表
// ---------------------------------------------------------------------------

// builtinWidgetSchema 内置 Widget 的配置 schema。
// 用最小合法 JSON 对象，而不是描述真实配置项：这些 Widget 目前只声明存在性与命令，
// 尚未定义配置字段。写成看起来很完整的 schema 会让人以为配置已被校验。
const builtinWidgetSchema = `{}`

// builtinWidgetDefinitions 内置 Widget 定义。
// 必须与前端 src/views/visualization/scada-editor/index.vue 的 WIDGET_REGISTRY 一致。
func builtinWidgetDefinitions() []WidgetDefinition {
	return []WidgetDefinition{
		{
			Type:         "gauge",
			Version:      "1",
			Schema:       builtinWidgetSchema,
			Capabilities: []string{WidgetCapability2D},
			Commands: []CommandDefinition{
				// 只读刷新：不下发到设备，无需二次确认。
				{Name: "refresh", RequiresConfirmation: false},
			},
		},
		{
			Type:         "chart",
			Version:      "1",
			Schema:       builtinWidgetSchema,
			Capabilities: []string{WidgetCapability2D},
			Commands: []CommandDefinition{
				{Name: "refresh", RequiresConfirmation: false},
			},
		},
		{
			Type:         "valve",
			Version:      "1",
			Schema:       builtinWidgetSchema,
			Capabilities: []string{WidgetCapability2D},
			Commands: []CommandDefinition{
				// 阀门开合会真实改变现场设备状态，必须二次确认。
				{Name: "open_valve", RequiresConfirmation: true},
			},
		},
		{
			Type:         "twin3d",
			Version:      "1",
			Schema:       builtinWidgetSchema,
			Capabilities: []string{WidgetCapability3D},
			Commands:     nil,
		},
	}
}

// DefaultWidgetRegistry 构造内置 Widget 注册表。
// 注册失败即返回错误：内置定义是常量，注册不上说明注册表校验逻辑被改坏，
// 静默跳过会让"全部控件未知"看起来像正常状态。
func DefaultWidgetRegistry() (*WidgetRegistry, error) {
	registry := NewWidgetRegistry()
	for _, def := range builtinWidgetDefinitions() {
		if err := registry.Register(def); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// ---------------------------------------------------------------------------
// 命令下发执行器
// ---------------------------------------------------------------------------

// commandDeliveryExecutor 把控制命令交给真实的命令下发通道。
// 同一个实例同时满足 ControlCommandExecutor（SCADA 画布）与
// MobileCommandSender（移动端）——两侧最终走的是同一条下发链路，
// 拆成两个实现反而会让两条路径的校验规则各走各的。
type commandDeliveryExecutor struct{}

// NewCommandDeliveryExecutor 创建命令下发执行器。
func NewCommandDeliveryExecutor() ControlCommandExecutor {
	return commandDeliveryExecutor{}
}

// Execute 下发一条控制命令。
func (commandDeliveryExecutor) Execute(ctx context.Context, exec ControlExecution) error {
	if exec.ActorClaims == nil {
		// 见文件头注意事项 1：没有 claims 的下发会绕过设备写权限校验。
		return ErrControlActorClaimsMissing
	}
	deviceID := strings.TrimSpace(exec.DeviceID)
	if deviceID == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "control command requires a target device")
	}
	command := strings.TrimSpace(exec.Command)
	if command == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "control command requires a command name")
	}

	// 载荷用命令参数；空参数下发空串会让设备侧无法区分"没带参数"和"参数丢失"，
	// 统一成空对象。
	payload := exec.Params
	if strings.TrimSpace(payload) == "" {
		payload = "{}"
	}

	req := &model.PutMessageForCommand{
		DeviceID: deviceID,
		Value:    &payload,
		Identify: command,
	}
	// operationType 用 Manual：这是有人在界面上点出来的动作，不是自动化触发。
	_, err := GroupApp.CommandData.CommandPutMessageWithTracking(
		ctx,
		exec.ActorUserID,
		req,
		strconv.Itoa(constant.Manual),
		exec.ActorClaims,
	)
	return err
}

// mobileCommandSender 把 ControlCommandExecutor 适配成移动端的 MobileCommandSender。
// 两个接口只是方法名不同（Execute / Send），语义完全一致。用一行适配而不是复制一份
// 执行逻辑——复制一份必然会在某一侧漏掉 claims 校验这类关键闸门。
type mobileCommandSender struct {
	exec ControlCommandExecutor
}

// Send 下发一条命令，直接委托给执行器。
func (m mobileCommandSender) Send(ctx context.Context, exec ControlExecution) error {
	return m.exec.Execute(ctx, exec)
}

// ---------------------------------------------------------------------------
// 装配入口
// ---------------------------------------------------------------------------

// ScadaControlWiring 描述控制服务装配所需的可配置项。
type ScadaControlWiring struct {
	// ConfirmationSecret 二次确认令牌的 HMAC 密钥。留空则不签发令牌，
	// 所有 RequiresConfirmation 的命令一律被拒（fail closed）。
	ConfirmationSecret string
	// ConfirmationTTL 令牌有效期，<=0 用默认值 5 分钟。
	ConfirmationTTL time.Duration
}

// AssembleScadaControl 装配 SCADA 控制服务。
// 返回 (service, issuerConfigured, error)：
//   - issuerConfigured=false 表示密钥未配置，命令服务已接线但需确认的命令会被拒；
//     调用方应据此打启动告警，别让"控制能用"掩盖"危险操作其实点不动"。
func AssembleScadaControl(cfg ScadaControlWiring) (*ScadaControlService, bool, error) {
	registry, err := DefaultWidgetRegistry()
	if err != nil {
		return nil, false, err
	}

	var issuer *ConfirmationIssuer
	if strings.TrimSpace(cfg.ConfirmationSecret) != "" {
		issuer, err = NewConfirmationIssuer(cfg.ConfirmationSecret, cfg.ConfirmationTTL)
		if err != nil {
			return nil, false, err
		}
	}

	return NewScadaControlService(registry, issuer, NewCommandDeliveryExecutor()), issuer != nil, nil
}

// AssemblePush 按配置构造推送服务。
//
// 返回 (service, configured, error)：
//   - configured=false 表示没有任何可用 Provider（未配置 FCM 凭据），
//     此时返回的 service 为 **nil**：传一个空 Provider 注册表会让能力矩阵报
//     Push=true 而每次发送都失败，那正是本项目要消灭的假成功。
//   - 配置了但凭据非法时返回 error 并阻断启动：那是配置错误，
//     静默降级成"不发推送"会让运维以为推送是通的。
//
// PushWiringConfig 推送装配所需的全部配置。
type PushWiringConfig struct {
	FCM  FCMConfig
	APNs APNsConfig
}

// AssemblePush 按配置构造推送服务。
//
// 返回 (service, configured, error)：
//   - configured=false 表示一个 Provider 都没配上，返回的 service 为 **nil**：
//     传一个空 Provider 注册表会让能力矩阵报 Push=true 而每次发送都失败，
//     那正是本项目要消灭的假成功。
//   - 配置了但凭据非法时返回 error 并阻断启动：那是配置错误，
//     静默降级成"不发推送"会让运维以为推送是通的。
//   - 只配了其中一个通道时，另一个平台**没有 Provider**，该平台的推送会被明确拒绝
//     （注册表按平台查找），而不是静默丢弃。
func AssemblePush(cfg PushWiringConfig) (*PushService, bool, error) {
	registry := NewPushProviderRegistry()

	// FCM：未配置（ErrFCMNotConfigured）不算错误，只是没接线。
	if provider, err := NewFCMProvider(cfg.FCM); err == nil {
		if err := registry.Register(provider); err != nil {
			return nil, false, err
		}
	} else if !errors.Is(err, ErrFCMNotConfigured) {
		return nil, false, err
	}

	// APNs：同上。
	if provider, err := NewAPNsProvider(cfg.APNs); err == nil {
		if err := registry.Register(provider); err != nil {
			return nil, false, err
		}
	} else if !errors.Is(err, ErrAPNSNotConfigured) {
		return nil, false, err
	}

	if registry.Len() == 0 {
		return nil, false, nil
	}
	return NewPushService(registry, DefaultPushRetryPolicy()), true, nil
}

// AssembleMobile 装配移动端服务。
//
// 已接线（有真实实现且复用既有归属过滤）：
//   - Devices：复用设备列表查询与 `applyDeviceListOwnerFilterForClaims`
//   - Alarms：告警列表 + 确认，复用 `GetAlarmHisttoryListByPage` / `AcknowledgeAlarmHistory`
//   - Shadow：设备影子读写，复用 `DeviceShadow.GetShadowMessages` / `SetShadowMessage`
//   - OTA：单台设备最近一次升级状态，先过 `ensureTelemetryDeviceReadAccess`，
//     再按包路径 task → package.tenant_id 过滤（明细表自身没有租户列）
//   - Dashboards：复用 `Board.GetBoardListByPage`（看板是租户级共享资产，无归属列，
//     由既有 `resolveBoardListTenant` 按角色裁决）
//   - Commands：与 SCADA 相同的真实下发链路
//   - Idempotency：进程内幂等存储
//
// Push 由 AssemblePush 决定：配置了 FCM 凭据才注入，否则为 nil。
// 令牌登记/撤销不依赖 Provider，始终可用。
func AssembleMobile(push *PushService) *MobileService {
	return NewMobileService(MobileServiceDeps{
		Devices:     NewMobileDeviceLister(),
		AlarmLister: NewMobileAlarmLister(),
		Alarms:      NewMobileAlarmAcker(),
		Shadow:      NewMobileShadowStore(),
		OTA:         NewMobileOTAStatusReader(),
		Dashboards:  NewMobileDashboardReader(),
		Commands:    mobileCommandSender{exec: NewCommandDeliveryExecutor()},
		Idempotency: NewInMemoryCommandIdempotencyStore(0),
		Push:        push,
	})
}
