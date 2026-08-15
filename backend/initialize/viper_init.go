// 文件用途：初始化全局配置读取器，为后续数据库、Redis、日志等启动逻辑提供统一配置源。
// 核心逻辑：设置环境变量前缀与键替换规则，并按指定路径或默认路径加载 YML 配置文件。
// 关键注意事项：配置文件路径与环境变量覆盖规则会直接影响全部初始化流程，修改时要保持启动兼容性。
// 重构建议：后续可把默认路径、环境变量前缀和配置名抽成常量或参数，降低隐式约定。

package initialize

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

// ViperInit 装配 Viper 的环境变量映射和配置文件读取规则。
func ViperInit(path string) error {
	viper.SetEnvPrefix("GOTP")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if path != "" {
		viper.SetConfigFile(path)
	} else {
		viper.SetConfigName("./configs/conf")
	}

	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %s", err)
	}
	log.Println("viper加载conf.yml配置文件完成...")
	return nil
}
