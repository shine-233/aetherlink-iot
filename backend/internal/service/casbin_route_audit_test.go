// 文件用途：验证 Casbin 路由覆盖审计纯函数的去重、排序与报告格式契约。
// 核心逻辑：钉死"未登记即暴露"的判定口径与中间件一致的路径规范化行为。
// 关键注意事项：安全边界回归防线；isRegistered 注入桩，不依赖真实 enforcer。
package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuditUnregisteredCasbinRoutes(t *testing.T) {
	registered := map[string]struct{}{
		"api/v1/user/list": {},
		"api/v1/board/get": {},
	}
	isRegistered := func(route string) bool {
		_, ok := registered[route]
		return ok
	}

	missing := AuditUnregisteredCasbinRoutes([]string{
		"api/v1/board/get",
		"api/v1/user/list",
		"api/v1/sneaky/admin-action",
		"api/v1/user/list", // 重复项应去重
		"",                 // 空路径跳过
		" api/v1/padded ",  // 首尾空白归一化后参与比对
	}, isRegistered)

	assert.Equal(t, []string{"api/v1/padded", "api/v1/sneaky/admin-action"}, missing)
}

func TestAuditUnregisteredCasbinRoutesAllRegisteredReturnsEmpty(t *testing.T) {
	missing := AuditUnregisteredCasbinRoutes([]string{"api/v1/a", "api/v1/b"}, func(string) bool { return true })
	assert.Empty(t, missing)
}

func TestFormatCasbinRouteAuditReportListsEachOnOwnLine(t *testing.T) {
	report := FormatCasbinRouteAuditReport([]string{"api/v1/b", "api/v1/a"})
	lines := strings.Split(strings.TrimRight(report, "\n"), "\n")
	assert.Len(t, lines, 2)
	assert.Equal(t, "  - api/v1/b", lines[0])
	assert.Equal(t, "  - api/v1/a", lines[1])
}
