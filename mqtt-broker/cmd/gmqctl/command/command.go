// Package command registers gmqctl subcommands.
//
// The current command tree exposes code-generation helpers used by broker
// plugin development and generated-interface maintenance.
//
// 文件用途：注册 gmqctl 的命令树入口，当前主要承载代码生成类子命令。
// 核心逻辑：创建 gen 命令并在初始化阶段挂载 gen-plugin 子命令。
// 关键注意事项：这里只做命令编排，不放具体生成逻辑，避免根命令膨胀。
// 重构建议：后续如果生成器增多，可按领域拆分子命令包并补充命令级测试。
package command

import (
	"github.com/spf13/cobra"

	gen_plugin "github.com/DrmagicE/gmqtt/cmd/gmqctl/command/gen-plugin"
)

// Gen is the command for code generator.
var Gen = &cobra.Command{
	Use:   "gen",
	Short: "Code generator",
}

func init() {
	Gen.AddCommand(gen_plugin.Command)
}
