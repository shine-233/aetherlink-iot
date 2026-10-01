// 文件用途：role.go 迁移到 handler_adapter 骨架后的 golden 对比测试（编号 2 专属文件）。
// 核心逻辑：service.GroupApp.Role / service.RolePermission 是具体结构体、无法注入替身，因此沿用
// handler_adapter_test.go 的探针复刻法：把 role.go 迁移前的手写 handler 原样复刻为 legacy 形态、
// 迁移后的适配器形态逐行复刻为 adapted 形态，两侧的 service 调用替换为同一组探针函数（出参相同），
// 在同一套 response 中间件下渲染，对 HTTP 状态码与原始响应体做全等比较。两侧唯一差异是 handler 形态本身，
// 因此能直接证伪「迁移改变了 JSON 包络」。
// 渲染探针用本文件的 roleGolden2Render 而非共享的 renderProbe：后者用同一字符串注册路由并发起请求，
// 带 query 的 target 会被注册成含 "?" 的字面量路径，请求按 URL.Path 匹配即落 404，比对退化为 404==404 空转；
// roleGolden2Render 分离注册路径与请求串，并用 404 守卫保证用例真实触达 handler。
// 覆盖的迁移前特例：
//   - AssignRolePermissions / AssignRoleUsers 的参数错误是 errcode.WithData(CodeParamError, {"error": ...}) 包络
//     （响应带 data 字段、消息取错误码默认文案），与 reportParamError 的 NewWithMessage 形态不同，逐字节比对固化；
//   - UpdateRole 的空更新分支直接 c.JSON {"code":400,...}（HTTP 不可达，仅保留迁移前行为），单独测试固化短路语义；
//   - 缺失/类型不符 claims 的归一化差异（CodeNoPermission/"unauthorized"、MustGet panic -> CodeUnauthorized）
//     不参与逐字节比对，由 TestRoleGolden2MissingClaimsDeltaIsUnauthorized 单独断言。
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiresponse "aetherlink-iot/backend/internal/middleware/response"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ---- 探针：替代 service 出参，legacy / adapted 两侧共用，保证唯一变量是 handler 形态 ----

