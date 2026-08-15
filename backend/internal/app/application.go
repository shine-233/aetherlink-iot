package app

import (
	"sync"

	"aetherlink-iot/backend/internal/downlink"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/internal/storage"
	"aetherlink-iot/backend/internal/uplink"

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

	if app.DB != nil {
		query.SetDefault(app.DB)
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
