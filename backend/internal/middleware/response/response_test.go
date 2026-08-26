// 文件用途：统一响应封装中间件的行为契约测试。
// 核心逻辑：钉死 panic 恢复、已写响应跳过、Gin 错误转错误包、data 成功包裹、变量替换与多语言选择六条链路。
// 关键注意事项：响应结构是存量客户端契约；本测试是双实现并存期的行为防线，改动结构须同步此处与调用方。
package response

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	dir := t.TempDir()
	codePath := filepath.Join(dir, "codes.yaml")
	stringPath := filepath.Join(dir, "strings.yaml")
	codeYaml := `
metadata:
  version: "1"
  last_updated: "2026-08-26"
  supported_languages: ["zh_CN", "en_US"]
messages:
  200:
    zh_CN: "成功"
    en_US: "success"
  100002:
    zh_CN: "参数错误"
    en_US: "param error"
  200053:
    zh_CN: "名称含非法字符 ${invalid_chars}"
    en_US: "name contains invalid chars ${invalid_chars}"
`
	if err := os.WriteFile(codePath, []byte(codeYaml), 0o600); err != nil {
		t.Fatalf("write codes yaml: %v", err)
	}
	if err := os.WriteFile(stringPath, []byte(codeYaml), 0o600); err != nil {
		t.Fatalf("write strings yaml: %v", err)
	}
	handler, err := NewHandler(codePath, stringPath)
	require.NoError(t, err)
	return handler
}

// perform 构造一条 POST /t 路由：response 中间件在最外层（与生产挂载一致），业务逻辑在其 c.Next() 内执行，
// 这样 panic 恢复、已写响应跳过等包装语义才能真实生效。
func perform(t *testing.T, handler *Handler, lang string, business gin.HandlerFunc) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(recorder)
	var captured *gin.Context
	engine.POST("/t", handler.Middleware(), func(c *gin.Context) {
		if lang != "" {
			c.Request.Header.Set("Accept-Language", lang)
		}
		business(c)
		captured = c
	})
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/t", nil))
	return recorder, captured
}

func TestMiddlewareWrapsDataIntoSuccessEnvelope(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "en-US", func(c *gin.Context) {
		c.Set("data", gin.H{"ok": true})
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"code":200,"message":"success","data":{"ok":true}}`, recorder.Body.String())
}

func TestMiddlewareConvertsErrcodeErrorAndReplacesVariables(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "en-US", func(c *gin.Context) {
		c.Error(errcode.WithVars(200053, map[string]interface{}{"invalid_chars": "#"}))
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":200053`)
	assert.Contains(t, recorder.Body.String(), `name contains invalid chars #`)
}

func TestMiddlewarePrefersCustomMessageOverLocalizedTemplate(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "", func(c *gin.Context) {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "custom-message"))
	})

	assert.Contains(t, recorder.Body.String(), `"message":"custom-message"`)
}

func TestMiddlewareWrapsUnknownErrorAsSystemError(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "", func(c *gin.Context) {
		c.Error(assert.AnError)
	})

	assert.Contains(t, recorder.Body.String(), `"code":100000`)
}

func TestMiddlewareRecoversFromHandlerPanic(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "", func(c *gin.Context) {
		panic("boom")
	})

	assert.Contains(t, recorder.Body.String(), `"code":100000`)
}

func TestMiddlewareKeepsResponseWhenAlreadyWritten(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "", func(c *gin.Context) {
		c.String(http.StatusTeapot, "raw-upstream")
	})

	assert.Equal(t, http.StatusTeapot, recorder.Code)
	assert.Equal(t, "raw-upstream", recorder.Body.String())
}

func TestMiddlewareWritesNothingWithoutDataOrError(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "", func(c *gin.Context) {})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, recorder.Body.String())
}

func TestMiddlewareRespectsAcceptLanguageForSuccessMessage(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "zh-CN", func(c *gin.Context) {
		c.Set("data", gin.H{})
	})

	assert.Contains(t, recorder.Body.String(), `"message":"成功"`)
}
