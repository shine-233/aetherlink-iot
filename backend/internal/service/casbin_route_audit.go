// 文件用途：Casbin 路由覆盖审计的纯函数部分。
// 核心逻辑：判定"受 CasbinRBAC 保护的路由"是否全部登记进 g2 资源表，未登记即意味着运行时静默放行
// （middleware.CasbinRBAC 对未注册资源不做校验），必须在启动期暴露而不是留到被扫描利用。
// 关键注意事项：路径口径与中间件一致——去除前导 "/" 后精确比对；isRegistered 由调用方注入便于测试。
package service

import (
	"fmt"
	"sort"
	"strings"
)

// AuditUnregisteredCasbinRoutes 返回未登记进 Casbin 资源表的路由列表（升序、去重）。
func AuditUnregisteredCasbinRoutes(routes []string, isRegistered func(string) bool) []string {
	seen := make(map[string]struct{}, len(routes))
	missing := make([]string, 0)
	for _, route := range routes {
		route = strings.TrimSpace(route)
		if route == "" {
			continue
		}
		if _, dup := seen[route]; dup {
			continue
		}
		seen[route] = struct{}{}
		if !isRegistered(route) {
			missing = append(missing, route)
		}
	}
	sort.Strings(missing)
	return missing
}

// FormatCasbinRouteAuditReport 把缺失清单格式化为可贴进工单/迁移说明的文本。
func FormatCasbinRouteAuditReport(missing []string) string {
	var builder strings.Builder
	for _, route := range missing {
		builder.WriteString(fmt.Sprintf("  - %s\n", route))
	}
	return builder.String()
}
