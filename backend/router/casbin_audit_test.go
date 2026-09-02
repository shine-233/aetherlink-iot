// 文件用途：验证 Casbin 路由审计 Gin 集成辅助的快照差集与路径口径。
// 核心逻辑：基线差集必须精确圈定"挂载 CasbinRBAC 之后新增的路由"，这是 fail-fast 不误报的前提。
// 关键注意事项：路径统一去除前导 "/"，与 middleware.CasbinRBAC 的运行时口径一致。
package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newAuditTestEngine(paths ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	for _, path := range paths {
		engine.GET(path, func(c *gin.Context) {})
	}
	return engine
}

func TestGinRoutePathsTrimsLeadingSlash(t *testing.T) {
	engine := newAuditTestEngine("/health", "/api/v1/login")
	paths := ginRoutePaths(engine)
	assert.ElementsMatch(t, []string{"health", "api/v1/login"}, paths)
}

func TestPathsAddedSinceOnlyReportsNewRoutes(t *testing.T) {
	baselineEngine := newAuditTestEngine("/health", "/api/v1/login", "/api/v1/devices/:device_id/diagnostics")

	fullEngine := newAuditTestEngine(
		"/health",
		"/api/v1/login",
		"/api/v1/devices/:device_id/diagnostics",
		"/api/v1/device/list",
		"/api/v1/alarm/rules",
	)
	// 同一路径的第二个 HTTP 方法会在 Routes() 中产生重复 Path 条目，差集必须去重。
	fullEngine.POST("/api/v1/device/list", func(c *gin.Context) {})

	added := pathsAddedSince(ginRoutePaths(fullEngine), ginRoutePaths(baselineEngine))
	assert.ElementsMatch(t, []string{"api/v1/device/list", "api/v1/alarm/rules"}, added)
}

func TestPathsAddedSinceNoBaselineReturnsAll(t *testing.T) {
	engine := newAuditTestEngine("/a", "/b")
	added := pathsAddedSince(ginRoutePaths(engine), nil)
	assert.ElementsMatch(t, []string{"a", "b"}, added)
}
