package server

import (
	"context"
	"fmt"
	"net"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"go.uber.org/zap"
)

func (srv *server) initPluginHooks() error {
	zaplog.Info("init plugin hook wrappers")

	if err := srv.initConfiguredPlugins(); err != nil {
		return err
	}

	wrappers := collectPluginHookWrappers(srv.plugins)
	srv.installPluginHookWrappers(wrappers)
	return nil
}

type pluginHookWrappers struct {
	onAccept            []OnAcceptWrapper
	onBasicAuth         []OnBasicAuthWrapper
	onEnhancedAuth      []OnEnhancedAuthWrapper
	onReAuth            []OnReAuthWrapper
	onConnected         []OnConnectedWrapper
	onSessionCreated    []OnSessionCreatedWrapper
	onSessionResumed    []OnSessionResumedWrapper
	onSessionTerminated []OnSessionTerminatedWrapper
	onSubscribe         []OnSubscribeWrapper
	onSubscribed        []OnSubscribedWrapper
	onUnsubscribe       []OnUnsubscribeWrapper
	onUnsubscribed      []OnUnsubscribedWrapper
	onMsgArrived        []OnMsgArrivedWrapper
	onDelivered         []OnDeliveredWrapper
	onClosed            []OnClosedWrapper
	onStop              []OnStopWrapper
	onMsgDropped        []OnMsgDroppedWrapper
	onWillPublish       []OnWillPublishWrapper
	onWillPublished     []OnWillPublishedWrapper
}

func (srv *server) initConfiguredPlugins() error {
	seenPluginNames := make(map[string]struct{}, len(srv.config.PluginOrder))
	// plugin_order is both a config contract and the hook-wrapper chain order;
	// aliases must resolve before wrappers are composed below.
	for _, v := range srv.config.PluginOrder {
		newPlugin, ok := plugins[v]
		if !ok || newPlugin == nil {
			return fmt.Errorf("plugin %q is configured in plugin_order but not registered", v)
		}
		plg, err := newPlugin(srv.config)
		if err != nil {
			return err
		}
		pluginName := plg.Name()
		if _, ok := seenPluginNames[pluginName]; ok {
			return fmt.Errorf("duplicated plugin aliases in plugin_order for %s", pluginName)
		}
		seenPluginNames[pluginName] = struct{}{}
		srv.plugins = append(srv.plugins, plg)
	}
	return nil
}

func collectPluginHookWrappers(plugins []Plugin) pluginHookWrappers {
	var wrappers pluginHookWrappers
	for _, p := range plugins {
		appendPluginHookWrappers(&wrappers, p.HookWrapper())
	}
	return wrappers
}

