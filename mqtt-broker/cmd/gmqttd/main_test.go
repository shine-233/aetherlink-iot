// 文件用途：维护 cmd\gmqttd\main_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package main

import (
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/DrmagicE/gmqtt/cmd/gmqttd/command"
)

func TestRootCommandRegistersStartCommandAndConfigFlag(t *testing.T) {
	if rootCmd.Use != "gmqttd" {
		t.Fatalf("root command use = %q", rootCmd.Use)
	}
	if rootCmd.Version != Version {
		t.Fatalf("root command version = %q, want %q", rootCmd.Version, Version)
	}
	if command.ConfigFile == "" {
		t.Fatal("default command config file should be initialized")
	}
	if flag := rootCmd.PersistentFlags().Lookup("config"); flag == nil || flag.DefValue != command.ConfigFile {
		t.Fatalf("config flag not wired to command.ConfigFile")
	}

	start, _, err := rootCmd.Find([]string{"start"})
	if err != nil {
		t.Fatalf("find start command: %v", err)
	}
	if start == nil || start.Use != "start" {
		t.Fatalf("start command not registered: %#v", start)
	}
}

func TestGeneratedPluginCatalogImportsAetherLinkAndPrometheus(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "plugins.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse plugins.go imports: %v", err)
	}
	imports := map[string]bool{}
	for _, spec := range parsed.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("unquote import %s: %v", spec.Path.Value, err)
		}
		imports[value] = true
	}

	for _, importPath := range []string{
		"github.com/DrmagicE/gmqtt/plugin/aetherlink",
		"github.com/DrmagicE/gmqtt/plugin/prometheus",
	} {
		if !imports[importPath] {
			t.Fatalf("generated plugin catalog missing %s", importPath)
		}
	}
}