func roleGolden2UpdateProbe(req *model.UpdateRoleReq, claims *utils.UserClaims) (interface{}, error) {
	if req.Id == "boom-role" {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return gin.H{"id": req.Id, "name": req.Name, "tenant": claims.TenantID}, nil
}

func roleGolden2HasRole(id string) bool {
	return id == "role-in-use"
}

func roleGolden2DeleteProbe(id string, _ *utils.UserClaims) error {
	if id == "boom-role" {
		return errcode.New(errcode.CodeSystemError)
	}
	return nil
}

func roleGolden2ListByPageProbe(req *model.GetRoleListByPageReq, claims *utils.UserClaims) (interface{}, error) {
	if req.Page >= 999 {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return gin.H{"page": req.Page, "page_size": req.PageSize, "name": req.Name, "tenant": claims.TenantID}, nil
}

func roleGolden2ListPermissionsProbe(_ context.Context, module string, _ *utils.UserClaims) ([]model.SysPermission, error) {
	if module == "boom" {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return []model.SysPermission{{Code: "p1", Name: "perm", Module: module}}, nil
}

func roleGolden2RolePermissionsProbe(_ context.Context, roleID string, claims *utils.UserClaims) (*model.RolePermissionsResp, error) {
	if roleID == "boom-role" {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return &model.RolePermissionsResp{RoleID: roleID, RoleName: "ops", TenantID: claims.TenantID, Codes: []string{"p1"}}, nil
}

func roleGolden2AssignPermissionsProbe(_ context.Context, roleID string, _ *model.AssignRolePermissionsReq, _ *utils.UserClaims) error {
	if roleID == "boom-role" {
		return errcode.New(errcode.CodeSystemError)
	}
	return nil
}

func roleGolden2RoleUsersProbe(_ context.Context, roleID string, claims *utils.UserClaims) (*model.RoleUsersResp, error) {
	if roleID == "boom-role" {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return &model.RoleUsersResp{RoleID: roleID, RoleName: "ops", Users: []model.RoleUserSummary{{ID: claims.ID}}}, nil
}

func roleGolden2AssignUsersProbe(_ context.Context, roleID string, _ *model.AssignRoleUsersReq, _ *utils.UserClaims) error {
	if roleID == "boom-role" {
		return errcode.New(errcode.CodeSystemError)
	}
	return nil
}

// ---- legacy 形态：role.go 迁移前手写 handler 的逐行复刻（service 调用替换为探针）----

func roleGolden2LegacyUpdate(c *gin.Context) {
	var req model.UpdateRoleReq
	if !BindAndValidate(c, &req) {
		return
	}

	if req.Description == nil && req.Name == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "修改内容不能为空"})
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := roleGolden2UpdateProbe(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

func roleGolden2LegacyDelete(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	if roleGolden2HasRole(id) {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"role_id": id,
			"error":   "Role in use",
		}))
		return
	}

	err := roleGolden2DeleteProbe(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func roleGolden2LegacyListByPage(c *gin.Context) {
	var req model.GetRoleListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}

	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	roleList, err := roleGolden2ListByPageProbe(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", roleList)
}

func roleGolden2LegacyListPermissions(c *gin.Context) {
	claimsVal, exists := c.Get("claims")
	if !exists {
		c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized"))
		return
	}
	userClaims, ok := claimsVal.(*utils.UserClaims)
	if !ok || userClaims == nil {
		c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized"))
		return
	}

	module := c.Query("module")
	list, err := roleGolden2ListPermissionsProbe(c.Request.Context(), module, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", list)
}

func roleGolden2LegacyGetRolePermissions(c *gin.Context) {
	roleID := c.Param("id")
	claimsVal, exists := c.Get("claims")
	if !exists {
		c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized"))
		return
	}
	userClaims, ok := claimsVal.(*utils.UserClaims)
	if !ok || userClaims == nil {
		c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized"))
		return
	}

	resp, err := roleGolden2RolePermissionsProbe(c.Request.Context(), roleID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

func roleGolden2LegacyAssignPermissions(c *gin.Context) {
	roleID := c.Param("id")
	claimsVal, exists := c.Get("claims")
	if !exists {
		c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized"))
		return
	}
	userClaims, ok := claimsVal.(*utils.UserClaims)
	if !ok || userClaims == nil {
		c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized"))
		return
	}

	var req model.AssignRolePermissionsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{"error": err.Error()}))
		return
	}

	if err := roleGolden2AssignPermissionsProbe(c.Request.Context(), roleID, &req, userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"status": "ok"})
}

func roleGolden2LegacyGetRoleUsers(c *gin.Context) {
	roleID := c.Param("id")
	claimsVal, exists := c.Get("claims")
	if !exists {
		c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized"))
		return
	}
	userClaims, ok := claimsVal.(*utils.UserClaims)
	if !ok || userClaims == nil {
		c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized"))
		return
	}

	resp, err := roleGolden2RoleUsersProbe(c.Request.Context(), roleID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

func roleGolden2LegacyAssignUsers(c *gin.Context) {
	roleID := c.Param("id")
	claimsVal, exists := c.Get("claims")
	if !exists {
		c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized"))
		return
	}
	userClaims, ok := claimsVal.(*utils.UserClaims)
	if !ok || userClaims == nil {
		c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized"))
		return
	}

	var req model.AssignRoleUsersReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{"error": err.Error()}))
		return
	}

	if err := roleGolden2AssignUsersProbe(c.Request.Context(), roleID, &req, userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"status": "ok"})
}

// ---- adapted 形态：role.go 迁移后 handler 的逐行复刻（service 调用替换为探针）----

func roleGolden2AdaptedUpdate(c *gin.Context) {
	Handle(c, func(req *model.UpdateRoleReq, userClaims *utils.UserClaims) (interface{}, error) {
		if req.Description == nil && req.Name == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "修改内容不能为空"})
			return nil, nil
		}
		return roleGolden2UpdateProbe(req, userClaims)
	})
}

