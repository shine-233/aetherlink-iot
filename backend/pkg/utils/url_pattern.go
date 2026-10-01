// 文件用途：锚定的 RESTful URL 模式匹配（casbin 资源登记双通道之"模式通道"）。
// 核心逻辑：模式中 ":name" 段（gin 参数路由约定）匹配单个非空、非 "/" 段，其余段按字面
//
//	逐段比较，整体锚定（段数必须相等）。模式解析一次后缓存（sync.Map），
//	匹配阶段零分配：不再每次调用 regexp.Compile。
//
// 关键注意事项：不用 casbin util.KeyMatch2——其内部是非锚定正则，"api/v1/devices"
//
//	模式会子串命中 "api/v1/devicesXYZ"，在 Enforce 侧构成越权放大；本实现锚定整串，
//	段级通配仅限 ":name"，杜绝该类误匹配。
//	语义与旧正则实现（"^/" + 段 + "/?$"，字面段 QuoteMeta）逐位等价，
//	由 url_pattern_test.go 中保留的正则 oracle 做等价性/模糊测试守护，含两处历史边角：
//	  · 模式字面段含非法 UTF-8 → 旧实现 regexp.Compile 失败恒为 false，此处同样恒不命中
//	    （参数段内的非法字节不影响，与旧实现一致）；
//	  · 正则按 rune 比较，请求中的非法字节解码为 U+FFFD，可与模式中的字面 U+FFFD 相等，
//	    此处对含 U+FFFD 的字面段走 rune 级比较以保持一致。
package utils

