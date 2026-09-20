package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	apiresponse "aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func TestSetAcceptedReportResultUsesExact202LocationAndEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responseHandler, err := apiresponse.NewHandler("../../configs/messages.yaml", "../../configs/messages_str.yaml")
	if err != nil {
		t.Fatalf("load response messages: %v", err)
	}
	engine := gin.New()
	engine.Use(responseHandler.Middleware())
	engine.POST("/api/v1/report/schedules/schedule-1/run", func(context *gin.Context) {
		setAcceptedReportResult(context, &model.ReportRunActionResponse{
			RunID: "run-1", ScheduleID: "schedule-1", Status: "queued",
			StatusURL: "/api/v1/report/schedules/schedule-1/runs/run-1",
		}, nil)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/report/schedules/schedule-1/run", nil)
	request.Header.Set("Accept-Language", "en-US")
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("HTTP status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if got := recorder.Header().Get("Location"); got != "/api/v1/report/schedules/schedule-1/runs/run-1" {
		t.Fatalf("Location = %q", got)
	}
	want := `{"code":200,"message":"Success","data":{"run_id":"run-1","schedule_id":"schedule-1","status":"queued","status_url":"/api/v1/report/schedules/schedule-1/runs/run-1","idempotent_replay":false,"duplicate_delivery_risk":false}}`
	if recorder.Body.String() != want {
		t.Fatalf("body = %s, want %s", recorder.Body.String(), want)
	}
}

func TestRequireReportIdempotencyKey(t *testing.T) {
	for _, test := range []struct {
		name string
		key  string
		ok   bool
	}{{"valid", "request-1", true}, {"missing", "", false}, {"surrounding whitespace", " request-1", false}, {"too long", string(make([]byte, 129)), false}} {
		t.Run(test.name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			context.Request.Header.Set("Idempotency-Key", test.key)
			_, ok := requireReportIdempotencyKey(context)
			if ok != test.ok {
				t.Fatalf("ok = %t, want %t", ok, test.ok)
			}
		})
	}
}