func roleGolden2AdaptedDelete(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		if roleGolden2HasRole(id) {
			return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"role_id": id,
				"error":   "Role in use",
			})
		}
		return roleGolden2DeleteProbe(id, userClaims)
	})
}

func roleGolden2AdaptedListByPage(c *gin.Context) {
	Handle(c, func(req *model.GetRoleListByPageReq, userClaims *utils.UserClaims) (interface{}, error) {
		return roleGolden2ListByPageProbe(req, userClaims)
	})
}

func roleGolden2AdaptedListPermissions(c *gin.Context) {
	HandleNoBody(c, func(userClaims *utils.UserClaims) ([]model.SysPermission, error) {
		return roleGolden2ListPermissionsProbe(c.Request.Context(), c.Query("module"), userClaims)
	})
}

func roleGolden2AdaptedGetRolePermissions(c *gin.Context) {
	HandlePath(c, "id", func(roleID string, userClaims *utils.UserClaims) (*model.RolePermissionsResp, error) {
		return roleGolden2RolePermissionsProbe(c.Request.Context(), roleID, userClaims)
	})
}

func roleGolden2AdaptedAssignPermissions(c *gin.Context) {
	// 与 role.go 迁移后实现同构（集成阶段起经共享助手 bindBodyLegacyParamError 绑定）。
	HandlePath(c, "id", func(roleID string, userClaims *utils.UserClaims) (interface{}, error) {
		var req model.AssignRolePermissionsReq
		if err := bindBodyLegacyParamError(c, &req); err != nil {
			return nil, err
		}
		if err := roleGolden2AssignPermissionsProbe(c.Request.Context(), roleID, &req, userClaims); err != nil {
			return nil, err
		}
		return gin.H{"status": "ok"}, nil
	})
}

func roleGolden2AdaptedGetRoleUsers(c *gin.Context) {
	HandlePath(c, "id", func(roleID string, userClaims *utils.UserClaims) (*model.RoleUsersResp, error) {
		return roleGolden2RoleUsersProbe(c.Request.Context(), roleID, userClaims)
	})
}

func roleGolden2AdaptedAssignUsers(c *gin.Context) {
	// 与 role.go 迁移后实现同构（集成阶段起经共享助手 bindBodyLegacyParamError 绑定）。
	HandlePath(c, "id", func(roleID string, userClaims *utils.UserClaims) (interface{}, error) {
		var req model.AssignRoleUsersReq
		if err := bindBodyLegacyParamError(c, &req); err != nil {
			return nil, err
		}
		if err := roleGolden2AssignUsersProbe(c.Request.Context(), roleID, &req, userClaims); err != nil {
			return nil, err
		}
		return gin.H{"status": "ok"}, nil
	})
}

// ---- 逐字节对比用例 ----

// roleGolden2Render 是本文件专用的渲染探针：与 handler_adapter_test.go 的 renderProbe 等价，
// 但把注册路径与 query 分离——gin 路由树按 URL.Path 匹配，renderProbe 用同一字符串注册+请求时
// 带 query 的 target 会被注册成字面量路径而落到 404，比对退化为 404==404 的空转。
func roleGolden2Render(t *testing.T, method, target, body string, withClaims bool, handler gin.HandlerFunc) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	responseHandler := &apiresponse.Handler{
		ErrManager: errcode.NewErrorManager("", ""),
	}
	router := gin.New()
	router.Use(responseHandler.Middleware())
	if withClaims {
		router.Use(func(c *gin.Context) {
			c.Set(claimsContextKey, &utils.UserClaims{ID: "golden2-user", TenantID: "golden2-tenant"})
			c.Next()
		})
	}
	routePath := target
	if idx := strings.IndexByte(target, '?'); idx >= 0 {
		routePath = target[:idx]
	}
	router.Handle(method, routePath, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", "en-US")
	router.ServeHTTP(recorder, request)

	return recorder.Code, recorder.Body.String()
}

