// 文件用途：定义 broker 对外服务接口，包括发布、客户端、订阅、保留消息和统计读取能力。
// 核心逻辑：把 server 内部能力暴露为稳定接口，供插件、管理 API 和其他模块查询或操作。
// 使用注意：接口签名和语义属于扩展兼容面，新增方法应避免破坏现有插件实现。
// 重构建议：后续可按读接口、写接口和管理接口拆分说明，减少外部调用方误用成本。

package server

import (
	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/session"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/retained"
)

// Publisher provides the ability to Publish messages to the broker.
type Publisher interface {
	// Publish Publish a message to broker.
	// Calling this method will not trigger OnMsgArrived hook.
	Publish(message *gmqtt.Message)
}

// ClientIterateFn is the callback function used by ClientService.IterateClient
// Return false means to stop the iteration.
type ClientIterateFn = func(client Client) bool

// ClientService provides the ability to query and close clients.
type ClientService interface {
	IterateSession(fn session.IterateFn) error
	GetSession(clientID string) (*gmqtt.Session, error)
	GetClient(clientID string) Client
	IterateClient(fn ClientIterateFn)
	// TerminateClientIfCurrent atomically closes client only when it is still the
	// active connection registered for its client ID.
	TerminateClientIfCurrent(client Client) bool
	TerminateSession(clientID string)
}

// SubscriptionService providers the ability to query and add/delete subscriptions.
type SubscriptionService interface {
	// Subscribe adds subscriptions to a specific client.
	// Notice:
	// This method will succeed even if the client is not exists, the subscriptions
	// will affect the new client with the client id.
	Subscribe(clientID string, subscriptions ...*gmqtt.Subscription) (rs subscription.SubscribeResult, err error)
	// Unsubscribe removes subscriptions of a specific client.
	Unsubscribe(clientID string, topics ...string) error
	// UnsubscribeAll removes all subscriptions of a specific client.
	UnsubscribeAll(clientID string) error
	// Iterate iterates all subscriptions. The callback is called once for each subscription.
	// If callback return false, the iteration will be stopped.
	// Notice:
	// The results are not sorted in any way, no ordering of any kind is guaranteed.
	// This method will walk through all subscriptions,
	// so it is a very expensive operation. Do not call it frequently.
	Iterate(fn subscription.IterateFn, options subscription.IterationOptions)
	subscription.StatsReader
}

// RetainedService providers the ability to query and add/delete retained messages.
type RetainedService interface {
	retained.Store
}
