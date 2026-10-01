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
		// 全局 viper 是 pkg/secrets、service/device_template_market_integrity 等包的
		// 配置源，但它自身没有环境变量解析能力，而 AllKeys() 只覆盖「配置文件里出现过
		// 的叶子键」。secrets.master_keys.<id> / market.bundle_signing_keys.<id> 在
		// conf.yml 里是空 map（没有任何叶子键），值只能来自环境变量——少了这一句，
		// 它们永远搬不进全局 viper，主密钥与包签名密钥恒为空，写入一律 fail closed
		// （POST /api/v1/secrets 返回 100000，导出资源包返回 100002）。
		// 复用同一套环境变量规则，保证局部 viper 与全局 viper 的取值口径一致。
		configureEnvironment(viper.GetViper())
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
