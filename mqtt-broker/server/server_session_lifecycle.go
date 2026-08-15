package server

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/persistence/unack"
	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

func setWillProperties(willPpt *packets.Properties, msg *gmqtt.Message) {
	if willPpt != nil {
		if willPpt.PayloadFormat != nil {
			msg.PayloadFormat = *willPpt.PayloadFormat
		}
		if willPpt.MessageExpiry != nil {
			msg.MessageExpiry = *willPpt.MessageExpiry
		}
		if willPpt.ContentType != nil {
			msg.ContentType = string(willPpt.ContentType)
		}
		if willPpt.ResponseTopic != nil {
			msg.ResponseTopic = string(willPpt.ResponseTopic)
		}
		if willPpt.CorrelationData != nil {
			msg.CorrelationData = willPpt.CorrelationData
		}
		msg.UserProperties = willPpt.User
	}
}

func (srv *server) lockDuplicatedID(c *client) (oldSession *gmqtt.Session, err error) {
	for {
		srv.mu.Lock()
		oldSession, err = srv.sessionStore.Get(c.opts.ClientID)
		if err != nil {
			srv.mu.Unlock()
			zaplog.Error("fail to get session",
				zap.String("remote_addr", c.rwc.RemoteAddr().String()),
				zap.String("client_id", c.opts.ClientID))
			return
		}
		if oldSession != nil {
			oldClient := srv.clients[oldSession.ClientID]
			srv.mu.Unlock()
			if oldClient == nil {
				srv.mu.Lock()
				break
			}
			// MQTT allows a new connection to take over the same ClientID session.
			// Release srv.mu before waiting for oldClient shutdown to avoid deadlock.
			zaplog.Info("logging with duplicate ClientID",
				zap.String("remote", c.rwc.RemoteAddr().String()),
				zap.String("client_id", oldSession.ClientID),
			)
			oldClient.setError(codes.NewError(codes.SessionTakenOver))
			oldClient.Close()
			<-oldClient.closed
			continue
		}
		break
	}
	return
}

// registerClient wires session state, queue/unack stores, hooks, and runtime
// maps after authentication succeeds.
func (srv *server) registerClient(connect *packets.Connect, client *client) (sessionResume bool, err error) {
	var qs queue.Store
	var ua unack.Store
	var oldSession *gmqtt.Session
	now := time.Now()
	oldSession, err = srv.lockDuplicatedID(client)
	if err != nil {
		return
	}
	defer func() {
		err = srv.establishClientSessionLocked(connect, client, qs, ua, sessionResume, err)
		srv.mu.Unlock()
	}()

	client.setConnected(time.Now())
	if srv.hooks.OnConnected != nil {
		srv.hooks.OnConnected(context.Background(), client)
	}
	srv.statsManager.clientConnected(client.opts.ClientID)

	if oldSession != nil {
		sessionResume, qs, ua, err = srv.prepareExistingSessionLocked(connect, client, oldSession, now)
		if sessionResume && err != nil {
			return
		}
	}
	if !sessionResume {
		qs, ua, err = srv.createNewSessionStoresLocked(client)
		if err != nil {
			return
		}
		zaplog.Info("logged in with new session",
			zap.String("remote_addr", client.rwc.RemoteAddr().String()),
			zap.String("client_id", client.opts.ClientID),
		)
	}
	delete(srv.offlineClients, client.opts.ClientID)
	return
}

func (srv *server) prepareExistingSessionLocked(connect *packets.Connect, client *client, oldSession *gmqtt.Session, now time.Time) (sessionResume bool, qs queue.Store, ua unack.Store, err error) {
	sessionResume = shouldResumeSession(oldSession, connect, now)
	if !sessionResume {
		err = srv.replaceOldSessionLocked(client, oldSession)
		return
	}
	qs, ua, sessionResume, err = srv.resumeSessionStoresLocked(client)
	return
}

func shouldResumeSession(oldSession *gmqtt.Session, connect *packets.Connect, now time.Time) bool {
	return !connect.CleanStart && !oldSession.IsExpired(now)
}

func (srv *server) replaceOldSessionLocked(client *client, oldSession *gmqtt.Session) (err error) {
	err = srv.sessionTerminatedLocked(oldSession.ClientID, TakenOverTermination)
	if err != nil {
		err = fmt.Errorf("session terminated fail: %w", err)
		zaplog.Error("session terminated fail", zap.Error(err))
	}
	// When an old session cannot be resumed, flush any delayed will path.
	if w, ok := srv.willMessage[client.opts.ClientID]; ok {
		w.signal(true)
	}
	return
}

