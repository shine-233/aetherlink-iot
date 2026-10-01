// 文件用途：GetUrl 快照索引的等价性、失效重建与基准测试。
// 核心逻辑：以重写前的"精确过滤 + 全量模式枚举"实现为 oracle，断言索引结果逐一致；
//
//	覆盖 LoadPolicy 整体换模型、增量 Add/Remove、替换 enforcer 三类失效路径。
package service

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyGetUrlScan 是重写前的 GetUrl 实现（oracle）。
func legacyGetUrlScan(url string) bool {
	if global.CasbinEnforcer == nil {
		return false
	}
	stringList, err := global.CasbinEnforcer.GetFilteredNamedGroupingPolicy("g2", 0, url)
	if err == nil && len(stringList) != 0 {
		return true
	}
	rules, err := global.CasbinEnforcer.GetNamedGroupingPolicy("g2")
	if err != nil {
		return false
	}
	for _, rule := range rules {
		if len(rule) > 0 && utils.MatchURLPattern(url, rule[0]) {
			return true
		}
	}
	return false
}

var urlIndexPatterns = []string{
	"api/v1/devices", "api/v1/device/:id", "api/v1/device/:id/msg/:msgId",
	"api/v1/device/list", "api/v1/device/:id/list", "api/v1/:tenant/device/stats",
	"/api/v1/slash/", "api/v1/:", "api/v1/v1.2/:id", "api/v1/a//b", "/",
	"api/v1/�/:id", "api/v1/bad\xff", "api/v1/:p\xff",
}

var urlIndexProbes = []string{
	"", "/", "//", "api", "api/v1", "api/v1/devices", "api/v1/devices/", "/api/v1/devices",
	"api/v1/devicesXYZ", "api/v1/device", "api/v1/device/1", "api/v1/device/1/",
	"api/v1/device/:id", "api/v1/device/list", "api/v1/device/1/list", "api/v1/device/list/list",
	"api/v1/device/1/msg/2", "api/v1/device//msg/2", "api/v1/device/1/msg", "api/v1/t1/device/stats",
	"api/v1/device/device/stats", "api/v1/slash", "api/v1/slash/", "api/v1/:", "api/v1/x",
	"api/v1/v1.2/9", "api/v1/v1x2/9", "api/v1/a//b", "api/v1/a/b", "api/v1/\xff/1",
	"api/v1/�/1", "api/v1/bad\xff", "api/v1/p", "api/v1/device/1/msg/2/3",
}

func addG2(t testing.TB, patterns ...string) {
	t.Helper()
	rules := make([][]string, 0, len(patterns))
	for i, p := range patterns {
		rules = append(rules, []string{p, "res-" + strconv.Itoa(i)})
	}
	_, err := global.CasbinEnforcer.AddNamedGroupingPolicies("g2", rules)
	require.NoError(t, err)
}

func assertIndexMatchesOracle(t *testing.T) {
	t.Helper()
	c := &Casbin{}
	for _, u := range urlIndexProbes {
		assert.Equal(t, legacyGetUrlScan(u), c.GetUrl(u), "GetUrl(%q) 与旧全量扫描结果不一致", u)
	}
}

func TestGetUrlURLIndexEquivalentToLegacyScan(t *testing.T) {
	setupPatternCasbinEnforcer(t)
	assertIndexMatchesOracle(t) // 空 g2
	addG2(t, urlIndexPatterns...)
	assertIndexMatchesOracle(t)
}

func TestGetUrlURLIndexRebuildsAfterIncrementalChanges(t *testing.T) {
	setupPatternCasbinEnforcer(t)
	c := &Casbin{}
	addG2(t, "api/v1/devices")
	require.False(t, c.GetUrl("api/v1/device/7"))

	addG2(t, "api/v1/device/:id") // 增量新增
	assert.True(t, c.GetUrl("api/v1/device/7"), "新增模式后索引须自动重建")

	_, err := global.CasbinEnforcer.RemoveFilteredNamedGroupingPolicy("g2", 0, "api/v1/device/:id")
	require.NoError(t, err)
	assert.False(t, c.GetUrl("api/v1/device/7"), "删除模式后索引不得保留陈旧登记")

	_, err = global.CasbinEnforcer.UpdateNamedGroupingPolicy("g2",
		[]string{"api/v1/devices", "res-0"}, []string{"api/v1/products/:id", "res-0"})
	require.NoError(t, err)
	assert.False(t, c.GetUrl("api/v1/devices"), "原地更新后旧串不得命中")
	assert.True(t, c.GetUrl("api/v1/products/3"), "原地更新后新模式须命中")
	assertIndexMatchesOracle(t)
}

