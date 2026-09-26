// 文件用途：移动应用中心 API 层单测（ROADMAP TB-23）——租户边界解析与上传入口 fail-closed 守卫。
// 核心逻辑：不依赖数据库，直接以 gin 测试上下文驱动 appBundleTenant 与 UploadAppBundle：
//
//	SYS_ADMIN 可指定目标租户、TENANT_ADMIN 锁死本租户（跨租户请求拒绝）、缺凭证/缺租户
//	上下文拒绝；上传入口对超大请求体（ContentLength 夹紧）与缺文件表单的拒绝路径。
//
// 关键注意事项：appBundleTenant 与 mobileTenant（api/mobile.go）同口径，若调整任一侧
//
//	必须同步另一侧的用例；这里断的是业务码而非文案，避免绑定 messages.yaml 措辞。
//
// 重构建议：uniapp 对接阶段若引入设备侧凭证（非 UserClaims），本文件需补对应身份拒绝用例。
package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// appBundleContext 构造带（或缺失）claims 的测试上下文。
func appBundleContext(t *testing.T, method, target string, claims *utils.UserClaims) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, nil)
	if claims != nil {
		ctx.Set("claims", claims)
	}
	return ctx, recorder
}

// lastErrorCode 取最后一条 gin error 的业务码（无错误时返回 0）。
func lastErrorCode(t *testing.T, ctx *gin.Context) int {
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

// TestAppBundleTenantParsing 租户边界：SYS_ADMIN 可指定、TENANT_ADMIN 锁死、缺凭证拒绝。
func TestAppBundleTenantParsing(t *testing.T) {
	cases := []struct {
		name      string
		claims    *utils.UserClaims
		requested string
		want      string
		wantCode  int
	}{
		{
			name:      "TENANT_ADMIN 锁死本租户",
			claims:    &utils.UserClaims{ID: "u1", TenantID: "tenant-a", Authority: "TENANT_ADMIN"},
			requested: "",
			want:      "tenant-a",
		},
		{
			name:      "TENANT_ADMIN 传本租户放行",
			claims:    &utils.UserClaims{ID: "u1", TenantID: "tenant-a", Authority: "TENANT_ADMIN"},
			requested: "tenant-a",
			want:      "tenant-a",
		},
		{
			name:      "TENANT_ADMIN 传他租户拒绝",
			claims:    &utils.UserClaims{ID: "u1", TenantID: "tenant-a", Authority: "TENANT_ADMIN"},
			requested: "tenant-b",
			wantCode:  errcode.CodeNoPermission,
		},
		{
			name:      "SYS_ADMIN 可指定目标租户",
			claims:    &utils.UserClaims{ID: "u0", TenantID: "sys", Authority: "SYS_ADMIN"},
			requested: "tenant-b",
			want:      "tenant-b",
		},
		{
			name:      "SYS_ADMIN 未指定回落自身租户",
			claims:    &utils.UserClaims{ID: "u0", TenantID: "sys", Authority: "SYS_ADMIN"},
			requested: "  ",
			want:      "sys",
		},
		{
			name:      "TENANT_ADMIN 缺租户上下文拒绝",
			claims:    &utils.UserClaims{ID: "u2", Authority: "TENANT_ADMIN"},
			requested: "",
			wantCode:  errcode.CodeNoPermission,
		},
		{
			name:      "缺凭证拒绝",
			claims:    nil,
			requested: "",
			wantCode:  errcode.CodeNoPermission,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, _ := appBundleContext(t, http.MethodGet, "/api/v1/mobile/app_bundles", testCase.claims)
			got, err := appBundleTenant(ctx, testCase.requested)
			if testCase.wantCode != 0 {
				if err == nil {
					t.Fatalf("expected error code %d, got tenant %q", testCase.wantCode, got)
				}
				if code := lastErrorCode(t, ctx); code != 0 && code != testCase.wantCode {
					t.Fatalf("error code = %d, want %d", code, testCase.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != testCase.want {
				t.Fatalf("tenant = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestUploadAppBundleRejectsOversize 上传入口的 ContentLength 夹紧：超限直接拒（不解 multipart）。
func TestUploadAppBundleRejectsOversize(t *testing.T) {
	// 将 package 级 maxAppBundleUploadRequestSize 抬高暴露为问题：这里构造一个
	// 超过上限的 ContentLength（上限 + 1 字节），服务必须以 CodeFileTooLarge 拒绝。
	ctx, _ := appBundleContext(t, http.MethodPost, "/api/v1/mobile/app_bundles/upload",
		&utils.UserClaims{ID: "u1", TenantID: "tenant-a", Authority: "TENANT_ADMIN"})
	ctx.Request.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	ctx.Request.ContentLength = appBundleUploadRequestSize + 1

	(&MobileAppBundleApi{}).UploadAppBundle(ctx)

	if code := lastErrorCode(t, ctx); code != errcode.CodeFileTooLarge {
		t.Fatalf("oversize upload error code = %d, want %d", code, errcode.CodeFileTooLarge)
	}
}

// TestUploadAppBundleRejectsMissingFile 缺文件表单拒绝（CodeFileEmpty）。
func TestUploadAppBundleRejectsMissingFile(t *testing.T) {
	ctx, _ := appBundleContext(t, http.MethodPost, "/api/v1/mobile/app_bundles/upload",
		&utils.UserClaims{ID: "u1", TenantID: "tenant-a", Authority: "TENANT_ADMIN"})
	ctx.Request.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	ctx.Request.ContentLength = -1

	(&MobileAppBundleApi{}).UploadAppBundle(ctx)

	if code := lastErrorCode(t, ctx); code != errcode.CodeFileEmpty {
		t.Fatalf("missing file error code = %d, want %d", code, errcode.CodeFileEmpty)
	}
}
