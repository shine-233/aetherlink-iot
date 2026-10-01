// 文件用途：device_templates.go 适配器迁移（golden #4）的逐字节对比测试。
// 核心逻辑：对迁移到 handler_adapter 骨架的 8 个 handler 形态，各自按「迁移前手写形态」（BindAndValidate /
// MustGet("claims") / c.Error / c.Set 的旧样板）和「迁移后适配器形态」（Handle / HandlePath / HandleNoBody /
// HandlePublic / HandlePathAction + respond / respondAction）实现一遍，走同一套 response 中间件渲染同一请求，然后全等比较 HTTP 状态码与原始响应体。
// service 调用用本地桩替代（service.GroupApp.DeviceTemplate 为具体结构体，无法注入假实现），桩按迁移代码里的
// 分支覆盖：成功、业务错误、SerializeData 失败、参数缺失、市场客户端失败。Import 的审计副作用额外比对发射顺序与实参。
// 有意的差异（不在对比范围）：缺失 claims 时旧写法 MustGet panic 被响应中间件兜底成 CodeSystemError，
// 新写法返回标准 CodeUnauthorized，由 handler_adapter_test.go 的共享测试统一固化。
// 命名约定：本文件所有符号带 dtg4 前缀（device templates golden #4），不与共享测试或其他迁移方的符号重名。
package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	apiresponse "aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ---- 探针请求体与序列化目标 ----

// dtg4ListReq 模拟 GetDeviceTemplateListByPageReq 的最小 query 绑定面（GET 走 ShouldBindQuery）。
type dtg4ListReq struct {
	Keyword string `json:"keyword" form:"keyword"`
	Page    int    `json:"page" form:"page"`
}

// dtg4SerializeTarget 模拟 GetDeviceTemplateListData / DeviceTemplateReadSchema 的读取视图：
// ID 为 string，桩返回数字即可稳定触发 SerializeData 的反序列化失败分支。
type dtg4SerializeTarget struct {
	ID string `json:"id"`
}

// dtg4MarketLoginReq 模拟 MarketLoginReq 的 JSON 绑定面。
type dtg4MarketLoginReq struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

// dtg4MarketListReq 模拟 MarketTemplateListReq 的指针可选字段绑定面。
type dtg4MarketListReq struct {
	Keyword *string `json:"keyword" form:"keyword"`
	Page    int     `json:"page" form:"page"`
}

// dtg4ImportReq 模拟 ImportDeviceTemplateReq 的 JSON 绑定面。
type dtg4ImportReq struct {
	Name string `json:"name" form:"name"`
}

// ---- 桩 service：行为面与被迁移的 service 调用一致 ----

// dtg4BoomKeyword 触发桩的业务错误分支。
const dtg4BoomKeyword = "boom"

func dtg4ListService(req dtg4ListReq, claims *utils.UserClaims) (interface{}, error) {
	if req.Keyword == dtg4BoomKeyword {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	if req.Keyword == "badshape" {
		// 数字写不进 string 字段：触发 utils.SerializeData 的 Unmarshal 失败分支。
		return map[string]interface{}{"id": 7}, nil
	}
	return map[string]interface{}{"id": "tpl-1", "keyword": req.Keyword, "tenant": claims.TenantID}, nil
}

func dtg4DetailService(id string, claims *utils.UserClaims) (interface{}, error) {
	if id == dtg4BoomKeyword {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	if id == "badshape" {
		return map[string]interface{}{"id": 7}, nil
	}
	return map[string]interface{}{"id": id, "tenant": claims.TenantID}, nil
}

func dtg4MarketLoginService(_ *gin.Context, username, password string) (string, error) {
	if username == dtg4BoomKeyword {
		return "", errcode.New(errcode.CodeSystemError)
	}
	return "token-for-" + username + "-" + password, nil
}

func dtg4MarketListService(_ *gin.Context, req dtg4MarketListReq) (interface{}, error) {
	if req.Keyword != nil && *req.Keyword == dtg4BoomKeyword {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "Failed to list market thing models: " + dtg4BoomKeyword,
		})
	}
	return map[string]interface{}{"page": req.Page}, nil
}

