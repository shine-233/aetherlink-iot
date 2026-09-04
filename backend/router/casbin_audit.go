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

// ginRouteKeys 返回引擎当前全部路由，形如 "GET api/v1/logo"（METHOD + 去前导斜杠路径）。
// 方法感知：基线/新增的比较必须带 METHOD——同一路径可同时存在公开方法（如 GET /logo）
// 与受保护方法（如 PUT /logo），按纯路径去重会让后者逃过审计（实测缺陷，2026-09-04）。
func ginRouteKeys(engine *gin.Engine) []string {
	registered := engine.Routes()
	keys := make([]string, 0, len(registered))
	for _, route := range registered {
		keys = append(keys, route.Method+" "+strings.TrimLeft(route.Path, "/"))
	}
	return keys
}

// pathsAddedSince 返回 all 中相对 baseline 新增的路由键（保持稳定去重）。
func pathsAddedSince(all, baseline []string) []string {
	base := make(map[string]struct{}, len(baseline))
	for _, key := range baseline {
		base[key] = struct{}{}
	}
	added := make([]string, 0, len(all))
	seen := make(map[string]struct{}, len(all))
	for _, key := range all {
		if _, ok := base[key]; ok {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		added = append(added, key)
	}
	return added
}

// addedKeysToPaths 从 "METHOD path" 键中取出去重后的路径集合（登记粒度是路径，act 由
// Enforce 契约固定为 "allow"，与 HTTP 方法无关）。
func addedKeysToPaths(keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	paths := make([]string, 0, len(keys))
	for _, key := range keys {
		parts := strings.SplitN(key, " ", 2)
		if len(parts) != 2 {
			continue
		}
		if _, dup := seen[parts[1]]; dup {
			continue
		}
		seen[parts[1]] = struct{}{}
		paths = append(paths, parts[1])
	}
	return paths
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

	gatedKeys := pathsAddedSince(ginRouteKeys(engine), baselineRoutes)
	gated := addedKeysToPaths(gatedKeys)
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
