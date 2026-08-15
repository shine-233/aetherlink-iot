// 文件用途：实现离线/持久化队列的丢弃通知器，负责统计 message dropped 并触发插件 hook。
// 核心逻辑：在消息过期、inflight 丢弃等场景释放 packet id、记录统计并调用 OnMsgDropped。
// 使用注意：该路径影响离线队列可靠性和 QoS/inflight 回收，不能绕过 stats 或 dropHook。
// 重构建议：后续可补充 dropped reason 映射说明，并把日志、统计、hook 三步封装为更清晰的 helper。

package server

import (
	"context"

	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/queue"
)

// queueNotifier implements queue.Notifier interface.
type queueNotifier struct {
	dropHook OnMsgDropped
	sts      *statsManager
	cli      *client
}

// defaultNotifier is used to init the notifier when using a persistent session store (e.g redis) which can load session data
// while bootstrapping.
func defaultNotifier(dropHook OnMsgDropped, sts *statsManager, clientID string) *queueNotifier {
	return &queueNotifier{
		dropHook: dropHook,
		sts:      sts,
		cli:      &client{opts: &ClientOptions{ClientID: clientID}, status: Connected + 1},
	}
}

func (q *queueNotifier) notifyDropped(msg *gmqtt.Message, err error) {
	cid := q.cli.opts.ClientID
	zaplog.Warn("message dropped", zap.String("client_id", cid), zap.Error(err))
	q.sts.messageDropped(msg.QoS, q.cli.opts.ClientID, err)
	if q.dropHook != nil {
		q.dropHook(context.Background(), cid, msg, err)
	}
}

func (q *queueNotifier) NotifyDropped(elem *queue.Elem, err error) {
	cid := q.cli.opts.ClientID
	if err == queue.ErrDropExpiredInflight && q.cli.IsConnected() {
		q.cli.pl.release(elem.ID())
	}
	if pub, ok := elem.MessageWithID.(*queue.Publish); ok {
		q.notifyDropped(pub.Message, err)
	} else {
		zaplog.Warn("message dropped", zap.String("client_id", cid), zap.Error(err))
	}
}

func (q *queueNotifier) NotifyInflightAdded(delta int) {
	cid := q.cli.opts.ClientID
	if delta > 0 {
		q.sts.addInflight(cid, uint64(delta))
	}
	if delta < 0 {
		q.sts.decInflight(cid, uint64(-delta))
	}

}

func (q *queueNotifier) NotifyMsgQueueAdded(delta int) {
	cid := q.cli.opts.ClientID
	if delta > 0 {
		q.sts.addQueueLen(cid, uint64(delta))
	}
	if delta < 0 {
		q.sts.decQueueLen(cid, uint64(-delta))
	}
}
