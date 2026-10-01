// 文件用途：维护 cmd\gmqttd\command\start.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package command

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/pkg/pidfile"
	"github.com/DrmagicE/gmqtt/server"
)

var (
	ConfigFile string
	logger     *zap.Logger
)

func must(err error) {
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

// installSignal 把进程信号接到 broker 生命周期上，实际状态机见 signal_loop.go。
func installSignal(srv server.Server) {
	reloadCh := make(chan os.Signal, 1)
	signal.Notify(reloadCh, syscall.SIGHUP)
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	timeout, warn := shutdownTimeoutFromEnv(os.Getenv(shutdownTimeoutEnv))
	if warn != "" {
		logger.Warn(warn)
	}
	loop := signalLoop{
		srv:             srv,
		logger:          logger,
		reloadCh:        reloadCh,
		stopCh:          stopCh,
		loadConfig:      loadReloadConfig,
		shutdownTimeout: timeout,
		// 收到第一个停止信号后立即解除捕获：Stop 期间再来一次 SIGTERM/Ctrl-C
		// 走 Go 默认行为直接终止进程，运维保留"二次信号强杀"的逃生口。
		releaseSignals: func() {
			signal.Stop(stopCh)
			signal.Stop(reloadCh)
		},
	}
	loop.run()
}

func GetListeners(c config.Config) (tcpListeners []net.Listener, websockets []*server.WsServer, err error) {
	for _, v := range c.Listeners {
		var ln net.Listener
		if v.Websocket != nil {
			ws := &server.WsServer{
				Server: &http.Server{Addr: v.Address},
				Path:   v.Websocket.Path,
			}
			if v.TLSOptions != nil {
				ws.KeyFile = v.Key
				ws.CertFile = v.Cert
			}
			websockets = append(websockets, ws)
			continue
		}
		if v.TLSOptions != nil {
			var cert tls.Certificate
			cert, err = tls.LoadX509KeyPair(v.Cert, v.Key)
			if err != nil {
				return
			}
			ln, err = tls.Listen("tcp", v.Address, &tls.Config{
				Certificates: []tls.Certificate{cert},
			})
		} else {
			ln, err = net.Listen("tcp", v.Address)
		}
		tcpListeners = append(tcpListeners, ln)
	}
	return
}

// NewStartCmd creates a *cobra.Command object for start command.
func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start gmqtt broker",
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			must(err)
			c, err := config.ParseConfig(ConfigFile)
			if os.IsNotExist(err) {
				must(err)
			} else {
				must(err)
			}
			// 部署期持久化覆盖 + fail-fast 校验：Compose 通过 GMQTT_PERSISTENCE_*
			// 把会话/订阅/QoS 队列切到 Redis，broker 重启不再丢离线会话与排队消息。
			applyPersistenceEnvOverrides(&c)
			must(validatePersistenceConfig(&c))
			if c.PidFile != "" {
				pid, err := pidfile.New(c.PidFile)
				if err != nil {
					must(fmt.Errorf("open pid file failed: %s", err))
				}
				defer pid.Remove()
			}

			tcpListeners, websockets, err := GetListeners(c)
			must(err)
			l, err := c.GetLogger(c.Log)
			must(err)
			logger = l

			// 添加 OnAccept Hook 来禁用 TCP Keep-Alive
			hooks := server.Hooks{
				OnAccept: func(ctx context.Context, conn net.Conn) bool {
					if tcpConn, ok := conn.(*net.TCPConn); ok {
						// 禁用 TCP Keep-Alive (Go 默认是 15 秒)
						// MQTT 应用层已经有 Keep-Alive 机制，不需要 TCP 层的 Keep-Alive
						_ = tcpConn.SetKeepAlive(false)
					}
					return true
				},
			}

			s := server.New(
				server.WithConfig(c),
				server.WithTCPListener(tcpListeners...),
				server.WithWebsocketServer(websockets...),
				server.WithLogger(l),
				server.WithHook(hooks),
			)

			err = s.Init()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
				return
			}
			go installSignal(s)
			err = s.Run()
			if err != nil {
				fmt.Fprint(os.Stderr, err.Error())
				os.Exit(1)
				return
			}
		},
	}
	return cmd
}
