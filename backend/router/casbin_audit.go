// 文件用途：Casbin 路由覆盖审计的 Gin 集成与启动期 fail-fast 执行点。
// 核心逻辑：以"挂载 CasbinRBAC 之前"的路由快照为基线，其后注册的一切路由都被视为受保护路由，
// 启动时逐一核对 g2 资源表；未登记路由按配置阻断启动（fail-fast）或降级告警。
// 关键注意事项：基线快照必须严格取在 v1.Use(middleware.CasbinRBAC()) 之前，
// 否则会把公开/JWT-only 路由误判为受保护路由导致误报阻断。
package router

import (
	"strings"

	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/global"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ginRoutePaths 返回引擎当前全部路由（去除前导 "/"，与 CasbinRBAC 中间件口径一致）。
func ginRoutePaths(engine *gin.Engine) []string {
	registered := engine.Routes()
	paths := make([]string, 0, len(registered))
	for _, route := range registered {
		paths = append(paths, strings.TrimLeft(route.Path, "/"))
	}
	return paths
}

// pathsAddedSince 返回 all 中相对 baseline 新增的路径（保持稳定去重）。
func pathsAddedSince(all, baseline []string) []string {
	base := make(map[string]struct{}, len(baseline))
	for _, path := range baseline {
		base[path] = struct{}{}
	}
	added := make([]string, 0, len(all))
	seen := make(map[string]struct{}, len(all))
	for _, path := range all {
		if _, ok := base[path]; ok {
			continue
		}
		if _, dup := seen[path]; dup {
			continue
		}
		seen[path] = struct{}{}
		added = append(added, path)
	}
	return added
}

// auditCasbinRouteCoverage 在 RouterInit 尾部执行：
//   - fail-fast（默认）：存在未登记路由直接 Fatal，杜绝"新接口忘记登记=绕过鉴权"；
//   - warn：仅告警，供存量库升级过渡期使用；
//   - off：关闭（不推荐）。
//
// enforcer 未初始化（单测/极简装配）时跳过并告警，避免误伤无 DB 场景。
func auditCasbinRouteCoverage(engine *gin.Engine, baselineRoutes []string) {
	mode := strings.TrimSpace(strings.ToLower(viper.GetString("casbin.route-audit-mode")))
	if mode == "" {
		mode = "fail-fast"
	}
	if mode == "off" {
		return
	}
	if global.CasbinEnforcer == nil {
		logrus.Warn("casbin route audit skipped: casbin enforcer is not initialized")
		return
	}

	gated := pathsAddedSince(ginRoutePaths(engine), baselineRoutes)
	missing := service.AuditUnregisteredCasbinRoutes(gated, service.GroupApp.Casbin.GetUrl)
	if len(missing) == 0 {
		logrus.Infof("casbin route audit passed: %d protected routes registered", len(gated))
		return
	}

	report := service.FormatCasbinRouteAuditReport(missing)
	if mode == "warn" {
		logrus.Warnf("casbin route audit: %d routes NOT registered in casbin table (they bypass permission checks at runtime):\n%s",
			len(missing), report)
		return
	}
	logrus.Fatalf("casbin route audit: %d protected routes are not registered in the casbin resource table "+
		"(runtime would skip permission checks for them). Register them via menu/API management or move them to the public section:\n%s%s",
		len(missing), report, "Set casbin.route-audit-mode: warn|off to temporarily downgrade during upgrades.")
}