func appendPluginHookWrappers(wrappers *pluginHookWrappers, hooks HookWrapper) {
	if hooks.OnAcceptWrapper != nil {
		wrappers.onAccept = append(wrappers.onAccept, hooks.OnAcceptWrapper)
	}
	if hooks.OnBasicAuthWrapper != nil {
		wrappers.onBasicAuth = append(wrappers.onBasicAuth, hooks.OnBasicAuthWrapper)
	}
	if hooks.OnEnhancedAuthWrapper != nil {
		wrappers.onEnhancedAuth = append(wrappers.onEnhancedAuth, hooks.OnEnhancedAuthWrapper)
	}
	if hooks.OnReAuthWrapper != nil {
		wrappers.onReAuth = append(wrappers.onReAuth, hooks.OnReAuthWrapper)
	}
	if hooks.OnConnectedWrapper != nil {
		wrappers.onConnected = append(wrappers.onConnected, hooks.OnConnectedWrapper)
	}
	if hooks.OnSessionCreatedWrapper != nil {
		wrappers.onSessionCreated = append(wrappers.onSessionCreated, hooks.OnSessionCreatedWrapper)
	}
	if hooks.OnSessionResumedWrapper != nil {
		wrappers.onSessionResumed = append(wrappers.onSessionResumed, hooks.OnSessionResumedWrapper)
	}
	if hooks.OnSessionTerminatedWrapper != nil {
		wrappers.onSessionTerminated = append(wrappers.onSessionTerminated, hooks.OnSessionTerminatedWrapper)
	}
	if hooks.OnSubscribeWrapper != nil {
		wrappers.onSubscribe = append(wrappers.onSubscribe, hooks.OnSubscribeWrapper)
	}
	if hooks.OnSubscribedWrapper != nil {
		wrappers.onSubscribed = append(wrappers.onSubscribed, hooks.OnSubscribedWrapper)
	}
	if hooks.OnUnsubscribeWrapper != nil {
		wrappers.onUnsubscribe = append(wrappers.onUnsubscribe, hooks.OnUnsubscribeWrapper)
	}
	if hooks.OnUnsubscribedWrapper != nil {
		wrappers.onUnsubscribed = append(wrappers.onUnsubscribed, hooks.OnUnsubscribedWrapper)
	}
	if hooks.OnMsgArrivedWrapper != nil {
		wrappers.onMsgArrived = append(wrappers.onMsgArrived, hooks.OnMsgArrivedWrapper)
	}
	if hooks.OnMsgDroppedWrapper != nil {
		wrappers.onMsgDropped = append(wrappers.onMsgDropped, hooks.OnMsgDroppedWrapper)
	}
	if hooks.OnDeliveredWrapper != nil {
		wrappers.onDelivered = append(wrappers.onDelivered, hooks.OnDeliveredWrapper)
	}
	if hooks.OnClosedWrapper != nil {
		wrappers.onClosed = append(wrappers.onClosed, hooks.OnClosedWrapper)
	}
	if hooks.OnStopWrapper != nil {
		wrappers.onStop = append(wrappers.onStop, hooks.OnStopWrapper)
	}
	if hooks.OnWillPublishWrapper != nil {
		wrappers.onWillPublish = append(wrappers.onWillPublish, hooks.OnWillPublishWrapper)
	}
	if hooks.OnWillPublishedWrapper != nil {
		wrappers.onWillPublished = append(wrappers.onWillPublished, hooks.OnWillPublishedWrapper)
	}
}

func (srv *server) installPluginHookWrappers(wrappers pluginHookWrappers) {
	if wrappers.onAccept != nil {
		srv.hooks.OnAccept = wrapOnAcceptHook(wrappers.onAccept)
	}
	if wrappers.onBasicAuth != nil {
		srv.hooks.OnBasicAuth = wrapOnBasicAuthHook(wrappers.onBasicAuth)
	}
	if wrappers.onEnhancedAuth != nil {
		srv.hooks.OnEnhancedAuth = wrapOnEnhancedAuthHook(wrappers.onEnhancedAuth)
	}
	if wrappers.onReAuth != nil {
		srv.hooks.OnReAuth = wrapOnReAuthHook(wrappers.onReAuth)
	}
	if wrappers.onConnected != nil {
		srv.hooks.OnConnected = wrapOnConnectedHook(wrappers.onConnected)
	}
	if wrappers.onSessionCreated != nil {
		srv.hooks.OnSessionCreated = wrapOnSessionCreatedHook(wrappers.onSessionCreated)
	}
	if wrappers.onSessionResumed != nil {
		srv.hooks.OnSessionResumed = wrapOnSessionResumedHook(wrappers.onSessionResumed)
	}
	if wrappers.onSessionTerminated != nil {
		srv.hooks.OnSessionTerminated = wrapOnSessionTerminatedHook(wrappers.onSessionTerminated)
	}
	if wrappers.onSubscribe != nil {
		srv.hooks.OnSubscribe = wrapOnSubscribeHook(wrappers.onSubscribe)
	}
	if wrappers.onSubscribed != nil {
		srv.hooks.OnSubscribed = wrapOnSubscribedHook(wrappers.onSubscribed)
	}
	if wrappers.onUnsubscribe != nil {
		srv.hooks.OnUnsubscribe = wrapOnUnsubscribeHook(wrappers.onUnsubscribe)
	}
	if wrappers.onUnsubscribed != nil {
		srv.hooks.OnUnsubscribed = wrapOnUnsubscribedHook(wrappers.onUnsubscribed)
	}
	if wrappers.onMsgArrived != nil {
		srv.hooks.OnMsgArrived = wrapOnMsgArrivedHook(wrappers.onMsgArrived)
	}
	if wrappers.onDelivered != nil {
		srv.hooks.OnDelivered = wrapOnDeliveredHook(wrappers.onDelivered)
	}
	if wrappers.onClosed != nil {
		srv.hooks.OnClosed = wrapOnClosedHook(wrappers.onClosed)
	}
	if wrappers.onStop != nil {
		srv.hooks.OnStop = wrapOnStopHook(wrappers.onStop)
	}
	if wrappers.onMsgDropped != nil {
		srv.hooks.OnMsgDropped = wrapOnMsgDroppedHook(wrappers.onMsgDropped)
	}
	if wrappers.onWillPublish != nil {
		srv.hooks.OnWillPublish = wrapOnWillPublishHook(wrappers.onWillPublish)
	}
	if wrappers.onWillPublished != nil {
		srv.hooks.OnWillPublished = wrapOnWillPublishedHook(wrappers.onWillPublished)
	}
}

