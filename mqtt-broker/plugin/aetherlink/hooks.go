// Hook handlers for the AetherLink MQTT broker plugin.
// Keep behavior unchanged when editing this file.

package aetherlink

import "github.com/DrmagicE/gmqtt/server"

func (t *AetherLinkPlugin) HookWrapper() server.HookWrapper {
	return t.coreHookWrapper()
}

func (t *AetherLinkPlugin) coreHookWrapper() server.HookWrapper {
	return server.HookWrapper{
		OnBasicAuthWrapper:   t.OnBasicAuthWrapper,
		OnSubscribeWrapper:   t.OnSubscribeWrapper,
		OnMsgArrivedWrapper:  t.OnMsgArrivedWrapper,
		OnWillPublishWrapper: t.OnWillPublishWrapper,
		OnConnectedWrapper:   t.OnConnectedWrapper,
		OnClosedWrapper:      t.OnClosedWrapper,
	}
}
