package gmqtt

import (
	"os"
	"strings"
	"testing"
)

// TestOptionalHeavyPluginsRemainCompiledButDisabledByDefault protects two
// separate contracts: compatibility plugins remain buildable, while the local
// single-node deployment does not activate their heavy runtime dependencies.
func TestOptionalHeavyPluginsRemainCompiledButDisabledByDefault(t *testing.T) {
	importsSource, err := os.ReadFile("plugin_imports.yml")
	if err != nil {
		t.Fatalf("read plugin_imports.yml: %v", err)
	}
	configSource, err := os.ReadFile("cmd/gmqttd/default_config.yml")
	if err != nil {
		t.Fatalf("read default_config.yml: %v", err)
	}

	imports := string(importsSource)
	config := strings.ReplaceAll(string(configSource), "\r\n", "\n")
	for _, plugin := range []string{"admin", "auth", "federation"} {
		if !strings.Contains(imports, "  - "+plugin+"\n") {
			t.Errorf("optional plugin %s must remain compiled for compatibility", plugin)
		}
		if strings.Contains(config, "\n  - "+plugin+" #") {
			t.Errorf("optional plugin %s must not load in the default plugin_order", plugin)
		}
		if !strings.Contains(config, "\n  #- "+plugin+" #") {
			t.Errorf("default config must show %s as an explicit opt-in example", plugin)
		}
	}

	for _, plugin := range []string{"aetherlink", "prometheus"} {
		if !strings.Contains(config, "\n  - "+plugin+" #") {
			t.Errorf("default plugin_order must keep local-default plugin %s enabled", plugin)
		}
	}
}

// TestDefaultMQTTListenerUsesAetherLinkAuthentication locks the deployed path:
// Compose mounts the checked-in daemon config, which enables AetherLink's
// OnBasicAuth hook while keeping anonymous fallback and admin disabled.
// OnWillPublishWrapper 也属于默认安全边界：will message 必须经过同一插件授权，
// 该契约防止后续重构把遗嘱钩子从 HookWrapper 中悄悄移除。
// Go 源码标记使用空白归一化匹配，gofmt 对齐调整不再破坏契约检查。
func TestDefaultMQTTListenerUsesAetherLinkAuthentication(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return strings.ReplaceAll(string(content), "\r\n", "\n")
	}
	normalizeWhitespace := func(source string) string {
		return strings.Join(strings.Fields(source), " ")
	}

	config := read("cmd/gmqttd/default_config.yml")
	compose := read("../docker-compose.yml")
	hooks := normalizeWhitespace(read("plugin/aetherlink/hooks.go"))
	mqttConfig := normalizeWhitespace(read("config/mqtt.go"))
	clientConnect := normalizeWhitespace(read("server/client_connect.go"))

	for marker, source := range map[string]string{
		`- address: ":1883"`:     config,
		`allow_anonymous: false`: config,
		"\n  - aetherlink # AetherLink IoT MQTT integration and default authentication boundary": config,
		"\n  #- auth #":  config,
		"\n  #- admin #": config,
		"./mqtt-broker/cmd/gmqttd/default_config.yml:/gmqttd/default_config.yml:ro": compose,
		`OnBasicAuthWrapper: t.OnBasicAuthWrapper`:                                  hooks,
		`OnWillPublishWrapper: t.OnWillPublishWrapper`:                              hooks,
		`AllowAnonymous: false`:                                                     mqttConfig,
		`if client.config.MQTT.AllowAnonymous {`:                                    clientConnect,
		`Code: codes.NotAuthorized`:                                                 clientConnect,
	} {
		if !strings.Contains(source, marker) {
			t.Errorf("default MQTT authentication contract missing marker %q", marker)
		}
	}
	if strings.Contains(config, "\n  - auth #") {
		t.Error("standalone auth plugin must remain optional; AetherLink owns default device authentication")
	}
	if strings.Contains(config, "\n  - admin #") {
		t.Error("admin plugin must remain disabled by default")
	}
}

func TestPluginDeploymentModesAreDocumented(t *testing.T) {
	content, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readme := string(content)
	for _, marker := range []string{
		"local-default",
		"optional-external",
		"blocked-external",
		"admin`、`auth`、`federation",
		"protoc-gen-grpc-gateway",
	} {
		if !strings.Contains(readme, marker) {
			t.Errorf("README must document broker deployment boundary marker %q", marker)
		}
	}
}

func TestDockerContextExcludesWindowsBinaries(t *testing.T) {
	content, err := os.ReadFile(".dockerignore")
	if err != nil {
		t.Fatalf("read .dockerignore: %v", err)
	}

	for _, rule := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(rule) == "*.exe" {
			return
		}
	}
	t.Error(".dockerignore must exclude Windows executables with *.exe")
}
