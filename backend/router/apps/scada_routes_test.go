package apps

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// TestScadaAndMobileRoutesAreRegistered 在真实 Gin 引擎上挂载路由并核对注册结果。
//
// 为什么需要这份证据：router 包的契约测试只做**静态 AST 解析**，能证明
// `router_init.go` 里那行 Init 还在，却证明不了 Init 内部真的把路径挂上了。
// 门禁"项目 CRUD 不再返回 unsupported"要求端点实际可达，故这里跑一次真实注册。
func TestScadaAndMobileRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")

	(&Scada{}).Init(v1)
	(&Mobile{}).Init(v1)

	got := map[string]bool{}
	for _, r := range engine.Routes() {
		got[r.Method+" "+r.Path] = true
	}

	want := []string{
		// P1.3 项目 CRUD：此前没有"项目"这一层，这些端点根本不存在。
		"POST /api/v1/scada/projects",
		"GET /api/v1/scada/projects",
		"GET /api/v1/scada/projects/:id",
		"DELETE /api/v1/scada/projects/:id",
		// P1.3 画布文档
		"POST /api/v1/scada/projects/:id/documents",
		"GET /api/v1/scada/projects/:id/documents",
		"GET /api/v1/scada/documents/:id",
		"PUT /api/v1/scada/documents/:id",
		"POST /api/v1/scada/documents/:id/publish",
		"POST /api/v1/scada/documents/:id/rollback",
		"POST /api/v1/scada/documents/:id/archive",
		"GET /api/v1/scada/documents/:id/versions",
		"GET /api/v1/scada/documents/:id/audits",
		// P1.3 实时控制
		"POST /api/v1/scada/control/confirm",
		"POST /api/v1/scada/control",
		// P1.4 移动端
		"GET /api/v1/mobile/capabilities",
		"POST /api/v1/mobile/push/subscribe",
		"DELETE /api/v1/mobile/push/:id",
		"POST /api/v1/mobile/commands",
		// P1.4 设备 / 告警 / 影子（可见范围一律由 claims 推导，接口不收 tenant_id）
		"GET /api/v1/mobile/devices",
		"GET /api/v1/mobile/alarms",
		"POST /api/v1/mobile/alarms/:id/ack",
		"GET /api/v1/mobile/devices/:id/shadow",
		"PUT /api/v1/mobile/devices/:id/shadow",
		"GET /api/v1/mobile/devices/:id/ota",
		"GET /api/v1/mobile/dashboards",
	}

	for _, w := range want {
		if !got[w] {
			t.Fatalf("route %q is not registered", w)
		}
	}
}
