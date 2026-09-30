// 文件用途：证明泛型 Handler 适配器与迁移前手写 handler 的 HTTP 响应逐字节一致。
// 核心逻辑：同一个请求结构体分别用「旧写法」（BindAndValidate + MustGet("claims") + c.Error + c.Set）
// 和「新写法」（Handle / HandleAction / HandleNoBody / HandlePath / HandlePathBody / HandlePathBodyOptional /
// HandlePublic 适配器与 bindBodyLegacyParamError 助手）实现，走同一套 response 中间件渲染，然后对 HTTP 状态码与原始响应体字符串做全等比较。
// 这是渲染结果的真实比对，不是声明式契约，因此能直接证伪「适配器改变了响应」这一假设。
// 已知的有意差异：缺失 claims 时旧写法 MustGet 触发 panic，新写法返回 CodeUnauthorized，
// 该差异由 TestHandleAdapterMissingClaimsReturnsUnauthorizedInsteadOfPanic 单独固化。
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

// adapterProbeReq 是适配器一致性比对使用的探针请求体，覆盖必填与数值区间两类校验。
type adapterProbeReq struct {
	Name string `json:"name" form:"name" binding:"required"`
	Age  int    `json:"age" form:"age" binding:"gte=0,lte=120"`
}

const (
	// adapterProbeBoomName 触发 service 返回业务错误，用于比对错误分支。
	adapterProbeBoomName = "boom"
	// adapterProbeBodyMissing 缺失必填字段，用于比对参数校验分支。
	adapterProbeBodyMissing = `{"age":18}`
	// adapterProbeBodyRange 触发区间校验失败，用于比对第二条校验错误分支。
	adapterProbeBodyRange = `{"name":"ok","age":200}`
	// adapterProbeBodyValid 正常入参。
	adapterProbeBodyValid = `{"name":"ok","age":18}`
	// adapterProbeBodyBoom 触发业务错误分支。
	adapterProbeBodyBoom = `{"name":"boom","age":18}`
)

