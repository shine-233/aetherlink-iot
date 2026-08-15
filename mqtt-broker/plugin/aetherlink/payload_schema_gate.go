// 文件用途：把纯决策函数 DecidePayloadSchemaEnforcement 接入上行消息处理路径的“可门控”接线层。
// 核心逻辑：一个包级 resolver 钩子,默认 nil —— 即 payload-schema 强制“关闭”,
//
//	此时上行处理路径行为逐字节不变(与接入本文件之前完全一致)。
//	只有当部署方显式注入一个 registry resolver(设备->schema 绑定的运行时查询)时,
//	才会对上行 payload 调用纯决策;reject 被翻译成既有的 errMQTTMessageDiscarded 丢弃路径。
//
// 关键注意事项：这是 payload-schema broker 强制能力中“接线”的一半,默认关闭以保护外部 MQTT 契约。
//
//	真正启用(注入实时 registry + 打开开关 + 拒收=断开/丢弃)属于破坏性契约变更,
//	必须在部署环境(broker+PG+真实设备)做运行时验证,静态环境无法证明其端到端正确性。
//	本文件只保证:①关闭时零行为变化(默认路径),②开启时 reject 走既有丢弃语义,③决策纯函数复用单一来源。
package aetherlink

import (
	"sync"

	"go.uber.org/zap"
)

// PayloadSchemaResolver 由部署方在运行时注入:给定设备与设备配置,返回该设备生效的
// payload-schema 强制配置;第二个返回值 false 表示“该设备无绑定 schema”,应放行不校验。
// 保留这个兼容接口供静态测试及外部装配使用；生产 DB resolver 通过内部错误感知接口接入。
type PayloadSchemaResolver func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool)

type payloadSchemaResolution struct {
	enforcement PayloadSchemaEnforcement
	bound       bool
	err         error
}

type payloadSchemaResolverWithError func(deviceID, deviceConfigID string) payloadSchemaResolution

// payloadSchemaResolver 是包级门控钩子。默认 nil ⇒ 强制关闭 ⇒ 上行路径行为不变。
// 仅由部署侧初始化代码通过 setter 注入,静态/单测默认保持关闭。
var (
	payloadSchemaResolverMu sync.RWMutex
	payloadSchemaResolver   payloadSchemaResolverWithError
)

// SetPayloadSchemaResolver 注入(或清空,传 nil)兼容 resolver。
// 传 nil 恢复“强制关闭”默认。兼容 resolver 没有错误通道，行为与既有调用方一致。
func SetPayloadSchemaResolver(resolver PayloadSchemaResolver) {
	if resolver == nil {
		setPayloadSchemaResolverWithError(nil)
		return
	}
	setPayloadSchemaResolverWithError(func(deviceID, deviceConfigID string) payloadSchemaResolution {
		enforcement, bound := resolver(deviceID, deviceConfigID)
		return payloadSchemaResolution{enforcement: enforcement, bound: bound}
	})
}

func setPayloadSchemaResolverWithError(resolver payloadSchemaResolverWithError) {
	payloadSchemaResolverMu.Lock()
	payloadSchemaResolver = resolver
	payloadSchemaResolverMu.Unlock()
}

func payloadSchemaResolverSnapshot() payloadSchemaResolverWithError {
	payloadSchemaResolverMu.RLock()
	resolver := payloadSchemaResolver
	payloadSchemaResolverMu.RUnlock()
	return resolver
}

// payloadSchemaEnforcementEnabled 报告强制是否已被部署方启用(注入了 resolver)。
func payloadSchemaEnforcementEnabled() bool {
	return payloadSchemaResolverSnapshot() != nil
}

// enforcePayloadSchemaOnUplink 在上行 payload 被接受前应用 payload-schema 强制(若已启用)。
// 返回 true 表示“应拒收该消息”(调用方翻译成 errMQTTMessageDiscarded);
// 返回 false 表示放行(强制关闭、确认无绑定 schema、或 payload 满足约束/仅告警)。
// 已启用的生产 resolver 无法查询或解析 schema 时拒收，避免把依赖故障误判成“无 schema”。
func enforcePayloadSchemaOnUplink(deviceID, deviceConfigID string, rawPayload []byte) bool {
	resolver := payloadSchemaResolverSnapshot()
	if resolver == nil {
		return false
	}

	resolution := resolver(deviceID, deviceConfigID)
	if resolution.err != nil {
		if Log != nil {
			Log.Error(
				"payload schema resolution failed; uplink rejected",
				zap.String("device_id", deviceID),
				zap.String("device_config_id", deviceConfigID),
				zap.Error(resolution.err),
			)
		}
		return true
	}
	if !resolution.bound {
		return false
	}

	decision := DecidePayloadSchemaEnforcement(resolution.enforcement, rawPayload)
	switch decision.Outcome {
	case PayloadSchemaReject:
		if Log != nil {
			Log.Warn(
				"payload schema enforcement rejected uplink",
				zap.String("device_id", deviceID),
				zap.String("device_config_id", deviceConfigID),
				zap.Int("error_count", len(decision.Errors)),
				zap.String("reason", decision.Reason),
				zap.String("category", "schema_constraint"),
			)
		}
		return true
	case PayloadSchemaWarn:
		if Log != nil {
			Log.Info(
				"payload schema enforcement warned on uplink",
				zap.String("device_id", deviceID),
				zap.String("device_config_id", deviceConfigID),
				zap.Int("warning_count", len(decision.Warnings)),
				zap.String("reason", decision.Reason),
				zap.String("category", "undeclared_field"),
			)
		}
		return false
	default:
		return false
	}
}
