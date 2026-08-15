package server

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

type willMsg struct {
	msg *gmqtt.Message
	// true 表示发送 will，false 表示丢弃 will；缓冲 1 可避免重连取消信号阻塞。
	send chan bool
}

func (w *willMsg) signal(send bool) {
	select {
	case w.send <- send:
	default:
	}
}

// sendWillLocked 发送客户端 will message，调用方必须持有 srv.mu。
// 使用注意：OnWillPublish 可以把 req.Message 置空来丢弃 will，后续拆分时必须保留这个插件语义。
func (srv *server) sendWillLocked(msg *gmqtt.Message, clientID string) {
	req := &WillMsgRequest{
		Message: msg,
	}
	if srv.hooks.OnWillPublish != nil {
		srv.hooks.OnWillPublish(context.Background(), clientID, req)
	}
	// 插件显式丢弃 will message。
	if req.Message == nil {
		return
	}
	srv.deliverMessage(clientID, msg, defaultIterateOptions(msg.Topic))
	if srv.hooks.OnWillPublished != nil {
		srv.hooks.OnWillPublished(context.Background(), clientID, req.Message)
	}
}

func (srv *server) unregisterClient(client *client) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	now := time.Now()
	if sess, err := srv.sessionStore.Get(client.opts.ClientID); sess != nil {
		storeSession := srv.shouldStoreSessionOnDisconnect(client, sess)
		srv.handleWillOnDisconnectLocked(client, sess, storeSession)
		if storeSession {
			srv.storeOfflineSessionLocked(client, sess, now)
			return
		}
	} else {
		zaplog.Error("fail to get session",
			zap.String("remote_addr", client.rwc.RemoteAddr().String()),
			zap.String("client_id", client.opts.ClientID),
			zap.Error(err))
	}
	zaplog.Info("【连接断开】logged out and cleaning session",
		zap.String("remote_addr", client.rwc.RemoteAddr().String()),
		zap.String("client_id", client.opts.ClientID),
	)
	_ = srv.sessionTerminatedLocked(client.opts.ClientID, NormalTermination)
}

func (srv *server) handleWillOnDisconnectLocked(client *client, sess *gmqtt.Session, storeSession bool) {
	if client.cleanWillFlag || sess.Will == nil {
		return
	}
	msg := sess.Will.Copy()
	if willDelayInterval := delayedWillInterval(sess, storeSession); willDelayInterval != 0 {
		srv.scheduleDelayedWillLocked(client.opts.ClientID, msg, willDelayInterval)
		return
	}
	srv.sendWillLocked(msg, client.opts.ClientID)
}

func (srv *server) scheduleDelayedWillLocked(clientID string, msg *gmqtt.Message, willDelayInterval uint32) {
	wm := &willMsg{
		msg:  msg,
		send: make(chan bool, 1),
	}
	srv.willMessage[clientID] = wm
	t := time.NewTimer(time.Duration(willDelayInterval) * time.Second)
	go func() {
		var send bool
		select {
		case send = <-wm.send:
			t.Stop()
		case <-t.C:
			send = true
		}
		srv.mu.Lock()
		defer srv.mu.Unlock()
		delete(srv.willMessage, clientID)
		// 等待 srv.mu 时可能已经发生重连取消，因此发布前再次读取信号，保证 MQTT-3.1.3-9。
		select {
		case send = <-wm.send:
		default:
		}
		if !send {
			return
		}
		srv.sendWillLocked(msg, clientID)
	}()
}

func (srv *server) shouldStoreSessionOnDisconnect(client *client, sess *gmqtt.Session) bool {
	if atomic.LoadInt32(&client.forceRemoveSession) == 1 {
		return false
	}
	sess.ExpiryInterval = disconnectSessionExpiryInterval(client, sess)
	return sess.ExpiryInterval != 0
}

func disconnectSessionExpiryInterval(client *client, sess *gmqtt.Session) uint32 {
	if client.version == packets.Version5 && client.disconnect != nil {
		return convertUint32(client.disconnect.Properties.SessionExpiryInterval, sess.ExpiryInterval)
	}
	return sess.ExpiryInterval
}

func effectiveWillDelayInterval(sess *gmqtt.Session) uint32 {
	if sess.ExpiryInterval <= sess.WillDelayInterval {
		return sess.ExpiryInterval
	}
	return sess.WillDelayInterval
}

func delayedWillInterval(sess *gmqtt.Session, storeSession bool) uint32 {
	if !storeSession {
		return 0
	}
	return effectiveWillDelayInterval(sess)
}

func (srv *server) storeOfflineSessionLocked(client *client, sess *gmqtt.Session, now time.Time) {
	expiredTime := offlineSessionExpiryTime(now, sess)
	storeOfflineClientSession(srv.offlineClients, srv.clients, client.opts.ClientID, expiredTime)
	zaplog.Info("logged out and storing session",
		zap.String("remote_addr", client.rwc.RemoteAddr().String()),
		zap.String("client_id", client.opts.ClientID),
		zap.Time("expired_at", expiredTime),
	)
}

func offlineSessionExpiryTime(now time.Time, sess *gmqtt.Session) time.Time {
	return now.Add(time.Duration(sess.ExpiryInterval) * time.Second)
}

func storeOfflineClientSession(offlineClients map[string]time.Time, clients map[string]*client, clientID string, expiredTime time.Time) {
	offlineClients[clientID] = expiredTime
	delete(clients, clientID)
}