func wrapOnAcceptHook(wrappers []OnAcceptWrapper) OnAccept {
	onAccept := func(ctx context.Context, conn net.Conn) bool {
		return true
	}
	for i := len(wrappers); i > 0; i-- {
		onAccept = wrappers[i-1](onAccept)
	}
	return onAccept
}

func wrapOnBasicAuthHook(wrappers []OnBasicAuthWrapper) OnBasicAuth {
	onBasicAuth := func(ctx context.Context, client Client, req *ConnectRequest) error {
		return nil
	}
	for i := len(wrappers); i > 0; i-- {
		onBasicAuth = wrappers[i-1](onBasicAuth)
	}
	return onBasicAuth
}

func wrapOnEnhancedAuthHook(wrappers []OnEnhancedAuthWrapper) OnEnhancedAuth {
	onEnhancedAuth := func(ctx context.Context, client Client, req *ConnectRequest) (resp *EnhancedAuthResponse, err error) {
		return &EnhancedAuthResponse{
			Continue: false,
		}, nil
	}
	for i := len(wrappers); i > 0; i-- {
		onEnhancedAuth = wrappers[i-1](onEnhancedAuth)
	}
	return onEnhancedAuth
}

func wrapOnReAuthHook(wrappers []OnReAuthWrapper) OnReAuth {
	onReAuth := func(ctx context.Context, client Client, auth *packets.Auth) (*AuthResponse, error) {
		return &AuthResponse{
			Continue: false,
		}, nil
	}
	for i := len(wrappers); i > 0; i-- {
		onReAuth = wrappers[i-1](onReAuth)
	}
	return onReAuth
}

func wrapOnConnectedHook(wrappers []OnConnectedWrapper) OnConnected {
	onConnected := func(ctx context.Context, client Client) {}
	for i := len(wrappers); i > 0; i-- {
		onConnected = wrappers[i-1](onConnected)
	}
	return onConnected
}

func wrapOnSessionCreatedHook(wrappers []OnSessionCreatedWrapper) OnSessionCreated {
	onSessionCreated := func(ctx context.Context, client Client) {}
	for i := len(wrappers); i > 0; i-- {
		onSessionCreated = wrappers[i-1](onSessionCreated)
	}
	return onSessionCreated
}

func wrapOnSessionResumedHook(wrappers []OnSessionResumedWrapper) OnSessionResumed {
	onSessionResumed := func(ctx context.Context, client Client) {}
	for i := len(wrappers); i > 0; i-- {
		onSessionResumed = wrappers[i-1](onSessionResumed)
	}
	return onSessionResumed
}

func wrapOnSessionTerminatedHook(wrappers []OnSessionTerminatedWrapper) OnSessionTerminated {
	onSessionTerminated := func(ctx context.Context, clientID string, reason SessionTerminatedReason) {}
	for i := len(wrappers); i > 0; i-- {
		onSessionTerminated = wrappers[i-1](onSessionTerminated)
	}
	return onSessionTerminated
}

