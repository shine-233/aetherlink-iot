// 文件用途：edge_node.go 迁移到 handler_adapter 骨架后的 golden 对比测试（迁移员 5 专用，编号 5）。
// 核心逻辑：把迁移前的手写 handler 形态（可选绑定 + MustGet("claims") + c.Error + c.Set("data")）
// 与迁移后 edge_node.go 的等价形态（HandlePathBodyOptional / RequireClaims / HandleNoBody / HandlePath / respond）并排实现，
// 走同一套 response 中间件渲染，对 HTTP 状态码与原始响应体字符串做全等比较，证明 JSON 包络逐字节一致。
// service 调用替换为探针函数（真实 EdgeNodeService 依赖 DB，无法在单测中调用）：
// 探针签名与 service.GroupApp.EdgeNode 对应方法一致，仅按入参模拟成功/失败分支，
// 新写法探针侧与 edge_node.go 迁移后的代码逐行同构，旧写法探针侧为迁移前的逐字形态。
// 已知的有意差异：缺失 claims 时旧写法 MustGet panic 被 response 中间件 recover 成 CodeSystemError，
// 新写法 RequireClaims 返回 CodeUnauthorized——由 TestGolden5EdgeNodeMissingClaimsIntentionalDifference 单独固化，
// 与共享测试 TestHandleAdapterMissingClaimsReturnsUnauthorizedInsteadOfPanic 的结论一致。
package api

