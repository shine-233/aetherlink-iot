// 文件用途：sys_dict.go 迁移到 handler_adapter 骨架后的 golden 对比测试（编号 7 专用，仅覆盖字典接口）。
// 核心逻辑：8 个 handler 各保留两份实现——「旧写法」逐行复刻迁移前的手写样板
// （BindAndValidate / MustGet("claims") / c.Error / c.Set），「新写法」复刻迁移后 sys_dict.go 中的
// 适配器调用形态；两者调用同一组探针 service（签名与 service.GroupApp.Dict 的真实方法一致，
// 入参出参使用真实 model 类型），走统一响应中间件渲染后对 HTTP 状态码与原始响应体做全等比较，
// 证明迁移前后 JSON 包络逐字节一致。这是渲染结果的真实比对，不是声明式契约。
// 已知的有意差异：缺失 claims 时旧写法 MustGet panic 被响应中间件 recover 成 CodeSystemError，
// 新写法 RequireClaims 返回标准 CodeUnauthorized（handler_adapter.go 头部声明），
// 由 TestDict7MissingClaimsReturnsUnauthorizedInsteadOfPanic 单独固化。
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiresponse "aetherlink-iot/backend/internal/middleware/response"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ---- 探针 service：签名与 service.GroupApp.Dict 真实方法一致，错误码形态也对齐 ----

const (
	dict7BoomDictCode = "boom"                                     // 触发写/查探针返回业务错误
	dict7BoomLangCode = "boom"                                     // 触发协议菜单探针返回业务错误
	dict7BoomPathID   = "boom-id"                                  // 触发删除/详情探针返回业务错误
	dict7OverMaxCode  = "cccccccccccccccccccccccccccccccccccccccc" // 40 字符，超过 max=36
)

func dict7ServiceErr(msg string) error {
	return errcode.WithData(errcode.CodeParamError, map[string]interface{}{"err": msg})
}

func dict7ProbeCreateColumn(req *model.CreateDictReq, claims *utils.UserClaims) (*model.SysDict, error) {
	if req.DictCode == dict7BoomDictCode {
		return nil, dict7ServiceErr("dict code duplicated")
	}
	return &model.SysDict{ID: "dict-1", DictCode: req.DictCode, DictValue: req.DictValue}, nil
}

func dict7ProbeCreateLanguage(req *model.CreateDictLanguageReq, claims *utils.UserClaims) (*model.SysDictLanguage, error) {
	if req.Translation == dict7BoomDictCode {
		return nil, dict7ServiceErr("dict not found")
	}
	return &model.SysDictLanguage{ID: "dict-lang-1", DictID: req.DictId, LanguageCode: req.LanguageCode, Translation: req.Translation}, nil
}

func dict7ProbeDeleteDict(id string, claims *utils.UserClaims) error {
	if id == dict7BoomPathID {
		return dict7ServiceErr("dict referenced")
	}
	return nil
}

func dict7ProbeDeleteDictLanguage(id string, claims *utils.UserClaims) error {
	if id == dict7BoomPathID {
		return dict7ServiceErr("dict language referenced")
	}
	return nil
}

func dict7ProbeGetDict(req *model.DictListReq, lang string) ([]model.DictListRsp, error) {
	if req.DictCode == dict7BoomDictCode {
		return nil, dict7ServiceErr("dict list query failed")
	}
	return []model.DictListRsp{{DictValue: req.DictCode, Translation: "T(" + lang + ")"}}, nil
}

func dict7ProbeGetProtocolMenu(req *model.ProtocolMenuReq) ([]map[string]interface{}, error) {
	lang := "zh"
	if req.LanguageCode != nil {
		lang = *req.LanguageCode
	}
	if lang == dict7BoomLangCode {
		return nil, dict7ServiceErr("protocol dict query failed")
	}
	return []map[string]interface{}{{"dict_value": "MQTT", "translation": "T(" + lang + ")", "device_type": "1"}}, nil
}

func dict7ProbeGetDictLanguageListById(id string) ([]*model.SysDictLanguage, error) {
	if id == dict7BoomPathID {
		return nil, dict7ServiceErr("dict not found")
	}
	return []*model.SysDictLanguage{{ID: "dict-lang-1", DictID: id, LanguageCode: "zh", Translation: "示例"}}, nil
}

func dict7ProbeGetDictListByPage(req *model.GetDictLisyByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if req.DictCode != nil && *req.DictCode == dict7BoomDictCode {
		return nil, dict7ServiceErr("dict page query failed")
	}
	return map[string]interface{}{"total": 1, "list": []string{"dict-" + claims.TenantID}}, nil
}

