// 文件用途：启动期输出「受保护路由 ↔ casbin 资源登记」一致性报告。
// 核心逻辑：遍历 gin 已注册路由，剔除公开/运维/JWT-only 白名单后，对剩余路由用与
//
//	CasbinRBAC 中间件完全相同的 GetUrl 语义判定是否已登记；未登记者以 WARN 列出。
//
// 关键注意事项：casbin 当前为 fail-open（资源未登记即不校验），新增接口忘记登记等于
//
//	无 RBAC——本报告把该静默风险显性化。只报告不阻断（防止资源种子滞后把升级砖死）。
//	白名单必须与 router_init.go 的公开路由区块保持同步：新增公开路由时须在下方登记，
//	否则会在报告中产生误报噪音。
package router

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	service "aetherlink-iot/backend/internal/service"
)

// casbinExemptRoutes 是无需 casbin 登记的路由白名单（键为去掉前导斜杠的 path）。
var casbinExemptRoutes = map[string]struct{}{
	// 运维/健康面（router_init.go 顶部区块）
	"health":            {},
	"ready":             {},
	"metrics":           {},
	"metrics-viewer":    {},
	"deployment/health": {},

	// 公开 API（router_init.go 「无需权限校验」区块）
	"api/v1/login":                                         {},
	"api/v1/verification/code":                             {},
	"api/v1/reset/password/link":                           {},
	"api/v1/reset/password":                                {},
	"api/v1/logo":                                          {},
	"api/v1/telemetry/datas/current/ws":                    {},
	"api/v1/device/online/status/ws":                       {},
	"api/v1/device/online/status/ws/batch":                 {},
	"api/v1/telemetry/datas/current/keys/ws":               {},
	"api/v1/ota/download/files/upgradePackage/:path/:file": {},
	"api/v1/rdi/shared/:token":                             {},
	"api/v1/board/shared/:token":                           {},
	"api/v1/systime":                                       {},
	"api/v1/sys_function":                                  {},
	"api/v1/deployment/health":                             {},
	"api/v1/tenant/email/register":                         {},
	"api/v1/tenant/has-admin":                              {},
	"api/v1/tenant/setup-state":                            {},
	"api/v1/tenant/super-admin/init":                       {},
	"api/v1/tenant/market-register":                        {},
	"api/v1/device/gateway-register":                       {},
	"api/v1/device/gateway-sub-register":                   {},
	"api/v1/sys_version":                                   {},
	"api/v1/device/auth":                                   {},

	// JWT-only、明确不走 casbin 菜单权限的特例
	"api/v1/devices/:device_id/diagnostics": {},
}

// casbinExemptPrefixes 是按前缀豁免的路径（协议插件走独立 PluginAuth 边界）。
var casbinExemptPrefixes = []string{
	"swagger/",
	"metrics-viewer/",
	"files/",
	"api/v1/plugin/",
}

func casbinRouteExempt(path string) bool {
	if _, ok := casbinExemptRoutes[path]; ok {
		return true
	}
	for _, prefix := range casbinExemptPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// collectCasbinRegistrationGaps 返回所有「会被 casbin fail-open 放行」的受保护路由。
// isRegistered 注入判定函数以便测试；生产传入与 CasbinRBAC 中间件相同的 GetUrl 语义。
func collectCasbinRegistrationGaps(routes []gin.RouteInfo, isRegistered func(string) bool) []string {
	gaps := make([]string, 0)
	for _, r := range routes {
		path := strings.TrimLeft(r.Path, "/")
		if path == "" || casbinRouteExempt(path) {
			continue
		}
		if isRegistered(path) {
			continue
		}
		gaps = append(gaps, r.Method+" "+r.Path)
	}
	return gaps
}

// LogCasbinRegistrationGaps 在启动装配完成后输出一致性报告；enforcer 未就绪时跳过。
func LogCasbinRegistrationGaps(engine *gin.Engine) {
	if engine == nil {
		return
	}
	gaps := collectCasbinRegistrationGaps(engine.Routes(), func(path string) bool {
		return service.GroupApp.Casbin.GetUrl(path)
	})
	if len(gaps) == 0 {
		logrus.Info("[Casbin][registration-report] all protected routes are registered in casbin resources")
		return
	}
	logrus.Warnf("[Casbin][registration-report] %d protected route(s) NOT registered in casbin resources; they currently bypass RBAC (fail-open):", len(gaps))
	for _, gap := range gaps {
		logrus.Warnf("[Casbin][registration-report] unregistered route: %s", gap)
	}
}