func wrapOnSubscribeHook(wrappers []OnSubscribeWrapper) OnSubscribe {
	onSubscribe := func(ctx context.Context, client Client, req *SubscribeRequest) error {
		return nil
	}
	for i := len(wrappers); i > 0; i-- {
		onSubscribe = wrappers[i-1](onSubscribe)
	}
	return onSubscribe
}

func wrapOnSubscribedHook(wrappers []OnSubscribedWrapper) OnSubscribed {
	onSubscribed := func(ctx context.Context, client Client, subscription *gmqtt.Subscription) {}
	for i := len(wrappers); i > 0; i-- {
		onSubscribed = wrappers[i-1](onSubscribed)
	}
	return onSubscribed
}

func wrapOnUnsubscribeHook(wrappers []OnUnsubscribeWrapper) OnUnsubscribe {
	onUnsubscribe := func(ctx context.Context, client Client, req *UnsubscribeRequest) error {
		return nil
	}
	for i := len(wrappers); i > 0; i-- {
		onUnsubscribe = wrappers[i-1](onUnsubscribe)
	}
	return onUnsubscribe
}

func wrapOnUnsubscribedHook(wrappers []OnUnsubscribedWrapper) OnUnsubscribed {
	onUnsubscribed := func(ctx context.Context, client Client, topicName string) {}
	for i := len(wrappers); i > 0; i-- {
		onUnsubscribed = wrappers[i-1](onUnsubscribed)
	}
	return onUnsubscribed
}

func wrapOnMsgArrivedHook(wrappers []OnMsgArrivedWrapper) OnMsgArrived {
	onMsgArrived := func(ctx context.Context, client Client, req *MsgArrivedRequest) error {
		return nil
	}
	for i := len(wrappers); i > 0; i-- {
		onMsgArrived = wrappers[i-1](onMsgArrived)
	}
	return onMsgArrived
}

func wrapOnDeliveredHook(wrappers []OnDeliveredWrapper) OnDelivered {
	onDelivered := func(ctx context.Context, client Client, msg *gmqtt.Message) {}
	for i := len(wrappers); i > 0; i-- {
		onDelivered = wrappers[i-1](onDelivered)
	}
	return onDelivered
}

func wrapOnClosedHook(wrappers []OnClosedWrapper) OnClosed {
	onClosed := func(ctx context.Context, client Client, err error) {}
	for i := len(wrappers); i > 0; i-- {
		onClosed = wrappers[i-1](onClosed)
	}
	return onClosed
}

func wrapOnStopHook(wrappers []OnStopWrapper) OnStop {
	onStop := func(ctx context.Context) {}
	for i := len(wrappers); i > 0; i-- {
		onStop = wrappers[i-1](onStop)
	}
	return onStop
}

func wrapOnMsgDroppedHook(wrappers []OnMsgDroppedWrapper) OnMsgDropped {
	onMsgDropped := func(ctx context.Context, clientID string, msg *gmqtt.Message, err error) {}
	for i := len(wrappers); i > 0; i-- {
		onMsgDropped = wrappers[i-1](onMsgDropped)
	}
	return onMsgDropped
}

func wrapOnWillPublishHook(wrappers []OnWillPublishWrapper) OnWillPublish {
	onWillPublish := func(ctx context.Context, clientID string, req *WillMsgRequest) {}
	for i := len(wrappers); i > 0; i-- {
		onWillPublish = wrappers[i-1](onWillPublish)
	}
	return onWillPublish
}

func wrapOnWillPublishedHook(wrappers []OnWillPublishedWrapper) OnWillPublished {
	onWillPublished := func(ctx context.Context, clientID string, msg *gmqtt.Message) {}
	for i := len(wrappers); i > 0; i-- {
		onWillPublished = wrappers[i-1](onWillPublished)
	}
	return onWillPublished
}

func (srv *server) loadPlugins() error {
	for _, p := range srv.plugins {
		zaplog.Info("loading plugin", zap.String("name", p.Name()))
		err := p.Load(srv)
		if err != nil {
			return err
		}
	}
	return nil
}
