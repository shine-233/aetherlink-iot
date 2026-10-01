// 文件用途：维护 plugin\aetherlink\topicmap_matcher_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"regexp"
	"strings"
	"testing"
)

func TestTryExtractDeviceNumberFromNormalizedTopics(t *testing.T) {
	tests := []struct {
		topic string
		want  string
	}{
		{"devices/telemetry/control/dev-001", "dev-001"},
		{"devices/attributes/set/dev-002/mode", "dev-002"},
		{"gateway/command/dev-003/reboot", "dev-003"},
		{"ota/devices/inform/dev-004", "dev-004"},
	}

	for _, tt := range tests {
		got, ok := TryExtractDeviceNumberFromNormalized(tt.topic)
		if !ok {
			t.Fatalf("TryExtractDeviceNumberFromNormalized(%q) did not match", tt.topic)
		}
		if got != tt.want {
			t.Fatalf("device number = %q, want %q", got, tt.want)
		}
	}
}

func TestTopicMapMatcherRejectsMultiLevelWildcardAndRendersVariables(t *testing.T) {
	if _, ok := compileSourcePattern("devices/#"); ok {
		t.Fatal("compileSourcePattern should reject multi-level wildcard")
	}
	if _, ok := compileTargetPattern("gateway/#"); ok {
		t.Fatal("compileTargetPattern should reject multi-level wildcard")
	}

	got := renderTopicFromTemplate("devices/{device_number}/command/{method}", map[string]string{
		"device_number": "dev-1",
		"method":        "reboot",
	})
	if got != "devices/dev-1/command/reboot" {
		t.Fatalf("rendered topic = %q", got)
	}
}

// referenceTryExtractDeviceNumber 是按模板“本意”构造的正则参考实现（逐段转义），用于差分测试。
func referenceTryExtractDeviceNumber(topic string) (string, bool) {
	for _, template := range normalizedDownlinkTemplates {
		parts := strings.Split(template, "/")
		for i, p := range parts {
			switch {
			case p == "{device_number}":
				parts[i] = `([^/]+)`
			case p == "+":
				parts[i] = `[^/]+`
			default:
				parts[i] = regexp.QuoteMeta(p)
			}
		}
		rx := regexp.MustCompile("^" + strings.Join(parts, "/") + "$")
		if m := rx.FindStringSubmatch(topic); len(m) >= 2 {
			return m[1], true
		}
	}
	return "", false
}

func downlinkTopicCorpus() []string {
	corpus := []string{
		"", "/", "//", "devices", "devices/", "ota", "ota/devices/inform/",
		"devices/telemetry/control/dev-001",
		"devices/telemetry/control/dev-001/extra",
		"devices/telemetry/control/",
		"devices/telemetry/control//",
		"/devices/telemetry/control/dev-1",
		"devices/attributes/set/dev-002/mode",
		"devices/attributes/set/dev-002/",
		"devices/attributes/set//mode",
		"devices/attributes/set/dev-002",
		"devices/attributes/get/dev-x",
		"devices/attributes/get/dev-x/1",
		"devices/command/dev-3/reboot",
		"devices/command/dev-3",
		"gateway/command/dev-003/reboot",
		"gateway/telemetry/control/gw.1",
		"gateway/attributes/set/gw 1/m",
		"gateway/attributes/get/网关-1",
		"gateway/attributes/response/gw/msg-1",
		"gateway/event/response/gw/msg-1",
		"devices/attributes/response/d/msg",
		"devices/event/response/d/msg",
		"devices/event/response/d",
		"ota/devices/inform/dev-004",
		"ota/devices/inform/dev-004/x",
		"Devices/telemetry/control/dev",
		"devices/telemetry/Control/dev",
		"devices/telemetry/control/+",
		"devices/telemetry/control/#",
		"devices/telemetry/control/dev\n",
		"devices/me/telemetry",
		"plugin/xx/devices/telemetry/control/d",
	}
	// 每个模板的规范实例及其逐段变异（替换/删除/追加段）。
	for _, tpl := range normalizedDownlinkTemplates {
		base := strings.ReplaceAll(strings.ReplaceAll(tpl, "{device_number}", "DN-9"), "+", "m")
		corpus = append(corpus, base, base+"/", base+"/z", "x/"+base)
		parts := strings.Split(base, "/")
		for i := range parts {
			mut := append([]string(nil), parts...)
			mut[i] = ""
			corpus = append(corpus, strings.Join(mut, "/"))
			mut[i] = parts[i] + "q"
			corpus = append(corpus, strings.Join(mut, "/"))
			corpus = append(corpus, strings.Join(append(append([]string(nil), parts[:i]...), parts[i+1:]...), "/"))
		}
	}
	return corpus
}

func TestTryExtractDeviceNumberMatchesReferenceRegex(t *testing.T) {
	for _, topic := range downlinkTopicCorpus() {
		wantDN, wantOK := referenceTryExtractDeviceNumber(topic)
		gotDN, gotOK := TryExtractDeviceNumberFromNormalized(topic)
		if gotDN != wantDN || gotOK != wantOK {
			t.Errorf("topic %q: got (%q,%v), reference (%q,%v)", topic, gotDN, gotOK, wantDN, wantOK)
		}
	}
}