// roleGolden2RequireIdentical 断言新旧两种 handler 形态在同一入参下渲染出完全相同的响应，
// 并用 404 守卫确保请求真的到达了被测 handler（否则比对空转）。
func roleGolden2RequireIdentical(t *testing.T, name, method, target, body string, withClaims bool, legacy, adapted gin.HandlerFunc) {
	t.Helper()
	legacyCode, legacyBody := roleGolden2Render(t, method, target, body, withClaims, legacy)
	adaptedCode, adaptedBody := roleGolden2Render(t, method, target, body, withClaims, adapted)

	if legacyCode == http.StatusNotFound {
		t.Fatalf("%s: request hit gin 404 (route %q not matched), comparison is meaningless", name, target)
	}
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

// TestRoleGolden2UpdateRoleIsByteIdentical 覆盖 UpdateRole：成功、绑定失败、校验失败、service 错误分支。
// 空更新分支经 HTTP 不可达（Name 带 required 校验），其短路语义由 TestRoleGolden2EmptyUpdateGuardKeepsCustomJSON 固化。
func TestRoleGolden2UpdateRoleIsByteIdentical(t *testing.T) {
	for _, body := range []string{
		`{"id":"role-1","name":"ops"}`,
		`{"id":"role-1","name":"ops","description":"desc"}`,
		`{"id":"role-1"}`,
		`{"id":"role-1","name":""}`,
		`not-json`,
		`{"id":"boom-role","name":"x"}`,
	} {
		roleGolden2RequireIdentical(t, "UpdateRole/"+body, http.MethodPut, "/golden2/role", body, true, roleGolden2LegacyUpdate, roleGolden2AdaptedUpdate)
	}
}

// TestRoleGolden2DeleteRoleIsByteIdentical 覆盖 DeleteRole：成功（data 省略）、Casbin 占用（WithData 错误包络）、service 错误分支。
func TestRoleGolden2DeleteRoleIsByteIdentical(t *testing.T) {
	for _, id := range []string{"role-1", "role-in-use", "boom-role"} {
		roleGolden2RequireIdentical(t, "DeleteRole/"+id, http.MethodDelete, "/golden2/role/"+id, "", true, roleGolden2LegacyDelete, roleGolden2AdaptedDelete)
	}
}

// TestRoleGolden2RoleListByPageIsByteIdentical 覆盖 HandleRoleListByPage 的 GET query 绑定：成功、缺参、越界、service 错误分支。
func TestRoleGolden2RoleListByPageIsByteIdentical(t *testing.T) {
	for _, target := range []string{
		"/golden2/role?page=1&page_size=10",
		"/golden2/role?page=1&page_size=10&name=ops",
		"/golden2/role",
		"/golden2/role?page=0&page_size=10",
		"/golden2/role?page=999&page_size=10",
	} {
		roleGolden2RequireIdentical(t, "ListByPage"+target, http.MethodGet, target, "", true, roleGolden2LegacyListByPage, roleGolden2AdaptedListByPage)
	}
}

// TestRoleGolden2ListPermissionsIsByteIdentical 覆盖 ListPermissions：带/不带 module、service 错误分支。
func TestRoleGolden2ListPermissionsIsByteIdentical(t *testing.T) {
	for _, target := range []string{
		"/golden2/permissions?module=users",
		"/golden2/permissions",
		"/golden2/permissions?module=boom",
	} {
		roleGolden2RequireIdentical(t, "ListPermissions"+target, http.MethodGet, target, "", true, roleGolden2LegacyListPermissions, roleGolden2AdaptedListPermissions)
	}
}

// TestRoleGolden2RolePermissionsAndUsersIsByteIdentical 覆盖 GetRolePermissions / GetRoleUsers 两个只读 path 接口。
func TestRoleGolden2RolePermissionsAndUsersIsByteIdentical(t *testing.T) {
	for _, id := range []string{"role-1", "boom-role"} {
		roleGolden2RequireIdentical(t, "GetRolePermissions/"+id, http.MethodGet, "/golden2/roles/"+id+"/permissions", "", true, roleGolden2LegacyGetRolePermissions, roleGolden2AdaptedGetRolePermissions)
		roleGolden2RequireIdentical(t, "GetRoleUsers/"+id, http.MethodGet, "/golden2/roles/"+id+"/users", "", true, roleGolden2LegacyGetRoleUsers, roleGolden2AdaptedGetRoleUsers)
	}
}

// TestRoleGolden2AssignPathsAreByteIdentical 覆盖 AssignRolePermissions / AssignRoleUsers：
// 重点固化迁移前特有的 WithData 参数错误包络（空 body 触发 EOF、损坏 JSON 触发语法错误，响应均带 data 字段）。
func TestRoleGolden2AssignPathsAreByteIdentical(t *testing.T) {
	type assignCase struct {
		name   string
		method string
		target string
		body   string
		legacy gin.HandlerFunc
		server gin.HandlerFunc
	}
	cases := []assignCase{
		{"AssignPermissions/valid", http.MethodPost, "/golden2/roles/role-1/permissions", `{"permission_codes":["p1","p2"]}`, roleGolden2LegacyAssignPermissions, roleGolden2AdaptedAssignPermissions},
		{"AssignPermissions/empty-body-eof", http.MethodPost, "/golden2/roles/role-1/permissions", ``, roleGolden2LegacyAssignPermissions, roleGolden2AdaptedAssignPermissions},
		{"AssignPermissions/broken-json", http.MethodPost, "/golden2/roles/role-1/permissions", `{"permission_codes":}`, roleGolden2LegacyAssignPermissions, roleGolden2AdaptedAssignPermissions},
		{"AssignPermissions/service-error", http.MethodPost, "/golden2/roles/boom-role/permissions", `{"permission_codes":["p1"]}`, roleGolden2LegacyAssignPermissions, roleGolden2AdaptedAssignPermissions},
		{"AssignUsers/valid", http.MethodPost, "/golden2/roles/role-1/users", `{"user_ids":["u1","u2"]}`, roleGolden2LegacyAssignUsers, roleGolden2AdaptedAssignUsers},
		{"AssignUsers/empty-body-eof", http.MethodPost, "/golden2/roles/role-1/users", ``, roleGolden2LegacyAssignUsers, roleGolden2AdaptedAssignUsers},
		{"AssignUsers/broken-json", http.MethodPost, "/golden2/roles/role-1/users", `{"user_ids":}`, roleGolden2LegacyAssignUsers, roleGolden2AdaptedAssignUsers},
		{"AssignUsers/service-error", http.MethodPost, "/golden2/roles/boom-role/users", `{"user_ids":["u1"]}`, roleGolden2LegacyAssignUsers, roleGolden2AdaptedAssignUsers},
	}
	for _, tc := range cases {
		roleGolden2RequireIdentical(t, tc.name, tc.method, tc.target, tc.body, true, tc.legacy, tc.server)
	}
}

// TestRoleGolden2EmptyUpdateGuardKeepsCustomJSON 固化 UpdateRole 空更新分支的短路语义：
// 直接 c.JSON 写出 {"code":400,...} 后返回零值、经 respond 收尾（c.Set("data", ...)），
// 响应中间件对已写出的响应短路（middleware/response/response.go:62），输出与迁移前手写分支逐字节一致。
// 触发条件在测试内直接构造（生产路径经 Name required 校验不可达）。
func TestRoleGolden2EmptyUpdateGuardKeepsCustomJSON(t *testing.T) {
	legacy := func(c *gin.Context) {
		req := model.UpdateRoleReq{Id: "role-1"}
		if req.Description == nil && req.Name == "" {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "修改内容不能为空"})
			return
		}
		c.Set("data", "unreachable")
	}
	adapted := func(c *gin.Context) {
		data, closureErr := func(req model.UpdateRoleReq) (interface{}, error) {
			if req.Description == nil && req.Name == "" {
				c.JSON(http.StatusOK, gin.H{"code": 400, "message": "修改内容不能为空"})
				return nil, nil
			}
			return nil, nil
		}(model.UpdateRoleReq{Id: "role-1"})
		respond(c, data, closureErr)
	}
	roleGolden2RequireIdentical(t, "UpdateRole/empty-update-guard", http.MethodPut, "/golden2/role", `{"id":"role-1"}`, true, legacy, adapted)
}

