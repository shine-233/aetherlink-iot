// 文件用途：rule_chain.go 迁移到 handler_adapter 骨架后的 golden 对比测试（编号 1）。
// 核心逻辑：把迁移前的手写 handler 原样快照为 legacyRuleChain* 函数，与迁移后的
// (*RuleChainApi).Handle* 在同一套响应中间件 + 独立内存 sqlite 上渲染同一请求，
// 对 HTTP 状态码与原始响应体做全等比较，证明 JSON 包络逐字节一致。
// 覆盖形态：原始 body（Create/Update）、路径参数（Delete/Get/Replay/Resolve）、
// query 绑定（List）、双路径参数 + DefaultQuery 容错（NodeTraces/DeadLetters/ExecutionTraces）。
// 已知的有意差异（不在本文件比对范围）：缺失 claims 时旧写法 MustGet panic 被兜底成
// CodeSystemError，新写法 RequireClaims 返回 CodeUnauthorized，由
// TestRuleChainGolden1MissingClaimsBehavior 单独固化。
// 不覆盖项：HandleListRuleChainReplayRecords 迁移前就已是 HandlePath 形态（本次未改动，无新旧对比意义）。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/authz"
	apiresponse "aetherlink-iot/backend/internal/middleware/response"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ---- golden 固定入参 ----

const (
	ruleChainGoldenTenantID  = "rc-golden-tenant"
	ruleChainGoldenChainID   = "rc-golden-chain-1"
	ruleChainGoldenExecID    = "rc-golden-exec-1"
	ruleChainGoldenNodeID    = "n1"
	ruleChainGoldenValidBody = `{"name":"golden chain","graph":{"nodes":[{"id":"t1","type":"trigger.telemetry"}]}}`
)

// ruleChainGoldenFixedTime 返回固定的种子时间，保证响应里的时间戳跨运行逐字节一致。
func ruleChainGoldenFixedTime(minute int) time.Time {
	return time.Date(2026, 8, 1, 12, minute, 0, 0, time.UTC)
}

// ---- legacy 快照：迁移前 rule_chain.go 的手写 handler 原样复制 ----

func legacyRuleChainCreate(c *gin.Context) {
	raw, ok := readRuleChainBody(c)
	if !ok {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	chain, err := service.GroupApp.RuleChain.CreateChain(raw, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", chain)
}

func legacyRuleChainUpdate(c *gin.Context) {
	raw, ok := readRuleChainBody(c)
	if !ok {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	chain, err := service.GroupApp.RuleChain.UpdateChain(raw, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", chain)
}

func legacyRuleChainDelete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "id is required"))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.RuleChain.DeleteChain(id, userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}

func legacyRuleChainGet(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "id is required"))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	chain, err := service.GroupApp.RuleChain.GetChain(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", chain)
}

func legacyRuleChainList(c *gin.Context) {
	type listReq struct {
		Keyword  string `form:"keyword"`
		Page     int    `form:"page"`
		PageSize int    `form:"page_size" binding:"omitempty,max=200"`
	}
	var req listReq
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, err.Error()))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.RuleChain.ListChains(req.Keyword, req.Page, req.PageSize, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

func legacyRuleChainNodeTraces(c *gin.Context) {
	chainID := c.Param("id")
	nodeID := c.Param("nodeId")
	if chainID == "" || nodeID == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "id and nodeId are required"))
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil {
		limit = 0
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	traces, svcErr := service.GroupApp.RuleChain.GetNodeTraces(chainID, nodeID, limit, userClaims)
	if svcErr != nil {
		c.Error(svcErr)
		return
	}
	c.Set("data", traces)
}

func legacyRuleChainDeadLetters(c *gin.Context) {
	chainID := c.Param("id")
	if chainID == "" {
		chainID = c.Query("chain_id")
	}
	execID := c.Query("exec_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	res, err := service.GroupApp.RuleChain.ListRuleChainDeadLetters(chainID, execID, page, pageSize, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", res)
}

