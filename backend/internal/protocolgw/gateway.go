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

// BuildRegistry 组装 CoAP 资源注册表：LwM2M /rd 注册 + 3303 温度传感器对象读写。
func BuildRegistry() *coap.Registry {
	reg, _ := BuildRegistryWithStore(nil)
	return reg
}

// BuildRegistryWithStore 同 BuildRegistry，但：1) /rd 注册成功时回调端点名（可 nil）；
// 2) 额外返回对象存储句柄（遥测汇入装配用）。
func BuildRegistryWithStore(onRegister func(endpoint string)) (*coap.Registry, *lwm2m.ObjectStore) {
	reg := coap.NewRegistry()
	var rd coap.Handler
	if onRegister != nil {
		rd = lwm2m.NewRegistry().HandleRegisterWithNotify(onRegister)
	} else {
		rd = lwm2m.NewRegistry().HandleRegister()
	}
	reg.Register("/rd", rd)
	store := lwm2m.NewObjectStore()
	lwm2m.BindObjects(reg, 3303, store)
	return reg, store
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
// bridge 非空时：/rd 注册驱动端点绑定，资源写入异步汇入 uplink 管道。
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

	var onRegister func(endpoint string)
	if o.bridge != nil {
		onRegister = o.bridge.OnRegister
	}
	reg, store := BuildRegistryWithStore(onRegister)
	g := &Gateway{cfg: cfg, reg: reg, log: log, bridge: o.bridge, started: make(chan struct{})}

	if o.bridge != nil {
		o.bridge.Attach(store)
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
