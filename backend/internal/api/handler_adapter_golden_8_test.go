// 文件用途：customer.go 迁移到 handler_adapter 骨架后的 golden 对比测试（迁移员 8 专用，编号 8）。
// 核心逻辑：把迁移前 customer.go 的手写 handler 形态（MustGet("claims") + c.Param 检查 + 可选 BindAndValidate +
// c.Error + c.Set("data", ...)）与迁移后等价形态（RequireClaims / HandlePath / HandleNoBody / Handle + respond）
// 并排实现，走同一套 response 中间件渲染，对 HTTP 状态码与原始响应体字符串做全等比较，证明 JSON 包络逐字节一致。
// service 调用替换为探针函数（真实 CustomerService 依赖 dal/DB，无法在单测中调用）：
// 探针签名与 service.GroupApp.Customer 对应方法一致（含 ctx 入参），仅按入参模拟成功/失败分支；
// 新写法探针侧与 customer.go 迁移后的代码逐行同构，旧写法探针侧为迁移前的逐字形态。
// 已知的有意差异：缺失 claims 时旧写法 MustGet panic 被 response 中间件 recover 成 CodeSystemError，
// 新写法 RequireClaims 返回标准 CodeUnauthorized——由 TestGolden8CustomerMissingClaimsIntentionalDifference 单独固化，
// 与共享测试 TestHandleAdapterMissingClaimsReturnsUnauthorizedInsteadOfPanic 的结论一致。
// 另注：各 handler 内 "客户ID不能为空" 等 :id 空值防御分支在 gin 路由下不可达（:id 不会匹配空串），
// 新旧两份实现逐字保留同一防御代码，但无法通过路由触达，故不参与 golden 比对。
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	apiresponse "aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ---- golden 探针常量 ----

const (
	// golden8BoomName 触发 SaveCustomer 探针返回业务错误。
	golden8BoomName = "golden8-boom-name"
	// golden8BoomID 触发按客户 ID 调用的探针返回业务错误。
	golden8BoomID = "golden8-boom-id"
	// golden8BoomTenant 触发 ListCustomers 探针返回业务错误。
	golden8BoomTenant = "golden8-boom-tenant"
)

// ---- 探针（签名对齐 service.GroupApp.Customer 各方法，含 ctx 入参）----

