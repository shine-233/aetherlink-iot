package apps

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestReportScheduleRoutesMatchPublicContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	(&ReportSchedule{}).InitReportSchedule(engine.Group("/api/v1"))
	got := make(map[string]bool)
	for _, route := range engine.Routes() {
		got[route.Method+" "+route.Path] = true
	}
	want := []string{
		http.MethodPost + " /api/v1/report/schedules",
		http.MethodGet + " /api/v1/report/schedules",
		http.MethodGet + " /api/v1/report/schedules/:id",
		http.MethodPut + " /api/v1/report/schedules/:id",
		http.MethodDelete + " /api/v1/report/schedules/:id",
		http.MethodPost + " /api/v1/report/schedules/:id/run",
		http.MethodGet + " /api/v1/report/schedules/:id/runs",
		http.MethodGet + " /api/v1/report/schedules/:id/runs/:run_id",
		http.MethodPost + " /api/v1/report/schedules/:id/runs/:run_id/retry",
	}
	if len(got) != len(want) {
		t.Fatalf("registered routes = %v, want %v", got, want)
	}
	for _, route := range want {
		if !got[route] {
			t.Fatalf("missing route %s; got %v", route, got)
		}
	}
}
