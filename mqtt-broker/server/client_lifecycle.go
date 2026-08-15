// 文件用途：集中管理单个 MQTT 客户端连接的生命周期清理与 goroutine 编排。
// 核心逻辑：启动读写循环、连接成功后的 worker、readHandle 分发、队列关闭和 packet id limiter 清理。
// 使用注意：这里控制连接退出顺序，调整 WaitGroup、queueStore 或 packet limiter 清理时要避免 goroutine 泄漏。
// 重构建议：后续如继续拆分，应优先围绕 read/write loop 装配做小步纯搬迁，不要改变 close/internalClose 的调用顺序。
package server

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

func (client *client) internalClose() {
	if client.IsConnected() {
		client.notifyClosedHook()
		client.unregister(client)
		client.server.statsManager.clientDisconnected(client.opts.ClientID)
	}
	client.releaseConnectionBuffers()
	close(client.closed)
}

func (client *client) notifyClosedHook() {
	if client.server.hooks.OnClosed != nil {
		client.server.hooks.OnClosed(context.Background(), client, client.err)
	}
}

func (client *client) releaseConnectionBuffers() {
	putBufioReader(client.bufr)
	putBufioWriter(client.bufw)
}

// readHandle 分发 readLoop 已接收的协议包，并统一处理包大小校验和协议错误。
func (client *client) readHandle() {
	var err error
	defer func() {
		if re := recover(); re != nil {
			err = errors.New(fmt.Sprint(re))
		}
		client.setError(err)
	}()
	for packet := range client.in {
		if packetErr := client.validateIncomingPacketSize(packet); packetErr != nil {
			err = packetErr
			return
		}
		if packetErr := client.dispatchIncomingPacket(packet); packetErr != nil {
			err = packetErr
			return
		}
	}
}

// serve 运行客户端连接生命周期，直到读循环退出后执行资源清理。
func (client *client) serve() {
	defer client.internalClose()

	readWg := client.startReadLoop()
	client.startWriteLoop()
	if ok := client.connectWithTimeOut(); ok {
		client.startConnectedWorkers()
	}

	readWg.Wait()
	client.closeLifecycleStores()
	client.wg.Wait()
	_ = client.rwc.Close()
}

func (client *client) startReadLoop() *sync.WaitGroup {
	readWg := &sync.WaitGroup{}
	readWg.Add(1)
	go func() {
		client.readLoop()
		readWg.Done()
	}()
	return readWg
}

func (client *client) startWriteLoop() {
	client.wg.Add(1)
	go func() {
		client.writeLoop()
		client.wg.Done()
	}()
}

func (client *client) startConnectedWorkers() {
	client.wg.Add(2)
	go func() {
		client.pollMessageHandler()
		client.wg.Done()
	}()
	go func() {
		client.readHandle()
		client.wg.Done()
	}()
}

func (client *client) closeLifecycleStores() {
	if client.queueStore != nil {
		if qerr := client.queueStore.Close(); qerr != nil {
			zaplog.Error("关闭客户端消息队列失败", zap.String("client_id", client.opts.ClientID), zap.Error(qerr))
		}
	}
	if client.pl != nil {
		client.pl.close()
	}
}