func TestTryExtractDeviceNumberEveryTemplate(t *testing.T) {
	for _, tpl := range normalizedDownlinkTemplates {
		topic := strings.ReplaceAll(strings.ReplaceAll(tpl, "{device_number}", "dev/../"), "+", "m")
		if _, ok := TryExtractDeviceNumberFromNormalized(topic); ok {
			t.Errorf("template %q: device number containing '/' must not match (%q)", tpl, topic)
		}
		topic = strings.ReplaceAll(strings.ReplaceAll(tpl, "{device_number}", "SN-42"), "+", "anything")
		got, ok := TryExtractDeviceNumberFromNormalized(topic)
		if !ok || got != "SN-42" {
			t.Errorf("template %q: got (%q,%v) for %q", tpl, got, ok, topic)
		}
	}
}

func TestTryExtractDeviceNumberZeroAlloc(t *testing.T) {
	topics := []string{"gateway/event/response/gw-1/msg-9", "devices/unknown/x", "ota/devices/inform/dev"}
	allocs := testing.AllocsPerRun(200, func() {
		for _, topic := range topics {
			_, _ = TryExtractDeviceNumberFromNormalized(topic)
		}
	})
	if allocs != 0 {
		t.Fatalf("hot path allocates %.1f times per run, want 0", allocs)
	}
}

func TestTryExtractDeviceNumberTemplateCompileRejectsInvalid(t *testing.T) {
	for _, tpl := range []string{
		"", "devices/#", "devices/{device_number}/#", "devices//{device_number}",
		"{device_number}/x", "+/x/{device_number}", "devices/telemetry", // 无捕获
		"a/{device_number}/{device_number}", "a/dev-{device_number}", "a/x+/{device_number}",
	} {
		if _, err := compileDownlinkTemplate(tpl); err == nil {
			t.Errorf("compileDownlinkTemplate(%q) should fail", tpl)
		}
	}
	if _, err := compileDownlinkTemplate("a/{method}/+/{device_number}"); err != nil {
		t.Errorf("valid template rejected: %v", err)
	}
}

func TestTryExtractDeviceNumberBucketPriorityOrder(t *testing.T) {
	// 同段数同首段的两个模板都能匹配时，先声明者胜出（与旧线性扫描一致）。
	m := mustBuildDownlinkMatcher([]string{"a/+/{device_number}", "a/{device_number}/+"})
	if got, ok := m.match("a/x/y"); !ok || got != "y" {
		t.Fatalf("got (%q,%v), want (\"y\",true)", got, ok)
	}
}

func BenchmarkTryExtractDeviceNumberFromNormalized(b *testing.B) {
	topics := []string{
		"gateway/event/response/gw-1/msg-9", // 最后一个模板，旧实现最坏情况
		"devices/telemetry/control/dev-001",
		"devices/me/telemetry", // 不匹配
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = TryExtractDeviceNumberFromNormalized(topics[i%len(topics)])
	}
}

func BenchmarkLegacyTryExtractDeviceNumber(b *testing.B) {
	topics := []string{"gateway/event/response/gw-1/msg-9", "devices/telemetry/control/dev-001", "devices/me/telemetry"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = legacyTryExtractDeviceNumberCached(topics[i%len(topics)])
	}
}

// legacyTryExtractDeviceNumberCached 复刻旧热路径（字符串拼接 + sync.Map 缓存），用于基准对比。
func legacyTryExtractDeviceNumberCached(topic string) (string, bool) {
	for _, template := range normalizedDownlinkTemplates {
		pattern := strings.ReplaceAll(template, "{device_number}", "([^/]+)")
		pattern = varPlaceholderRegexp.ReplaceAllString(pattern, `[^/]+`)
		pattern = strings.ReplaceAll(pattern, "+", `[^/]+`)
		rx, ok := cachedCompile("^" + pattern + "$")
		if !ok {
			continue
		}
		if m := rx.FindStringSubmatch(topic); len(m) >= 2 {
			return m[1], true
		}
	}
	return "", false
}

func TestTryExtractDeviceNumberSingleCharSegments(t *testing.T) {
	// 回归：旧实现二次替换 '+'，把捕获组改写成 ([^/][^/]+)，单字符设备号/段被静默丢弃。
	cases := map[string]string{
		"devices/telemetry/control/d":     "d",
		"devices/command/dev-1/1":         "dev-1",
		"gateway/event/response/g/7":      "g",
		"devices/attributes/response/d/m": "d",
	}
	for topic, want := range cases {
		if got, ok := TryExtractDeviceNumberFromNormalized(topic); !ok || got != want {
			t.Errorf("%q: got (%q,%v), want %q", topic, got, ok, want)
		}
	}
}

func TestTopicMapMatcherUserPatternSemantics(t *testing.T) {
	rx, ok := compileSourcePattern("v1/{device_number}/cmd.{method}/+")
	if !ok {
		t.Fatal("pattern should compile")
	}
	for topic, want := range map[string]bool{
		"v1/a/cmd.b/c":     true, // 单字符变量与 '+' 均可匹配
		"v1/dev/cmd.set/x": true,
		"v1/dev/cmdXset/x": false, // '.' 按字面量
		"v1//cmd.set/x":    false,
		"v1/a/b/cmd.s/x":   false, // 变量不跨段
		"v1/a/cmd./x":      false,
	} {
		if got := rx.MatchString(topic); got != want {
			t.Errorf("%q: match=%v, want %v", topic, got, want)
		}
	}
	if rx2, _ := compileTargetPattern("v1/{device_number}/cmd.{method}/+"); rx2 != rx {
		t.Error("source/target patterns with identical templates should share the cached regexp")
	}
}
