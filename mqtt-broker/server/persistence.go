// 文件用途：定义 broker 持久化工厂和队列、订阅、会话、未确认消息存储接口。
// 核心逻辑：为 server 初始化和 session 恢复提供统一 persistence 抽象。
// 使用注意：接口签名会影响外部 persistence 实现，调整前要确认插件和配置兼容性。
// 重构建议：后续可补充各 Store 生命周期说明，并把注册/初始化错误策略集中记录。

package server

import (
	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/persistence/session"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/persistence/unack"
)

type NewPersistence func(config config.Config) (Persistence, error)

type Persistence interface {
	Open() error
	NewQueueStore(config config.Config, defaultNotifier queue.Notifier, clientID string) (queue.Store, error)
	NewSubscriptionStore(config config.Config) (subscription.Store, error)
	NewSessionStore(config config.Config) (session.Store, error)
	NewUnackStore(config config.Config, clientID string) (unack.Store, error)
	Close() error
}
