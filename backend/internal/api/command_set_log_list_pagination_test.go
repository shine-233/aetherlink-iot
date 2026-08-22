// 文件用途：覆盖 Command Job 公开列表 API 分页契约的 Go 测试。
// 核心逻辑：构造 Gin 路由与内存 sqlite，验证无参默认、翻页、越界收敛与 total/page 回显。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的路由/请求构造器。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apiresponse "aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newCommandJobListAPITestRouter(t *testing.T) (*gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.CommandJob{}, &model.CommandJobDetail{}); err != nil {
		t.Fatalf("migrate command job tables: %v", err)
	}
	global.DB = db

	responseHandler := &apiresponse.Handler{ErrManager: errcode.NewErrorManager("", "")}
	router := gin.New()
	router.Use(responseHandler.Middleware())
	// 测试桩：跳过 JWT 中间件，直接注入租户 claims。
	router.Use(func(c *gin.Context) {
		c.Set("claims", &utils.UserClaims{ID: "operator-1", TenantID: "tenant-a"})
		c.Next()
	})
	router.GET("/api/v1/command/datas/jobs", (&CommandSetLogApi{}).ListFleetCommandJobs)
	return router, func() { global.DB = oldDB }
}

func seedCommandJobListAPIRows(t *testing.T, db *gorm.DB, count int) {
	t.Helper()
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	rows := make([]*model.CommandJob, 0, count)
	for i := 0; i < count; i++ {
		createdAt := base.Add(time.Duration(count-i) * time.Minute)
		rows = append(rows, &model.CommandJob{
			ID:             fmt.Sprintf("job-%03d", i),
			TenantID:       "tenant-a",
			OperatorID:     "operator-1",
			JobType:        "command",
			ScopeType:      "all",
			Identify:       fmt.Sprintf("identify-%03d", i),
			Status:         "completed",
			TimeoutSeconds: 60,
			RequestedCount: 1,
			CreatedAt:      createdAt,
			UpdatedAt:      createdAt,
		})
	}
	if err := db.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatalf("seed command jobs: %v", err)
	}
}

func performCommandJobListRequest(t *testing.T, router *gin.Engine, target string) (int, apiresponse.Response) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	router.ServeHTTP(recorder, request)
	var payload apiresponse.Response
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response body %q: %v", recorder.Body.String(), err)
	}
	return recorder.Code, payload
}

func TestCommandJobListAPIPaginationDefaultsAndClamp(t *testing.T) {
	router, cleanup := newCommandJobListAPITestRouter(t)
	defer cleanup()
	const seededJobs = 12
	seedCommandJobListAPIRows(t, global.DB, seededJobs)

	// 无参默认：缺省回退第 1 页固定页大小，响应回显 total/page/page_size。
	status, got := performCommandJobListRequest(t, router, "/api/v1/command/datas/jobs")
	if status != http.StatusOK || got.Code != errcode.CodeSuccess {
		t.Fatalf("default request status=%d code=%d body=%s", status, got.Code, got.Message)
	}
	raw, err := json.Marshal(got.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	var first model.FleetCommandJobListResult
	if err := json.Unmarshal(raw, &first); err != nil {
		t.Fatalf("unmarshal list result: %v", err)
	}
	if first.Total != seededJobs {
		t.Fatalf("default total = %d, want %d", first.Total, seededJobs)
	}
	if first.Page != 1 || first.PageSize != 10 {
		t.Fatalf("default paging echo page=%d page_size=%d, want 1/10", first.Page, first.PageSize)
	}
	if len(first.List) != 10 {
		t.Fatalf("default list length = %d, want 10", len(first.List))
	}

	// 翻页：第 2 页返回剩余行并回显请求页参数。
	status, got = performCommandJobListRequest(t, router, "/api/v1/command/datas/jobs?page=2&page_size=8")
	if status != http.StatusOK || got.Code != errcode.CodeSuccess {
		t.Fatalf("second page request status=%d code=%d body=%s", status, got.Code, got.Message)
	}
	raw, err = json.Marshal(got.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	var second model.FleetCommandJobListResult
	if err := json.Unmarshal(raw, &second); err != nil {
		t.Fatalf("unmarshal list result: %v", err)
	}
	if second.Page != 2 || second.PageSize != 8 {
		t.Fatalf("second page paging echo page=%d page_size=%d, want 2/8", second.Page, second.PageSize)
	}
	if second.Total != seededJobs || len(second.List) != seededJobs-8 {
		t.Fatalf("second page total=%d len=%d, want total %d len %d", second.Total, len(second.List), seededJobs, seededJobs-8)
	}

	// 越界收敛：超大 page_size 被截断到上限后回显。
	status, got = performCommandJobListRequest(t, router, "/api/v1/command/datas/jobs?page=1&page_size=5000")
	if status != http.StatusOK || got.Code != errcode.CodeSuccess {
		t.Fatalf("oversized request status=%d code=%d body=%s", status, got.Code, got.Message)
	}
	raw, err = json.Marshal(got.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	var clamped model.FleetCommandJobListResult
	if err := json.Unmarshal(raw, &clamped); err != nil {
		t.Fatalf("unmarshal list result: %v", err)
	}
	if clamped.PageSize != 50 {
		t.Fatalf("oversized page_size echo = %d, want clamped to 50", clamped.PageSize)
	}
}

func TestCommandJobListAPIRejectsNonIntegerPagingParams(t *testing.T) {
	router, cleanup := newCommandJobListAPITestRouter(t)
	defer cleanup()

	for _, target := range []string{
		"/api/v1/command/datas/jobs?page=abc",
		"/api/v1/command/datas/jobs?page_size=abc",
	} {
		status, got := performCommandJobListRequest(t, router, target)
		if status != http.StatusOK || got.Code != errcode.CodeParamError {
			t.Fatalf("%s status=%d code=%d, want param error", target, status, got.Code)
		}
	}
}
