// 文件用途：C6 协议网关（ROADMAP C6）——把库级 CoAP/LwM2M 栈组装为平台可启动的 UDP 接入层。
// 核心链路：config.enabled 为 true 时，构建 coap.Registry（挂 LwM2M /rd 注册 + 对象资源），
//
//	以 goroutine 启动 UDP 监听；可选装配 TelemetryBridge 把资源写入汇入 uplink 管道（P1-C）。
//
// 边界：默认关闭；UDP 服务为进程级常驻，Stop 仅置位（ListenAndServe 由进程退出回收）。
package protocolgw

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"aetherlink-iot/backend/internal/coap"
	"aetherlink-iot/backend/internal/lwm2m"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config 协议网关配置。
type Config struct {
	Enabled bool
	Addr    string // 完整 UDP 地址（host:port）
}

// DefaultConfig 读取 viper（键 protocols.coap.enabled/port，host 可 protocols.coap.host），默认关闭。
func DefaultConfig() Config {
	host := viper.GetString("protocols.coap.host")
	if host == "" {
		host = "0.0.0.0"
	}
	port := viper.GetInt("protocols.coap.port")
	if port <= 0 {
		port = 5683
	}
	return Config{
		Enabled: viper.GetBool("protocols.coap.enabled"),
		Addr:    net.JoinHostPort(host, strconv.Itoa(port)),
	}
}

// BuildRegistry 组装纯接入层（无遥测汇入）注册表：LwM2M /rd 注册 + 单一共享对象存储读写。
// 无 bridge 即无归因语义，对象写入落共享 store（单机/演示形态，行为与既往一致）。
func BuildRegistry() *coap.Registry {
	reg := coap.NewRegistry()
	reg.Register("/rd*", lwm2m.NewRegistry().HandleRegister())
	store := lwm2m.NewObjectStore()
	lwm2m.BindObjects(reg, 3303, store)
	return reg
}

// BuildRegistryWithIsolation 组装多客户端隔离注册表（TB-22）：
//  1. "/rd*" 挂载（前缀）使 DELETE /rd/{id} 去注册可达（原 "/rd" 精确挂载下去注册 404），
//     注册/去注册事件回调 onEvent（上层据此挂接 per-endpoint store，可 nil）；
//  2. "/3303*" 对象读写按 CoAP 源地址路由到 lookup(addr) 返回的端点 store，
//     未注册源一律 4.04（fail-closed）。
func BuildRegistryWithIsolation(onEvent func(lwm2m.RegistryEvent), lookup func(addr string) *lwm2m.ObjectStore) *coap.Registry {
	reg := coap.NewRegistry()
	reg.Register("/rd*", lwm2m.NewRegistry().HandleRegisterWithEvents(onEvent))
	if lookup != nil {
		reg.Register("/3303*", isolatedObjectRouter{lookup: lookup}.handle)
	}
	return reg
}

// isolatedObjectRouter 按源地址分发对象读写的路由器（多客户端隔离核心）。
// 每个 UDP 源地址在注册时绑定到端点，其对象读写落到该端点独享的 ObjectStore，
// 写入经 OnChange 携带端点名进入遥测管道——端点间资源值互不可见。
type isolatedObjectRouter struct {
	lookup func(addr string) *lwm2m.ObjectStore
}

func (r isolatedObjectRouter) handle(req *coap.Message) (coap.Code, []byte, int, error) {
	store := r.lookup(req.RemoteAddr)
	if store == nil {
		return coap.CodeNotFound, []byte("lwm2m: source not registered"), 0, nil
	}
	return store.ObjectHandler()(req)
}

// Gateway CoAP 网关实例。
type Gateway struct {
	cfg    Config
	reg    *coap.Registry
	log    *logrus.Logger
	bridge *TelemetryBridge
	// started 仅做一次性标记（服务为进程级常驻）。
	started chan struct{}
}

// GatewayOption 网关可选装配项。
type GatewayOption func(*gatewayOptions)

type gatewayOptions struct {
	bridge *TelemetryBridge
}

// WithTelemetry 装配遥测汇入桥（nil 等价于不装配——保持纯接入层语义）。
func WithTelemetry(bridge *TelemetryBridge) GatewayOption {
	return func(o *gatewayOptions) { o.bridge = bridge }
}

// Start 按配置启动网关；未启用返回 nil,nil。启动失败（如端口占用）返回错误。
// bridge 非空时：多客户端隔离拓扑——/rd 注册/去注册事件挂接 per-endpoint store，
// 对象写入按源地址路由并异步汇入 uplink 管道；无 bridge 为纯接入层（共享 store）。
func Start(cfg Config, log *logrus.Logger, opts ...GatewayOption) (*Gateway, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if log == nil {
		log = logrus.New()
	}
	o := &gatewayOptions{}
	for _, opt := range opts {
		opt(o)
	}

	var reg *coap.Registry
	if o.bridge != nil {
		reg = BuildRegistryWithIsolation(o.bridge.OnEvent, o.bridge.StoreForAddr)
	} else {
		reg = BuildRegistry()
	}
	g := &Gateway{cfg: cfg, reg: reg, log: log, bridge: o.bridge, started: make(chan struct{})}

	if o.bridge != nil {
		go o.bridge.Run()
	}

	server := &coap.Server{Registry: g.reg}
	go func() {
		close(g.started)
		if err := server.ListenAndServe(cfg.Addr); err != nil {
			log.WithError(err).Error("coap gateway server exited")
		}
	}()
	select {
	case <-g.started:
	case <-time.After(3 * time.Second):
		return nil, fmt.Errorf("coap gateway failed to start within timeout")
	}
	log.WithField("addr", cfg.Addr).Info("coap/lwm2m gateway listening")
	return g, nil
}