func adapterProbeData(req adapterProbeReq, claims *utils.UserClaims) (interface{}, error) {
	if req.Name == adapterProbeBoomName {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return map[string]interface{}{"name": req.Name, "age": req.Age, "tenant": claims.TenantID}, nil
}

func adapterProbeAction(req adapterProbeReq, claims *utils.UserClaims) error {
	if req.Name == adapterProbeBoomName {
		return errcode.New(errcode.CodeSystemError)
	}
	return nil
}

func adapterProbeNoBody(claims *utils.UserClaims) (interface{}, error) {
	return map[string]interface{}{"tenant": claims.TenantID}, nil
}

func adapterProbePath(id string, claims *utils.UserClaims) (interface{}, error) {
	return map[string]interface{}{"id": id, "tenant": claims.TenantID}, nil
}

func adapterProbePathBody(id string, req adapterProbeReq, claims *utils.UserClaims) (interface{}, error) {
	if req.Name == adapterProbeBoomName {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return map[string]interface{}{"id": id, "name": req.Name, "tenant": claims.TenantID}, nil
}

// adapterProbePathBodyOptionalBody 同 adapterProbePathBody，但入参零值有意义（body 可选场景下 fn 收到零值结构体）。
func adapterProbePathBodyOptionalBody(id string, req adapterProbeReq, claims *utils.UserClaims) (interface{}, error) {
	if req.Name == adapterProbeBoomName {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return map[string]interface{}{"id": id, "name": req.Name, "age": req.Age, "tenant": claims.TenantID}, nil
}

func adapterProbePublic(req adapterProbeReq) (interface{}, error) {
	if req.Name == adapterProbeBoomName {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return map[string]interface{}{"name": req.Name}, nil
}

// ---- 旧写法（迁移前形态）----

func legacyProbeData(c *gin.Context) {
	var req adapterProbeReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := adapterProbeData(req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func legacyProbeAction(c *gin.Context) {
	var req adapterProbeReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := adapterProbeAction(req, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func legacyProbeNoBody(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := adapterProbeNoBody(claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func legacyProbePath(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := adapterProbePath(c.Param("id"), claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func legacyProbePathBody(c *gin.Context) {
	id := c.Param("id")
	var req adapterProbeReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := adapterProbePathBody(id, req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// legacyProbePathBodyOptional 是迁移前“请求体可选”手写形态（edge_node.go Heartbeat/IssueCertificate 原样）：
// 绑定失败不拒绝，req 保持零值。
func legacyProbePathBodyOptional(c *gin.Context) {
	id := c.Param("id")
	var req adapterProbeReq
	_ = c.ShouldBindJSON(&req)
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := adapterProbePathBodyOptionalBody(id, req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// legacyProbeWithDataParamError 是迁移前“参数错误走 errcode.WithData 包络”的手写形态
// （role.go Assign 系列原样）：先取 claims，再绑定，失败时 c.Error(errcode.WithData(...))。
func legacyProbeWithDataParamError(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	var req adapterProbeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{"error": err.Error()}))
		return
	}
	data, err := adapterProbeData(req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func legacyProbePublic(c *gin.Context) {
	var req adapterProbeReq
	if !BindAndValidate(c, &req) {
		return
	}
	data, err := adapterProbePublic(req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ---- 新写法（适配器形态）----

func adaptedProbeData(c *gin.Context) {
	Handle(c, func(req *adapterProbeReq, claims *utils.UserClaims) (interface{}, error) {
		return adapterProbeData(*req, claims)
	})
}

func adaptedProbeAction(c *gin.Context) {
	HandleAction(c, func(req *adapterProbeReq, claims *utils.UserClaims) error {
		return adapterProbeAction(*req, claims)
	})
}

func adaptedProbeNoBody(c *gin.Context) {
	HandleNoBody(c, func(claims *utils.UserClaims) (interface{}, error) {
		return adapterProbeNoBody(claims)
	})
}

func adaptedProbePath(c *gin.Context) {
	HandlePath(c, "id", func(value string, claims *utils.UserClaims) (interface{}, error) {
		return adapterProbePath(value, claims)
	})
}

func adaptedProbePathBody(c *gin.Context) {
	HandlePathBody(c, "id", func(value string, req *adapterProbeReq, claims *utils.UserClaims) (interface{}, error) {
		return adapterProbePathBody(value, *req, claims)
	})
}

// adaptedProbePathBodyOptional 走共享助手 HandlePathBodyOptional：绑定失败放行，仅缺 claims 短路。
func adaptedProbePathBodyOptional(c *gin.Context) {
	HandlePathBodyOptional(c, "id", func(value string, req *adapterProbeReq, claims *utils.UserClaims) (interface{}, error) {
		return adapterProbePathBodyOptionalBody(value, *req, claims)
	})
}

// adaptedProbeWithDataParamError 用共享助手 bindBodyLegacyParamError 收敛迁移前的
// errcode.WithData(CodeParamError, {"error": ...}) 参数错误出口。
func adaptedProbeWithDataParamError(c *gin.Context) {
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	var req adapterProbeReq
	if err := bindBodyLegacyParamError(c, &req); err != nil {
		c.Error(err)
		return
	}
	data, err := adapterProbeData(req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func adaptedProbePublic(c *gin.Context) {
	HandlePublic(c, func(req *adapterProbeReq) (interface{}, error) {
		return adapterProbePublic(*req)
	})
}

// renderProbe 用与线上一致的响应中间件渲染一次请求，返回状态码与原始响应体。
func renderProbe(t *testing.T, method, target, body string, withClaims bool, handler gin.HandlerFunc) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	responseHandler := &apiresponse.Handler{
		ErrManager: errcode.NewErrorManager("", ""),
	}
	router := gin.New()
	router.Use(responseHandler.Middleware())
	if withClaims {
		router.Use(func(c *gin.Context) {
			c.Set(claimsContextKey, &utils.UserClaims{ID: "probe-user", TenantID: "probe-tenant"})
			c.Next()
		})
	}
	router.Handle(method, target, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", "en-US")
	router.ServeHTTP(recorder, request)

	return recorder.Code, recorder.Body.String()
}

// requireIdenticalResponses 断言新旧两种写法在同一入参下渲染出完全相同的响应。
func requireIdenticalResponses(t *testing.T, name, method, target, body string, withClaims bool, legacy, adapted gin.HandlerFunc) {
	t.Helper()
	legacyCode, legacyBody := renderProbe(t, method, target, body, withClaims, legacy)
	adaptedCode, adaptedBody := renderProbe(t, method, target, body, withClaims, adapted)

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

// TestHandleAdapterDataPathIsByteIdentical 覆盖带请求体 + 返回数据的接口形态。
func TestHandleAdapterDataPathIsByteIdentical(t *testing.T) {
	for _, body := range []string{adapterProbeBodyValid, adapterProbeBodyMissing, adapterProbeBodyRange, adapterProbeBodyBoom} {
		requireIdenticalResponses(t, "Handle/"+body, http.MethodPost, "/probe", body, true, legacyProbeData, adaptedProbeData)
	}
}

// TestHandleAdapterActionPathIsByteIdentical 覆盖无返回数据的写操作形态（成功时 data 省略）。
func TestHandleAdapterActionPathIsByteIdentical(t *testing.T) {
	for _, body := range []string{adapterProbeBodyValid, adapterProbeBodyMissing, adapterProbeBodyBoom} {
		requireIdenticalResponses(t, "HandleAction/"+body, http.MethodPost, "/probe/action", body, true, legacyProbeAction, adaptedProbeAction)
	}
}

// TestHandleAdapterQueryPathIsByteIdentical 覆盖 GET query 绑定形态。
func TestHandleAdapterQueryPathIsByteIdentical(t *testing.T) {
	for _, target := range []string{"/probe/query?name=ok&age=18", "/probe/query?age=18", "/probe/query?name=ok&age=200"} {
		requireIdenticalResponses(t, "HandleQuery"+target, http.MethodGet, target, "", true, legacyProbeData, adaptedProbeData)
	}
}

// TestHandleAdapterNoBodyPathIsByteIdentical 覆盖只依赖路径参数与 claims 的只读形态。
func TestHandleAdapterNoBodyPathIsByteIdentical(t *testing.T) {
	requireIdenticalResponses(t, "HandleNoBody", http.MethodGet, "/probe/nobody", "", true, legacyProbeNoBody, adaptedProbeNoBody)
}

// TestHandleAdapterPathVariantsAreByteIdentical 覆盖路径参数与路径参数+请求体两种组合形态。
func TestHandleAdapterPathVariantsAreByteIdentical(t *testing.T) {
	requireIdenticalResponses(t, "HandlePath", http.MethodGet, "/probe/device-1", "", true, legacyProbePath, adaptedProbePath)

	for _, body := range []string{adapterProbeBodyValid, adapterProbeBodyMissing, adapterProbeBodyBoom} {
		requireIdenticalResponses(t, "HandlePathBody/"+body, http.MethodPut, "/probe/device-1", body, true, legacyProbePathBody, adaptedProbePathBody)
	}
}

// TestHandleAdapterPathBodyOptionalToleratesMissingBody 覆盖请求体可选形态（集成阶段收编
// edge_node.go Heartbeat/IssueCertificate 所用助手 HandlePathBodyOptional）：
// 空 body、坏 JSON 均放行且 req 保持零值，仅缺 claims 时短路（后者由 MissingClaims 用例覆盖）。
func TestHandleAdapterPathBodyOptionalToleratesMissingBody(t *testing.T) {
	for _, body := range []string{adapterProbeBodyValid, "", `{not-json`, adapterProbeBodyBoom} {
		requireIdenticalResponses(t, "HandlePathBodyOptional/"+body, http.MethodPost, "/probe/device-1", body, true, legacyProbePathBodyOptional, adaptedProbePathBodyOptional)
	}
}

// TestHandleAdapterLegacyParamErrorEnvelopeIsByteIdentical 覆盖迁移前 WithData 参数错误包络的收敛
// （集成阶段收编 role.go Assign 系列所用助手 bindBodyLegacyParamError）：绑定失败时响应必须带
// data 字段（{"error": ...}）且 message 取错误码默认文案——与 reportParamError 的 NewWithMessage
// 形态逐字节不同，两侧在同一入参下渲染结果必须全等。
func TestHandleAdapterLegacyParamErrorEnvelopeIsByteIdentical(t *testing.T) {
	for _, body := range []string{adapterProbeBodyMissing, adapterProbeBodyValid, adapterProbeBodyBoom} {
		requireIdenticalResponses(t, "bindBodyLegacyParamError/"+body, http.MethodPost, "/probe/assign", body, true, legacyProbeWithDataParamError, adaptedProbeWithDataParamError)
	}
}

// TestHandleAdapterPublicPathIsByteIdentical 覆盖匿名接口（不取 claims）形态。
func TestHandleAdapterPublicPathIsByteIdentical(t *testing.T) {
	for _, body := range []string{adapterProbeBodyValid, adapterProbeBodyMissing, adapterProbeBodyBoom} {
		requireIdenticalResponses(t, "HandlePublic/"+body, http.MethodPost, "/probe/public", body, false, legacyProbePublic, adaptedProbePublic)
	}
}

// TestHandleAdapterMissingClaimsReturnsUnauthorizedInsteadOfPanic 固化有意的行为差异：
// 固化有意的行为差异：路由漏配鉴权中间件时，旧写法 MustGet panic 会被统一响应
// 中间件 recover 成 CodeSystemError；新写法不 panic，直接返回标准 CodeUnauthorized。
func TestHandleAdapterMissingClaimsReturnsUnauthorizedInsteadOfPanic(t *testing.T) {
	legacyCode, legacyBody := renderProbe(t, http.MethodPost, "/probe", adapterProbeBodyValid, false, legacyProbeData)
	if legacyCode != http.StatusOK {
		t.Fatalf("legacy HTTP status = %d, want %d (panic recovered by response middleware)", legacyCode, http.StatusOK)
	}
	var legacyPayload apiresponse.Response
	if err := json.Unmarshal([]byte(legacyBody), &legacyPayload); err != nil {
		t.Fatalf("decode legacy response body %q: %v", legacyBody, err)
	}
	if legacyPayload.Code != errcode.CodeSystemError {
		t.Fatalf("legacy response code = %d, want %d (recovered MustGet panic)", legacyPayload.Code, errcode.CodeSystemError)
	}

	code, body := renderProbe(t, http.MethodPost, "/probe", adapterProbeBodyValid, false, adaptedProbeData)
	if code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", code, http.StatusOK)
	}
	var payload apiresponse.Response
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode response body %q: %v", body, err)
	}
	if payload.Code != errcode.CodeUnauthorized {
		t.Fatalf("response code = %d, want %d (body=%s)", payload.Code, errcode.CodeUnauthorized, body)
	}
}

// didPanic 执行 fn 并返回它是否发生 panic，用于固化旧写法的崩溃行为。
func didPanic(t *testing.T, fn func()) (panicked bool) {
	t.Helper()
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	fn()
	return false
}