func dtg4MarketDetailService(_ *gin.Context, marketID string) (interface{}, error) {
	if marketID == dtg4BoomKeyword {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return map[string]interface{}{"market_id": marketID}, nil
}

func dtg4ImportService(req dtg4ImportReq, claims *utils.UserClaims) (interface{}, bool, error) {
	if req.Name == dtg4BoomKeyword {
		return nil, false, errcode.New(errcode.CodeSystemError)
	}
	created := req.Name == "fresh"
	return map[string]interface{}{"name": req.Name, "tenant": claims.TenantID}, created, nil
}

func dtg4DeleteService(id string, claims *utils.UserClaims) error {
	if id == dtg4BoomKeyword {
		return errcode.New(errcode.CodeSystemError)
	}
	return nil
}

// ---- 审计副作用桩（ImportDeviceTemplate 形态）----

// dtg4ImportAudit 记录一次审计发射的实参，用于比对迁移前后审计顺序与内容一致。
type dtg4ImportAudit struct {
	tenantID string
	actorID  string
	created  bool
	hasErr   bool
}

var dtg4ImportAuditLog []dtg4ImportAudit

func dtg4EmitImportAudit(claims *utils.UserClaims, created bool, importErr error) {
	dtg4ImportAuditLog = append(dtg4ImportAuditLog, dtg4ImportAudit{
		tenantID: claims.TenantID,
		actorID:  claims.ID,
		created:  created,
		hasErr:   importErr != nil,
	})
}

// ---- legacy：迁移前手写形态（逐行转录自迁移前的 device_templates.go，仅把 service 调用换成桩）----

func dtg4LegacyListByPage(c *gin.Context) {
	var req dtg4ListReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := dtg4ListService(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	serilizedData, err := utils.SerializeData(data, dtg4SerializeTarget{})
	if err != nil {
		c.Error(errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	c.Set("data", serilizedData)
}

func dtg4LegacyTemplateById(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := dtg4DetailService(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	serilizedData, err := utils.SerializeData(data, dtg4SerializeTarget{})
	if err != nil {
		c.Error(errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}
	c.Set("data", serilizedData)
}

func dtg4LegacyTemplateByDeviceId(c *gin.Context) {
	deviceId := c.Query("device_id")
	if deviceId == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"device_id": deviceId,
			"msg":       "device_id is required",
		}))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := dtg4DetailService(deviceId, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func dtg4LegacyMarketLogin(c *gin.Context) {
	var req dtg4MarketLoginReq
	if !BindAndValidate(c, &req) {
		return
	}

	token, err := dtg4MarketLoginService(c, req.Username, req.Password)
	if err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeSystemError, err.Error()))
		return
	}

	c.Set("data", map[string]string{
		"token": token,
	})
}

func dtg4LegacyMarketList(c *gin.Context) {
	var req dtg4MarketListReq
	if !BindAndValidate(c, &req) {
		return
	}

	data, err := dtg4MarketListService(c, req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func dtg4LegacyMarketDetail(c *gin.Context) {
	marketID := c.Param("market_id")
	if marketID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "market_id is required"))
		return
	}

	data, err := dtg4MarketDetailService(c, marketID)
	if err != nil {
		c.Error(errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "Failed to get market thing model detail: " + err.Error(),
		}))
		return
	}
	c.Set("data", data)
}

func dtg4LegacyImport(c *gin.Context) {
	var req dtg4ImportReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, created, err := dtg4ImportService(req, userClaims)
	if err != nil {
		dtg4EmitImportAudit(userClaims, false, err)
		c.Error(err)
		return
	}
	dtg4EmitImportAudit(userClaims, created, nil)
	c.Set("data", gin.H{
		"template": data,
		"created":  created,
	})
}

