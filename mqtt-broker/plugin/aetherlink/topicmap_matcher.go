// 文件用途：MQTT 主题模板匹配：下行主题设备号提取（预编译分段匹配器）与用户映射模式的正则编译。
// 核心逻辑：13 个固定下行模板在包初始化时编译为按“段数 + 首段字面量”索引的分段匹配器，
//          热路径（OnMsgArrived → resolveMQTTDownlinkRoute）只做零分配的逐段比较，不再拼接正则字符串。
// 关键注意事项：分段语义等价于正则 ^...$（修正了旧实现二次替换 '+' 导致单字符段无法匹配的缺陷）：{device_number} 捕获、{var} 与 + 各匹配一个非空且不含 '/' 的段；
//          含 '#' 的模板被拒绝。动态用户映射（compileSourcePattern/compileTargetPattern）仍走 cachedCompile。
// 重构建议：若下行模板改为可配置，可复用 compileDownlinkTemplate 在配置加载时重建 downlinkMatcher。

package aetherlink

import (
	"fmt"
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

// normalizedDownlinkTemplates 是平台规范化下行主题模板，顺序即优先级（与旧实现的线性扫描顺序一致）。
var normalizedDownlinkTemplates = [...]string{
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

const deviceNumberPlaceholder = "{device_number}"

type segmentKind uint8

const (
	segLiteral segmentKind = iota // 必须逐字相等
	segAny                        // '+' 或 {var}：一个非空段
	segCapture                    // {device_number}：一个非空段并捕获
)

type topicSegment struct {
	kind    segmentKind
	literal string
}

// downlinkTemplate 是一个已编译的模板；首段一定是字面量（由索引键承载，匹配时跳过）。
type downlinkTemplate struct {
	source   string
	segments []topicSegment
}

// downlinkMatcher 按段数 → 首段字面量索引模板；每个桶内按优先级排序。
type downlinkMatcher struct {
	bySegments map[int]map[string][]*downlinkTemplate
	maxSegs    int
}

var downlinkMatcherInstance = mustBuildDownlinkMatcher(normalizedDownlinkTemplates[:])

// compileDownlinkTemplate 把模板拆成段。要求：无 '#'、无空段、首段为字面量、
// 恰有一个独占整段的 {device_number}；占位符不得与字面量混在同一段内（旧正则虽允许，但固定模板从不使用）。
func compileDownlinkTemplate(template string) (*downlinkTemplate, error) {
	if template == "" || strings.Contains(template, "#") {
		return nil, fmt.Errorf("template %q: empty or contains multi-level wildcard", template)
	}
	parts := strings.Split(template, "/")
	segs := make([]topicSegment, 0, len(parts))
	captures := 0
	for i, p := range parts {
		switch {
		case p == "":
			return nil, fmt.Errorf("template %q: empty segment at %d", template, i)
		case p == deviceNumberPlaceholder:
			captures++
			segs = append(segs, topicSegment{kind: segCapture})
		case p == "+" || varPlaceholderRegexp.FindString(p) == p:
			segs = append(segs, topicSegment{kind: segAny})
		case strings.ContainsAny(p, "+{}"):
			return nil, fmt.Errorf("template %q: mixed placeholder segment %q", template, p)
		default:
			segs = append(segs, topicSegment{kind: segLiteral, literal: p})
		}
	}
	if captures != 1 {
		return nil, fmt.Errorf("template %q: want exactly one %s, got %d", template, deviceNumberPlaceholder, captures)
	}
	if segs[0].kind != segLiteral {
		return nil, fmt.Errorf("template %q: first segment must be literal", template)
	}
	return &downlinkTemplate{source: template, segments: segs}, nil
}

func buildDownlinkMatcher(templates []string) (*downlinkMatcher, error) {
	m := &downlinkMatcher{bySegments: make(map[int]map[string][]*downlinkTemplate)}
	for _, tpl := range templates {
		dt, err := compileDownlinkTemplate(tpl)
		if err != nil {
			return nil, err
		}
		n := len(dt.segments)
		byFirst := m.bySegments[n]
		if byFirst == nil {
			byFirst = make(map[string][]*downlinkTemplate)
			m.bySegments[n] = byFirst
		}
		first := dt.segments[0].literal
		byFirst[first] = append(byFirst[first], dt) // 按输入顺序追加，桶内天然按优先级升序
		if n > m.maxSegs {
			m.maxSegs = n
		}
	}
	return m, nil
}

func mustBuildDownlinkMatcher(templates []string) *downlinkMatcher {
	m, err := buildDownlinkMatcher(templates)
	if err != nil {
		panic("aetherlink: invalid downlink topic template: " + err.Error())
	}
	return m
}

// match 在不分配内存的前提下提取设备号：先数段数并切出首段定位候选桶，再逐段比较。
func (m *downlinkMatcher) match(topic string) (string, bool) {
	if topic == "" {
		return "", false
	}
	n := strings.Count(topic, "/") + 1
	if n > m.maxSegs {
		return "", false
	}
	byFirst := m.bySegments[n]
	if byFirst == nil {
		return "", false
	}
	first, rest, _ := strings.Cut(topic, "/")
	candidates := byFirst[first]
	for _, dt := range candidates {
		if dn, ok := dt.matchRest(rest); ok {
			return dn, true
		}
	}
	return "", false
}

// matchRest 匹配首段之后的部分；段数已由调用方保证一致。
func (dt *downlinkTemplate) matchRest(rest string) (string, bool) {
	captured := ""
	for i := 1; i < len(dt.segments); i++ {
		var seg string
		if i == len(dt.segments)-1 {
			seg = rest
		} else {
			seg, rest, _ = strings.Cut(rest, "/")
		}
		s := dt.segments[i]
		switch s.kind {
		case segLiteral:
			if seg != s.literal {
				return "", false
			}
		case segAny:
			if seg == "" {
				return "", false
			}
		case segCapture:
			if seg == "" {
				return "", false
			}
			captured = seg
		}
	}
	return captured, true
}

// TryExtractDeviceNumberFromNormalized 从规范化下行主题中提取设备号；签名与语义保持不变。
func TryExtractDeviceNumberFromNormalized(topic string) (string, bool) {
	return downlinkMatcherInstance.match(topic)
}

// topicTemplateRegexpSource 把用户映射模板转成锚定正则源串：{var} 与 '+' 各匹配一个非空且不含 '/' 的片段，
// 其余字符按字面量转义。旧实现先替换 {var} 再全局替换 "+"，会把已生成的 `[^/]+` 改写成 `[^/][^/]+`
// （单字符变量匹配失败），且未转义 '.' 等元字符；这里一次扫描生成，杜绝二次改写。
func topicTemplateRegexpSource(template string) (string, bool) {
	if strings.Contains(template, "#") {
		return "", false
	}
	var b strings.Builder
	b.Grow(len(template) + 16)
	b.WriteByte('^')
	last := 0
	for _, loc := range varPlaceholderRegexp.FindAllStringIndex(template, -1) {
		writeLiteralWithPlus(&b, template[last:loc[0]])
		b.WriteString(`[^/]+`)
		last = loc[1]
	}
	writeLiteralWithPlus(&b, template[last:])
	b.WriteByte('$')
	return b.String(), true
}

func writeLiteralWithPlus(b *strings.Builder, literal string) {
	for {
		before, after, found := strings.Cut(literal, "+")
		b.WriteString(regexp.QuoteMeta(before))
		if !found {
			return
		}
		b.WriteString(`[^/]+`)
		literal = after
	}
}

func compileTopicTemplate(template string) (*regexp.Regexp, bool) {
	pattern, ok := topicTemplateRegexpSource(template)
	if !ok {
		return nil, false
	}
	return cachedCompile(pattern)
}

func compileSourcePattern(source string) (*regexp.Regexp, bool) {
	return compileTopicTemplate(source)
}

func applyTarget(target string, _ string) string {
	return target
}

func compileTargetPattern(target string) (*regexp.Regexp, bool) {
	return compileTopicTemplate(target)
}

func renderTopicFromTemplate(template string, vars map[string]string) string {
	out := template
	for key, value := range vars {
		placeholder := "{" + key + "}"
		out = strings.ReplaceAll(out, placeholder, value)
	}
	return out
}