func TestGetUrlURLIndexFollowsEnforcerReplacement(t *testing.T) {
	setupPatternCasbinEnforcer(t)
	addG2(t, "api/v1/device/:id")
	c := &Casbin{}
	require.True(t, c.GetUrl("api/v1/device/1"))

	setupPatternCasbinEnforcer(t) // 替换为全新 enforcer（无 g2 行）
	assert.False(t, c.GetUrl("api/v1/device/1"), "enforcer 替换后不得沿用旧索引")
}

func TestGetUrlURLIndexWithoutG2Definition(t *testing.T) {
	m, err := model.NewModelFromString(`
[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`)
	require.NoError(t, err)
	e, err := casbin.NewSyncedEnforcer(m)
	require.NoError(t, err)
	old := global.CasbinEnforcer
	t.Cleanup(func() { global.CasbinEnforcer = old })
	global.CasbinEnforcer = e
	assert.False(t, legacyGetUrlScan("api/v1/x"), "oracle 同样为 false")
	assert.False(t, (&Casbin{}).GetUrl("api/v1/x"))
}

// 并发读 + 写交错：-race 下验证索引读锁内校验/重建无数据竞争，且结果始终与策略一致。
func TestGetUrlURLIndexConcurrentWithPolicyWrites(t *testing.T) {
	setupPatternCasbinEnforcer(t)
	addG2(t, "api/v1/devices")
	c := &Casbin{}
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_ = c.GetUrl("api/v1/devices")
				_ = c.GetUrl("api/v1/w/" + strconv.Itoa(i))
			}
		}()
	}
	for i := 0; i < 50; i++ {
		_, _ = global.CasbinEnforcer.AddNamedGroupingPolicy("g2", "api/v1/w"+strconv.Itoa(i)+"/:id", "r")
	}
	wg.Wait()
	assert.True(t, c.GetUrl("api/v1/devices"))
	assert.True(t, c.GetUrl("api/v1/w49/1"))
}

// benchG2Patterns 生成 ~300 条与 sql/63.sql 规模相当的 g2 登记。
func benchG2Patterns() []string {
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

// legacyRegexMatch 复刻重写前每次调用都 regexp.Compile 的匹配实现，仅供基准对比。
func legacyRegexMatch(url, pattern string) bool {
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

// BenchmarkGetUrlParamRoute：参数路由（精确必未命中）——旧实现在此路径全量枚举模式。
// legacy-regex-scan 为重写前真实成本（全量枚举 + 每模式 regexp.Compile）。
func BenchmarkGetUrlParamRoute(b *testing.B) {
	m, err := model.NewModelFromString(`
[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
g2 = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = g(r.sub, p.sub) && (g2(r.obj, p.obj) || urlPatternMatch(r.obj, p.obj)) && r.act == p.act
`)
	require.NoError(b, err)
	e, err := casbin.NewSyncedEnforcer(m)
	require.NoError(b, err)
	e.AddFunction("urlPatternMatch", utils.URLPatternCasbinFunction())
	old := global.CasbinEnforcer
	b.Cleanup(func() { global.CasbinEnforcer = old })
	global.CasbinEnforcer = e
	addG2(b, benchG2Patterns()...)
	url := "api/v1/notify/abc/op29/xyz" // 命中最后一条模式
	miss := "api/v1/notify/abc/nope/xyz"
	c := &Casbin{}
	b.Run("index-hit", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if !c.GetUrl(url) {
				b.Fatal("want hit")
			}
		}
	})
	b.Run("index-miss", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if c.GetUrl(miss) {
				b.Fatal("want miss")
			}
		}
	})
	b.Run("legacy-regex-scan-hit", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rules, _ := global.CasbinEnforcer.GetNamedGroupingPolicy("g2")
			hit := false
			for _, rule := range rules {
				if legacyRegexMatch(url, rule[0]) {
					hit = true
					break
				}
			}
			if !hit {
				b.Fatal("want hit")
			}
		}
	})
	b.Run("legacy-scan-cached-matcher-hit", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if !legacyGetUrlScan(url) {
				b.Fatal("want hit")
			}
		}
	})
}
