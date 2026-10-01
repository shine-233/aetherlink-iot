// 文件用途：MatchURLPattern 锚定模式匹配的表驱动测试——重点防"子串误命中"回归。
package utils

import (
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestMatchURLPattern(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		pattern string
		want    bool
	}{
		// 参数段命中
		{"param route hit", "api/v1/device/123", "api/v1/device/:id", true},
		{"param route uuid", "api/v1/device/6f04df62-6704", "api/v1/device/:id", true},
		{"multi param", "api/v1/device/1/msg/2", "api/v1/device/:id/msg/:msgId", true},
		// 字面段不误伤
		{"sibling prefix no hit", "api/v1/devices", "api/v1/device/:id", false},
		{"trailing junk no hit", "api/v1/devicesXYZ", "api/v1/devices", false},
		{"longer path no hit", "api/v1/device/123/extra", "api/v1/device/:id", false},
		{"different tail no hit", "api/v1/device/123", "api/v1/device/:id/msg", false},
		// 正则元字符按字面
		{"dot literal", "api/v1/v1.2/x", "api/v1/v1.2/:id", true},
		{"dot not wildcard", "api/v1/v1x2/x", "api/v1/v1.2/:id", false},
		// 空模式与空路径
		{"empty pattern", "api/v1/x", "", false},
		{"empty url vs literal", "", "api/v1/x", false},
		// 尾斜杠宽容（中间件 TrimLeft 口径下两侧均无尾斜杠，但登记方可能带）
		{"pattern trailing slash", "api/v1/devices", "api/v1/devices/", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchURLPattern(tc.url, tc.pattern); got != tc.want {
				t.Fatalf("MatchURLPattern(%q, %q) = %v, want %v", tc.url, tc.pattern, got, tc.want)
			}
		})
	}
}

// legacyMatchURLPatternRegex 是重写前的正则实现，原样保留为等价性 oracle：
// 新的分段匹配器必须在任何输入上与之结果一致。
func legacyMatchURLPatternRegex(url, pattern string) bool {
	if pattern == "" {
		return false
	}
	segs := strings.Split(strings.Trim(pattern, "/"), "/")
	for i, s := range segs {
		if len(s) > 1 && strings.HasPrefix(s, ":") {
			segs[i] = "[^/]+"
		} else {
			segs[i] = regexp.QuoteMeta(s)
		}
	}
	re, err := regexp.Compile("^/" + strings.Join(segs, "/") + "/?$")
	if err != nil {
		return false
	}
	return re.MatchString("/" + strings.Trim(url, "/"))
}

// urlEquivalenceCorpus 构造覆盖边角的路径/模式对（前后缀攻击、空段、单独 ':'、
// 双斜杠、尾斜杠、正则元字符、非法 UTF-8 与 U+FFFD、换行等）。
func urlEquivalenceCorpus() []string {
	base := []string{
		"", "/", "//", "///", ":", "::", ":a", "/:a/", ":a:b", "a:", "a/:", ":/a",
		"api", "api/", "/api", "api/v1", "api/v1/", "/api/v1/", "api//v1", "api/v1//",
		"api/v1/devices", "api/v1/devicesXYZ", "api/v1/device", "api/v1/device/:id",
		"api/v1/device/123", "api/v1/device/123/", "api/v1/device/123/extra",
		"api/v1/device/:id/msg/:msgId", "api/v1/device/1/msg/2", "api/v1/device//msg/2",
		"api/v1/device/:id/msg", "api/v1/v1.2/:id", "api/v1/v1.2/x", "api/v1/v1x2/x",
		"api/v1/(a|b)/*", "api/v1/a+/[x]", "api/v1/^$", `api/v1/\d`,
		"api/v1/x\n", "api/v1/x\ny", "\napi", "api/v1/:id\n",
		"api/v1/\xff", "api/v1/\uFFFD", "api/v1/a\xffb", "api/v1/a\uFFFDb",
		"api/v1/\xff/:id", "api/v1/\uFFFD/:id", "api/v1/\xc3", "api/v1/é/:id", "api/v1/é/1",
		"API/V1/DEVICES", " api/v1", "api/v1 ",
	}
	return base
}

