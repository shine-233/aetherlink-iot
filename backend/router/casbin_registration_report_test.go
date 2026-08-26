// 文件用途：验证 casbin 登记一致性报告的白名单剔除与 gap 收集逻辑。
// 核心逻辑：以最小路由集 + 假判定函数锁定「公开豁免 / 已登记 / 未登记」三类分流。
// 关键注意事项：本测试不构造真实 enforcer；与中间件相同语义的集成由
//   middleware/casbin_middle_test.go 与启动日志人工核对承担。

package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func newReportTestRoutes() []gin.RouteInfo {
	engine := gin.New()
	register := func(method, path string) {
		engine.Handle(method, path, func(c *gin.Context) { c.Status(200) })
	}
	// 公开/运维面
	register("GET", "/health")
	register("GET", "/metrics")
	register("GET", "/files/*filepath")
	register("GET", "/api/v1/login")
	register("POST", "/api/v1/plugin/heartbeat")
	register("GET", "/api/v1/devices/:device_id/diagnostics")
	// 受保护：已登记 + 未登记
	register("GET", "/api/v1/devices")
	register("POST", "/api/v1/not-in-casbin")
	return engine.Routes()
}

func TestCollectCasbinRegistrationGaps(t *testing.T) {
	registered := map[string]bool{
		"api/v1/devices": true,
	}
	gaps := collectCasbinRegistrationGaps(newReportTestRoutes(), func(path string) bool {
		return registered[path]
	})

	want := []string{"POST /api/v1/not-in-casbin"}
	if len(gaps) != len(want) {
		t.Fatalf("gaps = %v, want %v", gaps, want)
	}
	for i, gap := range gaps {
		if gap != want[i] {
			t.Fatalf("gaps[%d] = %q, want %q", i, gap, want[i])
		}
	}
}

func TestCasbinRouteExemptCoversPublicSurface(t *testing.T) {
	exempt := []string{
		"health",
		"metrics-viewer/echarts.min.js",
		"files/reports/a.pdf",
		"api/v1/plugin/device/config",
		"api/v1/tenant/super-admin/init",
		"api/v1/board/shared/:token",
		"api/v1/devices/:device_id/diagnostics",
	}
	for _, path := range exempt {
		if !casbinRouteExempt(path) {
			t.Fatalf("path %q should be exempt from casbin report", path)
		}
	}
	protected := []string{"api/v1/user/list", "api/v1/device/shadow/:deviceId"}
	for _, path := range protected {
		if casbinRouteExempt(path) {
			t.Fatalf("protected path %q must not be exempt", path)
		}
	}
}
