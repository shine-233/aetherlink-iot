// 文件用途：维护 plugin\aetherlink\topicmap_matcher.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"regexp"
	"strings"
	"sync"
)

var varPlaceholderRegexp = regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`)

var topicRegexCache sync.Map

func cachedCompile(pattern string) (*regexp.Regexp, bool) {
	if v, ok := topicRegexCache.Load(pattern); ok {
		if v == nil {
			return nil, false
		}
		return v.(*regexp.Regexp), true
	}

	rx, err := regexp.Compile(pattern)
	if err != nil {
		topicRegexCache.Store(pattern, (*regexp.Regexp)(nil))
		return nil, false
	}

	topicRegexCache.Store(pattern, rx)
	return rx, true
}

func tryExtractWithPattern(template string, actual string) (string, bool) {
	if strings.Contains(template, "#") {
		return "", false
	}

	pattern := template
	pattern = strings.ReplaceAll(pattern, "{device_number}", "([^/]+)")
	pattern = varPlaceholderRegexp.ReplaceAllString(pattern, `[^/]+`)
	pattern = strings.ReplaceAll(pattern, "+", `[^/]+`)
	pattern = "^" + pattern + "$"

	rx, ok := cachedCompile(pattern)
	if !ok {
		return "", false
	}

	matches := rx.FindStringSubmatch(actual)
	if len(matches) >= 2 {
		return matches[1], true
	}
	return "", false
}

func TryExtractDeviceNumberFromNormalized(topic string) (string, bool) {
	patterns := []string{
		"devices/telemetry/control/{device_number}",
		"devices/attributes/set/{device_number}/+",
		"devices/attributes/get/{device_number}",
		"devices/command/{device_number}/+",
		"ota/devices/inform/{device_number}",
		"gateway/telemetry/control/{device_number}",
		"gateway/attributes/set/{device_number}/+",
		"gateway/attributes/get/{device_number}",
		"gateway/command/{device_number}/+",
		"devices/attributes/response/{device_number}/+",
		"devices/event/response/{device_number}/+",
		"gateway/attributes/response/{device_number}/+",
		"gateway/event/response/{device_number}/+",
	}

	for _, template := range patterns {
		if deviceNumber, ok := tryExtractWithPattern(template, topic); ok {
			return deviceNumber, true
		}
	}
	return "", false
}

func compileSourcePattern(source string) (*regexp.Regexp, bool) {
	if strings.Contains(source, "#") {
		return nil, false
	}

	pattern := source
	pattern = varPlaceholderRegexp.ReplaceAllString(pattern, `[^/]+`)
	pattern = strings.ReplaceAll(pattern, "+", `[^/]+`)
	pattern = "^" + pattern + "$"

	rx, ok := cachedCompile(pattern)
	if !ok {
		return nil, false
	}
	return rx, true
}

func applyTarget(target string, _ string) string {
	return target
}

func compileTargetPattern(target string) (*regexp.Regexp, bool) {
	if strings.Contains(target, "#") {
		return nil, false
	}

	pattern := target
	pattern = varPlaceholderRegexp.ReplaceAllString(pattern, `[^/]+`)
	pattern = strings.ReplaceAll(pattern, "+", `[^/]+`)
	pattern = "^" + pattern + "$"

	rx, ok := cachedCompile(pattern)
	if !ok {
		return nil, false
	}
	return rx, true
}

func renderTopicFromTemplate(template string, vars map[string]string) string {
	out := template
	for key, value := range vars {
		placeholder := "{" + key + "}"
		out = strings.ReplaceAll(out, placeholder, value)
	}
	return out
}
