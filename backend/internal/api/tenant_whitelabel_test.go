// 文件用途：白标（TB-47）API 层单测——claims 边界与 fail-closed 路径。
// 核心逻辑：不依赖数据库，以 gin 测试上下文驱动 6 个处理器：缺 claims 拒绝、
//
//	TENANT_USER 写操作拒绝（读 overrides 放行）、非法语言/键拒绝（业务码 202007）、
//	'</style' 序列拒绝（202008）；TENANT_ADMIN 合法入参在 DB 未注入时应到达
//	dal 层报 DB 错误（101001），以此证明边界检查已放行到数据访问层。
//
// 关键注意事项：断的是业务码而非文案，避免绑定 messages.yaml 措辞；
//
//	overrides 对 TENANT_USER 的"放行到 DB 层"同样用 101001 表达（登录后可读语义）。
//
// 重构建议：若路由组改造（如 overrides 改 OptionalJWT），同步本文件的边界用例。
package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// whitelabelContext 构造带（或缺失）claims 的测试上下文。
func whitelabelContext(t *testing.T, method, target, body string, claims *utils.UserClaims) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	ctx.Request = httptest.NewRequest(method, target, reader)
	if body != "" {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	if claims != nil {
		ctx.Set("claims", claims)
	}
	return ctx, recorder
}

// whitelabelLastErrorCode 取最后一条 gin error 的业务码（无错误时返回 0）。
func whitelabelLastErrorCode(t *testing.T, ctx *gin.Context) int {
	t.Helper()
	last := ctx.Errors.Last()
	if last == nil {
		return 0
	}
	var bizErr *errcode.Error
	if !errors.As(last.Err, &bizErr) {
		t.Fatalf("expected errcode.Error, got %T: %v", last.Err, last.Err)
	}
	return bizErr.Code
}

var (
	whitelabelSysAdmin    = &utils.UserClaims{ID: "u-sys", Authority: "SYS_ADMIN", TenantID: ""}
	whitelabelTenantAdmin = &utils.UserClaims{ID: "u-tenant", Authority: "TENANT_ADMIN", TenantID: "tenant-a"}
	whitelabelTenantUser  = &utils.UserClaims{ID: "u-user", Authority: "TENANT_USER", TenantID: "tenant-a"}
)

// TestWhitelHandlersFailClosedWithoutClaims 全部入口缺 claims 一律 201001。
func TestWhitelHandlersFailClosedWithoutClaims(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		target  string
		body    string
		handler func(*gin.Context)
	}{
		{"upsert translations", http.MethodPut, "/api/v1/whitelabel/translations", `{"items":[{"lang":"zh-cn","key":"page.customer.title","value":"客户"}]}`, func(c *gin.Context) { Controllers.TenantWhitelabelApi.UpsertTenantTranslations(c) }},
		{"list translations", http.MethodGet, "/api/v1/whitelabel/translations", "", func(c *gin.Context) { Controllers.TenantWhitelabelApi.ListTenantTranslations(c) }},
		{"delete translations", http.MethodDelete, "/api/v1/whitelabel/translations", `{"items":[{"lang":"zh-cn","key":"page.customer.title"}]}`, func(c *gin.Context) { Controllers.TenantWhitelabelApi.DeleteTenantTranslations(c) }},
		{"get custom css", http.MethodGet, "/api/v1/whitelabel/custom-css", "", func(c *gin.Context) { Controllers.TenantWhitelabelApi.GetTenantCustomCSS(c) }},
		{"upsert custom css", http.MethodPut, "/api/v1/whitelabel/custom-css", `{}`, func(c *gin.Context) { Controllers.TenantWhitelabelApi.UpsertTenantCustomCSS(c) }},
		{"get overrides", http.MethodGet, "/api/v1/whitelabel/overrides", "", func(c *gin.Context) { Controllers.TenantWhitelabelApi.GetWhitelabelOverrides(c) }},
	}
	for _, tc := range cases {
		ctx, _ := whitelabelContext(t, tc.method, tc.target, tc.body, nil)
		tc.handler(ctx)
		if code := whitelabelLastErrorCode(t, ctx); code != errcode.CodeNoPermission {
			t.Fatalf("%s: code = %d, want %d (fail-closed without claims)", tc.name, code, errcode.CodeNoPermission)
		}
	}
}