func dtg4LegacyDelete(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := dtg4DeleteService(id, userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// ---- adapted：迁移后适配器形态（与 device_templates.go 现状同构）----

func dtg4AdaptedListByPage(c *gin.Context) {
	Handle(c, func(req *dtg4ListReq, userClaims *utils.UserClaims) (interface{}, error) {
		data, err := dtg4ListService(*req, userClaims)
		if err != nil {
			return nil, err
		}
		serilizedData, err := utils.SerializeData(data, dtg4SerializeTarget{})
		if err != nil {
			return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
				"error": err.Error(),
			})
		}
		return serilizedData, nil
	})
}

func dtg4AdaptedTemplateById(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		data, err := dtg4DetailService(id, userClaims)
		if err != nil {
			return nil, err
		}
		serilizedData, err := utils.SerializeData(data, dtg4SerializeTarget{})
		if err != nil {
			return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
				"error": err.Error(),
			})
		}
		return serilizedData, nil
	})
}

func dtg4AdaptedTemplateByDeviceId(c *gin.Context) {
	HandleNoBody(c, func(userClaims *utils.UserClaims) (interface{}, error) {
		deviceId := c.Query("device_id")
		if deviceId == "" {
			return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"device_id": deviceId,
				"msg":       "device_id is required",
			})
		}
		data, err := dtg4DetailService(deviceId, userClaims)
		if err != nil {
			return nil, err
		}
		return data, nil
	})
}

func dtg4AdaptedMarketLogin(c *gin.Context) {
	HandlePublic(c, func(req *dtg4MarketLoginReq) (interface{}, error) {
		token, err := dtg4MarketLoginService(c, req.Username, req.Password)
		if err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
		}
		return map[string]string{
			"token": token,
		}, nil
	})
}

func dtg4AdaptedMarketList(c *gin.Context) {
	HandlePublic(c, func(req *dtg4MarketListReq) (interface{}, error) {
		return dtg4MarketListService(c, *req)
	})
}

func dtg4AdaptedMarketDetail(c *gin.Context) {
	HandlePath(c, "market_id", func(marketID string, _ *utils.UserClaims) (interface{}, error) {
		if marketID == "" {
			return nil, errcode.WithData(errcode.CodeParamError, "market_id is required")
		}

		data, err := dtg4MarketDetailService(c, marketID)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
				"error": "Failed to get market thing model detail: " + err.Error(),
			})
		}
		return data, nil
	})
}

func dtg4AdaptedImport(c *gin.Context) {
	Handle(c, func(req *dtg4ImportReq, userClaims *utils.UserClaims) (interface{}, error) {
		data, created, err := dtg4ImportService(*req, userClaims)
		if err != nil {
			dtg4EmitImportAudit(userClaims, false, err)
			return nil, err
		}
		dtg4EmitImportAudit(userClaims, created, nil)
		return gin.H{
			"template": data,
			"created":  created,
		}, nil
	})
}

func dtg4AdaptedDelete(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		return dtg4DeleteService(id, userClaims)
	})
}

// ---- 渲染与断言 ----

