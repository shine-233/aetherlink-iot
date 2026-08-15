// 文件用途：覆盖入口认证测试相关 API 行为的 Go 测试。
// 核心逻辑：构造 Gin 路由或测试上下文，验证接口契约、参数处理和关键响应。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的路由/请求构造器。
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"

	"github.com/gin-gonic/gin"
)

type bindValidationTestReq struct {
	Name  string `json:"name" form:"name" validate:"required,max=8"`
	Page  int    `json:"page" form:"page" validate:"gte=1,lte=100"`
	Email string `json:"email" form:"email" validate:"omitempty,email"`
}

func newBindValidationContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx.Request = req
	return ctx, recorder
}

func TestValidateStructLangReturnsBusinessFriendlyEnglishMessages(t *testing.T) {
	tests := []struct {
		name string
		req  bindValidationTestReq
		want string
	}{
		{
			name: "required field",
			req:  bindValidationTestReq{Page: 1},
			want: "Field 'Name' is required",
		},
		{
			name: "maximum length",
			req:  bindValidationTestReq{Name: "very-long-name", Page: 1},
			want: "Field 'Name' failed validation (At most 8 characters)",
		},
		{
			name: "minimum page",
			req:  bindValidationTestReq{Name: "sensor", Page: 0},
			want: "The value of field 'Page' must be at least 1",
		},
		{
			name: "email format",
			req:  bindValidationTestReq{Name: "sensor", Page: 1, Email: "not-email"},
			want: "Field 'Email' must be a valid email address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStructLang(tt.req, "en-US")
			if err == nil {
				t.Fatal("ValidateStructLang expected validation error")
			}
			if err.Error() != tt.want {
				t.Fatalf("ValidateStructLang error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestValidateStructDefaultsToChineseMessages(t *testing.T) {
	err := ValidateStruct(bindValidationTestReq{Page: 1})
	if err == nil {
		t.Fatal("ValidateStruct expected validation error")
	}
	const want = `字段 "Name" 为必填项`
	if err.Error() != want {
		t.Fatalf("ValidateStruct error = %q, want %q", err.Error(), want)
	}
}

func TestBindAndValidateBindsGetQueryAndPostJSON(t *testing.T) {
	getCtx, _ := newBindValidationContext(http.MethodGet, "/devices?name=sensor&page=2&email=ops@example.com", "")
	var getReq bindValidationTestReq
	if !BindAndValidate(getCtx, &getReq) {
		t.Fatalf("BindAndValidate GET returned false with errors: %v", getCtx.Errors)
	}
	if getReq.Name != "sensor" || getReq.Page != 2 || getReq.Email != "ops@example.com" {
		t.Fatalf("GET bound request = %+v", getReq)
	}

	postCtx, _ := newBindValidationContext(http.MethodPost, "/devices", `{"name":"gateway","page":3,"email":"ops@example.com"}`)
	var postReq bindValidationTestReq
	if !BindAndValidate(postCtx, &postReq) {
		t.Fatalf("BindAndValidate POST returned false with errors: %v", postCtx.Errors)
	}
	if postReq.Name != "gateway" || postReq.Page != 3 || postReq.Email != "ops@example.com" {
		t.Fatalf("POST bound request = %+v", postReq)
	}
}

func TestBindAndValidateAddsContextErrorsForBadPayloads(t *testing.T) {
	missingRequiredCtx, _ := newBindValidationContext(http.MethodGet, "/devices?page=1", "")
	missingRequiredCtx.Request.Header.Set("Accept-Language", "en-US")
	var missingRequiredReq bindValidationTestReq
	if BindAndValidate(missingRequiredCtx, &missingRequiredReq) {
		t.Fatal("BindAndValidate expected false for missing required query field")
	}
	requireLastBindErrorMessage(t, missingRequiredCtx, "Field 'Name' is required")

	malformedJSONCtx, _ := newBindValidationContext(http.MethodPost, "/devices", `{"name":`)
	var malformedJSONReq bindValidationTestReq
	if BindAndValidate(malformedJSONCtx, &malformedJSONReq) {
		t.Fatal("BindAndValidate expected false for malformed JSON")
	}
	requireLastBindErrorMessage(t, malformedJSONCtx, "unexpected EOF")
}

func requireLastBindErrorMessage(t *testing.T, ctx *gin.Context, want string) {
	t.Helper()

	if len(ctx.Errors) != 1 {
		t.Fatalf("context errors = %v, want one error", ctx.Errors)
	}
	apiErr, ok := ctx.Errors.Last().Err.(*errcode.Error)
	if !ok {
		t.Fatalf("context error type = %T, want *errcode.Error", ctx.Errors.Last().Err)
	}
	if !apiErr.UseCustomMsg || !strings.Contains(apiErr.CustomMsg, want) {
		t.Fatalf("context error custom message = %q, want to contain %q", apiErr.CustomMsg, want)
	}
}
