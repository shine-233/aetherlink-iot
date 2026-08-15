package model

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetBoardListByPageReqBindsVisTypeQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("GET", "/board?page=1&page_size=20&vis_type=native&tenant_id=tenant-a", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	var req GetBoardListByPageReq
	if err := context.ShouldBindQuery(&req); err != nil {
		t.Fatalf("ShouldBindQuery() error = %v", err)
	}
	if req.VisType == nil || *req.VisType != "native" {
		t.Fatalf("VisType = %#v, want native", req.VisType)
	}
	if req.TenantID == nil || *req.TenantID != "tenant-a" {
		t.Fatalf("TenantID = %#v, want tenant-a", req.TenantID)
	}
}