import (
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

// URLPatternSegment 是已解析模式的一个段：Param=true 表示 ":name" 整段通配。
type URLPatternSegment struct {
	Literal string
	Param   bool
}

// CompiledURLPattern 是解析后的不可变模式，可安全并发复用。
type CompiledURLPattern struct {
	segs []URLPatternSegment
	// never：空模式或字面段含非法 UTF-8（旧实现编译失败）——恒不命中。
	never bool
	// fuzzy：存在含 U+FFFD 的字面段，需 rune 级比较（见文件头注释）。
	fuzzy bool
}

// Segments 返回解析出的段（只读视图，调用方不得修改）。
func (p *CompiledURLPattern) Segments() []URLPatternSegment { return p.segs }

// Never 报告该模式是否恒不命中任何路径。
func (p *CompiledURLPattern) Never() bool { return p.never }

// Fuzzy 报告该模式是否含需要 rune 级比较的字面段（U+FFFD）。
func (p *CompiledURLPattern) Fuzzy() bool { return p.fuzzy }

// urlPatternCacheLimit 限制缓存条目数：MatchURLPattern 为导出函数，
// 防止调用方以任意模式串撑爆内存（生产规模 ~300 模式，远低于上限）。
const urlPatternCacheLimit = 8192

var (
	urlPatternCache     sync.Map // pattern string -> *CompiledURLPattern
	urlPatternCacheSize atomic.Int64
)

// ParseURLPattern 解析模式（不查缓存）。
func ParseURLPattern(pattern string) *CompiledURLPattern {
	if pattern == "" {
		return &CompiledURLPattern{never: true}
	}
	trimmed := strings.Trim(pattern, "/")
	segs := make([]URLPatternSegment, 0, strings.Count(trimmed, "/")+1)
	p := &CompiledURLPattern{}
	for {
		j := strings.IndexByte(trimmed, '/')
		s := trimmed
		if j >= 0 {
			s = trimmed[:j]
		}
		// 与旧实现一致：仅 len>1 且以 ':' 开头才是参数段；单独的 ":" 按字面处理。
		if len(s) > 1 && s[0] == ':' {
			segs = append(segs, URLPatternSegment{Param: true})
		} else {
			segs = append(segs, URLPatternSegment{Literal: s})
			// 旧实现仅对字面段 QuoteMeta 后编译；字面段含非法 UTF-8 即编译失败。
			// 参数段整体被替换为 [^/]+，其中的非法字节不影响编译。
			if !utf8.ValidString(s) {
				p.never = true
			}
			if strings.ContainsRune(s, utf8.RuneError) {
				p.fuzzy = true
			}
		}
		if j < 0 {
			break
		}
		trimmed = trimmed[j+1:]
	}
	p.segs = segs
	return p
}

// CompileURLPattern 返回缓存的已解析模式（首次解析后复用）。
func CompileURLPattern(pattern string) *CompiledURLPattern {
	if v, ok := urlPatternCache.Load(pattern); ok {
		return v.(*CompiledURLPattern)
	}
	p := ParseURLPattern(pattern)
	if urlPatternCacheSize.Load() >= urlPatternCacheLimit {
		return p
	}
	if v, loaded := urlPatternCache.LoadOrStore(pattern, p); loaded {
		return v.(*CompiledURLPattern)
	}
	urlPatternCacheSize.Add(1)
	return p
}

// PrewarmURLPatterns 预解析一批模式进缓存（策略加载后调用，消除首请求解析抖动）。
// 返回本次处理的非空模式数。
func PrewarmURLPatterns(patterns ...string) int {
	n := 0
	for _, s := range patterns {
		if s == "" {
			continue
		}
		CompileURLPattern(s)
		n++
	}
	return n
}

// NormalizeURLPath 把请求路径归一为比较口径：去掉首尾 "/"（与旧实现 Trim 一致）。
func NormalizeURLPath(url string) string { return strings.Trim(url, "/") }

// LiteralSegmentEqual 按旧正则语义比较字面段：fuzzy=false 时即字节相等；
// fuzzy=true 时逐 rune 比较（非法字节按 U+FFFD 计）。
func LiteralSegmentEqual(lit, seg string, fuzzy bool) bool {
	if lit == seg {
		return true
	}
	if !fuzzy {
		return false
	}
	for lit != "" && seg != "" {
		r1, n1 := utf8.DecodeRuneInString(lit)
		r2, n2 := utf8.DecodeRuneInString(seg)
		if r1 != r2 {
			return false
		}
		lit, seg = lit[n1:], seg[n2:]
	}
	return lit == "" && seg == ""
}

// MatchNormalized 匹配已归一化（NormalizeURLPath）的路径；零分配。
func (p *CompiledURLPattern) MatchNormalized(path string) bool {
	if p.never {
		return false
	}
	rest := path
	for i := range p.segs {
		j := strings.IndexByte(rest, '/')
		last := j < 0
		seg := rest
		if !last {
			seg = rest[:j]
		}
		if last != (i == len(p.segs)-1) {
			return false // 段数不等：锚定语义
		}
		ps := &p.segs[i]
		if ps.Param {
			if seg == "" {
				return false // [^/]+ 至少一个字符
			}
		} else if !LiteralSegmentEqual(ps.Literal, seg, p.fuzzy) {
			return false
		}
		if !last {
			rest = rest[j+1:]
		}
	}
	return true
}

// Match 判断具体请求路径 url 是否命中该模式。
func (p *CompiledURLPattern) Match(url string) bool {
	return p.MatchNormalized(NormalizeURLPath(url))
}

// MatchURLPattern 判断具体请求路径 url 是否命中登记模式 pattern。
// 双方均为去掉前导 "/" 的路径（与 CasbinRBAC 中间件口径一致）。
// 约定：":name" 仅在段首生效，整段通配（非空、不含 "/"）；空段按字面处理。
func MatchURLPattern(url, pattern string) bool {
	if pattern == "" {
		return false
	}
	return CompileURLPattern(pattern).Match(url)
}

// URLPatternCasbinFunction 返回可注入 casbin Enforcer 的自定义 matcher 函数
// （configs/casbin.conf 的 urlPatternMatch；initialize.CasbinInit 经 AddFunction 注册）。
// 生产与测试共用同一实现，杜绝夹具模型与线上 matcher 漂移。
// Enforce 对每条 p 策略求值一次，此处走缓存模式，热路径无正则编译。
func URLPatternCasbinFunction() func(args ...interface{}) (interface{}, error) {
	return func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return false, nil
		}
		url, ok1 := args[0].(string)
		pattern, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return false, nil
		}
		return MatchURLPattern(url, pattern), nil
	}
}
