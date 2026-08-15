// 文件用途：维护 cmd\gmqttd\command\reload.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package command

import (
	"os"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/DrmagicE/gmqtt/config"
)

// NewReloadCommand creates a *cobra.Command object for reload command.
func NewReloadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reload",
		Short: "Reload gmqtt broker",
		Run: func(cmd *cobra.Command, args []string) {
			var c config.Config
			var err error
			c, err = config.ParseConfig(ConfigFile)
			if os.IsNotExist(err) {
				c = config.DefaultConfig()
			} else {
				must(err)
			}
			b, err := os.ReadFile(c.PidFile)
			must(errors.Wrap(err, "read pid file error"))
			pid, err := strconv.Atoi(string(b))
			must(errors.Wrap(err, "read pid file error"))
			p, err := os.FindProcess(pid)
			must(errors.Wrap(err, "find process error"))
			err = p.Signal(syscall.SIGHUP)
			must(err)
		},
	}
	return cmd
}
