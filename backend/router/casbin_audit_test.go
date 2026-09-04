// 文件用途：验证 Casbin 路由审计 Gin 集成辅助的快照差集与路径口径（方法感知）。
// 核心逻辑：基线差集必须精确圈定"挂载 CasbinRBAC 之后新增的路由"，这是 fail-fast 不误报的前提。
// 关键注意事项：路由键形如 "GET api/v1/logo"（METHOD + 去前导斜杠路径）——同一路径可同时
//
//	存在公开方法与受保护方法，按纯路径去重会让受保护方法逃过审计（2026-09-04 实测缺陷：
//	GET /logo 在公开基线，PUT /logo 受保护却被去重掩盖，激活 deny-unregistered 后 403）。
package router

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newAuditTestEngine(routes ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	for _, route := range routes {
		method, path := splitRouteKey(route)
		engine.Handle(method, path, func(c *gin.Context) {})
	}
	return engine
}

func splitRouteKey(key string) (string, string) {
	parts := strings.SplitN(key, " ", 2)
	return parts[0], "/" + parts[1]
}

func TestGinRouteKeysTrimLeadingSlashAndKeepMethod(t *testing.T) {
	engine := newAuditTestEngine("GET /health", "POST /api/v1/login")
	keys := ginRouteKeys(engine)
	assert.ElementsMatch(t, []string{"GET health", "POST api/v1/login"}, keys)
}

func TestPathsAddedSinceOnlyReportsNewRouteKeys(t *testing.T) {
	baselineEngine := newAuditTestEngine(
		"GET /health",
		"GET /api/v1/login",
		"GET /api/v1/devices/:device_id/diagnostics",
	)

	fullEngine := newAuditTestEngine(
		"GET /health",
		"GET /api/v1/login",
		"GET /api/v1/devices/:device_id/diagnostics",
		"GET /api/v1/device/list",
		"GET /api/v1/alarm/rules",
		"POST /api/v1/device/list",
	)

	added := pathsAddedSince(ginRouteKeys(fullEngine), ginRouteKeys(baselineEngine))
	assert.ElementsMatch(t, []string{
		"GET api/v1/device/list",
		"GET api/v1/alarm/rules",
		"POST api/v1/device/list",
	}, added)
}

// 回归：公开基线里的 GET /api/v1/logo 不得掩盖后来注册的受保护 PUT /api/v1/logo。
func TestPathsAddedSinceMethodAwareSamePathDifferentMethod(t *testing.T) {
	baselineEngine := newAuditTestEngine("GET /api/v1/logo")
	fullEngine := newAuditTestEngine("GET /api/v1/logo", "PUT /api/v1/logo")

	added := pathsAddedSince(ginRouteKeys(fullEngine), ginRouteKeys(baselineEngine))
	assert.ElementsMatch(t, []string{"PUT api/v1/logo"}, added)
}

func TestPathsAddedSinceNoBaselineReturnsAll(t *testing.T) {
	engine := newAuditTestEngine("GET /a", "POST /b")
	added := pathsAddedSince(ginRouteKeys(engine), nil)
	assert.ElementsMatch(t, []string{"GET a", "POST b"}, added)
}

func TestAddedKeysToPathsDedupesAcrossMethods(t *testing.T) {
	paths := addedKeysToPaths([]string{
		"GET api/v1/device/list",
		"POST api/v1/device/list",
		"GET api/v1/alarm/rules",
	})
	assert.ElementsMatch(t, []string{"api/v1/device/list", "api/v1/alarm/rules"}, paths)
}
