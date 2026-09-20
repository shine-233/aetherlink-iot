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
		// 先把 GOTP 环境变量映射补到全局单例，再逐键拷贝配置文件的值。
		// 拷贝循环只覆盖 YAML 声明过的键（空 map 的嵌套键会被 AllKeys 漏掉），
		// 漏掉的那部分只能靠环境变量兜底，见 config.go 的详细说明。
		applyEnvMappingToGlobalViper()
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