// ---- 旧写法（迁移前 sys_dict.go 的逐行复刻，仅把 service 换成探针）----

func dict7LegacyCreateDictColumn(c *gin.Context) {
	var createDictReq model.CreateDictReq
	if !BindAndValidate(c, &createDictReq) {
		return
	}

	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	created, err := dict7ProbeCreateColumn(&createDictReq, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", created)
}

func dict7LegacyCreateDictLanguage(c *gin.Context) {
	var createDictLanguageReq model.CreateDictLanguageReq
	if !BindAndValidate(c, &createDictLanguageReq) {
		return
	}

	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	created, err := dict7ProbeCreateLanguage(&createDictLanguageReq, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", created)
}

func dict7LegacyDeleteDictColumn(c *gin.Context) {
	id := c.Param("id")
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	err := dict7ProbeDeleteDict(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func dict7LegacyDeleteDictLanguage(c *gin.Context) {
	id := c.Param("id")
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	err := dict7ProbeDeleteDictLanguage(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func dict7LegacyHandleDict(c *gin.Context) {
	var dictEnum model.DictListReq
	if !BindAndValidate(c, &dictEnum) {
		return
	}
	lang := c.GetHeader("Accept-Language")
	list, err := dict7ProbeGetDict(&dictEnum, lang)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", list)
}

func dict7LegacyHandleProtocolAndService(c *gin.Context) {
	var protocolMenuReq model.ProtocolMenuReq
	if !BindAndValidate(c, &protocolMenuReq) {
		return
	}
	list, err := dict7ProbeGetProtocolMenu(&protocolMenuReq)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", list)
}

func dict7LegacyHandleDictLanguage(c *gin.Context) {
	id := c.Param("id")
	data, err := dict7ProbeGetDictLanguageListById(id)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

func dict7LegacyHandleDictLisyByPage(c *gin.Context) {
	var byList model.GetDictLisyByPageReq
	if !BindAndValidate(c, &byList) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	logrus.Info("dictionary list request received")
	list, err := dict7ProbeGetDictListByPage(&byList, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", list)
}

// ---- 新写法（迁移后 sys_dict.go 的适配器形态复刻，仅把 service 换成探针）----

func dict7AdaptedCreateDictColumn(c *gin.Context) {
	Handle(c, func(req *model.CreateDictReq, userClaims *utils.UserClaims) (interface{}, error) {
		return dict7ProbeCreateColumn(req, userClaims)
	})
}

func dict7AdaptedCreateDictLanguage(c *gin.Context) {
	Handle(c, func(req *model.CreateDictLanguageReq, userClaims *utils.UserClaims) (interface{}, error) {
		return dict7ProbeCreateLanguage(req, userClaims)
	})
}

func dict7AdaptedDeleteDictColumn(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		return dict7ProbeDeleteDict(id, userClaims)
	})
}

func dict7AdaptedDeleteDictLanguage(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		return dict7ProbeDeleteDictLanguage(id, userClaims)
	})
}

func dict7AdaptedHandleDict(c *gin.Context) {
	HandlePublicQuery(c, func(req *model.DictListReq) (interface{}, error) {
		return dict7ProbeGetDict(req, c.GetHeader("Accept-Language"))
	})
}

func dict7AdaptedHandleProtocolAndService(c *gin.Context) {
	HandlePublicQuery(c, func(req *model.ProtocolMenuReq) (interface{}, error) {
		return dict7ProbeGetProtocolMenu(req)
	})
}

func dict7AdaptedHandleDictLanguage(c *gin.Context) {
	HandlePublicNoBody(c, func() (interface{}, error) {
		return dict7ProbeGetDictLanguageListById(c.Param("id"))
	})
}

func dict7AdaptedHandleDictLisyByPage(c *gin.Context) {
	Handle(c, func(req *model.GetDictLisyByPageReq, userClaims *utils.UserClaims) (interface{}, error) {
		logrus.Info("dictionary list request received")
		return dict7ProbeGetDictListByPage(req, userClaims)
	})
}

// ---- 渲染与断言辅助 ----

// dict7Render 用与线上一致的响应中间件渲染一次请求，返回状态码与原始响应体。
// 与 handler_adapter_test.go 的 renderProbe 的区别：Accept-Language 可按用例控制（覆盖缺省中文分支）。
func dict7Render(t *testing.T, method, target, body string, withClaims bool, lang string, handler gin.HandlerFunc) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	responseHandler := &apiresponse.Handler{
		ErrManager: errcode.NewErrorManager("", ""),
	}
	router := gin.New()
	router.Use(responseHandler.Middleware())
	if withClaims {
		router.Use(func(c *gin.Context) {
			c.Set(claimsContextKey, &utils.UserClaims{ID: "dict7-user", TenantID: "dict7-tenant"})
			c.Next()
		})
	}
	router.Handle(method, target, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if lang != "" {
		request.Header.Set("Accept-Language", lang)
	}
	router.ServeHTTP(recorder, request)

	return recorder.Code, recorder.Body.String()
}

// dict7RequireIdentical 断言新旧两种写法在同一入参下渲染出完全相同的响应。
func dict7RequireIdentical(t *testing.T, name, method, target, body, lang string, withClaims bool, legacy, adapted gin.HandlerFunc) {
	t.Helper()
	legacyCode, legacyBody := dict7Render(t, method, target, body, withClaims, lang, legacy)
	adaptedCode, adaptedBody := dict7Render(t, method, target, body, withClaims, lang, adapted)

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

const (
	dict7ColumnBodyValid   = `{"dict_code":"alpha","dict_value":"v1"}`
	dict7ColumnBodyMissing = `{"dict_value":"v1"}`
	dict7ColumnBodyOverMax = `{"dict_code":"` + dict7OverMaxCode + `","dict_value":"v1"}`
	dict7ColumnBodyBoom    = `{"dict_code":"boom","dict_value":"v1"}`

	dict7LangBodyValid   = `{"dict_id":"dict-1","language_code":"zh","translation":"示例"}`
	dict7LangBodyMissing = `{"dict_id":"dict-1","language_code":"zh"}`
	dict7LangBodyBoom    = `{"dict_id":"dict-1","language_code":"zh","translation":"boom"}`
)

// TestDict7CreateHandlersAreByteIdentical 覆盖两个 JSON body 创建接口：
// 正常成功、缺必填、超长校验失败、service 业务错误四条分支。
func TestDict7CreateHandlersAreByteIdentical(t *testing.T) {
	for _, lang := range []string{"", "zh-CN", "en-US"} {
		for _, body := range []string{dict7ColumnBodyValid, dict7ColumnBodyMissing, dict7ColumnBodyOverMax, dict7ColumnBodyBoom} {
			dict7RequireIdentical(t, "CreateDictColumn/"+lang+"/"+body, http.MethodPost, "/dict7/column", body, lang, true,
				dict7LegacyCreateDictColumn, dict7AdaptedCreateDictColumn)
		}
		for _, body := range []string{dict7LangBodyValid, dict7LangBodyMissing, dict7LangBodyBoom} {
			dict7RequireIdentical(t, "CreateDictLanguage/"+lang+"/"+body, http.MethodPost, "/dict7/language", body, lang, true,
				dict7LegacyCreateDictLanguage, dict7AdaptedCreateDictLanguage)
		}
	}
}

// TestDict7DeleteHandlersAreByteIdentical 覆盖两个路径参数删除接口：成功（data 省略）与业务错误分支。
func TestDict7DeleteHandlersAreByteIdentical(t *testing.T) {
	dict7RequireIdentical(t, "DeleteDictColumn/success", http.MethodDelete, "/dict7/column/dict-1", "", "", true,
		dict7LegacyDeleteDictColumn, dict7AdaptedDeleteDictColumn)
	dict7RequireIdentical(t, "DeleteDictColumn/boom", http.MethodDelete, "/dict7/column/"+dict7BoomPathID, "", "", true,
		dict7LegacyDeleteDictColumn, dict7AdaptedDeleteDictColumn)
	dict7RequireIdentical(t, "DeleteDictLanguage/success", http.MethodDelete, "/dict7/language/dict-1", "", "", true,
		dict7LegacyDeleteDictLanguage, dict7AdaptedDeleteDictLanguage)
	dict7RequireIdentical(t, "DeleteDictLanguage/boom", http.MethodDelete, "/dict7/language/"+dict7BoomPathID, "", "", true,
		dict7LegacyDeleteDictLanguage, dict7AdaptedDeleteDictLanguage)
}

// TestDict7PublicQueryHandlersAreByteIdentical 覆盖三个无 claims 的查询接口：
// 枚举查询（含 Accept-Language 缺省/中文/英文）、协议服务菜单（language_code 可选）、
// 字典多语言详情（路径参数），以及参数校验失败与业务错误分支。
func TestDict7PublicQueryHandlersAreByteIdentical(t *testing.T) {
	for _, lang := range []string{"", "zh-CN", "en-US"} {
		dict7RequireIdentical(t, "HandleDict/success/"+lang, http.MethodGet, "/dict7/enum?dict_code=alpha", "", lang, false,
			dict7LegacyHandleDict, dict7AdaptedHandleDict)
		dict7RequireIdentical(t, "HandleDict/missing/"+lang, http.MethodGet, "/dict7/enum", "", lang, false,
			dict7LegacyHandleDict, dict7AdaptedHandleDict)
		dict7RequireIdentical(t, "HandleDict/boom/"+lang, http.MethodGet, "/dict7/enum?dict_code=boom", "", lang, false,
			dict7LegacyHandleDict, dict7AdaptedHandleDict)
		dict7RequireIdentical(t, "HandleDict/overmax/"+lang, http.MethodGet, "/dict7/enum?dict_code="+dict7OverMaxCode, "", lang, false,
			dict7LegacyHandleDict, dict7AdaptedHandleDict)

		dict7RequireIdentical(t, "HandleProtocolAndService/success/"+lang, http.MethodGet, "/dict7/protocol/service?language_code=en", "", lang, false,
			dict7LegacyHandleProtocolAndService, dict7AdaptedHandleProtocolAndService)
		dict7RequireIdentical(t, "HandleProtocolAndService/default/"+lang, http.MethodGet, "/dict7/protocol/service", "", lang, false,
			dict7LegacyHandleProtocolAndService, dict7AdaptedHandleProtocolAndService)
		dict7RequireIdentical(t, "HandleProtocolAndService/boom/"+lang, http.MethodGet, "/dict7/protocol/service?language_code="+dict7BoomLangCode, "", lang, false,
			dict7LegacyHandleProtocolAndService, dict7AdaptedHandleProtocolAndService)

		dict7RequireIdentical(t, "HandleDictLanguage/success/"+lang, http.MethodGet, "/dict7/language/dict-1", "", lang, false,
			dict7LegacyHandleDictLanguage, dict7AdaptedHandleDictLanguage)
		dict7RequireIdentical(t, "HandleDictLanguage/boom/"+lang, http.MethodGet, "/dict7/language/"+dict7BoomPathID, "", lang, false,
			dict7LegacyHandleDictLanguage, dict7AdaptedHandleDictLanguage)
	}
}

// TestDict7PageListHandlerIsByteIdentical 覆盖分页查询：正常分页、缺分页参数、page_size 超上限、
// dict_code 过滤（含业务错误与超长校验）。
func TestDict7PageListHandlerIsByteIdentical(t *testing.T) {
	// 探针 handler 会打调试日志，静音避免污染测试输出（全局 logger，用例结束恢复）。
	previousOut := logrus.StandardLogger().Out
	logrus.StandardLogger().SetOutput(io.Discard)
	t.Cleanup(func() { logrus.StandardLogger().SetOutput(previousOut) })

	targets := []string{
		"/dict7?page=1&page_size=10",
		"/dict7",
		"/dict7?page=1&page_size=2000",
		"/dict7?page=1&page_size=10&dict_code=boom",
		"/dict7?page=1&page_size=10&dict_code=" + dict7OverMaxCode,
	}
	for _, target := range targets {
		dict7RequireIdentical(t, "HandleDictLisyByPage"+target, http.MethodGet, target, "", "", true,
			dict7LegacyHandleDictLisyByPage, dict7AdaptedHandleDictLisyByPage)
	}
}

// TestDict7MissingClaimsReturnsUnauthorizedInsteadOfPanic 固化有意的行为差异：
// 路由漏配鉴权中间件时，旧写法 MustGet panic 会被统一响应中间件 recover 成 CodeSystemError；
// 新写法 RequireClaims 不 panic，直接返回标准 CodeUnauthorized（handler_adapter.go 头部声明）。
// 以 CreateDictColumn 为带 claims handler 的代表，其余三个带 claims 接口走同一适配器路径。
func TestDict7MissingClaimsReturnsUnauthorizedInsteadOfPanic(t *testing.T) {
	legacyCode, legacyBody := dict7Render(t, http.MethodPost, "/dict7/column", dict7ColumnBodyValid, false, "", dict7LegacyCreateDictColumn)
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

	code, body := dict7Render(t, http.MethodPost, "/dict7/column", dict7ColumnBodyValid, false, "", dict7AdaptedCreateDictColumn)
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
}
