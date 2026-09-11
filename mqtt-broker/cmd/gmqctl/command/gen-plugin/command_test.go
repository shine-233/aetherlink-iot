// 文件用途：验证 `gmqctl gen plugin` 生成器的安全 scaffold 契约。
package gen_plugin

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func generatePlugin(t *testing.T, pluginName, hooks string, withConfig bool, dir string) error {
	t.Helper()
	oldName, oldHooks, oldConfig, oldOutput := name, hooksStr, configFlag, output
	name, hooksStr, configFlag, output = pluginName, hooks, withConfig, dir
	t.Cleanup(func() {
		name, hooksStr, configFlag, output = oldName, oldHooks, oldConfig, oldOutput
	})
	return run(nil, nil)
}

func TestGeneratedPluginGolden(t *testing.T) {
	outputDir := t.TempDir()
	if err := generatePlugin(t, "safe_plugin", "OnBasicAuth,OnSubscribed", true, outputDir); err != nil {
		t.Fatalf("generate plugin: %v", err)
	}
	for _, file := range []string{"safe_plugin.go", "hooks.go", "config.go"} {
		got, err := os.ReadFile(filepath.Join(outputDir, file))
		if err != nil {
			t.Fatalf("read generated %s: %v", file, err)
		}
		want, err := os.ReadFile(filepath.Join("testdata", file+".golden"))
		if err != nil {
			t.Fatalf("read golden %s: %v", file, err)
		}
		if string(got) != string(want) {
			t.Errorf("generated %s differs from testdata/%s.golden\n--- got ---\n%s\n--- want ---\n%s", file, file, got, want)
		}
	}
}

func TestRunRefusesOverwrite(t *testing.T) {
	outputDir := t.TempDir()
	target := filepath.Join(outputDir, "safe_plugin.go")
	const original = "do not overwrite\n"
	if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	err := generatePlugin(t, "safe_plugin", "OnBasicAuth", false, outputDir)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("run error = %v, want overwrite refusal", err)
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read target: %v", readErr)
	}
	if string(got) != original {
		t.Fatalf("target was modified: got %q, want %q", got, original)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "hooks.go")); !os.IsNotExist(statErr) {
		t.Fatalf("hooks.go should not be generated after refusal, stat error = %v", statErr)
	}
}

func TestRunRejectsInvalidHookBeforeWriting(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "generated")
	err := generatePlugin(t, "safe_plugin", "OnNotAHook", false, outputDir)
	if err == nil || !strings.Contains(err.Error(), "invalid hook name: OnNotAHook") {
		t.Fatalf("run error = %v, want invalid-hook error", err)
	}
	if _, statErr := os.Stat(outputDir); !os.IsNotExist(statErr) {
		t.Fatalf("output should not exist after invalid hook, stat error = %v", statErr)
	}
}

func TestValidateHooks(t *testing.T) {
	got, err := ValidateHooks("OnSubscribe, OnSubscribed")
	if err != nil {
		t.Fatalf("ValidateHooks valid input: %v", err)
	}
	if strings.Join(got, ",") != "OnSubscribe,OnSubscribed" {
		t.Fatalf("ValidateHooks = %v", got)
	}
	if _, err = ValidateHooks("OnAbc,OnDEF"); err == nil {
		t.Fatal("ValidateHooks accepted invalid hooks")
	}
}

func TestGeneratedModuleCompiles(t *testing.T) {
	moduleDir := t.TempDir()
	pluginDir := filepath.Join(moduleDir, "safe_plugin")
	if err := generatePlugin(t, "safe_plugin", "OnBasicAuth,OnSubscribe", true, pluginDir); err != nil {
		t.Fatalf("generate plugin: %v", err)
	}
	writeTemporaryModule(t, moduleDir)
	cmd := exec.Command("go", "test", "-mod=mod", "./...")
	cmd.Dir = moduleDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("temporary generated module did not compile: %v\n%s", err, out)
	}
}

func writeTemporaryModule(t *testing.T, moduleDir string) {
	t.Helper()
	brokerRoot, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve broker root: %v", err)
	}
	goMod := "module example.com/generated-plugin\n\ngo 1.25.0\n\nrequire github.com/DrmagicE/gmqtt v0.0.0\n\nreplace github.com/DrmagicE/gmqtt => " + filepath.ToSlash(brokerRoot) + "\n"
	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatalf("write temporary go.mod: %v", err)
	}
}

func TestGeneratedUnconfiguredPluginRejectsLoad(t *testing.T) {
	moduleDir := t.TempDir()
	pluginDir := filepath.Join(moduleDir, "safe_plugin")
	if err := generatePlugin(t, "safe_plugin", "OnBasicAuth", false, pluginDir); err != nil {
		t.Fatalf("generate plugin: %v", err)
	}
	writeTemporaryModule(t, moduleDir)
	loadTest := `package safe_plugin

import (
	"errors"
	"testing"

	"github.com/DrmagicE/gmqtt/config"
)

func TestScaffoldRejectsConstructionAndLoad(t *testing.T) {
	plugin, err := New(config.Config{})
	if plugin != nil || !errors.Is(err, ErrScaffoldIncomplete) {
		t.Fatalf("New() = (%v, %v), want (nil, ErrScaffoldIncomplete)", plugin, err)
	}
	if err := (&SafePlugin{}).Load(nil); !errors.Is(err, ErrScaffoldIncomplete) {
		t.Fatalf("Load() error = %v, want ErrScaffoldIncomplete", err)
	}
	if err := (&SafePlugin{}).Unload(); err != nil {
		t.Fatalf("Unload() error = %v, want nil", err)
	}
}
`
	if err := os.WriteFile(filepath.Join(pluginDir, "scaffold_load_test.go"), []byte(loadTest), 0o600); err != nil {
		t.Fatalf("write load test: %v", err)
	}
	cmd := exec.Command("go", "test", "-mod=mod", "./safe_plugin", "-run", "TestScaffoldRejectsConstructionAndLoad", "-count=1")
	cmd.Dir = moduleDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unconfigured generated plugin load contract failed: %v\n%s", err, out)
	}
}
