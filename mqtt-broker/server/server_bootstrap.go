// 文件用途：承接 broker 的轻量初始化装配入口，负责默认实例构造、选项应用、初始化顺序编排与对外 Init 门面。
// 核心逻辑：把 plugin hook、persistence/session 恢复、client service、topic alias manager、API registrar 与插件加载串成稳定启动主链。
// 使用注意：这里现在只保留“初始化顺序”职责；真正的 persistence 打开、session store 枚举与离线状态恢复已下沉到 `server_persistence_bootstrap.go`。
// 重构建议：后续如继续压缩，可再把 `init(...)` 的阶段编排抽成更清晰的 bootstrap plan/helper，但不要改变当前调用顺序。
package server

import (
	"time"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/persistence/unack"
	retained_trie "github.com/DrmagicE/gmqtt/retained/trie"
)

func defaultServer() *server {
	srv := &server{
		status:         serverStatusInit,
		exitChan:       make(chan struct{}),
		exitedChan:     make(chan struct{}),
		clients:        make(map[string]*client),
		offlineClients: make(map[string]time.Time),
		willMessage:    make(map[string]*willMsg),
		retainedDB:     retained_trie.NewStore(),
		config:         config.DefaultConfig(),
		queueStore:     make(map[string]queue.Store),
		unackStore:     make(map[string]unack.Store),
	}
	srv.publishService = &publishService{server: srv}
	return srv
}

// New returns a gmqtt server instance with the given options
func New(opts ...Options) *server {
	srv := defaultServer()
	for _, fn := range opts {
		fn(srv)
	}
	return srv
}

func (srv *server) applyOptions(opts ...Options) {
	for _, fn := range opts {
		fn(srv)
	}
}

func (srv *server) initClientServices() {
	srv.statsManager = newStatsManager(srv.subscriptionsDB)
	srv.clientService = &clientService{
		srv:          srv,
		sessionStore: srv.sessionStore,
	}
}

func (srv *server) init(opts ...Options) (err error) {
	srv.applyOptions(opts...)
	if err = srv.initPluginHooks(); err != nil {
		return err
	}
	_, peType, err := srv.initPersistence()
	if err != nil {
		return err
	}
	sts, cids, err := srv.initSessionStores(peType)
	if err != nil {
		return err
	}
	srv.initClientServices()
	if err = srv.restoreOfflineSessionState(sts, cids, peType); err != nil {
		return err
	}
	if err = srv.initTopicAliasManager(); err != nil {
		return err
	}
	if err = srv.initAPIRegistrar(); err != nil {
		return err
	}
	return srv.loadPlugins()
}

// Init initialises the options.
func (srv *server) Init(opts ...Options) (err error) {
	srv.initOnce.Do(func() {
		err = srv.init(opts...)
	})
	return err
}
