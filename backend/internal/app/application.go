package app

import (
	"sync"

	"aetherlink-iot/backend/internal/downlink"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/internal/storage"
	"aetherlink-iot/backend/internal/uplink"
	"aetherlink-iot/backend/pkg/global"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type Application struct {
	Config         *viper.Viper
	Logger         *logrus.Logger
	DB             *gorm.DB
	RedisClient    *redis.Client
	ServiceManager *ServiceManager

	storageService  storage.Storage
	storageInput    storage.DurableMessageInput
	uplinkService   *UplinkServiceWrapper
	mqttService     *MQTTService
	downlinkService *DownlinkServiceWrapper

	shutdownOnce sync.Once
}

func NewApplication(options ...Option) (*Application, error) {
	app := &Application{
		Logger:         logrus.New(),
		ServiceManager: NewServiceManager(),
	}

	for _, option := range options {
		if err := option(app); err != nil {
			return nil, err
		}
	}

	if app.Config != nil {
		// 安全关键配置 fail-fast：占位符/弱 JWT 密钥在启动即拒绝，防止带病上线后 token 可被伪造。
		if err := validateSecurityCriticalConfig(app.Config); err != nil {
			return nil, err
		}
	}

	if app.DB != nil {
		// P1 修复（2026-08-23，见 VALIDATION.md）：gen 单例链必须从"全新语句"会话根出发。
		// gorm 的 Statement.clone() 只清空 Clauses/Preloads，其余字段（含 Model/Dest）按值继承；
		// 一旦某次执行的 Model 残留非零主键，后续并发请求的 UPDATE/DELETE 会被 gorm 以
		// Dest != Model 为条件注入陈旧主键 WHERE（症状：间歇 record-not-found / 假成功删除）。
		// Session{NewDB:true} 使每个链式起点都拿到从未被写入的全新 Statement，切断跨请求继承。
		query.SetDefault(app.DB.Session(&gorm.Session{NewDB: true}))
	}

	return app, nil
}

func (app *Application) RegisterService(service Service) {
	app.ServiceManager.RegisterService(service)
}

func (app *Application) Start() error {
	return app.ServiceManager.StartAll()
}

func (app *Application) Shutdown() {
	app.shutdownOnce.Do(func() {
		logrus.Info("application shutdown started")

		app.ServiceManager.StopAll()

		// WS/SSE 的 Redis Pub/Sub 监听协程由 initialize.RedisInit 直接拉起，
		// 不经 ServiceManager 托管，必须在关闭 Redis 客户端之前显式取消，
		// 否则会带着已关闭的客户端继续重连。
		global.StopWSManagerListener()
		global.StopSSEManagerListener()

		if app.RedisClient != nil {
			if err := app.RedisClient.Close(); err != nil {
				app.Logger.WithError(err).Warn("failed to close Redis connection")
			} else {
				app.Logger.Info("Redis connection closed")
			}
		}

		app.Logger.Info("application resources cleaned up")
		logrus.Info("application shutdown completed")
	})
}

func (app *Application) Wait() {
	app.ServiceManager.Wait()
}

type Option func(*Application) error

func (a *Application) GetDownlinkBus() *downlink.Bus {
	if a.downlinkService == nil {
		return nil
	}
	return a.downlinkService.GetBus()
}

func (a *Application) GetUplinkBus() *uplink.Bus {
	if a.uplinkService == nil {
		return nil
	}
	return a.uplinkService.GetBus()
}