func golden8ProbeSaveCustomer(_ context.Context, tenantID string, req *model.CustomerReq) (*model.Customer, error) {
	if req.Name == golden8BoomName {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return &model.Customer{ID: "c-new", Name: req.Name, TenantID: tenantID, Email: req.Email}, nil
}

func golden8ProbeGetCustomer(_ context.Context, id, tenantID string) (*model.Customer, error) {
	if id == golden8BoomID {
		return nil, errors.New("customer not found in dal")
	}
	return &model.Customer{ID: id, Name: "golden8-customer", TenantID: tenantID}, nil
}

func golden8ProbeDeleteCustomer(_ context.Context, id, tenantID string) error {
	if id == golden8BoomID {
		return errcode.New(errcode.CodeSystemError)
	}
	return nil
}

func golden8ProbeListCustomers(_ context.Context, tenantID, search string, page, pageSize int) ([]model.Customer, int64, error) {
	if tenantID == golden8BoomTenant {
		return nil, 0, errcode.New(errcode.CodeSystemError)
	}
	return []model.Customer{
		{ID: "c-1", Name: "Customer One", TenantID: tenantID},
		{ID: "c-2", Name: search, TenantID: tenantID},
	}, 42, nil
}

func golden8ProbeAssignDevices(_ context.Context, customerID, tenantID string, deviceIDs []string) error {
	if customerID == golden8BoomID {
		return errcode.New(errcode.CodeSystemError)
	}
	return nil
}

func golden8ProbeUnassignDevice(_ context.Context, customerID, tenantID, deviceID string) error {
	if customerID == golden8BoomID {
		return errcode.New(errcode.CodeSystemError)
	}
	return nil
}

func golden8ProbeListCustomerDevices(_ context.Context, customerID, tenantID string) ([]string, error) {
	if customerID == golden8BoomID {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return []string{"dev-1", "dev-2", tenantID}, nil
}

// ---- 旧写法（迁移前 customer.go 的逐字形态，仅把 service 调用换成探针）----

func golden8LegacySaveCustomer(c *gin.Context) {
	var req model.CustomerReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	res, err := golden8ProbeSaveCustomer(c.Request.Context(), claims.TenantID, &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", res)
}

func golden8LegacyGetCustomer(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	id := c.Param("id")
	if id == "" {
		c.Error(errors.New("客户ID不能为空"))
		return
	}

	tenantID := claims.TenantID
	if claims.Authority == "SYS_ADMIN" {
		tenantID = "" // 超管可跨租户查
	}

	res, err := golden8ProbeGetCustomer(c.Request.Context(), id, tenantID)
	if err != nil {
		c.Error(errors.New("未找到该客户或无权访问"))
		return
	}
	c.Set("data", res)
}

func golden8LegacyDeleteCustomer(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	id := c.Param("id")
	if id == "" {
		c.Error(errors.New("客户ID不能为空"))
		return
	}

	if err := golden8ProbeDeleteCustomer(c.Request.Context(), id, claims.TenantID); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", "ok")
}

func golden8LegacyListCustomers(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	tenantID := claims.TenantID
	if claims.Authority == "SYS_ADMIN" && c.Query("tenant_id") != "" {
		tenantID = c.Query("tenant_id")
	}

	list, total, err := golden8ProbeListCustomers(c.Request.Context(), tenantID, search, page, pageSize)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func golden8LegacyAssignDevices(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	id := c.Param("id")
	if id == "" {
		c.Error(errors.New("客户ID不能为空"))
		return
	}

	var req model.CustomerAssignDevicesReq
	if !BindAndValidate(c, &req) {
		return
	}

	if err := golden8ProbeAssignDevices(c.Request.Context(), id, claims.TenantID, req.DeviceIDs); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", "ok")
}

func golden8LegacyUnassignDevice(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	id := c.Param("id")
	deviceID := c.Param("device_id")
	if id == "" || deviceID == "" {
		c.Error(errors.New("客户ID和设备ID不能为空"))
		return
	}

	if err := golden8ProbeUnassignDevice(c.Request.Context(), id, claims.TenantID, deviceID); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", "ok")
}

func golden8LegacyListCustomerDevices(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	id := c.Param("id")
	if id == "" {
		c.Error(errors.New("客户ID不能为空"))
		return
	}

	deviceIDs, err := golden8ProbeListCustomerDevices(c.Request.Context(), id, claims.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{
		"device_ids": deviceIDs,
	})
}

// ---- 新写法（迁移后 customer.go 的等价形态，仅把 service 调用换成探针）----

func golden8AdaptedSaveCustomer(c *gin.Context) {
	Handle(c, func(req *model.CustomerReq, claims *utils.UserClaims) (interface{}, error) {
		return golden8ProbeSaveCustomer(c.Request.Context(), claims.TenantID, req)
	})
}

func golden8AdaptedGetCustomer(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errors.New("客户ID不能为空")
		}

		tenantID := claims.TenantID
		if claims.Authority == "SYS_ADMIN" {
			tenantID = "" // 超管可跨租户查
		}

		res, err := golden8ProbeGetCustomer(c.Request.Context(), id, tenantID)
		if err != nil {
			return nil, errors.New("未找到该客户或无权访问")
		}
		return res, nil
	})
}

func golden8AdaptedDeleteCustomer(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errors.New("客户ID不能为空")
		}

		if err := golden8ProbeDeleteCustomer(c.Request.Context(), id, claims.TenantID); err != nil {
			return nil, err
		}
		return "ok", nil
	})
}

func golden8AdaptedListCustomers(c *gin.Context) {
	HandleNoBody(c, func(claims *utils.UserClaims) (interface{}, error) {
		search := c.Query("search")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

		tenantID := claims.TenantID
		if claims.Authority == "SYS_ADMIN" && c.Query("tenant_id") != "" {
			tenantID = c.Query("tenant_id")
		}

		list, total, err := golden8ProbeListCustomers(c.Request.Context(), tenantID, search, page, pageSize)
		if err != nil {
			return nil, err
		}

		return gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		}, nil
	})
}

func golden8AdaptedAssignDevices(c *gin.Context) {
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.Error(errors.New("客户ID不能为空"))
		return
	}

	var req model.CustomerAssignDevicesReq
	if !BindAndValidate(c, &req) {
		return
	}

	respond(c, "ok", golden8ProbeAssignDevices(c.Request.Context(), id, claims.TenantID, req.DeviceIDs))
}

