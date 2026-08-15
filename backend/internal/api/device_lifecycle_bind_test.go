// 文件用途: 探明 gin 对空 query 参数 ?lifecycle_status= 的真实绑定行为(nil vs &"").
// 核心逻辑: 用真实 gin engine + ShouldBindQuery 绑真实 DTO,断言空 query 值绑成什么,是否触发 oneof 拒绝。
// 关键注意事项: 这是 HTTP 绑定层真相,ValidateStruct 直接构造指针会绕过 gin,不能替代本测试。
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"aetherlink-iot/backend/internal/model"
)

func TestDeviceListLifecycleEmptyQueryBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	probe := func(rawQuery string) (*string, bool) {
		var bound *string
		var validationFailed bool
		r := gin.New()
		r.GET("/x", func(c *gin.Context) {
			var req model.GetDeviceListByPageReq
			_ = c.ShouldBindQuery(&req)
			bound = req.LifecycleStatus
			if err := ValidateStruct(&req); err != nil {
				validationFailed = true
			}
		})
		w := httptest.NewRecorder()
		rq := httptest.NewRequest(http.MethodGet, "/x?page=1&page_size=10"+rawQuery, nil)
		r.ServeHTTP(w, rq)
		return bound, validationFailed
	}

	t.Run("absent param binds to nil (safe default path)", func(t *testing.T) {
		bound, failed := probe("")
		if bound != nil {
			t.Fatalf("absent lifecycle_status should bind nil, got %q", *bound)
		}
		if failed {
			t.Fatalf("absent lifecycle_status should pass validation")
		}
	})

	t.Run("empty value ?lifecycle_status= reveals real binding", func(t *testing.T) {
		bound, failed := probe("&lifecycle_status=")
		// 实测契约: gin 把 ?lifecycle_status= 绑成 &""(非 nil 指针指向空串),不是 nil。
		// omitempty 只豁免 nil 指针,因此空串进入 oneof 校验并被拒(400)。
		// 这正是前端默认值必须用 'activated' 而非 '' 的根因——发空 query 会被 400。
		// 与 device_lifecycle_validation_test.go 中对 "" 的直接 ValidateStruct 断言保持同一口径。
		if bound == nil {
			t.Fatalf("gin should bind ?lifecycle_status= to &\"\", got nil")
		}
		if *bound != "" {
			t.Fatalf("gin should bind ?lifecycle_status= to empty string, got %q", *bound)
		}
		if !failed {
			t.Fatalf("empty lifecycle_status should be rejected by oneof (omitempty does not exempt empty string)")
		}
	})

	t.Run("valid value passes end to end", func(t *testing.T) {
		bound, failed := probe("&lifecycle_status=activated")
		if bound == nil || *bound != "activated" {
			t.Fatalf("expected activated bound")
		}
		if failed {
			t.Fatalf("activated should pass validation")
		}
	})
}
