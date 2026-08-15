package app

import (
	"aetherlink-iot/backend/initialize"
	"errors"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func WithConfig(config *viper.Viper) Option {
	return func(app *Application) error {
		app.Config = config
		for _, key := range config.AllKeys() {
			viper.Set(key, config.Get(key))
		}
		return nil
	}
}

func WithEnvironment(env string) Option {
	return func(app *Application) error {
		config, err := LoadEnvironmentConfig(env)
		if err != nil {
			return err
		}
		return WithConfig(config)(app)
	}
}

func WithProductionConfig() Option {
	return WithEnvironment("prod")
}

func WithDevelopmentConfig() Option {
	return WithEnvironment("dev")
}

func WithTestConfig() Option {
	return WithEnvironment("test")
}

func WithRsaDecrypt(keyPath string) Option {
	return func(app *Application) error {
		return initialize.RsaDecryptInit(keyPath)
	}
}

// WithOptionalRsaDecrypt enables frontend RSA password decryption only when a
// deployment-injected private key exists. Missing optional key material keeps
// the default local stack bootable; malformed material remains a hard error.
func WithOptionalRsaDecrypt(keyPath string) Option {
	return func(app *Application) error {
		if _, err := os.Stat(keyPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		return initialize.RsaDecryptInit(keyPath)
	}
}

func WithLogger() Option {
	return func(app *Application) error {
		if err := initialize.LogInIt(); err != nil {
			return err
		}
		app.Logger = logrus.StandardLogger()
		return nil
	}
}

func WithDatabase() Option {
	return func(app *Application) error {
		db, err := initialize.PgInit()
		if err != nil {
			return err
		}
		app.DB = db
		return nil
	}
}

func WithRedis() Option {
	return func(app *Application) error {
		client, err := initialize.RedisInit()
		if err != nil {
			return err
		}
		app.RedisClient = client
		return nil
	}
}

func WithConfigFile(configPath string) Option {
	return func(app *Application) error {
		config, err := LoadConfigFile(configPath)
		if err != nil {
			return err
		}
		return WithConfig(config)(app)
	}
}
