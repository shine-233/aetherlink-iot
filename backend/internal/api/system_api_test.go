// 文件用途：覆盖系统接口测试相关 API 行为的 Go 测试。
// 核心逻辑：构造 Gin 路由或测试上下文，验证接口契约、参数处理和关键响应。
// 关键注意事项：测试应保持轻量确定性，避免依赖真实外部服务或共享状态。
// 重构建议：新增场景时优先沉淀表驱动用例和可复用的路由/请求构造器。
package api

import (
	"net/http"
	"testing"
	"time"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
)

// These source-structure guards only prove that declarations remain present.
// They do not prove routing, authentication, status codes, response bodies, or
// system behavior; executable HTTP evidence lives in the test below.

func TestSystemPermissionAndOpenAPISourceStructureContractDeclaresP0P1Boundaries(t *testing.T) {
	requireAPIMethods(t, "sys_user.go", "UserApi",
		"HandleUserDetail",
		"HandleUserListByPage",
		"UpdateUser",
		"RefreshToken",
		"Logout",
	)
	requireAPIMethods(t, "role.go", "RoleApi",
		"CreateRole",
		"DeleteRole",
		"UpdateRole",
		"HandleRoleListByPage",
	)
	requireAPIMethods(t, "casbin.go", "CasbinApi",
		"AddFunctionToRole",
		"HandleFunctionFromRole",
		"UpdateFunctionFromRole",
		"DeleteFunctionFromRole",
		"AddRoleToUser",
		"HandleRolesFromUser",
		"UpdateRolesFromUser",
		"DeleteRolesFromUser",
	)
	requireAPIMethods(t, "open_api_keys.go", "OpenAPIKeyApi",
		"CreateOpenAPIKey",
		"GetOpenAPIKeyList",
		"UpdateOpenAPIKey",
		"DeleteOpenAPIKey",
	)
}

func TestSystemAndServiceSourceStructureContractDeclaresDeploymentReadinessSurfaces(t *testing.T) {
	requireAPIMethods(t, "system.go", "SystemApi",
		"HealthCheck",
		"HandleSystime",
		"HandleSysVersion",
	)
	requireAPIMethods(t, "deployment_health.go", "SystemApi",
		"Readiness",
		"DeploymentHealth",
	)
	requireAPIMethods(t, "sys_function.go", "SysFunctionApi",
		"HandleSysFcuntion",
		"UpdateSysFcuntion",
	)
	requireAPIMethods(t, "service_plugin.go", "ServicePluginApi",
		"Create",
		"HandleList",
		"Handle",
		"Update",
		"Delete",
	)
}

func TestSystemPublicHandlersReturnUnifiedHTTPResponses(t *testing.T) {
	t.Run("health check returns a successful empty payload", func(t *testing.T) {
		status, got := performAPIValidationRequest(
			t,
			http.MethodGet,
			"/health",
			"",
			(&SystemApi{}).HealthCheck,
		)

		requireSystemHTTPSuccess(t, status, got.Code)
		if got.Data != nil {
			t.Fatalf("response data = %#v, want omitted", got.Data)
		}
	})

	t.Run("system time is serialized as the current second", func(t *testing.T) {
		before := time.Now().Unix()
		status, got := performAPIValidationRequest(
			t,
			http.MethodGet,
			"/api/v1/systime",
			"",
			(&SystemApi{}).HandleSystime,
		)
		after := time.Now().Unix()

		requireSystemHTTPSuccess(t, status, got.Code)
		payload, ok := got.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("response data = %#v, want JSON object", got.Data)
		}
		if len(payload) != 1 {
			t.Fatalf("response data keys = %#v, want only systime", payload)
		}
		timestamp, ok := payload["systime"].(float64)
		if !ok {
			t.Fatalf("systime = %#v, want JSON number", payload["systime"])
		}
		if gotSecond := int64(timestamp); gotSecond < before || gotSecond > after {
			t.Fatalf("systime = %d, want between %d and %d", gotSecond, before, after)
		}
	})

	t.Run("system version is serialized from the runtime version", func(t *testing.T) {
		oldVersion := global.SYSTEM_VERSION
		global.SYSTEM_VERSION = "v-http-contract-test"
		t.Cleanup(func() {
			global.SYSTEM_VERSION = oldVersion
		})

		status, got := performAPIValidationRequest(
			t,
			http.MethodGet,
			"/api/v1/sys_version",
			"",
			(&SystemApi{}).HandleSysVersion,
		)

		requireSystemHTTPSuccess(t, status, got.Code)
		payload, ok := got.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("response data = %#v, want JSON object", got.Data)
		}
		if len(payload) != 1 {
			t.Fatalf("response data keys = %#v, want only version", payload)
		}
		if gotVersion := payload["version"]; gotVersion != "v-http-contract-test" {
			t.Fatalf("version = %#v, want v-http-contract-test", gotVersion)
		}
	})
}

func requireSystemHTTPSuccess(t *testing.T, status int, code int) {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", status, http.StatusOK)
	}
	if code != errcode.CodeSuccess {
		t.Fatalf("response code = %d, want %d", code, errcode.CodeSuccess)
	}
}
