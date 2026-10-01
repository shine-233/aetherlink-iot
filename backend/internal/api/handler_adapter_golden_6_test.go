// 文件用途：service_access.go 迁移到 handler_adapter 骨架后的 golden 对比测试（编号 6，仅归属轨道2迁移员-6）。
// 核心逻辑：把迁移前 service_access.go 的手写样板（BindAndValidate + MustGet("claims") + c.Error + c.Set）
// 与迁移后形态（Handle / HandleAction / HandlePathAction / HandlePublic / RequireClaims+respond 编排）
// 用同一批探针 service 函数、同一套响应中间件各渲染一次，对 HTTP 状态码与原始响应体做全等比较。
// 覆盖 service_access.go 的全部 handler 形态：
// 1. POST JSON 体 + claims + 返回数据（Create）；
// 2. GET query 绑定 + claims + 返回数据（HandleList / HandleDeviceList）；
// 3. PUT JSON 体 + claims + 无返回数据（Update）；
// 4. DELETE 路径参数 + claims + 无返回数据（Delete）；
// 5. 匿名 query 绑定（HandleVoucherForm，迁移前已是 HandlePublic 形态）；
// 6. 插件侧“绑定 -> OpenAPI 门禁 -> claims -> respond”编排（HandlePluginServiceAccessList / HandlePluginServiceAccess）。
// 已知的有意差异（缺失 claims 时旧写法 panic 兜底成 CodeSystemError、新写法返回 CodeUnauthorized）
// 由共享的 handler_adapter_test.go 固化，本文件所有比对均在注入 claims 的前提下进行。
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiresponse "aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// golden6ProbeReq 是 golden6 比对使用的探针请求体，标签形态镜像真实请求结构：
// Name 镜像 CreateAccessReq.Name / GetPluginServiceAccessListReq.ServiceIdentifier（json+form+required），
// Age 镜像可选数值字段，用于同时覆盖 JSON 绑定与 GET query 绑定两条路径。
type golden6ProbeReq struct {
	Name string `json:"name" form:"name" binding:"required"`
	Age  int    `json:"age" form:"age"`
}

const (
	// golden6TenantProbe JWT 中间件注入的探针租户，用于区分 claims 来源。
	golden6TenantProbe = "golden6-probe-tenant"
	// golden6TenantOpenAPI OpenAPI 门禁替身写入的等效租户，用于固化“门禁先于 claims”的顺序。
	golden6TenantOpenAPI = "golden6-openapi-tenant"
	// golden6NameBoom 触发探针 service 返回业务错误，用于比对错误分支。
	golden6NameBoom = "boom"

	golden6BodyValid   = `{"name":"ok","age":18}`
	golden6BodyMissing = `{"age":18}`
	golden6BodyBoom    = `{"name":"boom","age":18}`
)

// ---- 探针 service 函数（签名对齐真实 service.GroupApp.ServiceAccess.* 方法）----