// TestWhitelabelWriteDeniedForTenantUser TENANT_USER 写操作拒绝（管理面 fail-closed）。
func TestWhitelabelWriteDeniedForTenantUser(t *testing.T) {
	ctx, _ := whitelabelContext(t, http.MethodPut, "/api/v1/whitelabel/translations",
		`{"items":[{"lang":"zh-cn","key":"page.customer.title","value":"客户"}]}`, whitelabelTenantUser)
	Controllers.TenantWhitelabelApi.UpsertTenantTranslations(ctx)
	if code := whitelabelLastErrorCode(t, ctx); code != errcode.CodeNoPermission {
		t.Fatalf("TENANT_USER upsert code = %d, want %d", code, errcode.CodeNoPermission)
	}

	ctx, _ = whitelabelContext(t, http.MethodPut, "/api/v1/whitelabel/custom-css", `{"css":".app{}"}`, whitelabelTenantUser)
	Controllers.TenantWhitelabelApi.UpsertTenantCustomCSS(ctx)
	if code := whitelabelLastErrorCode(t, ctx); code != errcode.CodeNoPermission {
		t.Fatalf("TENANT_USER upsert css code = %d, want %d", code, errcode.CodeNoPermission)
	}
}

// TestWhitelabelUpsertTranslationsValidation 覆盖条目校验：非法语言与非法键拒绝（202007）。
func TestWhitelabelUpsertTranslationsValidation(t *testing.T) {
	ctx, _ := whitelabelContext(t, http.MethodPut, "/api/v1/whitelabel/translations",
		`{"items":[{"lang":"de-de","key":"page.customer.title","value":"Kunde"}]}`, whitelabelTenantAdmin)
	Controllers.TenantWhitelabelApi.UpsertTenantTranslations(ctx)
	if code := whitelabelLastErrorCode(t, ctx); code != errcode.CodeTenantTranslationInvalid {
		t.Fatalf("unsupported lang code = %d, want %d", code, errcode.CodeTenantTranslationInvalid)
	}

	ctx, _ = whitelabelContext(t, http.MethodPut, "/api/v1/whitelabel/translations",
		`{"items":[{"lang":"zh-cn","key":"bad key with spaces","value":"客户"}]}`, whitelabelTenantAdmin)
	Controllers.TenantWhitelabelApi.UpsertTenantTranslations(ctx)
	if code := whitelabelLastErrorCode(t, ctx); code != errcode.CodeTenantTranslationInvalid {
		t.Fatalf("invalid key code = %d, want %d", code, errcode.CodeTenantTranslationInvalid)
	}
}

// TestWhitelabelCustomCSSValidation 自定义 CSS 校验：'</style' 序列拒绝（202008，纵深防御）。
func TestWhitelabelCustomCSSValidation(t *testing.T) {
	ctx, _ := whitelabelContext(t, http.MethodPut, "/api/v1/whitelabel/custom-css",
		`{"css":".a{}</style><script>alert(1)</script>"}`, whitelabelTenantAdmin)
	Controllers.TenantWhitelabelApi.UpsertTenantCustomCSS(ctx)
	if code := whitelabelLastErrorCode(t, ctx); code != errcode.CodeTenantCustomCSSInvalid {
		t.Fatalf("'</style' sequence code = %d, want %d", code, errcode.CodeTenantCustomCSSInvalid)
	}
}

// TestWhitelabelAdminRequestsReachDAL 合法入参的租户作用域请求应越过边界检查，
// 在显式置空的 DB 句柄下以 DB 错误（101001）形式暴露——证明授权矩阵放行无误。
func TestWhitelabelAdminRequestsReachDAL(t *testing.T) {
	// 显式置空 global.DB（同包其他测试可能注入，恢复保证测试互不污染）。
	oldDB := global.DB
	global.DB = nil
	t.Cleanup(func() { global.DB = oldDB })

	ctx, _ := whitelabelContext(t, http.MethodPut, "/api/v1/whitelabel/translations",
		`{"items":[{"lang":"zh-cn","key":"page.customer.title","value":"客户"}]}`, whitelabelTenantAdmin)
	Controllers.TenantWhitelabelApi.UpsertTenantTranslations(ctx)
	if code := whitelabelLastErrorCode(t, ctx); code != errcode.CodeDBError {
		t.Fatalf("TENANT_ADMIN upsert should reach DAL, code = %d, want %d", code, errcode.CodeDBError)
	}

	// overrides 对 TENANT_USER 放行（登录后可读），同样到达 DAL 报 DB 错误。
	ctx, _ = whitelabelContext(t, http.MethodGet, "/api/v1/whitelabel/overrides", "", whitelabelTenantUser)
	Controllers.TenantWhitelabelApi.GetWhitelabelOverrides(ctx)
	if code := whitelabelLastErrorCode(t, ctx); code != errcode.CodeDBError {
		t.Fatalf("TENANT_USER overrides should reach DAL, code = %d, want %d", code, errcode.CodeDBError)
	}
}
