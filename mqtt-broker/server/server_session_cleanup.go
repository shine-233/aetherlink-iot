package server

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
)

func (srv *server) sessionTerminatedLocked(clientID string, reason SessionTerminatedReason) (err error) {
	err = srv.removeSessionLocked(clientID)
	if srv.hooks.OnSessionTerminated != nil {
		srv.hooks.OnSessionTerminated(context.Background(), clientID, reason)
	}
	srv.statsManager.sessionTerminated(clientID, reason)
	return err
}

func (srv *server) removeSessionLocked(clientID string) (err error) {
	delete(srv.clients, clientID)
	delete(srv.offlineClients, clientID)

	var errs []string
	if queueErr := srv.cleanQueueStoreLocked(clientID); queueErr != nil {
		errs = append(errs, "fail to clean message queue: "+queueErr.Error())
	}
	if sessionErr := srv.removeStoredSessionLocked(clientID); sessionErr != nil {
		errs = append(errs, "fail to remove session: "+sessionErr.Error())
	}
	if subErr := srv.unsubscribeSessionLocked(clientID); subErr != nil {
		errs = append(errs, "fail to remove subscription: "+subErr.Error())
	}

	if errs != nil {
		return errors.New(strings.Join(errs, ";"))
	}
	return nil
}

func (srv *server) cleanQueueStoreLocked(clientID string) error {
	qs := srv.queueStore[clientID]
	if qs == nil {
		return nil
	}
	err := qs.Clean()
	if err != nil {
		zaplog.Error("fail to clean message queue",
			zap.String("client_id", clientID),
			zap.Error(err))
	}
	delete(srv.queueStore, clientID)
	return err
}

func (srv *server) removeStoredSessionLocked(clientID string) error {
	err := srv.sessionStore.Remove(clientID)
	if err != nil {
		zaplog.Error("fail to remove session",
			zap.String("client_id", clientID),
			zap.Error(err))
	}
	return err
}

func (srv *server) unsubscribeSessionLocked(clientID string) error {
	err := srv.subscriptionsDB.UnsubscribeAll(clientID)
	if err != nil {
		zaplog.Error("fail to remove subscription",
			zap.String("client_id", clientID),
			zap.Error(err))
	}
	return err
}

// sessionExpireCheck periodically cleans expired offline sessions.
func (srv *server) sessionExpireCheck() {
	now := time.Now()
	srv.mu.Lock()
	for cid, expiredTime := range srv.offlineClients {
		if now.After(expiredTime) {
			zaplog.Info("session expired", zap.String("client_id", cid))
			_ = srv.sessionTerminatedLocked(cid, ExpiredTermination)
		}
	}
	srv.mu.Unlock()
}