// dtg4Render 用与线上一致的响应中间件渲染一次请求，返回状态码与原始响应体。
func dtg4Render(t *testing.T, method, target, body string, withClaims bool, handler gin.HandlerFunc) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	responseHandler := &apiresponse.Handler{
		ErrManager: errcode.NewErrorManager("", ""),
	}
	router := gin.New()
	router.Use(responseHandler.Middleware())
	if withClaims {
		router.Use(func(c *gin.Context) {
			c.Set(claimsContextKey, &utils.UserClaims{ID: "dtg4-user", TenantID: "dtg4-tenant"})
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

// dtg4RequireIdentical 断言新旧两种写法在同一入参下渲染出完全相同的响应。
func dtg4RequireIdentical(t *testing.T, name, method, target, body string, withClaims bool, legacy, adapted gin.HandlerFunc) {
	t.Helper()
	legacyCode, legacyBody := dtg4Render(t, method, target, body, withClaims, legacy)
	adaptedCode, adaptedBody := dtg4Render(t, method, target, body, withClaims, adapted)

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

// TestDTG4ListByPageShapeIsByteIdentical 覆盖 HandleDeviceTemplateListByPage 形态：
// GET query 绑定 -> claims -> service -> SerializeData -> 响应，含绑定失败/业务错误/序列化失败分支。
func TestDTG4ListByPageShapeIsByteIdentical(t *testing.T) {
	for _, tc := range []struct{ name, target string }{
		{"valid", "/dtg4/template?keyword=ok&page=2"},
		{"empty-query", "/dtg4/template"},
		{"service-error", "/dtg4/template?keyword=boom"},
		{"serialize-error", "/dtg4/template?keyword=badshape"},
		{"bind-error", "/dtg4/template?page=notanumber"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dtg4RequireIdentical(t, tc.name, http.MethodGet, tc.target, "", true, dtg4LegacyListByPage, dtg4AdaptedListByPage)
		})
	}
}

// TestDTG4TemplateByIdShapeIsByteIdentical 覆盖 HandleDeviceTemplateById 形态：
// 路径参数 -> claims -> service -> SerializeData -> 响应。
func TestDTG4TemplateByIdShapeIsByteIdentical(t *testing.T) {
	for _, tc := range []struct{ name, target string }{
		{"valid", "/dtg4/template/detail/tpl-9"},
		{"service-error", "/dtg4/template/detail/boom"},
		{"serialize-error", "/dtg4/template/detail/badshape"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dtg4RequireIdentical(t, tc.name, http.MethodGet, tc.target, "", true, dtg4LegacyTemplateById, dtg4AdaptedTemplateById)
		})
	}
}

// TestDTG4TemplateByDeviceIdShapeIsByteIdentical 覆盖 HandleDeviceTemplateByDeviceId 形态：
// 手工读取 query 参数 + 自定义参数缺失错误（保留原 errcode.WithData 载荷，不走 reportParamError）。
func TestDTG4TemplateByDeviceIdShapeIsByteIdentical(t *testing.T) {
	for _, tc := range []struct{ name, target string }{
		{"missing-device-id", "/dtg4/template/chart"},
		{"valid", "/dtg4/template/chart?device_id=dev-1"},
		{"service-error", "/dtg4/template/chart?device_id=boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dtg4RequireIdentical(t, tc.name, http.MethodGet, tc.target, "", true, dtg4LegacyTemplateByDeviceId, dtg4AdaptedTemplateByDeviceId)
		})
	}
}

// TestDTG4MarketLoginShapeIsByteIdentical 覆盖 MarketLogin 形态：
// POST JSON 绑定、无 claims、错误经 NewWithMessage 包装、成功返回 token map。
func TestDTG4MarketLoginShapeIsByteIdentical(t *testing.T) {
	for _, body := range []string{
		`{"username":"u","password":"p"}`,
		`{"username":"boom","password":"p"}`,
		`{"password":"p"}`,
		`not-json`,
	} {
		dtg4RequireIdentical(t, "MarketLogin/"+body, http.MethodPost, "/dtg4/market/login", body, true, dtg4LegacyMarketLogin, dtg4AdaptedMarketLogin)
	}
}

// TestDTG4MarketListShapeIsByteIdentical 覆盖 ListMarketTemplates 形态：
// GET query 绑定 + 指针可选字段 + 客户端错误经 WithData 包装。
func TestDTG4MarketListShapeIsByteIdentical(t *testing.T) {
	for _, tc := range []struct{ name, target string }{
		{"valid", "/dtg4/market/list?keyword=ok&page=3"},
		{"empty-query", "/dtg4/market/list"},
		{"service-error", "/dtg4/market/list?keyword=boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dtg4RequireIdentical(t, tc.name, http.MethodGet, tc.target, "", true, dtg4LegacyMarketList, dtg4AdaptedMarketList)
		})
	}
}

