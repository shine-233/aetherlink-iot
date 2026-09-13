package apps

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// registeredCasbinPaths 从 backend/sql/*.sql 提取所有被引号包裹的 'api/v1/...' 字面量，
// 即迁移里登记过的 Casbin 资源模式。
//
// 只跳过以 "--" 开头的整行注释：注释里的路径会污染模式集合，让本用例"看起来通过"。
// 行内尾注释不处理——本仓库的登记 SQL 都是整行 VALUES 项，尾注释不写路径。
func registeredCasbinPaths(t *testing.T) []string {
	t.Helper()

	dir := filepath.Join("..", "..", "sql")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read migration dir %s: %v", dir, err)
	}

	re := regexp.MustCompile(`'(api/v1/[^']*)'`)
	seen := make(map[string]bool)
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("cannot read %s: %v", entry.Name(), err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "--") {
				continue
			}
			for _, match := range re.FindAllStringSubmatch(line, -1) {
				if seen[match[1]] {
					continue
				}
				seen[match[1]] = true
				paths = append(paths, match[1])
			}
		}
	}
	if len(paths) == 0 {
		t.Fatalf("no 'api/v1/...' registration found under %s; the parser is broken and this test would be meaningless", dir)
	}
	return paths
}

// TestCasbinRegistrationCoversMountedRoutes 受 Casbin 保护的路由必须全部登记进资源表。
//
// 为什么需要这份证据：router_init.go 在 v1.Use(CasbinRBAC()) 之后才挂载各业务组的路由，
// 而 auditCasbinRouteCoverage 在 casbin.route-audit-mode 默认 fail-fast 下会对任何
// 未登记路由执行 logrus.Fatalf——后端启动期直接拒绝启动。2026-09-12 核查就发现
// SCADA 15 个端点、移动端 11 个端点、影子 ACK 端点全部漏登记（由 94.sql 修复）。
// 这类缺口在 CI 里是静默的：单测绿、构建通过，只有真正启动服务才会炸。
//
// 判定刻意复用生产口径 utils.MatchURLPattern（与 service.Casbin.GetUrl 的回退通道同源），
// 而不是自己再写一遍匹配——两份实现对不上时，测试会通过而线上照样阻断。
func TestCasbinRegistrationCoversMountedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")

	// 这些组的 Init 全部在 router_init.go 的 CasbinRBAC 之后调用，故其路由都属受保护集合。
	(&Scada{}).Init(v1)
	(&Mobile{}).Init(v1)
	(&DeviceShadow{}).InitDeviceShadow(v1)
	(&CommandData{}).InitCommandData(v1)
	(&EntityRelation{}).InitEntityRelation(v1)
	(&ReportSchedule{}).InitReportSchedule(v1)
	(&OTA{}).InitOTA(v1)
	(&RuleChain{}).InitRuleChain(v1)
	// EdgeSync 组含 P1.5 边缘节点四条新路由（97.sql 登记）；挂进来让契约测试持续守住。
	(&EdgeSync{}).InitEdgeSync(v1)
	// Board 组含 P1.x 看板项目分组八条新路由（97.sql 登记）；2026-09-12 全量核对
	// 组内 17 条路径在 63.sql/97.sql 均有登记后挂入。
	(&Board{}).InitBoard(v1)

	registered := registeredCasbinPaths(t)
	isRegistered := func(route string) bool {
		for _, pattern := range registered {
			if utils.MatchURLPattern(route, pattern) {
				return true
			}
		}
		return false
	}

	// 登记粒度是路径：审计用 addedKeysToPaths 去掉 METHOD 后按路径去重，
	// 与 casbin_audit.go 的口径保持一致。
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[strings.TrimLeft(route.Path, "/")] = true
	}
	if len(paths) == 0 {
		t.Fatal("no route mounted; the engine wiring in this test is broken")
	}

	var missing []string
	for path := range paths {
		if !isRegistered(path) {
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Fatalf("%d protected route(s) have no casbin resource registration; "+
			"startup would fail-fast (see router/casbin_audit.go). "+
			"Register them in a new migration under backend/sql/:\n  - %s",
			len(missing), strings.Join(missing, "\n  - "))
	}
}

// TestCasbinRegistrationParserDetectsMissingRoute 负向对照：确认上面的判定不是恒真。
// 若抽取器或匹配器失效（比如正则匹配不到任何东西、或 MatchURLPattern 恒返回 true），
// 一个明显未登记的路由也必须被判为缺失——否则主用例的"全部通过"毫无意义。
func TestCasbinRegistrationParserDetectsMissingRoute(t *testing.T) {
	registered := registeredCasbinPaths(t)

	const bogus = "api/v1/definitely-not-registered-anywhere/segment"
	for _, pattern := range registered {
		if utils.MatchURLPattern(bogus, pattern) {
			t.Fatalf("bogus route %q unexpectedly matched registered pattern %q; "+
				"the matcher is too permissive", bogus, pattern)
		}
	}
}
