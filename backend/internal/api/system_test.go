// 文件用途：覆盖系统管理测试相关 API 行为的 Go 测试。
// 核心逻辑：构造 Gin 路由或测试上下文，验证接口契约、参数处理和关键响应。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的路由/请求构造器。
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/global"

	"github.com/gin-gonic/gin"
)

func newSystemAPIContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return ctx
}

func TestSystemAPIHealthCheckSetsNilDataForDeploymentProbe(t *testing.T) {
	ctx := newSystemAPIContext()

	(&SystemApi{}).HealthCheck(ctx)

	value, exists := ctx.Get("data")
	if !exists {
		t.Fatal("HealthCheck should set data for response middleware")
	}
	if value != nil {
		t.Fatalf("HealthCheck data = %#v, want nil", value)
	}
}

func TestDeploymentHealthStatusCodeSeparatesReadyAndNotReady(t *testing.T) {
	tests := []struct {
		status string
		want   int
	}{
		{status: "ok", want: http.StatusOK},
		{status: "down", want: http.StatusServiceUnavailable},
		{status: "", want: http.StatusServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			got := deploymentHealthStatusCode(service.DeploymentHealthReport{Status: test.status})
			if got != test.want {
				t.Fatalf("deploymentHealthStatusCode(%q) = %d, want %d", test.status, got, test.want)
			}
		})
	}
}

func TestSystemAPIHandleSystimeReturnsCurrentSecondTimestamp(t *testing.T) {
	ctx := newSystemAPIContext()
	before := time.Now().Unix()

	(&SystemApi{}).HandleSystime(ctx)

	after := time.Now().Unix()
	value, exists := ctx.Get("data")
	if !exists {
		t.Fatal("HandleSystime should set response data")
	}
	payload, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("HandleSystime data = %#v, want map", value)
	}
	if len(payload) != 1 {
		t.Fatalf("HandleSystime payload keys = %#v, want only systime", payload)
	}
	got, ok := payload["systime"].(int64)
	if !ok {
		t.Fatalf("systime = %#v, want int64", payload["systime"])
	}
	if got < before || got > after {
		t.Fatalf("systime = %d, want between %d and %d", got, before, after)
	}
}

func TestSystemAPIHandleSysVersionReturnsCurrentGlobalVersion(t *testing.T) {
	oldVersion := global.SYSTEM_VERSION
	global.SYSTEM_VERSION = "v-test-2026"
	t.Cleanup(func() {
		global.SYSTEM_VERSION = oldVersion
	})

	ctx := newSystemAPIContext()
	(&SystemApi{}).HandleSysVersion(ctx)

	value, exists := ctx.Get("data")
	if !exists {
		t.Fatal("HandleSysVersion should set response data")
	}
	payload, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("HandleSysVersion data = %#v, want map", value)
	}
	if len(payload) != 1 {
		t.Fatalf("HandleSysVersion payload keys = %#v, want only version", payload)
	}
	if payload["version"] != "v-test-2026" {
		t.Fatalf("version = %#v, want v-test-2026", payload["version"])
	}
}
