// 文件用途：定义 broker 插件模型、插件生命周期接口和 HookWrapper 扩展点集合。
// 核心逻辑：把认证、连接、订阅、发布、遗嘱、停机等 hook 包装成插件可组合的扩展契约。
// 使用注意：插件接口属于公开兼容面，字段改名或签名调整会影响外部插件加载。
// 重构建议：后续可按生命周期拆分 hook 组，并在 README 中维护插件兼容矩阵。

package server

import (
	"github.com/DrmagicE/gmqtt/config"
)

// HookWrapper groups all hook wrappers function
type HookWrapper struct {
	OnBasicAuthWrapper         OnBasicAuthWrapper
	OnEnhancedAuthWrapper      OnEnhancedAuthWrapper
	OnConnectedWrapper         OnConnectedWrapper
	OnReAuthWrapper            OnReAuthWrapper
	OnSessionCreatedWrapper    OnSessionCreatedWrapper
	OnSessionResumedWrapper    OnSessionResumedWrapper
	OnSessionTerminatedWrapper OnSessionTerminatedWrapper
	OnSubscribeWrapper         OnSubscribeWrapper
	OnSubscribedWrapper        OnSubscribedWrapper
	OnUnsubscribeWrapper       OnUnsubscribeWrapper
	OnUnsubscribedWrapper      OnUnsubscribedWrapper
	OnMsgArrivedWrapper        OnMsgArrivedWrapper
	OnMsgDroppedWrapper        OnMsgDroppedWrapper
	OnDeliveredWrapper         OnDeliveredWrapper
	OnClosedWrapper            OnClosedWrapper
	OnAcceptWrapper            OnAcceptWrapper
	OnStopWrapper              OnStopWrapper
	OnWillPublishWrapper       OnWillPublishWrapper
	OnWillPublishedWrapper     OnWillPublishedWrapper
}

// NewPlugin is the constructor of a plugin.
type NewPlugin func(config config.Config) (Plugin, error)

// Plugin is the interface need to be implemented for every plugins.
type Plugin interface {
	// Load will be called in server.Run(). If return error, the server will panic.
	Load(service Server) error
	// Unload will be called when the server is shutdown, the return error is only for logging
	Unload() error
	// HookWrapper returns all hook wrappers that used by the plugin.
	// Return a empty wrapper  if the plugin does not need any hooks
	HookWrapper() HookWrapper
	// Name return the plugin name
	Name() string
}