func golden8AdaptedUnassignDevice(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		deviceID := c.Param("device_id")
		if id == "" || deviceID == "" {
			return nil, errors.New("客户ID和设备ID不能为空")
		}

		if err := golden8ProbeUnassignDevice(c.Request.Context(), id, claims.TenantID, deviceID); err != nil {
			return nil, err
		}
		return "ok", nil
	})
}

func golden8AdaptedListCustomerDevices(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errors.New("客户ID不能为空")
		}

		deviceIDs, err := golden8ProbeListCustomerDevices(c.Request.Context(), id, claims.TenantID)
		if err != nil {
			return nil, err
		}
		return gin.H{
			"device_ids": deviceIDs,
		}, nil
	})
}

// ---- 渲染与比对工具（自包含副本，避免耦合共享测试文件）----

// golden8Render 用与线上一致的 response 中间件渲染一次请求，返回状态码与原始响应体。
// claims 为 nil 时不注入鉴权中间件，用于固化缺失 claims 的有意差异。
func golden8Render(t *testing.T, method, route, target, body string, claims *utils.UserClaims, handler gin.HandlerFunc) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	responseHandler := &apiresponse.Handler{
		ErrManager: errcode.NewErrorManager("", ""),
	}
	router := gin.New()
	router.Use(responseHandler.Middleware())
	if claims != nil {
		router.Use(func(c *gin.Context) {
			c.Set(claimsContextKey, claims)
			c.Next()
		})
	}
	router.Handle(method, route, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", "en-US")
	router.ServeHTTP(recorder, request)

	return recorder.Code, recorder.Body.String()
}

