// 文件用途：维护 plugin\admin\hooks.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package admin

import (
	"context"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/server"
)

func (a *Admin) HookWrapper() server.HookWrapper {
	return server.HookWrapper{
		OnSessionCreatedWrapper:    a.OnSessionCreatedWrapper,
		OnSessionResumedWrapper:    a.OnSessionResumedWrapper,
		OnClosedWrapper:            a.OnClosedWrapper,
		OnSessionTerminatedWrapper: a.OnSessionTerminatedWrapper,
		OnSubscribedWrapper:        a.OnSubscribedWrapper,
		OnUnsubscribedWrapper:      a.OnUnsubscribedWrapper,
	}
}

func (a *Admin) OnSessionCreatedWrapper(pre server.OnSessionCreated) server.OnSessionCreated {
	return func(ctx context.Context, client server.Client) {
		pre(ctx, client)
		a.store.addClient(client)
	}
}

func (a *Admin) OnSessionResumedWrapper(pre server.OnSessionResumed) server.OnSessionResumed {
	return func(ctx context.Context, client server.Client) {
		pre(ctx, client)
		a.store.addClient(client)
	}
}

func (a *Admin) OnClosedWrapper(pre server.OnClosed) server.OnClosed {
	return func(ctx context.Context, client server.Client, err error) {
		pre(ctx, client, err)
		a.store.setClientDisconnected(client.ClientOptions().ClientID)
	}
}

func (a *Admin) OnSessionTerminatedWrapper(pre server.OnSessionTerminated) server.OnSessionTerminated {
	return func(ctx context.Context, clientID string, reason server.SessionTerminatedReason) {
		pre(ctx, clientID, reason)
		a.store.removeClient(clientID)
	}
}

func (a *Admin) OnSubscribedWrapper(pre server.OnSubscribed) server.OnSubscribed {
	return func(ctx context.Context, client server.Client, subscription *gmqtt.Subscription) {
		pre(ctx, client, subscription)
		a.store.addSubscription(client.ClientOptions().ClientID, subscription)
	}
}

func (a *Admin) OnUnsubscribedWrapper(pre server.OnUnsubscribed) server.OnUnsubscribed {
	return func(ctx context.Context, client server.Client, topicName string) {
		pre(ctx, client, topicName)
		a.store.removeSubscription(client.ClientOptions().ClientID, topicName)
	}
}