func (srv *server) resumeSessionStoresLocked(client *client) (qs queue.Store, ua unack.Store, sessionResume bool, err error) {
	sessionResume = true
	qs = srv.queueStore[client.opts.ClientID]
	if qs != nil {
		err = qs.Init(&queue.InitOptions{
			CleanStart:     false,
			Version:        client.version,
			ReadBytesLimit: client.opts.ClientMaxPacketSize,
			Notifier:       client.queueNotifier,
		})
		if err != nil {
			return
		}
	}
	ua = srv.unackStore[client.opts.ClientID]
	if ua != nil {
		err = ua.Init(false)
		if err != nil {
			return
		}
	}
	if ua == nil || qs == nil {
		// Persistence backends may lose queue or unack state independently.
		// Downgrade to a new session instead of resuming partial QoS state.
		sessionResume = false
		zaplog.Error("detect inconsistent session state",
			zap.String("remote_addr", client.rwc.RemoteAddr().String()),
			zap.String("client_id", client.opts.ClientID))
		return
	}
	zaplog.Info("logged in with session reuse",
		zap.String("remote_addr", client.rwc.RemoteAddr().String()),
		zap.String("client_id", client.opts.ClientID))
	return
}

func (srv *server) createNewSessionStoresLocked(client *client) (qs queue.Store, ua unack.Store, err error) {
	// Init replaces the notifier with client.queueNotifier.
	qs, err = srv.persistence.NewQueueStore(srv.config, nil, client.opts.ClientID)
	if err != nil {
		return
	}
	err = qs.Init(&queue.InitOptions{
		CleanStart:     true,
		Version:        client.version,
		ReadBytesLimit: client.opts.ClientMaxPacketSize,
		Notifier:       client.queueNotifier,
	})
	if err != nil {
		return
	}

	ua, err = srv.persistence.NewUnackStore(srv.config, client.opts.ClientID)
	if err != nil {
		return
	}
	err = ua.Init(true)
	return
}

func (srv *server) establishClientSessionLocked(connect *packets.Connect, client *client, qs queue.Store, ua unack.Store, sessionResume bool, err error) error {
	if err != nil {
		return err
	}
	sess := srv.newSession(connect, client)
	err = srv.sessionStore.Set(sess)
	if err != nil {
		return err
	}
	client.session = sess
	srv.registerClientStateLocked(client, qs, ua, sessionResume)
	return nil
}

func (srv *server) newSession(connect *packets.Connect, client *client) *gmqtt.Session {
	willDelayInterval, expiryInterval := srv.sessionIntervals(connect, client)
	return &gmqtt.Session{
		ClientID:          client.opts.ClientID,
		Will:              newWillMessage(connect),
		ConnectedAt:       time.Now(),
		WillDelayInterval: willDelayInterval,
		ExpiryInterval:    expiryInterval,
	}
}

func newWillMessage(connect *packets.Connect) *gmqtt.Message {
	if !connect.WillFlag {
		return nil
	}
	willMsg := &gmqtt.Message{
		QoS:     connect.WillQos,
		Topic:   string(connect.WillTopic),
		Payload: connect.WillMsg,
	}
	setWillProperties(connect.WillProperties, willMsg)
	return willMsg
}

func (srv *server) sessionIntervals(connect *packets.Connect, client *client) (willDelayInterval, expiryInterval uint32) {
	if packets.IsVersion3X(client.version) && !connect.CleanStart {
		expiryInterval = uint32(srv.config.MQTT.SessionExpiry.Seconds())
	} else if connect.Properties != nil {
		willDelayInterval = convertUint32(connect.WillProperties.WillDelayInterval, 0)
		expiryInterval = client.opts.SessionExpiry
	}
	return
}

func (srv *server) registerClientStateLocked(client *client, qs queue.Store, ua unack.Store, sessionResume bool) {
	if sessionResume {
		// If the session resumes within Will Delay Interval, do not publish will.
		if w, ok := srv.willMessage[client.opts.ClientID]; ok {
			w.signal(false)
		}
		if srv.hooks.OnSessionResumed != nil {
			srv.hooks.OnSessionResumed(context.Background(), client)
		}
		srv.statsManager.sessionActive(false)
	} else {
		if srv.hooks.OnSessionCreated != nil {
			srv.hooks.OnSessionCreated(context.Background(), client)
		}
		srv.statsManager.sessionActive(true)
	}
	srv.clients[client.opts.ClientID] = client
	srv.unackStore[client.opts.ClientID] = ua
	srv.queueStore[client.opts.ClientID] = qs
	client.queueStore = qs
	client.unackStore = ua
	if client.version == packets.Version5 {
		client.topicAliasManager = srv.newTopicAliasManager(client.config, client.opts.ClientTopicAliasMax, client.opts.ClientID)
	}
}
