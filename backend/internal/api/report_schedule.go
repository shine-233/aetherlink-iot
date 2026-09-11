package api

import (
	"net/http"
	"strconv"
	"strings"

	"aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type ReportScheduleApi struct{}

func (*ReportScheduleApi) Create(c *gin.Context) {
	var req model.CreateReportScheduleRequest
	if !BindAndValidate(c, &req) {
		return
	}
	result, err := service.GroupApp.ReportSchedule.CreateReportSchedule(c.Request.Context(), &req, reportClaims(c))
	setReportResult(c, result, err)
}

func (*ReportScheduleApi) Update(c *gin.Context) {
	var req model.UpdateReportScheduleRequest
	if !BindAndValidate(c, &req) {
		return
	}
	result, err := service.GroupApp.ReportSchedule.UpdateReportSchedule(c.Request.Context(), c.Param("id"), &req, reportClaims(c))
	setReportResult(c, result, err)
}

func (*ReportScheduleApi) Delete(c *gin.Context) {
	revision, err := strconv.ParseInt(c.Query("revision"), 10, 64)
	if err != nil || revision < 1 {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "revision query parameter is required"))
		return
	}
	if err := service.GroupApp.ReportSchedule.DeleteReportSchedule(c.Request.Context(), c.Param("id"), revision, reportClaims(c)); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"id": c.Param("id")})
}

func (*ReportScheduleApi) List(c *gin.Context) {
	var req model.ReportScheduleListRequest
	if !BindAndValidate(c, &req) {
		return
	}
	result, err := service.GroupApp.ReportSchedule.ListReportSchedules(c.Request.Context(), req, reportClaims(c))
	setReportResult(c, result, err)
}

func (*ReportScheduleApi) Get(c *gin.Context) {
	result, err := service.GroupApp.ReportSchedule.GetReportSchedule(c.Request.Context(), c.Param("id"), reportClaims(c))
	setReportResult(c, result, err)
}

func (*ReportScheduleApi) RunNow(c *gin.Context) {
	key, ok := requireReportIdempotencyKey(c)
	if !ok {
		return
	}
	result, err := service.GroupApp.ReportSchedule.SubmitManualRun(c.Request.Context(), c.Param("id"), key, reportClaims(c))
	setAcceptedReportResult(c, result, err)
}

func (*ReportScheduleApi) ListRuns(c *gin.Context) {
	var req model.ReportRunListRequest
	if !BindAndValidate(c, &req) {
		return
	}
	result, err := service.GroupApp.ReportSchedule.ListRuns(c.Request.Context(), c.Param("id"), req, reportClaims(c))
	setReportResult(c, result, err)
}

func (*ReportScheduleApi) GetRun(c *gin.Context) {
	result, err := service.GroupApp.ReportSchedule.GetRun(c.Request.Context(), c.Param("id"), c.Param("run_id"), reportClaims(c))
	setReportResult(c, result, err)
}

func (*ReportScheduleApi) RetryRun(c *gin.Context) {
	key, ok := requireReportIdempotencyKey(c)
	if !ok {
		return
	}
	result, err := service.GroupApp.ReportSchedule.SubmitRetry(c.Request.Context(), c.Param("id"), c.Param("run_id"), key, reportClaims(c))
	setAcceptedReportResult(c, result, err)
}

func reportClaims(c *gin.Context) *utils.UserClaims {
	value, exists := c.Get("claims")
	if !exists {
		return nil
	}
	claims, _ := value.(*utils.UserClaims)
	return claims
}

func requireReportIdempotencyKey(c *gin.Context) (string, bool) {
	key := c.GetHeader("Idempotency-Key")
	if len(key) < 1 || len(key) > 128 || strings.TrimSpace(key) != key {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "Idempotency-Key must contain 1..128 bytes without surrounding whitespace"))
		return "", false
	}
	return key, true
}

func setReportResult(c *gin.Context, result interface{}, err error) {
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", result)
}

func setAcceptedReportResult(c *gin.Context, result *model.ReportRunActionResponse, err error) {
	if err != nil {
		c.Error(err)
		return
	}
	c.Header("Location", result.StatusURL)
	response.SetSuccessStatus(c, http.StatusAccepted)
	c.Set("data", result)
}
