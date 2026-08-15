// 文件用途：维护 cmd\gmqctl\main.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/DrmagicE/gmqtt/cmd/gmqctl/command"
)

var (
	rootCmd = &cobra.Command{
		Use:     "gmqctl",
		Long:    "gmqctl is a command line tool for gmqtt",
		Version: Version,
	}
)

func init() {
	rootCmd.AddCommand(command.Gen)
}

func must(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	must(rootCmd.Execute())
}
