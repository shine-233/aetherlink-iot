// 文件用途：维护 plugin\admin\admin.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package admin

import (
	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/server"
)

var _ server.Plugin = (*Admin)(nil)

const Name = "admin"

func init() {
	if err := server.RegisterPlugin(Name, New); err != nil {
		panic(err)
	}
	config.RegisterDefaultPluginConfig(Name, &DefaultConfig)
}

func New(config config.Config) (server.Plugin, error) {
	return &Admin{}, nil
}

var log *zap.Logger

// Admin providers gRPC and HTTP API that enables the external system to interact with the broker.
type Admin struct {
	statsReader    server.StatsReader
	publisher      server.Publisher
	clientService  server.ClientService
	store          *store
	httpAuthSecret string
}

func (a *Admin) registerHTTP(g server.APIRegistrar) (err error) {
	err = g.RegisterHTTPHandler(registerAdminUI)
	if err != nil {
		return err
	}
	err = g.RegisterHTTPHandler(RegisterClientServiceHandlerFromEndpoint)
	if err != nil {
		return err
	}
	err = g.RegisterHTTPHandler(RegisterSubscriptionServiceHandlerFromEndpoint)
	if err != nil {
		return err
	}
	err = g.RegisterHTTPHandler(RegisterPublishServiceHandlerFromEndpoint)
	if err != nil {
		return err
	}
	return nil
}

func (a *Admin) Load(service server.Server) error {
	log = server.LoggerWithField(zap.String("plugin", Name))
	apiRegistrar := service.APIRegistrar()
	RegisterClientServiceServer(apiRegistrar, &clientService{a: a})
	RegisterSubscriptionServiceServer(apiRegistrar, &subscriptionService{a: a})
	RegisterPublishServiceServer(apiRegistrar, &publisher{a: a})
	err := a.registerHTTP(apiRegistrar)
	if err != nil {
		return err
	}
	a.statsReader = service.StatsManager()
	a.store = newStore(a.statsReader, service.GetConfig())
	a.store.subscriptionService = service.SubscriptionService()
	a.publisher = service.Publisher()
	a.clientService = service.ClientService()
	a.setupHTTPAuth(service.GetConfig(), apiRegistrar)
	a.setupGRPCAuth(apiRegistrar)
	return nil
}

func (a *Admin) Unload() error {
	return nil
}

func (a *Admin) Name() string {
	return Name
}
