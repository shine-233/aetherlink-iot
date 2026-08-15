// 文件用途：承接 broker 客户端查询/终止服务的适配层，实现对在线/离线会话的统一读控入口。
// 核心逻辑：围绕 `ClientService` 暴露 session 遍历、在线客户端读取和 session 终止，并保持既有锁顺序。
// 使用注意：`TerminateSession` 位于协议与会话一致性的敏感路径，不能随意改变在线/离线分支和锁持有范围。
// 重构建议：后续如继续细化，可把在线查询与终止语义分成更小协作者，但不要改变当前 broker 行为契约。
package server

import (
	"fmt"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/session"
)

type clientService struct {
	srv          *server
	sessionStore session.Store
}

func (c *clientService) IterateSession(fn session.IterateFn) error {
	return c.sessionStore.Iterate(fn)
}

func (c *clientService) IterateClient(fn ClientIterateFn) {
	c.srv.mu.Lock()
	defer c.srv.mu.Unlock()

	for _, v := range c.srv.clients {
		if !fn(v) {
			return
		}
	}
}

func (c *clientService) GetClient(clientID string) Client {
	c.srv.mu.Lock()
	defer c.srv.mu.Unlock()
	if c, ok := c.srv.clients[clientID]; ok {
		return c
	}
	return nil
}

func (c *clientService) GetSession(clientID string) (*gmqtt.Session, error) {
	return c.sessionStore.Get(clientID)
}

func (c *clientService) TerminateClientIfCurrent(expected Client) bool {
	if expected == nil {
		return false
	}
	options := expected.ClientOptions()
	if options == nil {
		return false
	}
	clientID := options.ClientID
	if clientID == "" {
		return false
	}

	c.srv.mu.Lock()
	defer c.srv.mu.Unlock()
	current, ok := c.srv.clients[clientID]
	if !ok || current != expected {
		return false
	}
	atomic.StoreInt32(&current.forceRemoveSession, 1)
	current.Close()
	return true
}

func (c *clientService) TerminateSession(clientID string) {
	c.srv.mu.Lock()
	defer c.srv.mu.Unlock()
	if cli, ok := c.srv.clients[clientID]; ok {
		atomic.StoreInt32(&cli.forceRemoveSession, 1)
		cli.Close()
		return
	}
	if _, ok := c.srv.offlineClients[clientID]; ok {
		err := c.srv.sessionTerminatedLocked(clientID, NormalTermination)
		if err != nil {
			err = fmt.Errorf("session terminated fail: %s", err.Error())
			zaplog.Error("session terminated fail", zap.Error(err))
		}
	}
}