// TestDTG4MarketDetailShapeIsByteIdentical 覆盖 GetMarketTemplateDetail 形态：
// 路径参数缺失（自定义 CodeParamError 字符串载荷）与客户端错误（WithData 包装）两个分支。
func TestDTG4MarketDetailShapeIsByteIdentical(t *testing.T) {
	for _, tc := range []struct{ name, target string }{
		{"valid", "/dtg4/market/detail/mkt-1"},
		{"service-error", "/dtg4/market/detail/boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dtg4RequireIdentical(t, tc.name, http.MethodGet, tc.target, "", true, dtg4LegacyMarketDetail, dtg4AdaptedMarketDetail)
		})
	}

	// 空 market_id 分支：挂到不含该路径参数的路由，使 c.Param("market_id") 为空串，直接命中参数缺失检查。
	dtg4RequireIdentical(t, "empty-market-id", http.MethodGet, "/dtg4/market/detail", "", true, dtg4LegacyMarketDetail, dtg4AdaptedMarketDetail)
}

// TestDTG4ImportShapeIsByteIdenticalAndAuditOrderPreserved 覆盖 ImportDeviceTemplate 形态：
// JSON 包络逐字节一致，且失败/成功两条路径的审计发射顺序与实参一致（审计桩记录后做 DeepEqual）。
func TestDTG4ImportShapeIsByteIdenticalAndAuditOrderPreserved(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"created", `{"name":"fresh"}`},
		{"idempotent-hit", `{"name":"existing"}`},
		{"service-error", `{"name":"boom"}`},
		{"bind-error", `not-json`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dtg4ImportAuditLog = nil
			legacyCode, legacyBody := dtg4Render(t, http.MethodPost, "/dtg4/template/import", tc.body, true, dtg4LegacyImport)
			legacyAudit := append([]dtg4ImportAudit(nil), dtg4ImportAuditLog...)

			dtg4ImportAuditLog = nil
			adaptedCode, adaptedBody := dtg4Render(t, http.MethodPost, "/dtg4/template/import", tc.body, true, dtg4AdaptedImport)
			adaptedAudit := append([]dtg4ImportAudit(nil), dtg4ImportAuditLog...)

			if legacyCode != adaptedCode {
				t.Fatalf("%s: HTTP status legacy=%d adapted=%d", tc.name, legacyCode, adaptedCode)
			}
			if legacyBody != adaptedBody {
				t.Fatalf("%s: response body mismatch\nlegacy  = %s\nadapted = %s", tc.name, legacyBody, adaptedBody)
			}
			if legacyBody == "" {
				t.Fatalf("%s: legacy response body is empty, comparison is meaningless", tc.name)
			}
			if !reflect.DeepEqual(legacyAudit, adaptedAudit) {
				t.Fatalf("%s: audit trail mismatch\nlegacy  = %+v\nadapted = %+v", tc.name, legacyAudit, adaptedAudit)
			}
			// 非空校验：确保审计比对不是空洞成立——成功路径恰好发射一条 created 审计，失败路径恰好一条失败审计。
			switch tc.name {
			case "created":
				if len(adaptedAudit) != 1 || !adaptedAudit[0].created || adaptedAudit[0].hasErr {
					t.Fatalf("created: unexpected audit trail %+v", adaptedAudit)
				}
			case "service-error":
				if len(adaptedAudit) != 1 || adaptedAudit[0].created || !adaptedAudit[0].hasErr {
					t.Fatalf("service-error: unexpected audit trail %+v", adaptedAudit)
				}
			}
		})
	}
}

// TestDTG4DeleteShapeIsByteIdentical 覆盖 DeleteDeviceTemplate 形态：
// HandlePathAction（respondAction 出口）——成功时 c.Set("data", nil)，包络省略 data 字段，
// 与迁移前手写形态逐字节一致；失败路径经 c.Error 渲染同一错误包络。
func TestDTG4DeleteShapeIsByteIdentical(t *testing.T) {
	for _, tc := range []struct{ name, target string }{
		{"valid", "/dtg4/template/tpl-1"},
		{"service-error", "/dtg4/template/boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dtg4RequireIdentical(t, tc.name, http.MethodDelete, tc.target, "", true, dtg4LegacyDelete, dtg4AdaptedDelete)
		})
	}
}
