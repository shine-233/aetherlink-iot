// 文件用途：维护 cmd\gmqttd\main.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package main

import (
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/DrmagicE/gmqtt/cmd/gmqttd/command"
	_ "github.com/DrmagicE/gmqtt/persistence"
	_ "github.com/DrmagicE/gmqtt/plugin/prometheus"
	_ "github.com/DrmagicE/gmqtt/topicalias/fifo"
)

var (
	rootCmd = &cobra.Command{
		Use:     "gmqttd",
		Long:    "Gmqtt is a MQTT broker that fully implements MQTT V5.0 and V3.1.1 protocol",
		Version: Version,
	}
)

func must(err error) {
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func init() {
	configDir, err := getDefaultConfigDir()
	must(err)
	command.ConfigFile = path.Join(configDir, "gmqttd.yml")
	rootCmd.PersistentFlags().StringVarP(&command.ConfigFile, "config", "c", command.ConfigFile, "The configuration file path")
	rootCmd.AddCommand(command.NewStartCmd())
	//rootCmd.AddCommand(command.NewReloadCommand())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
}