// golden8RequireIdentical 断言新旧两种写法在同一入参下渲染出完全相同的响应。
func golden8RequireIdentical(t *testing.T, name, method, route, target, body string, claims *utils.UserClaims, legacy, adapted gin.HandlerFunc) {
	t.Helper()
	legacyCode, legacyBody := golden8Render(t, method, route, target, body, claims, legacy)
	adaptedCode, adaptedBody := golden8Render(t, method, route, target, body, claims, adapted)

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

// ---- golden 对比用例 ----

// TestGolden8CustomerSaveMatchesLegacy 覆盖 SaveCustomer（Handle 形态）：绑定失败、校验失败与业务错误分支。
func TestGolden8CustomerSaveMatchesLegacy(t *testing.T) {
	claims := &utils.UserClaims{ID: "golden8-user", TenantID: "golden8-tenant"}
	route := "/api/v1/customer"
	cases := []struct {
		name string
		body string
	}{
		{"valid body", `{"name":"acme","email":"a@b.c"}`},
		{"missing required name", `{"email":"a@b.c"}`},
		{"empty body", ""},
		{"invalid json", `{not-json`},
		{"service error", `{"name":"` + golden8BoomName + `"}`},
	}
	for _, tc := range cases {
		golden8RequireIdentical(t, "SaveCustomer/"+tc.name, http.MethodPost, route, "/api/v1/customer", tc.body, claims, golden8LegacySaveCustomer, golden8AdaptedSaveCustomer)
	}
}

// TestGolden8CustomerGetMatchesLegacy 覆盖 GetCustomer（HandlePath 形态）：
// 普通租户、SYS_ADMIN 跨租户（tenantID 置空）、service 错误改写为"未找到该客户或无权访问"。
func TestGolden8CustomerGetMatchesLegacy(t *testing.T) {
	route := "/api/v1/customer/:id"
	cases := []struct {
		name     string
		claims   *utils.UserClaims
		customer string
	}{
		{"tenant scoped", &utils.UserClaims{ID: "golden8-user", TenantID: "tenant-a"}, "c-1"},
		{"sys admin crosses tenant", &utils.UserClaims{ID: "golden8-user", TenantID: "tenant-a", Authority: "SYS_ADMIN"}, "c-2"},
		{"service error rewritten", &utils.UserClaims{ID: "golden8-user", TenantID: "tenant-a"}, golden8BoomID},
	}
	for _, tc := range cases {
		golden8RequireIdentical(t, "GetCustomer/"+tc.name, http.MethodGet, route, "/api/v1/customer/"+tc.customer, "", tc.claims, golden8LegacyGetCustomer, golden8AdaptedGetCustomer)
	}
}

// TestGolden8CustomerDeleteMatchesLegacy 覆盖 DeleteCustomer（HandlePath 形态）：
// 成功包络必须保留 data:"ok"（respondAction 会省略该字段，故迁移走 respond）与错误分支。
func TestGolden8CustomerDeleteMatchesLegacy(t *testing.T) {
	claims := &utils.UserClaims{ID: "golden8-user", TenantID: "golden8-tenant"}
	route := "/api/v1/customer/:id"
	golden8RequireIdentical(t, "DeleteCustomer/success", http.MethodDelete, route, "/api/v1/customer/c-1", "", claims, golden8LegacyDeleteCustomer, golden8AdaptedDeleteCustomer)
	golden8RequireIdentical(t, "DeleteCustomer/service error", http.MethodDelete, route, "/api/v1/customer/"+golden8BoomID, "", claims, golden8LegacyDeleteCustomer, golden8AdaptedDeleteCustomer)
}

// TestGolden8CustomerListMatchesLegacy 覆盖 ListCustomers（HandleNoBody 形态）：
// page/page_size 宽松解析（缺省/非法/非正数）、search 透传、SYS_ADMIN 的 tenant_id 覆写、普通租户忽略 tenant_id、错误分支。
func TestGolden8CustomerListMatchesLegacy(t *testing.T) {
	tenantClaims := &utils.UserClaims{ID: "golden8-user", TenantID: "tenant-a"}
	adminClaims := &utils.UserClaims{ID: "golden8-user", TenantID: "tenant-a", Authority: "SYS_ADMIN"}
	boomClaims := &utils.UserClaims{ID: "golden8-user", TenantID: golden8BoomTenant}

	type listCase struct {
		name   string
		claims *utils.UserClaims
		target string
	}
	cases := []listCase{
		{"defaults", tenantClaims, "/api/v1/customers"},
		{"explicit page", tenantClaims, "/api/v1/customers?page=3&page_size=25"},
		{"search passthrough", tenantClaims, "/api/v1/customers?search=acme&page=2"},
		{"invalid page falls back to zero", tenantClaims, "/api/v1/customers?page=abc&page_size=-4"},
		{"tenant ignores tenant_id", tenantClaims, "/api/v1/customers?tenant_id=tenant-b"},
		{"admin without tenant_id keeps own", adminClaims, "/api/v1/customers"},
		{"admin overrides tenant_id", adminClaims, "/api/v1/customers?tenant_id=tenant-b"},
	}
	for _, tc := range cases {
		golden8RequireIdentical(t, "ListCustomers/"+tc.name, http.MethodGet, "/api/v1/customers", tc.target, "", tc.claims, golden8LegacyListCustomers, golden8AdaptedListCustomers)
	}
	golden8RequireIdentical(t, "ListCustomers/service error", http.MethodGet, "/api/v1/customers", "/api/v1/customers?page=2", "", boomClaims, golden8LegacyListCustomers, golden8AdaptedListCustomers)
}

// TestGolden8CustomerAssignDevicesMatchesLegacy 覆盖 AssignDevices（RequireClaims + BindAndValidate + respond 手工编排形态，
// 保持迁移前 claims → 路径参数 → 绑定 的顺序）：合法/缺失 device_ids、非法 JSON、错误分支。
func TestGolden8CustomerAssignDevicesMatchesLegacy(t *testing.T) {
	claims := &utils.UserClaims{ID: "golden8-user", TenantID: "golden8-tenant"}
	route := "/api/v1/customer/:id/devices"
	cases := []struct {
		name     string
		customer string
		body     string
	}{
		{"valid body", "c-1", `{"device_ids":["dev-1","dev-2"]}`},
		{"missing required device_ids", "c-1", `{}`},
		{"empty device_ids fails required", "c-1", `{"device_ids":[]}`},
		{"invalid json", "c-1", `{not-json`},
		{"service error", golden8BoomID, `{"device_ids":["dev-1"]}`},
	}
	for _, tc := range cases {
		golden8RequireIdentical(t, "AssignDevices/"+tc.name, http.MethodPost, route, "/api/v1/customer/"+tc.customer+"/devices", tc.body, claims, golden8LegacyAssignDevices, golden8AdaptedAssignDevices)
	}
}

// TestGolden8CustomerUnassignDeviceMatchesLegacy 覆盖 UnassignDevice（HandlePath 形态 + 闭包内读第二个路径参数）：
// 成功包络保留 data:"ok" 与错误分支。
func TestGolden8CustomerUnassignDeviceMatchesLegacy(t *testing.T) {
	claims := &utils.UserClaims{ID: "golden8-user", TenantID: "golden8-tenant"}
	route := "/api/v1/customer/:id/device/:device_id"
	golden8RequireIdentical(t, "UnassignDevice/success", http.MethodDelete, route, "/api/v1/customer/c-1/device/dev-9", "", claims, golden8LegacyUnassignDevice, golden8AdaptedUnassignDevice)
	golden8RequireIdentical(t, "UnassignDevice/service error", http.MethodDelete, route, "/api/v1/customer/"+golden8BoomID+"/device/dev-9", "", claims, golden8LegacyUnassignDevice, golden8AdaptedUnassignDevice)
}

// TestGolden8CustomerListDevicesMatchesLegacy 覆盖 ListCustomerDevices（HandlePath 形态）：成功包络与错误分支。
func TestGolden8CustomerListDevicesMatchesLegacy(t *testing.T) {
	claims := &utils.UserClaims{ID: "golden8-user", TenantID: "golden8-tenant"}
	route := "/api/v1/customer/:id/devices"
	golden8RequireIdentical(t, "ListCustomerDevices/success", http.MethodGet, route, "/api/v1/customer/c-1/devices", "", claims, golden8LegacyListCustomerDevices, golden8AdaptedListCustomerDevices)
	golden8RequireIdentical(t, "ListCustomerDevices/service error", http.MethodGet, route, "/api/v1/customer/"+golden8BoomID+"/devices", "", claims, golden8LegacyListCustomerDevices, golden8AdaptedListCustomerDevices)
}

// TestGolden8CustomerMissingClaimsIntentionalDifference 固化有意的行为差异（本文件迁移用到的四种形态各验一例）：
// 缺失 claims 时旧写法 MustGet panic 被 response 中间件 recover 成 CodeSystemError；
// 新写法 RequireClaims（含 Handle / HandlePath / HandleNoBody 内部与手工编排形态）返回标准 CodeUnauthorized。
func TestGolden8CustomerMissingClaimsIntentionalDifference(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		route   string
		target  string
		body    string
		legacy  gin.HandlerFunc
		adapted gin.HandlerFunc
	}{
		{"SaveCustomer/Handle", http.MethodPost, "/api/v1/customer", "/api/v1/customer", `{"name":"acme"}`, golden8LegacySaveCustomer, golden8AdaptedSaveCustomer},
		{"GetCustomer/HandlePath", http.MethodGet, "/api/v1/customer/:id", "/api/v1/customer/c-1", "", golden8LegacyGetCustomer, golden8AdaptedGetCustomer},
		{"ListCustomers/HandleNoBody", http.MethodGet, "/api/v1/customers", "/api/v1/customers?page=2", "", golden8LegacyListCustomers, golden8AdaptedListCustomers},
		{"AssignDevices/manual RequireClaims", http.MethodPost, "/api/v1/customer/:id/devices", "/api/v1/customer/c-1/devices", `{"device_ids":["dev-1"]}`, golden8LegacyAssignDevices, golden8AdaptedAssignDevices},
	}
	for _, tc := range cases {
		legacyCode, legacyBody := golden8Render(t, tc.method, tc.route, tc.target, tc.body, nil, tc.legacy)
		if legacyCode != http.StatusOK {
			t.Fatalf("%s: legacy HTTP status = %d, want %d (panic recovered by response middleware)", tc.name, legacyCode, http.StatusOK)
		}
		var legacyPayload apiresponse.Response
		if err := json.Unmarshal([]byte(legacyBody), &legacyPayload); err != nil {
			t.Fatalf("%s: decode legacy response body %q: %v", tc.name, legacyBody, err)
		}
		if legacyPayload.Code != errcode.CodeSystemError {
			t.Fatalf("%s: legacy response code = %d, want %d (recovered MustGet panic)", tc.name, legacyPayload.Code, errcode.CodeSystemError)
		}

		code, body := golden8Render(t, tc.method, tc.route, tc.target, tc.body, nil, tc.adapted)
		if code != http.StatusOK {
			t.Fatalf("%s: adapted HTTP status = %d, want %d", tc.name, code, http.StatusOK)
		}
		var payload apiresponse.Response
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			t.Fatalf("%s: decode adapted response body %q: %v", tc.name, body, err)
		}
		if payload.Code != errcode.CodeUnauthorized {
			t.Fatalf("%s: adapted response code = %d, want %d (body=%s)", tc.name, payload.Code, errcode.CodeUnauthorized, body)
		}
	}
}