// TestRoleGolden2MissingClaimsDeltaIsUnauthorized 固化迁移的有意行为差异（不参与逐字节比对）：
// 缺失 claims 时，迁移前 DeleteRole 因 MustGet panic 被响应中间件 recover 成 CodeSystemError(100000)，
// 手写检查的 ListPermissions / AssignRoleUsers 返回 CodeNoPermission(201001)/"unauthorized"；
// 迁移后统一为 RequireClaims 的 CodeUnauthorized(200001)。与 handler_adapter.go 头注释的声明一致。
func TestRoleGolden2MissingClaimsDeltaIsUnauthorized(t *testing.T) {
	// DeleteRole：legacy MustGet panic -> recover 成 CodeSystemError；adapted -> CodeUnauthorized。
	legacyCode, legacyBody := roleGolden2Render(t, http.MethodDelete, "/golden2/role/role-1", "", false, roleGolden2LegacyDelete)
	adaptedCode, adaptedBody := roleGolden2Render(t, http.MethodDelete, "/golden2/role/role-1", "", false, roleGolden2AdaptedDelete)
	assertGolden2Code(t, "DeleteRole/legacy", legacyCode, legacyBody, errcode.CodeSystemError, "")
	assertGolden2Code(t, "DeleteRole/adapted", adaptedCode, adaptedBody, errcode.CodeUnauthorized, "")
	if legacyBody == adaptedBody {
		t.Fatalf("DeleteRole missing-claims: expected legacy/adapted bodies to differ, both = %s", legacyBody)
	}

	// ListPermissions：legacy 手写检查 -> CodeNoPermission/"unauthorized"；adapted -> CodeUnauthorized。
	legacyCode, legacyBody = roleGolden2Render(t, http.MethodGet, "/golden2/permissions?module=users", "", false, roleGolden2LegacyListPermissions)
	adaptedCode, adaptedBody = roleGolden2Render(t, http.MethodGet, "/golden2/permissions?module=users", "", false, roleGolden2AdaptedListPermissions)
	assertGolden2Code(t, "ListPermissions/legacy", legacyCode, legacyBody, errcode.CodeNoPermission, "unauthorized")
	assertGolden2Code(t, "ListPermissions/adapted", adaptedCode, adaptedBody, errcode.CodeUnauthorized, "")
	if legacyBody == adaptedBody {
		t.Fatalf("ListPermissions missing-claims: expected legacy/adapted bodies to differ, both = %s", legacyBody)
	}

	// AssignRoleUsers：legacy 手写检查先于 body 绑定 -> CodeNoPermission；adapted RequireClaims 先于绑定 -> CodeUnauthorized。
	legacyCode, legacyBody = roleGolden2Render(t, http.MethodPost, "/golden2/roles/role-1/users", `{"user_ids":["u1"]}`, false, roleGolden2LegacyAssignUsers)
	adaptedCode, adaptedBody = roleGolden2Render(t, http.MethodPost, "/golden2/roles/role-1/users", `{"user_ids":["u1"]}`, false, roleGolden2AdaptedAssignUsers)
	assertGolden2Code(t, "AssignRoleUsers/legacy", legacyCode, legacyBody, errcode.CodeNoPermission, "unauthorized")
	assertGolden2Code(t, "AssignRoleUsers/adapted", adaptedCode, adaptedBody, errcode.CodeUnauthorized, "")
	if legacyBody == adaptedBody {
		t.Fatalf("AssignRoleUsers missing-claims: expected legacy/adapted bodies to differ, both = %s", legacyBody)
	}
}

// assertGolden2Code 校验响应码与可选的自定义消息（空串表示不校验消息）。
func assertGolden2Code(t *testing.T, name string, code int, body string, wantCode int, wantMessage string) {
	t.Helper()
	if code != http.StatusOK {
		t.Fatalf("%s: HTTP status = %d, want %d (body=%s)", name, code, http.StatusOK, body)
	}
	var payload apiresponse.Response
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("%s: decode response body %q: %v", name, body, err)
	}
	if payload.Code != wantCode {
		t.Fatalf("%s: response code = %d, want %d (body=%s)", name, payload.Code, wantCode, body)
	}
	if wantMessage != "" && payload.Message != wantMessage {
		t.Fatalf("%s: response message = %q, want %q", name, payload.Message, wantMessage)
	}
}
