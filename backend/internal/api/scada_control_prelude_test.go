package api

import (
	"errors"
	"net/http/httptest"
	"testing"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

func preludeTestContext(claims *utils.UserClaims) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil)
	if claims != nil {
		c.Set(claimsContextKey, claims)
	}
	return c
}

func lastErrCode(t *testing.T, c *gin.Context) int {
	t.Helper()
	if len(c.Errors) == 0 {
		t.Fatalf("expected c.Error to be recorded")
	}
	var e *errcode.Error
	if !errors.As(c.Errors.Last().Err, &e) {
		t.Fatalf("unexpected error type %T", c.Errors.Last().Err)
	}
	return e.Code
}

// TestScadaControlPreludeFailsClosed 缺 claims → 未授权；控制服务未接线 → 拒绝操作（不签发、不执行）。
func TestScadaControlPreludeFailsClosed(t *testing.T) {
	c := preludeTestContext(nil)
	if _, _, _, ok := scadaControlPrelude(c, ""); ok {
		t.Fatal("prelude without claims must fail")
	}
	if code := lastErrCode(t, c); code != errcode.CodeUnauthorized {
		t.Fatalf("missing claims code=%d want %d", code, errcode.CodeUnauthorized)
	}

	if service.GroupApp.ScadaControl != nil {
		t.Skip("scada control wired in this process; unwired branch not reachable")
	}
	c = preludeTestContext(&utils.UserClaims{TenantID: "t1", Authority: "TENANT_ADMIN"})
	if _, _, _, ok := scadaControlPrelude(c, "t1"); ok {
		t.Fatal("prelude with unwired control service must fail closed")
	}
	if code := lastErrCode(t, c); code != errcode.CodeOpDenied {
		t.Fatalf("unwired code=%d want %d", code, errcode.CodeOpDenied)
	}
}
