// 文件用途：承接 broker 初始化阶段的 persistence、session store 与离线 session 恢复链。
// 核心逻辑：集中打开 persistence、恢复订阅/会话 store、重建离线 queue/unack 状态，并初始化 topic alias manager。
// 使用注意：这里的恢复顺序、日志时机和离线 session 重建语义都会影响 broker 启动后的状态一致性，调整前需谨慎。
// 重构建议：后续可继续把 persistence 打开、session 枚举和离线状态恢复拆成更细 helper，但不要改变当前初始化顺序。
package server

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt"
)

func (srv *server) initPersistence() (Persistence, string, error) {
	peType := srv.config.Persistence.Type
	newFn := persistenceFactories[peType]
	if newFn == nil {
		return nil, "", fmt.Errorf("persistence factory: %s not found", peType)
	}
	pe, err := newFn(srv.config)
	if err != nil {
		return nil, "", err
	}
	if err := pe.Open(); err != nil {
		return nil, "", err
	}
	zaplog.Info("open persistence succeeded", zap.String("type", peType))
	srv.persistence = pe
	return pe, peType, nil
}

func (srv *server) initSessionStores(peType string) ([]*gmqtt.Session, []string, error) {
	subscriptionsDB, err := srv.persistence.NewSubscriptionStore(srv.config)
	if err != nil {
		return nil, nil, err
	}
	srv.subscriptionsDB = subscriptionsDB

	st, err := srv.persistence.NewSessionStore(srv.config)
	if err != nil {
		return nil, nil, err
	}
	srv.sessionStore = st

	var (
		sts  []*gmqtt.Session
		cids []string
	)
	if err := st.Iterate(func(session *gmqtt.Session) bool {
		sts = append(sts, session)
		cids = append(cids, session.ClientID)
		return true
	}); err != nil {
		return nil, nil, err
	}

	zaplog.Info("init session store succeeded", zap.String("type", peType), zap.Int("session_total", len(cids)))
	return sts, cids, nil
}

func (srv *server) restoreOfflineSessionState(sts []*gmqtt.Session, cids []string, peType string) error {
	for _, v := range sts {
		q, err := srv.persistence.NewQueueStore(srv.config, defaultNotifier(srv.hooks.OnMsgDropped, srv.statsManager, v.ClientID), v.ClientID)
		if err != nil {
			return err
		}
		srv.queueStore[v.ClientID] = q
		srv.offlineClients[v.ClientID] = time.Now().Add(time.Duration(v.ExpiryInterval) * time.Second)

		ua, err := srv.persistence.NewUnackStore(srv.config, v.ClientID)
		if err != nil {
			return err
		}
		srv.unackStore[v.ClientID] = ua
	}
	zaplog.Info("init queue store succeeded", zap.String("type", peType), zap.Int("session_total", len(cids)))
	zaplog.Info("init subscription store succeeded", zap.String("type", peType), zap.Int("client_total", len(cids)))
	return srv.subscriptionsDB.Init(cids)
}

func (srv *server) initTopicAliasManager() error {
	newFactory := topicAliasMgrFactory[srv.config.TopicAliasManager.Type]
	if newFactory == nil {
		return fmt.Errorf("topic alias manager : %s not found", srv.config.TopicAliasManager.Type)
	}
	srv.newTopicAliasManager = newFactory
	return nil
}