import (
	"encoding/json"
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

// golden5BoomTenant 触发探针业务错误分支的租户标识；"boom" 用作触发错误分支的 node_id。
const golden5BoomTenant = "boom"

// ---- 探针（签名对齐 service.GroupApp.EdgeNode 各方法）----

func golden5HeartbeatProbe(nodeID string, req model.EdgeNodeHeartbeatReq, claims *utils.UserClaims) (interface{}, error) {
	if nodeID == "boom" {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return map[string]interface{}{
		"node_id":      nodeID,
		"version":      req.Version,
		"capabilities": req.Capabilities,
		"tenant":       claims.TenantID,
	}, nil
}

func golden5ListProbe(limit int, claims *utils.UserClaims) (interface{}, error) {
	if claims.TenantID == golden5BoomTenant {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return map[string]interface{}{"limit": limit, "tenant": claims.TenantID}, nil
}

func golden5IssueCertificateProbe(nodeID string, req model.IssueEdgeNodeCertificateReq, claims *utils.UserClaims) (interface{}, error) {
	if nodeID == "boom" {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return map[string]interface{}{"node_id": nodeID, "validity_days": req.ValidityDays, "tenant": claims.TenantID}, nil
}

func golden5RevokeCertificateProbe(nodeID string, claims *utils.UserClaims) error {
	if nodeID == "boom" {
		return errcode.New(errcode.CodeSystemError)
	}
	return nil
}

func golden5UpgradeHistoryProbe(nodeID string, limit int, claims *utils.UserClaims) (interface{}, error) {
	if nodeID == "boom" {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return map[string]interface{}{"node_id": nodeID, "limit": limit, "tenant": claims.TenantID}, nil
}

// ---- 旧写法（迁移前 edge_node.go 的逐字形态，仅把 service 调用换成探针）----

func golden5LegacyHeartbeat(c *gin.Context) {
	nodeID := c.Param("node_id")
	var req model.EdgeNodeHeartbeatReq
	_ = c.ShouldBindJSON(&req)
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := golden5HeartbeatProbe(nodeID, req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

func golden5LegacyList(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	limit := 0
	if v := c.Query("limit"); v != "" {
		if parsed, perr := strconv.Atoi(v); perr == nil && parsed > 0 {
			limit = parsed
		}
	}
	resp, err := golden5ListProbe(limit, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

func golden5LegacyIssueCertificate(c *gin.Context) {
	nodeID := c.Param("node_id")
	var req model.IssueEdgeNodeCertificateReq
	_ = c.ShouldBindJSON(&req)
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := golden5IssueCertificateProbe(nodeID, req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

func golden5LegacyRevokeCertificate(c *gin.Context) {
	nodeID := c.Param("node_id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	err := golden5RevokeCertificateProbe(nodeID, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"message": "edge node certificate revoked"})
}

func golden5LegacyUpgradeHistory(c *gin.Context) {
	nodeID := c.Param("node_id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	limit := 0
	if v := c.Query("limit"); v != "" {
		if parsed, perr := strconv.Atoi(v); perr == nil && parsed > 0 {
			limit = parsed
		}
	}
	resp, err := golden5UpgradeHistoryProbe(nodeID, limit, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// ---- 新写法（迁移后 edge_node.go 的等价形态，仅把 service 调用换成探针）----

func golden5AdaptedHeartbeat(c *gin.Context) {
	// 与 edge_node.go 迁移后实现同构（集成阶段起走共享助手 HandlePathBodyOptional）。
	HandlePathBodyOptional(c, "node_id", func(nodeID string, req *model.EdgeNodeHeartbeatReq, claims *utils.UserClaims) (interface{}, error) {
		return golden5HeartbeatProbe(nodeID, *req, claims)
	})
}

func golden5AdaptedList(c *gin.Context) {
	HandleNoBody(c, func(claims *utils.UserClaims) (interface{}, error) {
		limit := 0
		if v := c.Query("limit"); v != "" {
			if parsed, perr := strconv.Atoi(v); perr == nil && parsed > 0 {
				limit = parsed
			}
		}
		return golden5ListProbe(limit, claims)
	})
}

func golden5AdaptedIssueCertificate(c *gin.Context) {
	// 与 edge_node.go 迁移后实现同构（集成阶段起走共享助手 HandlePathBodyOptional）。
	HandlePathBodyOptional(c, "node_id", func(nodeID string, req *model.IssueEdgeNodeCertificateReq, claims *utils.UserClaims) (interface{}, error) {
		return golden5IssueCertificateProbe(nodeID, *req, claims)
	})
}

func golden5AdaptedRevokeCertificate(c *gin.Context) {
	HandlePath(c, "node_id", func(nodeID string, claims *utils.UserClaims) (interface{}, error) {
		if err := golden5RevokeCertificateProbe(nodeID, claims); err != nil {
			return nil, err
		}
		return gin.H{"message": "edge node certificate revoked"}, nil
	})
}

func golden5AdaptedUpgradeHistory(c *gin.Context) {
	HandlePath(c, "node_id", func(nodeID string, claims *utils.UserClaims) (interface{}, error) {
		limit := 0
		if v := c.Query("limit"); v != "" {
			if parsed, perr := strconv.Atoi(v); perr == nil && parsed > 0 {
				limit = parsed
			}
		}
		return golden5UpgradeHistoryProbe(nodeID, limit, claims)
	})
}

// ---- 渲染与比对工具（自包含副本，避免耦合共享测试文件）----

// golden5Render 用与线上一致的 response 中间件渲染一次请求，返回状态码与原始响应体。
// claims 为 nil 时不注入鉴权中间件，用于固化缺失 claims 的有意差异。
func golden5Render(t *testing.T, method, route, target, body string, claims *utils.UserClaims, handler gin.HandlerFunc) (int, string) {
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

// golden5RequireIdentical 断言新旧两种写法在同一入参下渲染出完全相同的响应。
func golden5RequireIdentical(t *testing.T, name, method, route, target, body string, claims *utils.UserClaims, legacy, adapted gin.HandlerFunc) {
	t.Helper()
	legacyCode, legacyBody := golden5Render(t, method, route, target, body, claims, legacy)
	adaptedCode, adaptedBody := golden5Render(t, method, route, target, body, claims, adapted)

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

// TestGolden5EdgeNodeHeartbeatMatchesLegacy 覆盖 Heartbeat：可选绑定（空体/坏 JSON 均放行）与错误分支。
func TestGolden5EdgeNodeHeartbeatMatchesLegacy(t *testing.T) {
	claims := &utils.UserClaims{ID: "golden5-user", TenantID: "golden5-tenant"}
	route := "/edge/nodes/:node_id/heartbeat"
	cases := []struct {
		name   string
		nodeID string
		body   string
	}{
		{"valid body", "node-1", `{"version":"1.2.0","capabilities":["mqtt","ota"]}`},
		{"empty body tolerated", "node-1", ""},
		{"invalid json tolerated", "node-1", `{not-json`},
		{"service error", "boom", `{"version":"1.2.0"}`},
	}
	for _, tc := range cases {
		golden5RequireIdentical(t, "Heartbeat/"+tc.name, http.MethodPost, route, "/edge/nodes/"+tc.nodeID+"/heartbeat", tc.body, claims, golden5LegacyHeartbeat, golden5AdaptedHeartbeat)
	}
}

// TestGolden5EdgeNodeListMatchesLegacy 覆盖 List：limit 宽松解析（缺省/非法/非正数均按 0）与错误分支。
func TestGolden5EdgeNodeListMatchesLegacy(t *testing.T) {
	claims := &utils.UserClaims{ID: "golden5-user", TenantID: "golden5-tenant"}
	boomClaims := &utils.UserClaims{ID: "golden5-user", TenantID: golden5BoomTenant}
	targets := []string{"/edge/nodes", "/edge/nodes?limit=5", "/edge/nodes?limit=abc", "/edge/nodes?limit=-3", "/edge/nodes?limit=0"}
	for _, target := range targets {
		golden5RequireIdentical(t, "List"+target, http.MethodGet, "/edge/nodes", target, "", claims, golden5LegacyList, golden5AdaptedList)
	}
	golden5RequireIdentical(t, "List/service error", http.MethodGet, "/edge/nodes", "/edge/nodes?limit=5", "", boomClaims, golden5LegacyList, golden5AdaptedList)
}

// TestGolden5EdgeNodeIssueCertificateMatchesLegacy 覆盖 IssueCertificate：可选绑定与错误分支。
func TestGolden5EdgeNodeIssueCertificateMatchesLegacy(t *testing.T) {
	claims := &utils.UserClaims{ID: "golden5-user", TenantID: "golden5-tenant"}
	route := "/edge/nodes/:node_id/certificate"
	cases := []struct {
		name   string
		nodeID string
		body   string
	}{
		{"valid body", "node-1", `{"validity_days":365}`},
		{"empty body tolerated", "node-1", ""},
		{"invalid json tolerated", "node-1", `{not-json`},
		{"service error", "boom", `{"validity_days":365}`},
	}
	for _, tc := range cases {
		golden5RequireIdentical(t, "IssueCertificate/"+tc.name, http.MethodPost, route, "/edge/nodes/"+tc.nodeID+"/certificate", tc.body, claims, golden5LegacyIssueCertificate, golden5AdaptedIssueCertificate)
	}
}

// TestGolden5EdgeNodeRevokeCertificateMatchesLegacy 覆盖 RevokeCertificate：成功包络保留 data.message 与错误分支。
func TestGolden5EdgeNodeRevokeCertificateMatchesLegacy(t *testing.T) {
	claims := &utils.UserClaims{ID: "golden5-user", TenantID: "golden5-tenant"}
	route := "/edge/nodes/:node_id/certificate"
	golden5RequireIdentical(t, "RevokeCertificate/success", http.MethodDelete, route, "/edge/nodes/node-1/certificate", "", claims, golden5LegacyRevokeCertificate, golden5AdaptedRevokeCertificate)
	golden5RequireIdentical(t, "RevokeCertificate/service error", http.MethodDelete, route, "/edge/nodes/boom/certificate", "", claims, golden5LegacyRevokeCertificate, golden5AdaptedRevokeCertificate)
}

// TestGolden5EdgeNodeGetUpgradeHistoryMatchesLegacy 覆盖 GetUpgradeHistory：路径参数 + limit 宽松解析与错误分支。
func TestGolden5EdgeNodeGetUpgradeHistoryMatchesLegacy(t *testing.T) {
	claims := &utils.UserClaims{ID: "golden5-user", TenantID: "golden5-tenant"}
	route := "/edge/nodes/:node_id/upgrade/history"
	targets := []string{
		"/edge/nodes/node-1/upgrade/history",
		"/edge/nodes/node-1/upgrade/history?limit=10",
		"/edge/nodes/node-1/upgrade/history?limit=oops",
		"/edge/nodes/node-1/upgrade/history?limit=-1",
	}
	for _, target := range targets {
		golden5RequireIdentical(t, "GetUpgradeHistory"+target, http.MethodGet, route, target, "", claims, golden5LegacyUpgradeHistory, golden5AdaptedUpgradeHistory)
	}
	golden5RequireIdentical(t, "GetUpgradeHistory/service error", http.MethodGet, route, "/edge/nodes/boom/upgrade/history?limit=10", "", claims, golden5LegacyUpgradeHistory, golden5AdaptedUpgradeHistory)
}

// TestGolden5EdgeNodeMissingClaimsIntentionalDifference 固化有意的行为差异（与本文件迁移的三类入口各验一例）：
// 缺失 claims 时旧写法 MustGet panic 被 response 中间件 recover 成 CodeSystemError；
// 新写法 RequireClaims（含 HandleNoBody / HandlePath 内部）返回标准 CodeUnauthorized。
func TestGolden5EdgeNodeMissingClaimsIntentionalDifference(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		route   string
		target  string
		body    string
		legacy  gin.HandlerFunc
		adapted gin.HandlerFunc
	}{
		{"Heartbeat", http.MethodPost, "/edge/nodes/:node_id/heartbeat", "/edge/nodes/node-1/heartbeat", `{"version":"1.0.0"}`, golden5LegacyHeartbeat, golden5AdaptedHeartbeat},
		{"List", http.MethodGet, "/edge/nodes", "/edge/nodes?limit=5", "", golden5LegacyList, golden5AdaptedList},
		{"RevokeCertificate", http.MethodDelete, "/edge/nodes/:node_id/certificate", "/edge/nodes/node-1/certificate", "", golden5LegacyRevokeCertificate, golden5AdaptedRevokeCertificate},
	}
	for _, tc := range cases {
		legacyCode, legacyBody := golden5Render(t, tc.method, tc.route, tc.target, tc.body, nil, tc.legacy)
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

		code, body := golden5Render(t, tc.method, tc.route, tc.target, tc.body, nil, tc.adapted)
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