func legacyRuleChainExecutionTraces(c *gin.Context) {
	chainID := c.Param("id")
	execID := c.Param("execId")
	if execID == "" {
		execID = c.Query("exec_id")
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	traces, err := service.GroupApp.RuleChain.GetExecutionTraces(chainID, execID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", traces)
}

func legacyRuleChainReplay(c *gin.Context) {
	chainID := c.Param("id")
	if strings.TrimSpace(chainID) == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "chain id is required"))
		return
	}
	var req model.RuleChainReplayReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	res, err := service.GroupApp.RuleChain.ReplayExecution(c.Request.Context(), chainID, req.ExecutionID, req.ConfirmSideEffects, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", res)
}

func legacyRuleChainResolve(c *gin.Context) {
	deviceID := c.Param("deviceId")
	if strings.TrimSpace(deviceID) == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "deviceId is required"))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	res, svcErr := service.GroupApp.RuleChain.ResolveEffectiveRuleChainsForDevice(deviceID, userClaims)
	if svcErr != nil {
		c.Error(svcErr)
		return
	}
	c.Set("data", res)
}

// ---- golden 测试基建 ----

var ruleChainGoldenDBSide = 0

// ruleChainGoldenDB 每侧渲染前重建一块独立的共享缓存内存 sqlite 并迁移本次用到的表，
// 避免写操作（create/delete）在 legacy 与 adapted 两次渲染间产生状态串扰。
// rule_chain_replay_records 是裸 Table() 名（无 model TableName），用 DDL 显式建表。
func ruleChainGoldenDB(t *testing.T, side string) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	ruleChainGoldenDBSide++
	dbName := fmt.Sprintf("%s_%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), side, ruleChainGoldenDBSide)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.RuleChain{}, &model.RuleChainNodeTrace{}, &model.RuleChainDeadLetter{}, &model.DeviceConfig{}); err != nil {
		t.Fatalf("migrate golden tables: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS rule_chain_replay_records (
		id text PRIMARY KEY,
		tenant_id text NOT NULL,
		chain_id text NOT NULL,
		execution_id text NOT NULL,
		node_id text NOT NULL,
		node_type text NOT NULL,
		payload text,
		metadata text,
		pass boolean NOT NULL,
		error text,
		recorded_at timestamp NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create replay record table: %v", err)
	}
	global.DB = db
	t.Cleanup(func() {
		global.DB = oldDB
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func ruleChainGoldenSeedChain(t *testing.T, db *gorm.DB) {
	t.Helper()
	chain := &model.RuleChain{
		ID:        ruleChainGoldenChainID,
		TenantID:  ruleChainGoldenTenantID,
		Name:      "golden chain",
		Enabled:   true,
		Graph:     []byte(`{"nodes":[{"id":"t1","type":"trigger.telemetry"}]}`),
		CreatedAt: func() *time.Time { v := ruleChainGoldenFixedTime(0); return &v }(),
		UpdatedAt: func() *time.Time { v := ruleChainGoldenFixedTime(0); return &v }(),
	}
	if err := db.Create(chain).Error; err != nil {
		t.Fatalf("seed rule chain: %v", err)
	}
}

func ruleChainGoldenSeedSecondChain(t *testing.T, db *gorm.DB) {
	t.Helper()
	chain := &model.RuleChain{
		ID:        "rc-golden-chain-2",
		TenantID:  ruleChainGoldenTenantID,
		Name:      "another chain",
		Enabled:   false,
		Graph:     []byte(`{"nodes":[{"id":"t2","type":"trigger.telemetry"}]}`),
		CreatedAt: func() *time.Time { v := ruleChainGoldenFixedTime(1); return &v }(),
		UpdatedAt: func() *time.Time { v := ruleChainGoldenFixedTime(1); return &v }(),
	}
	if err := db.Create(chain).Error; err != nil {
		t.Fatalf("seed second rule chain: %v", err)
	}
}

func ruleChainGoldenSeedTraces(t *testing.T, db *gorm.DB) {
	t.Helper()
	errorMsg := "boom"
	rows := []*model.RuleChainNodeTrace{
		{ID: "trace-1", ExecID: ruleChainGoldenExecID, ChainID: ruleChainGoldenChainID, NodeID: ruleChainGoldenNodeID, NodeType: "trigger.telemetry", Pass: true, ElapsedMs: 3, TenantID: ruleChainGoldenTenantID, CreatedAt: ruleChainGoldenFixedTime(1)},
		{ID: "trace-2", ExecID: ruleChainGoldenExecID, ChainID: ruleChainGoldenChainID, NodeID: "n2", NodeType: "action.command", Pass: false, ErrorMsg: &errorMsg, ElapsedMs: 9, TenantID: ruleChainGoldenTenantID, CreatedAt: ruleChainGoldenFixedTime(2)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed node traces: %v", err)
	}
}

func ruleChainGoldenSeedDeadLetters(t *testing.T, db *gorm.DB) {
	t.Helper()
	rows := []*model.RuleChainDeadLetter{
		{ID: "dead-1", TenantID: ruleChainGoldenTenantID, ChainID: ruleChainGoldenChainID, ExecID: ruleChainGoldenExecID, NodeID: "n2", NodeType: "action.command", Attempts: 1, CreatedAt: ruleChainGoldenFixedTime(1)},
		{ID: "dead-2", TenantID: ruleChainGoldenTenantID, ChainID: ruleChainGoldenChainID, ExecID: ruleChainGoldenExecID, NodeID: "n2", NodeType: "action.command", Attempts: 2, CreatedAt: ruleChainGoldenFixedTime(2)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed dead letters: %v", err)
	}
}

// ruleChainGoldenRender 为一侧（legacy 或 adapted）重建独立数据库、注册路由并发送请求。
// claims 固定为 TENANT_USER：使 ListChains 走 self-only 作用域，不依赖租户层级表。
func ruleChainGoldenRender(t *testing.T, tc ruleChainGoldenCase, side string, handler gin.HandlerFunc) (int, string) {
	t.Helper()
	ruleChainGoldenDB(t, side)
	if tc.seed != nil {
		tc.seed(t, global.DB)
	}
	gin.SetMode(gin.TestMode)
	responseHandler := &apiresponse.Handler{ErrManager: errcode.NewErrorManager("", "")}
	router := gin.New()
	router.Use(responseHandler.Middleware())
	router.Use(func(c *gin.Context) {
		c.Set(claimsContextKey, &utils.UserClaims{
			ID:        "rc-golden-user",
			TenantID:  ruleChainGoldenTenantID,
			Authority: authz.TenantUser,
		})
		c.Next()
	})
	tc.register(router, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String()
}

var (
	ruleChainGoldenUUIDPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	// 时间戳有两种渲染形态：种子值（UTC，尾缀 Z）与 gorm 自动刷新的 updated_at
	//（sqlite 驱动按本地时区带 +08:00 偏移输出），掩码必须同时覆盖两者，
	// 否则 legacy 与 adapted 两次渲染间的墙钟抖动会被误判为包络差异。
	ruleChainGoldenTimePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})`)
)

// maskRuleChainGoldenVolatile 掩掉服务端生成值（uuid.New / time.Now），
// 仅用于响应体里必然含生成 ID/时间戳的成功用例；掩码之外的任何差异都会导致测试失败。
func maskRuleChainGoldenVolatile(body string) string {
	body = ruleChainGoldenUUIDPattern.ReplaceAllString(body, "<uuid>")
	body = ruleChainGoldenTimePattern.ReplaceAllString(body, "<ts>")
	return body
}

// ruleChainGoldenCase 一个 golden 对比用例：同一请求分别打到 legacy 与 adapted。
type ruleChainGoldenCase struct {
	name     string
	method   string
	target   string
	body     string
	register func(*gin.Engine, gin.HandlerFunc)
	seed     func(*testing.T, *gorm.DB)
	legacy   gin.HandlerFunc
	adapted  gin.HandlerFunc
	mask     bool
}

func (tc ruleChainGoldenCase) run(t *testing.T) {
	t.Helper()
	legacyCode, legacyBody := ruleChainGoldenRender(t, tc, "legacy", tc.legacy)
	adaptedCode, adaptedBody := ruleChainGoldenRender(t, tc, "adapted", tc.adapted)
	if tc.mask {
		legacyBody = maskRuleChainGoldenVolatile(legacyBody)
		adaptedBody = maskRuleChainGoldenVolatile(adaptedBody)
	}
	if legacyCode != adaptedCode {
		t.Fatalf("%s: HTTP status legacy=%d adapted=%d", tc.name, legacyCode, adaptedCode)
	}
	if legacyBody != adaptedBody {
		t.Fatalf("%s: response body mismatch\nlegacy  = %s\nadapted = %s", tc.name, legacyBody, adaptedBody)
	}
	if legacyBody == "" {
		t.Fatalf("%s: legacy response body is empty, comparison is meaningless", tc.name)
	}
}

func TestRuleChainGolden1ByteIdentical(t *testing.T) {
	cases := []ruleChainGoldenCase{
		{
			name:   "create/empty-body",
			method: http.MethodPost, target: "/rc", body: "",
			register: func(r *gin.Engine, h gin.HandlerFunc) { r.POST("/rc", h) },
			legacy:   legacyRuleChainCreate,
			adapted:  (&RuleChainApi{}).HandleCreateRuleChain,
		},
		{
			name:   "create/bad-json",
			method: http.MethodPost, target: "/rc", body: `not-json`,
			register: func(r *gin.Engine, h gin.HandlerFunc) { r.POST("/rc", h) },
			legacy:   legacyRuleChainCreate,
			adapted:  (&RuleChainApi{}).HandleCreateRuleChain,
		},
		{
			name:   "create/missing-name",
			method: http.MethodPost, target: "/rc", body: `{"graph":{"nodes":[]}}`,
			register: func(r *gin.Engine, h gin.HandlerFunc) { r.POST("/rc", h) },
			legacy:   legacyRuleChainCreate,
			adapted:  (&RuleChainApi{}).HandleCreateRuleChain,
		},
		{
			// 成功响应含 uuid.New 与 time.Now 生成值，掩码后比对；种子之外的语义必须逐字节一致。
			name:   "create/success",
			method: http.MethodPost, target: "/rc", body: ruleChainGoldenValidBody,
			register: func(r *gin.Engine, h gin.HandlerFunc) { r.POST("/rc", h) },
			legacy:   legacyRuleChainCreate,
			adapted:  (&RuleChainApi{}).HandleCreateRuleChain,
			mask:     true,
		},
		{
			name:   "update/empty-body",
			method: http.MethodPut, target: "/rc", body: "",
			register: func(r *gin.Engine, h gin.HandlerFunc) { r.PUT("/rc", h) },
			legacy:   legacyRuleChainUpdate,
			adapted:  (&RuleChainApi{}).HandleUpdateRuleChain,
		},
		{
			name:   "update/bad-json",
			method: http.MethodPut, target: "/rc", body: `{`,
			register: func(r *gin.Engine, h gin.HandlerFunc) { r.PUT("/rc", h) },
			legacy:   legacyRuleChainUpdate,
			adapted:  (&RuleChainApi{}).HandleUpdateRuleChain,
		},
		{
			name:   "update/success",
			method: http.MethodPut, target: "/rc",
			body: `{"id":"rc-golden-chain-1","name":"renamed","description":"new desc","enabled":false,` +
				`"graph":{"nodes":[{"id":"t1","type":"trigger.telemetry"}]}}`,
			register: func(r *gin.Engine, h gin.HandlerFunc) { r.PUT("/rc", h) },
			seed: func(t *testing.T, db *gorm.DB) {
				ruleChainGoldenSeedChain(t, db)
			},
			legacy:  legacyRuleChainUpdate,
			adapted: (&RuleChainApi{}).HandleUpdateRuleChain,
			mask:    true, // gorm Updates 自动刷新 updated_at
		},
		{
			name:   "delete/missing-id",
			method: http.MethodDelete, target: "/rc-bare",
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.DELETE("/rc/:id", h)
				r.DELETE("/rc-bare", h) // 无参静态路由：覆盖 c.Param("id") 缺失分支
			},
			legacy:  legacyRuleChainDelete,
			adapted: (&RuleChainApi{}).HandleDeleteRuleChain,
		},
		{
			// 成功包络 data 为空对象 {}：证明迁移保留了 c.Set("data", map[string]interface{}{}) 的字节输出
			//（若误用 HandlePathAction，data=nil 会被 omitempty 省略）。
			name:   "delete/success",
			method: http.MethodDelete, target: "/rc/" + ruleChainGoldenChainID,
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.DELETE("/rc/:id", h)
				r.DELETE("/rc-bare", h)
			},
			seed: func(t *testing.T, db *gorm.DB) {
				ruleChainGoldenSeedChain(t, db)
			},
			legacy:  legacyRuleChainDelete,
			adapted: (&RuleChainApi{}).HandleDeleteRuleChain,
		},
		{
			name:   "delete/not-found",
			method: http.MethodDelete, target: "/rc/" + ruleChainGoldenChainID,
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.DELETE("/rc/:id", h)
				r.DELETE("/rc-bare", h)
			},
			legacy:  legacyRuleChainDelete,
			adapted: (&RuleChainApi{}).HandleDeleteRuleChain,
		},
		{
			name:   "get/missing-id",
			method: http.MethodGet, target: "/rc-bare",
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rc/:id", h)
				r.GET("/rc-bare", h)
			},
			legacy:  legacyRuleChainGet,
			adapted: (&RuleChainApi{}).HandleGetRuleChain,
		},
		{
			name:   "get/not-found",
			method: http.MethodGet, target: "/rc/" + ruleChainGoldenChainID,
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rc/:id", h)
				r.GET("/rc-bare", h)
			},
			legacy:  legacyRuleChainGet,
			adapted: (&RuleChainApi{}).HandleGetRuleChain,
		},
		{
			name:   "get/success",
			method: http.MethodGet, target: "/rc/" + ruleChainGoldenChainID,
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rc/:id", h)
				r.GET("/rc-bare", h)
			},
			seed: func(t *testing.T, db *gorm.DB) {
				ruleChainGoldenSeedChain(t, db)
			},
			legacy:  legacyRuleChainGet,
			adapted: (&RuleChainApi{}).HandleGetRuleChain,
		},
		{
			name:   "list/bad-page-size",
			method: http.MethodGet, target: "/rcl/list?page=1&page_size=300", // 超过 binding max=200
			register: func(r *gin.Engine, h gin.HandlerFunc) { r.GET("/rcl/list", h) },
			legacy:  legacyRuleChainList,
			adapted: (&RuleChainApi{}).HandleListRuleChains,
		},
		{
			name:   "list/success",
			method: http.MethodGet, target: "/rcl/list?keyword=chain&page=1&page_size=20",
			register: func(r *gin.Engine, h gin.HandlerFunc) { r.GET("/rcl/list", h) },
			seed: func(t *testing.T, db *gorm.DB) {
				ruleChainGoldenSeedChain(t, db)
				ruleChainGoldenSeedSecondChain(t, db)
			},
			legacy:  legacyRuleChainList,
			adapted: (&RuleChainApi{}).HandleListRuleChains,
		},
		{
			name:   "node-traces/missing-params",
			method: http.MethodGet, target: "/rct-bare/traces",
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rct/:id/nodes/:nodeId/traces", h)
				r.GET("/rct-bare/traces", h)
			},
			legacy:  legacyRuleChainNodeTraces,
			adapted: (&RuleChainApi{}).HandleGetRuleChainNodeTraces,
		},
		{
			name:   "node-traces/success",
			method: http.MethodGet, target: "/rct/" + ruleChainGoldenChainID + "/nodes/" + ruleChainGoldenNodeID + "/traces",
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rct/:id/nodes/:nodeId/traces", h)
				r.GET("/rct-bare/traces", h)
			},
			seed: func(t *testing.T, db *gorm.DB) {
				ruleChainGoldenSeedChain(t, db)
				ruleChainGoldenSeedTraces(t, db)
			},
			legacy:  legacyRuleChainNodeTraces,
			adapted: (&RuleChainApi{}).HandleGetRuleChainNodeTraces,
		},
		{
			// limit 非法 → Atoi 失败置 0 → service 钳制回默认 10：与上一用例响应体应完全一致（两种实现都是）。
			name:   "node-traces/limit-invalid-fallback",
			method: http.MethodGet, target: "/rct/" + ruleChainGoldenChainID + "/nodes/" + ruleChainGoldenNodeID + "/traces?limit=abc",
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rct/:id/nodes/:nodeId/traces", h)
				r.GET("/rct-bare/traces", h)
			},
			seed: func(t *testing.T, db *gorm.DB) {
				ruleChainGoldenSeedChain(t, db)
				ruleChainGoldenSeedTraces(t, db)
			},
			legacy:  legacyRuleChainNodeTraces,
			adapted: (&RuleChainApi{}).HandleGetRuleChainNodeTraces,
		},
		{
			name:   "dead-letters/success",
			method: http.MethodGet, target: "/rcl/" + ruleChainGoldenChainID + "/dead-letters?exec_id=" + ruleChainGoldenExecID,
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rcl/:id/dead-letters", h)
				r.GET("/rcl-bare/dead-letters", h)
			},
			seed: func(t *testing.T, db *gorm.DB) {
				ruleChainGoldenSeedChain(t, db)
				ruleChainGoldenSeedDeadLetters(t, db)
			},
			legacy:  legacyRuleChainDeadLetters,
			adapted: (&RuleChainApi{}).HandleListRuleChainDeadLetters,
		},
		{
			// 无 :id 路由：chain_id 从 query 回退；page/page_size 缺省走 DefaultQuery("1"/"20")。
			name:   "dead-letters/chain-id-query-fallback",
			method: http.MethodGet, target: "/rcl-bare/dead-letters?chain_id=" + ruleChainGoldenChainID + "&exec_id=" + ruleChainGoldenExecID,
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rcl/:id/dead-letters", h)
				r.GET("/rcl-bare/dead-letters", h)
			},
			seed: func(t *testing.T, db *gorm.DB) {
				ruleChainGoldenSeedChain(t, db)
				ruleChainGoldenSeedDeadLetters(t, db)
			},
			legacy:  legacyRuleChainDeadLetters,
			adapted: (&RuleChainApi{}).HandleListRuleChainDeadLetters,
		},
		{
			// execId 与 exec_id 均为空：service 层 "execId is required" 业务参数错误。
			name:   "exec-traces/missing-exec-id",
			method: http.MethodGet, target: "/rce-bare/exec/traces",
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rce/:id/executions/:execId/traces", h)
				r.GET("/rce-bare/exec/traces", h)
			},
			legacy:  legacyRuleChainExecutionTraces,
			adapted: (&RuleChainApi{}).HandleGetRuleChainExecutionTraces,
		},
		{
			name:   "exec-traces/success",
			method: http.MethodGet, target: "/rce/" + ruleChainGoldenChainID + "/executions/" + ruleChainGoldenExecID + "/traces",
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rce/:id/executions/:execId/traces", h)
				r.GET("/rce-bare/exec/traces", h)
			},
			seed: func(t *testing.T, db *gorm.DB) {
				ruleChainGoldenSeedChain(t, db)
				ruleChainGoldenSeedTraces(t, db)
			},
			legacy:  legacyRuleChainExecutionTraces,
			adapted: (&RuleChainApi{}).HandleGetRuleChainExecutionTraces,
		},
		{
			name:   "replay/missing-chain-id",
			method: http.MethodPost, target: "/rcr-bare/replay", body: `{"execution_id":"e1"}`,
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.POST("/rcr/:id/replay", h)
				r.POST("/rcr-bare/replay", h)
			},
			legacy:  legacyRuleChainReplay,
			adapted: (&RuleChainApi{}).HandleReplayRuleChainExecution,
		},
		{
			// body 缺 execution_id：BindAndValidate 的 binding required 错误（路径检查先于绑定的顺序两侧一致）。
			name:   "replay/missing-execution-id",
			method: http.MethodPost, target: "/rcr/" + ruleChainGoldenChainID + "/replay", body: `{}`,
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.POST("/rcr/:id/replay", h)
				r.POST("/rcr-bare/replay", h)
			},
			seed: func(t *testing.T, db *gorm.DB) {
				ruleChainGoldenSeedChain(t, db)
			},
			legacy:  legacyRuleChainReplay,
			adapted: (&RuleChainApi{}).HandleReplayRuleChainExecution,
		},
		{
			// 链存在但回放快照为空：service 层 "no replay records found" 业务分支（覆盖到 service 内部而非仅参数层）。
			name:   "replay/no-replay-records",
			method: http.MethodPost, target: "/rcr/" + ruleChainGoldenChainID + "/replay",
			body:   `{"execution_id":"` + ruleChainGoldenExecID + `"}`,
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.POST("/rcr/:id/replay", h)
				r.POST("/rcr-bare/replay", h)
			},
			seed: func(t *testing.T, db *gorm.DB) {
				ruleChainGoldenSeedChain(t, db)
			},
			legacy:  legacyRuleChainReplay,
			adapted: (&RuleChainApi{}).HandleReplayRuleChainExecution,
		},
		{
			name:   "resolve/missing-device-id",
			method: http.MethodGet, target: "/rcd-bare/device-effective",
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rcd/device-effective/:deviceId", h)
				r.GET("/rcd-bare/device-effective", h)
			},
			legacy:  legacyRuleChainResolve,
			adapted: (&RuleChainApi{}).HandleResolveDeviceEffectiveRuleChains,
		},
		{
			// 设备不存在：service 层 not-found 业务分支（GetDeviceByIDUnscoped 查无此行）。
			name:   "resolve/device-not-found",
			method: http.MethodGet, target: "/rcd/device-effective/no-such-device",
			register: func(r *gin.Engine, h gin.HandlerFunc) {
				r.GET("/rcd/device-effective/:deviceId", h)
				r.GET("/rcd-bare/device-effective", h)
			},
			legacy:  legacyRuleChainResolve,
			adapted: (&RuleChainApi{}).HandleResolveDeviceEffectiveRuleChains,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t)
		})
	}
}

// TestRuleChainGolden1MissingClaimsBehavior 固化迁移的有意行为差异（不参与逐字节对比）：
// 路由漏配鉴权中间件时，旧写法 MustGet panic 被响应中间件 recover 成 CodeSystemError，
// 新写法 RequireClaims 直接返回标准 CodeUnauthorized（handler_adapter.go 头注声明的差异）。
func TestRuleChainGolden1MissingClaimsBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responseHandler := &apiresponse.Handler{ErrManager: errcode.NewErrorManager("", "")}

	newRouter := func(handler gin.HandlerFunc) *gin.Engine {
		router := gin.New()
		router.Use(responseHandler.Middleware())
		router.GET("/rc/:id", handler)
		return router
	}

	legacyCode, legacyBody := ruleChainGoldenServe(t, newRouter(legacyRuleChainGet), http.MethodGet, "/rc/"+ruleChainGoldenChainID)
	var legacyPayload apiresponse.Response
	if err := json.Unmarshal([]byte(legacyBody), &legacyPayload); err != nil {
		t.Fatalf("decode legacy body %q: %v", legacyBody, err)
	}
	if legacyCode != http.StatusOK || legacyPayload.Code != errcode.CodeSystemError {
		t.Fatalf("legacy without claims: status=%d code=%d body=%s, want 200/CodeSystemError (recovered MustGet panic)",
			legacyCode, legacyPayload.Code, legacyBody)
	}

	adaptedCode, adaptedBody := ruleChainGoldenServe(t, newRouter((&RuleChainApi{}).HandleGetRuleChain), http.MethodGet, "/rc/"+ruleChainGoldenChainID)
	var adaptedPayload apiresponse.Response
	if err := json.Unmarshal([]byte(adaptedBody), &adaptedPayload); err != nil {
		t.Fatalf("decode adapted body %q: %v", adaptedBody, err)
	}
	if adaptedCode != http.StatusOK || adaptedPayload.Code != errcode.CodeUnauthorized {
		t.Fatalf("adapted without claims: status=%d code=%d body=%s, want 200/CodeUnauthorized",
			adaptedCode, adaptedPayload.Code, adaptedBody)
	}
}

func ruleChainGoldenServe(t *testing.T, router *gin.Engine, method, target string) (int, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String()
}