func TestMatchURLPatternEquivalentToRegexOracle(t *testing.T) {
	corpus := urlEquivalenceCorpus()
	// 确定性随机扩充：从字母表拼接，覆盖段数/空段/参数段组合。
	rng := rand.New(rand.NewPCG(20261001, 7))
	alphabet := []string{"/", "/", "a", "b", ":", ":x", ".", "\xff", "\uFFFD", "1", ""}
	for i := 0; i < 3000; i++ {
		var sb strings.Builder
		for n := rng.IntN(7); n >= 0; n-- {
			sb.WriteString(alphabet[rng.IntN(len(alphabet))])
		}
		corpus = append(corpus, sb.String())
	}
	mismatches := 0
	for _, pattern := range corpus {
		for _, url := range corpus {
			want := legacyMatchURLPatternRegex(url, pattern)
			if got := MatchURLPattern(url, pattern); got != want {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("MatchURLPattern(%q, %q) = %v, oracle = %v", url, pattern, got, want)
				}
			}
		}
		if t.Failed() && mismatches > 20 {
			t.Fatalf("too many mismatches (%d+)", mismatches)
		}
	}
}

func FuzzMatchURLPatternEquivalence(f *testing.F) {
	for _, s := range urlEquivalenceCorpus() {
		f.Add(s, "api/v1/device/:id")
		f.Add("api/v1/device/1", s)
	}
	f.Fuzz(func(t *testing.T, url, pattern string) {
		if got, want := MatchURLPattern(url, pattern), legacyMatchURLPatternRegex(url, pattern); got != want {
			t.Fatalf("MatchURLPattern(%q, %q) = %v, oracle = %v", url, pattern, got, want)
		}
	})
}

func TestMatchURLPatternZeroAllocOnCachedPattern(t *testing.T) {
	MatchURLPattern("api/v1/device/1", "api/v1/device/:id")
	allocs := testing.AllocsPerRun(200, func() {
		MatchURLPattern("api/v1/device/123/msg/9", "api/v1/device/:id/msg/:msgId")
	})
	if allocs != 0 {
		t.Fatalf("cached MatchURLPattern allocs = %v, want 0", allocs)
	}
}

func TestURLPatternCacheBounded(t *testing.T) {
	// 本测试会填满全局缓存，结束后清空，避免影响其他用例。
	t.Cleanup(func() {
		urlPatternCache.Clear()
		urlPatternCacheSize.Store(0)
	})
	// 超过上限后仍返回正确结果（不入缓存），且缓存计数不越界。
	for i := 0; i < urlPatternCacheLimit+50; i++ {
		p := "bounded/" + strconv.Itoa(i) + "/:id"
		if !MatchURLPattern("bounded/"+strconv.Itoa(i)+"/7", p) {
			t.Fatalf("pattern %q should match", p)
		}
	}
	if n := urlPatternCacheSize.Load(); n > urlPatternCacheLimit {
		t.Fatalf("cache size %d exceeds limit %d", n, urlPatternCacheLimit)
	}
}

func TestPrewarmURLPatterns(t *testing.T) {
	if n := PrewarmURLPatterns("", "prewarm/:id", "prewarm/list"); n != 2 {
		t.Fatalf("PrewarmURLPatterns = %d, want 2", n)
	}
	if _, ok := urlPatternCache.Load("prewarm/:id"); !ok {
		t.Fatal("prewarmed pattern should be cached")
	}
}

// benchURLPatterns 生成 ~300 条与 sql/63.sql 规模相当的登记模式。
func benchURLPatterns() []string {
	res := []string{"device", "product", "alarm", "scene", "user", "role", "tenant", "ota", "rule", "notify"}
	out := make([]string, 0, 300)
	for _, r := range res {
		for k := 0; k < 30; k++ {
			switch k % 3 {
			case 0:
				out = append(out, "api/v1/"+r+"/op"+strconv.Itoa(k))
			case 1:
				out = append(out, "api/v1/"+r+"/op"+strconv.Itoa(k)+"/:id")
			default:
				out = append(out, "api/v1/"+r+"/:id/op"+strconv.Itoa(k)+"/:sub")
			}
		}
	}
	return out
}

// BenchmarkMatchURLPattern 模拟一次请求对 300 条模式全量求值（Enforce 侧最坏情况）。
func BenchmarkMatchURLPattern(b *testing.B) {
	patterns := benchURLPatterns()
	url := "api/v1/notify/abc/op29/xyz"
	b.Run("segment", func(b *testing.B) {
		PrewarmURLPatterns(patterns...)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, p := range patterns {
				MatchURLPattern(url, p)
			}
		}
	})
	b.Run("legacy-regex", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, p := range patterns {
				legacyMatchURLPatternRegex(url, p)
			}
		}
	})
}
