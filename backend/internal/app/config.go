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
