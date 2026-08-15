// 文件用途：提供 broker 内部发布服务入口，把外部调用转入 server 的订阅匹配与投递流程。
// 核心逻辑：在 server 锁保护下调用 deliverMessage，按默认订阅匹配选项分发消息。
// 使用注意：这里是跨模块发布入口，不能绕过 server 投递路径或破坏订阅匹配语义。
// 重构建议：后续如抽离 delivery handler，应让本服务依旧保持薄入口职责。

package server

import "github.com/DrmagicE/gmqtt"

type publishService struct {
	server *server
}

func (p *publishService) Publish(message *gmqtt.Message) {
	p.server.mu.Lock()
	p.server.deliverMessage("", message, defaultIterateOptions(message.Topic))
	p.server.mu.Unlock()
}
