// 文件用途：定义 broker 构造期 Options，用于注入配置、监听器、插件、持久化与 retained store。
// 核心逻辑：通过函数式选项组装 server 依赖，供 New/Init 阶段统一应用。
// 使用注意：Options 是启动配置兼容面，新增选项应保持默认行为不变。
// 重构建议：后续可按网络、插件、存储三类拆分选项说明，降低启动配置阅读成本。

package server

import (
	"net"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/retained"
	"go.uber.org/zap"
)

type Options func(srv *server)

// WithConfig set the config of the server
func WithConfig(config config.Config) Options {
	return func(srv *server) {
		srv.config = config
	}
}

// WithTCPListener set  tcp listener(s) of the server. Default listen on  :1883.
func WithTCPListener(lns ...net.Listener) Options {
	return func(srv *server) {
		srv.tcpListener = append(srv.tcpListener, lns...)
	}
}

// WithWebsocketServer set  websocket server(s) of the server.
func WithWebsocketServer(ws ...*WsServer) Options {
	return func(srv *server) {
		srv.websocketServer = ws
	}
}

// WithPlugin set plugin(s) of the server.
func WithPlugin(plugin ...Plugin) Options {
	return func(srv *server) {
		srv.plugins = append(srv.plugins, plugin...)
	}
}

// WithHook set hooks of the server. Notice: WithPlugin() will overwrite hooks.
func WithHook(hooks Hooks) Options {
	return func(srv *server) {
		srv.hooks = hooks
	}
}

func WithLogger(logger *zap.Logger) Options {
	return func(srv *server) {
		zaplog = logger
	}
}

// WithRetainedStore set retained db of the server. Notice: WithRetainedStore(s) will overwrite retainedDB.
func WithRetainedStore(store retained.Store) Options {
	return func(srv *server) {
		srv.retainedDB = store
	}
}