func golden6ProbeData(req *golden6ProbeReq, claims *utils.UserClaims) (interface{}, error) {
	if req.Name == golden6NameBoom {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return map[string]interface{}{"name": req.Name, "age": req.Age, "tenant": claims.TenantID}, nil
}

func golden6ProbeAction(req *golden6ProbeReq, claims *utils.UserClaims) error {
	if req.Name == golden6NameBoom {
		return errcode.New(errcode.CodeSystemError)
	}
	_ = claims
	return nil
}

func golden6ProbeDelete(id string, claims *utils.UserClaims) error {
	if id == golden6NameBoom {
		return errcode.New(errcode.CodeSystemError)
	}
	_ = claims
	return nil
}

// golden6OpenAPIKeyGate 以 middleware.OpenAPIKeyAuth 的对外契约做轻量替身：
// 失败时写错误并返回 false；成功时写入等效 claims 并返回 true。
// 真实现依赖 dal.VerifyOpenAPIKey（数据库）与失败限流，golden 比对只关心它
// 与“绑定 / claims / respond”之间的先后顺序与出口行为，两侧共用同一替身。
func golden6OpenAPIKeyGate(c *gin.Context) bool {
	if c.GetHeader("x-api-key") == "" {
		c.Error(errcode.New(errcode.CodeUnauthorized))
		c.Abort()
		return false
	}
	c.Set(claimsContextKey, &utils.UserClaims{ID: "golden6-openapi-user", TenantID: golden6TenantOpenAPI})
	return true
}

// ---- 旧写法（迁移前 service_access.go 的手写形态）----

func golden6LegacyData(c *gin.Context) {
	var req golden6ProbeReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	resp, err := golden6ProbeData(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

func golden6LegacyAction(c *gin.Context) {
	var req golden6ProbeReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	err := golden6ProbeAction(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func golden6LegacyDelete(c *gin.Context) {
	id := c.Param("id")
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	err := golden6ProbeDelete(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func golden6LegacyPublic(c *gin.Context) {
	var req golden6ProbeReq
	if !BindAndValidate(c, &req) {
		return
	}
	resp, err := golden6ProbeData(&req, &utils.UserClaims{})
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

func golden6LegacyPlugin(c *gin.Context) {
	var req golden6ProbeReq
	if !BindAndValidate(c, &req) {
		return
	}
	if !golden6OpenAPIKeyGate(c) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	resp, err := golden6ProbeData(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// ---- 新写法（迁移后 service_access.go 的形态）----

func golden6AdaptedData(c *gin.Context) {
	Handle(c, golden6ProbeData)
}

func golden6AdaptedAction(c *gin.Context) {
	HandleAction(c, golden6ProbeAction)
}

func golden6AdaptedDelete(c *gin.Context) {
	HandlePathAction(c, "id", golden6ProbeDelete)
}

func golden6AdaptedPublic(c *gin.Context) {
	HandlePublic(c, func(req *golden6ProbeReq) (interface{}, error) {
		return golden6ProbeData(req, &utils.UserClaims{})
	})
}

func golden6AdaptedPlugin(c *gin.Context) {
	var req golden6ProbeReq
	if !BindAndValidate(c, &req) {
		return
	}
	if !golden6OpenAPIKeyGate(c) {
		return
	}
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	resp, err := golden6ProbeData(&req, claims)
	respond(c, resp, err)
}

// ---- 渲染与比对基建 ----

// golden6RenderProbe 用与线上一致的响应中间件渲染一次请求，返回状态码与原始响应体。
// route 与 target 分开传以支持 :id 路径参数；headers 支持自定义请求头（OpenAPI 门禁替身需要 x-api-key）。
func golden6RenderProbe(t *testing.T, method, route, target, body string, withClaims bool, headers map[string]string, handler gin.HandlerFunc) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	responseHandler := &apiresponse.Handler{
		ErrManager: errcode.NewErrorManager("", ""),
	}
	router := gin.New()
	router.Use(responseHandler.Middleware())
	if withClaims {
		router.Use(func(c *gin.Context) {
			c.Set(claimsContextKey, &utils.UserClaims{ID: "golden6-probe-user", TenantID: golden6TenantProbe})
			c.Next()
		})
	}
	router.Handle(method, route, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", "en-US")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	router.ServeHTTP(recorder, request)

	return recorder.Code, recorder.Body.String()
}

// golden6RequireIdentical 断言新旧两种写法在同一入参下渲染出完全相同的响应。
func golden6RequireIdentical(t *testing.T, name, method, route, target, body string, withClaims bool, headers map[string]string, legacy, adapted gin.HandlerFunc) {
	t.Helper()
	legacyCode, legacyBody := golden6RenderProbe(t, method, route, target, body, withClaims, headers, legacy)
	adaptedCode, adaptedBody := golden6RenderProbe(t, method, route, target, body, withClaims, headers, adapted)

	if legacyCode != adaptedCode {
		t.Fatalf("%s: HTTP status legacy=%d adapted=%d", name, legacyCode, adaptedCode)
	}
	if legacyBody != adaptedBody {
		t.Fatalf("%s: response body mismatch\nlegacy  = %s\nadapted = %s", name, legacyBody, adaptedBody)
	}
	if legacyBody == "" {
		t.Fatalf("%s: legacy response body is empty, comparison is meaningless", name)
	}
}

// ---- 比对用例 ----

// TestGolden6ServiceAccessCreateBodyIsByteIdentical 覆盖 Create 形态：POST JSON 体 + claims + 返回数据。
func TestGolden6ServiceAccessCreateBodyIsByteIdentical(t *testing.T) {
	for _, body := range []string{golden6BodyValid, golden6BodyMissing, golden6BodyBoom} {
		golden6RequireIdentical(t, "Create/"+body, http.MethodPost, "/golden6/access", "/golden6/access", body, true, nil, golden6LegacyData, golden6AdaptedData)
	}
}

// TestGolden6ServiceAccessListQueryIsByteIdentical 覆盖 HandleList / HandleDeviceList 形态：
// GET query 绑定（BindAndValidate 对 GET 走 ShouldBindQuery）+ claims + 返回数据。
func TestGolden6ServiceAccessListQueryIsByteIdentical(t *testing.T) {
	for _, target := range []string{"/golden6/list?name=ok&age=18", "/golden6/list?age=18", "/golden6/list?name=boom&age=18"} {
		golden6RequireIdentical(t, "HandleList"+target, http.MethodGet, "/golden6/list", target, "", true, nil, golden6LegacyData, golden6AdaptedData)
	}
}

// TestGolden6ServiceAccessUpdateActionIsByteIdentical 覆盖 Update 形态：PUT JSON 体 + claims + 成功时 data 省略。
func TestGolden6ServiceAccessUpdateActionIsByteIdentical(t *testing.T) {
	for _, body := range []string{golden6BodyValid, golden6BodyMissing, golden6BodyBoom} {
		golden6RequireIdentical(t, "Update/"+body, http.MethodPut, "/golden6/access", "/golden6/access", body, true, nil, golden6LegacyAction, golden6AdaptedAction)
	}
}

// TestGolden6ServiceAccessDeletePathActionIsByteIdentical 覆盖 Delete 形态：DELETE 路径参数 + claims + 成功时 data 省略。
func TestGolden6ServiceAccessDeletePathActionIsByteIdentical(t *testing.T) {
	for _, id := range []string{"access-1", golden6NameBoom} {
		golden6RequireIdentical(t, "Delete/"+id, http.MethodDelete, "/golden6/access/:id", "/golden6/access/"+id, "", true, nil, golden6LegacyDelete, golden6AdaptedDelete)
	}
}

// TestGolden6ServiceAccessVoucherFormPublicIsByteIdentical 覆盖 HandleVoucherForm 形态：
// 匿名 query 绑定（迁移前该 handler 已经是 HandlePublic 形态，此处固化其等价性）。
func TestGolden6ServiceAccessVoucherFormPublicIsByteIdentical(t *testing.T) {
	for _, target := range []string{"/golden6/voucher/form?name=ok", "/golden6/voucher/form", "/golden6/voucher/form?name=boom"} {
		golden6RequireIdentical(t, "VoucherForm"+target, http.MethodGet, "/golden6/voucher/form", target, "", false, nil, golden6LegacyPublic, golden6AdaptedPublic)
	}
}

// TestGolden6ServiceAccessPluginGateIsByteIdentical 覆盖插件侧两个 handler 的编排形态：
// 绑定 -> OpenAPI 门禁 -> claims -> respond。门禁替身成功时覆盖 claims，
// 因此响应体中的 tenant 字段能固化“门禁先于 claims”的顺序。
func TestGolden6ServiceAccessPluginGateIsByteIdentical(t *testing.T) {
	withKey := map[string]string{"x-api-key": "golden6-key"}
	cases := []struct {
		name    string
		body    string
		headers map[string]string
	}{
		{"valid with key", golden6BodyValid, withKey},
		{"boom with key", golden6BodyBoom, withKey},
		{"missing required with key", golden6BodyMissing, withKey},
		{"valid without key", golden6BodyValid, nil},
		{"missing required without key", golden6BodyMissing, nil},
	}
	for _, tc := range cases {
		golden6RequireIdentical(t, "Plugin/"+tc.name, http.MethodPost, "/golden6/plugin/access", "/golden6/plugin/access", tc.body, true, tc.headers, golden6LegacyPlugin, golden6AdaptedPlugin)
	}

	// 顺序固化：门禁成功后 claims 必须来自门禁替身（而非外层 JWT 中间件），
	// 若未来有人把 claims 读取挪到门禁之前，这里会拿到 probe 租户而失败。
	code, body := golden6RenderProbe(t, http.MethodPost, "/golden6/plugin/access", "/golden6/plugin/access", golden6BodyValid, true, withKey, golden6AdaptedPlugin)
	if code != http.StatusOK {
		t.Fatalf("plugin adapted HTTP status = %d, want %d (body=%s)", code, http.StatusOK, body)
	}
	var payload apiresponse.Response
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode plugin response body %q: %v", body, err)
	}
	data, ok := payload.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("plugin response data = %#v, want map with tenant field (body=%s)", payload.Data, body)
	}
	if tenant, _ := data["tenant"].(string); tenant != golden6TenantOpenAPI {
		t.Fatalf("plugin claims tenant = %q, want %q (claims must come from OpenAPI gate, body=%s)", tenant, golden6TenantOpenAPI, body)
	}
}
