// 文件用途：device_config.go 迁移到 handler_adapter 骨架后的 golden 对比测试（编号 3，仅归属轨道2迁移员-3）。
// 核心逻辑：为 device_config.go 用到的每一种适配器形态（Handle / HandleAction / HandlePath / HandlePathAction /
// HandlePublic，以及闭包内读 query、读 Accept-Language、解引用 *req 等细节）各准备一份
// 「旧手写形态」（BindAndValidate + MustGet("claims") + c.Error / c.Set("data", ...)）与
// 「新适配器形态」的实现，使用文件里真实的 model 请求结构体和同一套 response 中间件渲染，
// 对 HTTP 状态码与原始响应体做逐字节全等比较，证明迁移前后 JSON 包络一致。
// 已知的有意差异（缺失 claims 时旧写法 panic 兜底成 CodeSystemError、新写法返回 CodeUnauthorized）
// 由 TestDeviceConfigGolden3MissingClaimsBehavior 单独固化，不参与逐字节比对。
package api

import (
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

// ---- golden 探针常量与桩 service（镜像真实 service 的入参形态，不依赖数据库） ----

const (
	// dc3BoomName 触发 create 探针返回业务错误。
	dc3BoomName = "dc3-boom"
	// dc3BoomID 触发 delete / batch 探针返回业务错误。
	dc3BoomID = "dc3-boom-id"
	// dc3ValidBatchUUID 满足 BatchUpdateDeviceConfigReq 的 uuid 校验。
	dc3ValidBatchUUID = "123e4567-e89b-12d3-a456-426614174000"
)

// dc3ProbeCreate 镜像 service.DeviceConfig.CreateDeviceConfig(req *model.CreateDeviceConfigReq, claims) 的形态。
// 回显 conflict_policy 的最终取值，用于证明 query 兜底逻辑在两种写法下表现一致。
func dc3ProbeCreate(req *model.CreateDeviceConfigReq, claims *utils.UserClaims) (interface{}, error) {
	if req.Name == dc3BoomName {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	policy := ""
	if req.ConflictPolicy != nil {
		policy = *req.ConflictPolicy
	}
	return map[string]interface{}{
		"name":            req.Name,
		"device_type":     req.DeviceType,
		"conflict_policy": policy,
		"tenant":          claims.TenantID,
	}, nil
}

// dc3ProbeUpdate 镜像 service.DeviceConfig.UpdateDeviceConfig(req model.UpdateDeviceConfigReq, claims) 的按值入参。
func dc3ProbeUpdate(req model.UpdateDeviceConfigReq, claims *utils.UserClaims) (interface{}, error) {
	return map[string]interface{}{"id": req.Id, "tenant": claims.TenantID}, nil
}

// dc3ProbeDelete 镜像 service.DeviceConfig.DeleteDeviceConfig(id string, claims) error 的形态。
func dc3ProbeDelete(id string, claims *utils.UserClaims) error {
	if id == dc3BoomID {
		return errcode.New(errcode.CodeSystemError)
	}
	return nil
}

// dc3ProbeGetByID 镜像 service.DeviceConfig.GetDeviceConfigByID(ctx, id, claims) 的形态；
// 第一参数接收 gin 上下文但不使用，仅保持与真实闭包一致「把 c 传进 service」的调用形状。
func dc3ProbeGetByID(_ *gin.Context, id string, claims *utils.UserClaims) (interface{}, error) {
	return map[string]interface{}{"id": id, "tenant": claims.TenantID}, nil
}

// dc3ProbeListByPage 镜像 service.DeviceConfig.GetDeviceConfigListByPage(req *model.GetDeviceConfigListByPageReq, claims)。
func dc3ProbeListByPage(req *model.GetDeviceConfigListByPageReq, claims *utils.UserClaims) (interface{}, error) {
	deviceType := ""
	if req.DeviceType != nil {
		deviceType = *req.DeviceType
	}
	return map[string]interface{}{
		"page":        req.Page,
		"page_size":   req.PageSize,
		"device_type": deviceType,
		"tenant":      claims.TenantID,
	}, nil
}

// dc3ProbeListMenu 镜像 service.DeviceConfig.GetDeviceConfigListMenu(req *model.GetDeviceConfigListMenuReq, claims)。
func dc3ProbeListMenu(req *model.GetDeviceConfigListMenuReq, claims *utils.UserClaims) (interface{}, error) {
	deviceType := ""
	if req.DeviceType != nil {
		deviceType = *req.DeviceType
	}
	return map[string]interface{}{"device_type": deviceType, "tenant": claims.TenantID}, nil
}

// dc3ProbeBatch 镜像 service.DeviceConfig.BatchUpdateDeviceConfig(req *model.BatchUpdateDeviceConfigReq, claims) error。
func dc3ProbeBatch(req *model.BatchUpdateDeviceConfigReq, claims *utils.UserClaims) error {
	if req.DeviceConfigID == dc3BoomID {
		return errcode.New(errcode.CodeSystemError)
	}
	return nil
}

// dc3ProbeConnect 镜像 service.DeviceConfig.GetDeviceConfigConnect(ctx, deviceID, lang, claims)；
// 回显 lang 用于证明两种写法读到的 Accept-Language 一致。
func dc3ProbeConnect(deviceID string, lang string, claims *utils.UserClaims) (interface{}, error) {
	if deviceID == dc3BoomID {
		return nil, errcode.New(errcode.CodeSystemError)
	}
	return map[string]interface{}{"device_id": deviceID, "lang": lang, "tenant": claims.TenantID}, nil
}

// dc3ProbeVoucherType 镜像 service.DeviceConfig.GetVoucherTypeForm(deviceType, protocolType, lang)。
func dc3ProbeVoucherType(deviceType string, protocolType string, lang string) (interface{}, error) {
	return map[string]interface{}{"device_type": deviceType, "protocol_type": protocolType, "lang": lang}, nil
}

// dc3ProbeActionMenu 镜像 service.DeviceConfig.GetActionByDeviceConfigID(deviceConfigID string, claims)；
// GetConditionByDeviceConfigID 与其形态一致，共享同一对探针。
func dc3ProbeActionMenu(deviceConfigID string, claims *utils.UserClaims) (interface{}, error) {
	return map[string]interface{}{"device_config_id": deviceConfigID, "tenant": claims.TenantID}, nil
}

// ---- 旧写法（迁移前手写形态，对照 handler_adapter.go 头注释与 handler_adapter_test.go 的 legacy 探针） ----

// legacyDC3Create 对应 device_config.go CreateDeviceConfig 的迁移前形态：
// 绑定后、调 service 前做 conflict_policy 的 query 兜底。
func legacyDC3Create(c *gin.Context) {
	var req model.CreateDeviceConfigReq
	if !BindAndValidate(c, &req) {
		return
	}
	if req.ConflictPolicy == nil || *req.ConflictPolicy == "" {
		if q := c.Query("conflict_policy"); q != "" {
			req.ConflictPolicy = &q
		}
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := dc3ProbeCreate(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// legacyDC3Update 对应 UpdateDeviceConfig 的迁移前形态：service 按值收参。
func legacyDC3Update(c *gin.Context) {
	var req model.UpdateDeviceConfigReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := dc3ProbeUpdate(req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// legacyDC3Delete 对应 DeleteDeviceConfig 的迁移前形态：路径参数 + 无返回数据。
func legacyDC3Delete(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := dc3ProbeDelete(c.Param("id"), claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// legacyDC3GetByID 对应 HandleDeviceConfigById 的迁移前形态：路径参数 + 返回数据。
func legacyDC3GetByID(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := dc3ProbeGetByID(c, c.Param("id"), claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// legacyDC3ListByPage 对应 HandleDeviceConfigListByPage 的迁移前形态：GET query 绑定。
func legacyDC3ListByPage(c *gin.Context) {
	var req model.GetDeviceConfigListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := dc3ProbeListByPage(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// legacyDC3ListMenu 对应 HandleDeviceConfigListMenu 的迁移前形态。
func legacyDC3ListMenu(c *gin.Context) {
	var req model.GetDeviceConfigListMenuReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := dc3ProbeListMenu(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// legacyDC3Batch 对应 BatchUpdateDeviceConfig 的迁移前形态：JSON 绑定 + 无返回数据。
func legacyDC3Batch(c *gin.Context) {
	var req model.BatchUpdateDeviceConfigReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := dc3ProbeBatch(&req, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// legacyDC3Connect 对应 HandleDeviceConfigConnect 的迁移前形态：query 绑定 + 闭包读语言头。
func legacyDC3Connect(c *gin.Context) {
	var req model.DeviceIDReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	lang := c.GetHeader("Accept-Language")
	data, err := dc3ProbeConnect(req.DeviceID, lang, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// legacyDC3VoucherType 对应 HandleVoucherType 的迁移前形态：匿名接口 + 闭包读语言头。
func legacyDC3VoucherType(c *gin.Context) {
	var req model.GetVoucherTypeReq
	if !BindAndValidate(c, &req) {
		return
	}
	lang := c.GetHeader("Accept-Language")
	data, err := dc3ProbeVoucherType(req.DeviceType, req.ProtocolType, lang)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// legacyDC3ActionMenu 对应 HandleActionByDeviceConfigID / HandleConditionByDeviceConfigID 的迁移前形态。
func legacyDC3ActionMenu(c *gin.Context) {
	var req model.GetActionByDeviceConfigIDReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := dc3ProbeActionMenu(req.DeviceConfigID, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ---- 新写法（与 device_config.go 当前实现逐字对应的适配器形态，仅把 service 换成探针） ----

func adaptedDC3Create(c *gin.Context) {
	Handle(c, func(req *model.CreateDeviceConfigReq, userClaims *utils.UserClaims) (interface{}, error) {
		if req.ConflictPolicy == nil || *req.ConflictPolicy == "" {
			if q := c.Query("conflict_policy"); q != "" {
				req.ConflictPolicy = &q
			}
		}
		return dc3ProbeCreate(req, userClaims)
	})
}

func adaptedDC3Update(c *gin.Context) {
	Handle(c, func(req *model.UpdateDeviceConfigReq, userClaims *utils.UserClaims) (interface{}, error) {
		return dc3ProbeUpdate(*req, userClaims)
	})
}

func adaptedDC3Delete(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		return dc3ProbeDelete(id, userClaims)
	})
}

func adaptedDC3GetByID(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		return dc3ProbeGetByID(c, id, userClaims)
	})
}

func adaptedDC3ListByPage(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceConfigListByPageReq, userClaims *utils.UserClaims) (interface{}, error) {
		return dc3ProbeListByPage(req, userClaims)
	})
}

func adaptedDC3ListMenu(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceConfigListMenuReq, userClaims *utils.UserClaims) (interface{}, error) {
		return dc3ProbeListMenu(req, userClaims)
	})
}

func adaptedDC3Batch(c *gin.Context) {
	HandleAction(c, func(req *model.BatchUpdateDeviceConfigReq, userClaims *utils.UserClaims) error {
		return dc3ProbeBatch(req, userClaims)
	})
}

func adaptedDC3Connect(c *gin.Context) {
	Handle(c, func(param *model.DeviceIDReq, userClaims *utils.UserClaims) (interface{}, error) {
		lang := c.GetHeader("Accept-Language")
		return dc3ProbeConnect(param.DeviceID, lang, userClaims)
	})
}

func adaptedDC3VoucherType(c *gin.Context) {
	HandlePublic(c, func(param *model.GetVoucherTypeReq) (interface{}, error) {
		lang := c.GetHeader("Accept-Language")
		return dc3ProbeVoucherType(param.DeviceType, param.ProtocolType, lang)
	})
}

func adaptedDC3ActionMenu(c *gin.Context) {
	Handle(c, func(param *model.GetActionByDeviceConfigIDReq, userClaims *utils.UserClaims) (interface{}, error) {
		return dc3ProbeActionMenu(param.DeviceConfigID, userClaims)
	})
}

// ---- 渲染与断言 ----

// dc3RenderGolden 用与线上一致的 response 中间件渲染一次请求，返回状态码与原始响应体。
// 独立命名（不复用 handler_adapter_test.go 的 renderProbe），避免与本文件归属约束产生耦合。
func dc3RenderGolden(t *testing.T, method, target, body string, withClaims bool, handler gin.HandlerFunc) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	responseHandler := &apiresponse.Handler{
		ErrManager: errcode.NewErrorManager("", ""),
	}
	router := gin.New()
	router.Use(responseHandler.Middleware())
	if withClaims {
		router.Use(func(c *gin.Context) {
			c.Set("claims", &utils.UserClaims{ID: "dc3-user", TenantID: "dc3-tenant"})
			c.Next()
		})
	}
	// gin 注册路由只认路径部分：把 query string 从注册路径剥离，仅保留在请求 target 上，
	// 否则带 query 的用例会双双 404，形成虚假的逐字节一致。
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

// dc3GoldenCase 一条 golden 比对用例；wantDataKey 说明成功包络是否应携带 data 字段，
// 用于防止「两边都空渲染」的虚假一致，并固化 respondAction 成功时 data 省略的语义。
type dc3GoldenCase struct {
	name        string
	method      string
	target      string
	body        string
	withClaims  bool
	legacy      gin.HandlerFunc
	adapted     gin.HandlerFunc
	wantDataKey bool
}

func dc3RunGoldenCase(t *testing.T, tc dc3GoldenCase) {
	t.Helper()
	legacyCode, legacyBody := dc3RenderGolden(t, tc.method, tc.target, tc.body, tc.withClaims, tc.legacy)
	adaptedCode, adaptedBody := dc3RenderGolden(t, tc.method, tc.target, tc.body, tc.withClaims, tc.adapted)

	if legacyCode != adaptedCode {
		t.Fatalf("%s: HTTP status legacy=%d adapted=%d", tc.name, legacyCode, adaptedCode)
	}
	if legacyBody != adaptedBody {
		t.Fatalf("%s: response body mismatch\nlegacy  = %s\nadapted = %s", tc.name, legacyBody, adaptedBody)
	}
	if legacyBody == "" {
		t.Fatalf("%s: legacy response body is empty, comparison is meaningless", tc.name)
	}
	hasDataKey := strings.Contains(adaptedBody, `"data":`)
	if hasDataKey != tc.wantDataKey {
		t.Fatalf("%s: adapted body data-key presence = %v, want %v (body=%s)", tc.name, hasDataKey, tc.wantDataKey, adaptedBody)
	}
}

// TestDeviceConfigGolden3AllHandlersByteIdentical 覆盖 device_config.go 全部 11 个 handler 对应的适配器形态：
// 成功、参数校验失败、业务错误三个分支逐一与迁移前手写形态做逐字节比对。
func TestDeviceConfigGolden3AllHandlersByteIdentical(t *testing.T) {
	cases := []dc3GoldenCase{
		// CreateDeviceConfig：Handle + JSON 绑定 + conflict_policy 的 query 兜底。
		{
			name: "create/success-query-fallback", method: http.MethodPost,
			target: "/api/v1/device_config?conflict_policy=rename", body: `{"name":"cfg","device_type":"1"}`,
			withClaims: true, legacy: legacyDC3Create, adapted: adaptedDC3Create, wantDataKey: true,
		},
		{
			name: "create/success-body-policy-wins", method: http.MethodPost,
			target: "/api/v1/device_config?conflict_policy=rename", body: `{"name":"cfg","device_type":"1","conflict_policy":"ignore"}`,
			withClaims: true, legacy: legacyDC3Create, adapted: adaptedDC3Create, wantDataKey: true,
		},
		{
			name: "create/param-error-missing-name", method: http.MethodPost,
			target: "/api/v1/device_config", body: `{"device_type":"1"}`,
			withClaims: true, legacy: legacyDC3Create, adapted: adaptedDC3Create, wantDataKey: false,
		},
		{
			name: "create/param-error-bad-device-type", method: http.MethodPost,
			target: "/api/v1/device_config", body: `{"name":"cfg","device_type":"9"}`,
			withClaims: true, legacy: legacyDC3Create, adapted: adaptedDC3Create, wantDataKey: false,
		},
		{
			name: "create/service-error", method: http.MethodPost,
			target: "/api/v1/device_config", body: `{"name":"dc3-boom","device_type":"1"}`,
			withClaims: true, legacy: legacyDC3Create, adapted: adaptedDC3Create, wantDataKey: false,
		},

		// UpdateDeviceConfig：Handle + JSON 绑定 + service 按值收参（*req 解引用）。
		{
			name: "update/success", method: http.MethodPut,
			target: "/api/v1/device_config", body: `{"id":"cfg-1","name":"new-name"}`,
			withClaims: true, legacy: legacyDC3Update, adapted: adaptedDC3Update, wantDataKey: true,
		},
		{
			name: "update/param-error-missing-id", method: http.MethodPut,
			target: "/api/v1/device_config", body: `{"name":"new-name"}`,
			withClaims: true, legacy: legacyDC3Update, adapted: adaptedDC3Update, wantDataKey: false,
		},

		// DeleteDeviceConfig：HandlePathAction，成功包络省略 data。
		{
			name: "delete/success", method: http.MethodDelete,
			target: "/api/v1/device_config/cfg-9",
			withClaims: true, legacy: legacyDC3Delete, adapted: adaptedDC3Delete, wantDataKey: false,
		},
		{
			name: "delete/service-error", method: http.MethodDelete,
			target: "/api/v1/device_config/dc3-boom-id",
			withClaims: true, legacy: legacyDC3Delete, adapted: adaptedDC3Delete, wantDataKey: false,
		},

		// HandleDeviceConfigById：HandlePath + 闭包捕获 c。
		{
			name: "get-by-id/success", method: http.MethodGet,
			target: "/api/v1/device_config/cfg-1",
			withClaims: true, legacy: legacyDC3GetByID, adapted: adaptedDC3GetByID, wantDataKey: true,
		},

		// HandleDeviceConfigListByPage：Handle + GET query 绑定（含内嵌 PageReq 的 required 校验）。
		{
			name: "list-by-page/success", method: http.MethodGet,
			target: "/api/v1/device_config?page=1&page_size=10&device_type=1",
			withClaims: true, legacy: legacyDC3ListByPage, adapted: adaptedDC3ListByPage, wantDataKey: true,
		},
		{
			name: "list-by-page/param-error-missing-page", method: http.MethodGet,
			target: "/api/v1/device_config?page_size=10",
			withClaims: true, legacy: legacyDC3ListByPage, adapted: adaptedDC3ListByPage, wantDataKey: false,
		},
		{
			name: "list-by-page/param-error-bad-device-type", method: http.MethodGet,
			target: "/api/v1/device_config?page=1&page_size=10&device_type=9",
			withClaims: true, legacy: legacyDC3ListByPage, adapted: adaptedDC3ListByPage, wantDataKey: false,
		},

		// HandleDeviceConfigListMenu：Handle + GET query 绑定，全可选字段。
		{
			name: "list-menu/success-empty-query", method: http.MethodGet,
			target: "/api/v1/device_config/menu",
			withClaims: true, legacy: legacyDC3ListMenu, adapted: adaptedDC3ListMenu, wantDataKey: true,
		},

		// BatchUpdateDeviceConfig：HandleAction，成功包络省略 data，uuid 校验。
		{
			name: "batch/success", method: http.MethodPut,
			target: "/api/v1/device_config/batch", body: `{"device_config_id":"123e4567-e89b-12d3-a456-426614174000","device_ids":["d1"]}`,
			withClaims: true, legacy: legacyDC3Batch, adapted: adaptedDC3Batch, wantDataKey: false,
		},
		{
			name: "batch/param-error-bad-uuid", method: http.MethodPut,
			target: "/api/v1/device_config/batch", body: `{"device_config_id":"not-a-uuid","device_ids":["d1"]}`,
			withClaims: true, legacy: legacyDC3Batch, adapted: adaptedDC3Batch, wantDataKey: false,
		},
		{
			name: "batch/service-error", method: http.MethodPut,
			target: "/api/v1/device_config/batch", body: `{"device_config_id":"dc3-boom-id","device_ids":["d1"]}`,
			withClaims: true, legacy: legacyDC3Batch, adapted: adaptedDC3Batch, wantDataKey: false,
		},

		// HandleDeviceConfigConnect：Handle + GET query 绑定 + 闭包读 Accept-Language。
		{
			name: "connect/success-with-lang", method: http.MethodGet,
			target: "/api/v1/device_config/connect?device_id=d-1",
			withClaims: true, legacy: legacyDC3Connect, adapted: adaptedDC3Connect, wantDataKey: true,
		},
		{
			name: "connect/param-error-missing-device-id", method: http.MethodGet,
			target: "/api/v1/device_config/connect",
			withClaims: true, legacy: legacyDC3Connect, adapted: adaptedDC3Connect, wantDataKey: false,
		},

		// HandleVoucherType：HandlePublic 匿名形态 + 闭包读 Accept-Language。
		{
			name: "voucher-type/success-with-lang", method: http.MethodGet,
			target: "/api/v1/device_config/voucher_type?device_type=1&protocol_type=mqtt",
			withClaims: false, legacy: legacyDC3VoucherType, adapted: adaptedDC3VoucherType, wantDataKey: true,
		},
		{
			name: "voucher-type/param-error-missing-device-type", method: http.MethodGet,
			target: "/api/v1/device_config/voucher_type?protocol_type=mqtt",
			withClaims: false, legacy: legacyDC3VoucherType, adapted: adaptedDC3VoucherType, wantDataKey: false,
		},

		// HandleActionByDeviceConfigID / HandleConditionByDeviceConfigID：Handle + GET query 绑定，
		// 两者共用 GetActionByDeviceConfigIDReq 契约与同一适配器形态，这里覆盖一次。
		{
			name: "action-menu/success", method: http.MethodGet,
			target: "/api/v1/device_config/metrics/menu?device_config_id=cfg-1",
			withClaims: true, legacy: legacyDC3ActionMenu, adapted: adaptedDC3ActionMenu, wantDataKey: true,
		},
		{
			name: "action-menu/param-error-missing-id", method: http.MethodGet,
			target: "/api/v1/device_config/metrics/menu",
			withClaims: true, legacy: legacyDC3ActionMenu, adapted: adaptedDC3ActionMenu, wantDataKey: false,
		},
	}

	for _, tc := range cases {
		dc3RunGoldenCase(t, tc)
	}
}

// TestDeviceConfigGolden3MissingClaimsBehavior 固化有意的行为差异（对 device_config.go 的两种形态各验一次）：
// 路由漏配鉴权中间件时，旧写法 MustGet panic 被响应中间件 recover 成 CodeSystemError；
// 新写法（Handle / HandlePathAction 内的 RequireClaims）不 panic，返回标准 CodeUnauthorized。
func TestDeviceConfigGolden3MissingClaimsBehavior(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		target  string
		body    string
		legacy  gin.HandlerFunc
		adapted gin.HandlerFunc
	}{
		{
			name: "Handle/create-shape", method: http.MethodPost,
			target: "/api/v1/device_config", body: `{"name":"cfg","device_type":"1"}`,
			legacy: legacyDC3Create, adapted: adaptedDC3Create,
		},
		{
			name: "HandlePathAction/delete-shape", method: http.MethodDelete,
			target: "/api/v1/device_config/cfg-9", body: "",
			legacy: legacyDC3Delete, adapted: adaptedDC3Delete,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			legacyCode, legacyBody := dc3RenderGolden(t, tc.method, tc.target, tc.body, false, tc.legacy)
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

			code, body := dc3RenderGolden(t, tc.method, tc.target, tc.body, false, tc.adapted)
			if code != http.StatusOK {
				t.Fatalf("adapted HTTP status = %d, want %d", code, http.StatusOK)
			}
			var payload apiresponse.Response
			if err := json.Unmarshal([]byte(body), &payload); err != nil {
				t.Fatalf("decode adapted response body %q: %v", body, err)
			}
			if payload.Code != errcode.CodeUnauthorized {
				t.Fatalf("adapted response code = %d, want %d (body=%s)", payload.Code, errcode.CodeUnauthorized, body)
			}
		})
	}
}
