// Configuration loading helpers.
package app

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func configureEnvironment(v *viper.Viper) {
	v.SetEnvPrefix("GOTP")
	// An explicitly provided empty environment value is still an intentional
	// override. This matters for local/runtime profiles whose credential is
	// deliberately empty while the checked-in config keeps a placeholder.
	v.AllowEmptyEnv(true)
	v.AutomaticEnv()
	// Keep environment names shell/Compose-friendly for both nested and
	// legacy hyphenated YAML keys (for example, classified-protect).
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
}

// applyEnvMappingToGlobalViper 把同一套 GOTP 环境变量映射补到包级 viper 单例上。
//
// 为什么需要它：配置加载发生在独立的 viper 实例上（LoadEnvironmentConfig /
// LoadConfigFile），随后由 WithConfig 逐键 viper.Set 拷贝进全局单例。但
// AllKeys() 只会枚举 YAML 里真正声明过的键——凡是 YAML 取值为**空 map** 的键
// （例如 conf.yml 的 `secrets.master_keys: {}`、`market.bundle_signing_keys: {}`），
// 其嵌套键永远不会被展开，也就永远不会被拷贝。
//
// 全局单例本身又从未开启 AutomaticEnv（initialize.ViperInit 只被 cmd/gen 调用，
// 后端主进程走的是 internal/app 这条路径），于是这些键即便对应的 GOTP_* 环境变量
// 已经注入容器，仍然读不到值。直接通过全局 viper 读取它们的代码（pkg/secrets 的
// 信封加密、模板市场包签名）就会一律 fail closed。
//
// 补上同一套映射后，全局单例与子实例的环境变量语义完全一致。注意 viper 的查找顺序
// 是 override 优先于 env，因此 WithConfig 已经 viper.Set 过的键不会改变取值。
func applyEnvMappingToGlobalViper() {
	configureEnvironment(viper.GetViper())
}

// LoadEnvironmentConfig loads a named local environment config and then
// overlays matching environment variables.
func LoadEnvironmentConfig(env string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigType("yml")

	var configFile string
	switch env {
	case "dev":
		configFile = "./configs/conf-localdev.yml"
	case "test":
		configFile = "./configs/conf-test.yml"
	case "prod":
		configFile = "./configs/conf.yml"
	default:
		return nil, fmt.Errorf("unsupported environment: %s", env)
	}

	v.SetConfigFile(configFile)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	configureEnvironment(v)

	return v, nil
}

// LoadConfigFile loads a specific config file path and then overlays matching
// environment variables.
func LoadConfigFile(configPath string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	configureEnvironment(v)

	return v, nil
}
