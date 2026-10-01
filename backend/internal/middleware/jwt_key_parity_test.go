// 文件用途：JWT / 插件共享密钥"签发侧与校验侧同源"的端到端回归测试。
// 核心逻辑：jwt.key 配置为合法 48 字符密钥 + 尾随换行（secret 文件/GOTP_JWT_KEY 常见形态），
// 经真实登录签发路径 UserLoginAfter 签发 token，再经 JWTAuth / OptionalJWTAuth 中间件校验；
// 修复前签发用 TrimSpace 值、校验用原值，全部请求 401 invalid token。
// 关键注意事项：使用 miniredis + 内存 sqlite，不依赖外部服务；全局状态均在 Cleanup 中复原。

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/authkeys"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

const parityStrongKey = "openssl-rand-base64-sample-key-with-48-bytes!!xy"

func setViperForTest(t *testing.T, key string, value interface{}) {
	t.Helper()
	old := viper.Get(key)
	viper.Set(key, value)
	t.Cleanup(func() { viper.Set(key, old) })
}

func setupKeyParityEnv(t *testing.T) {
	t.Helper()
	db := setupJWTAuthUserStatusDB(t)
	query.SetDefault(db)
	t.Cleanup(func() {
		if global.DB != nil {
			query.SetDefault(global.DB)
		}
	})
	seedJWTAuthUser(t, db, "parity-user", stringPtr("N"))
	resetJWTUserStatusCache()
	t.Cleanup(resetJWTUserStatusCache)

	server := miniredis.RunT(t)
	oldRedis := global.REDIS
	global.REDIS = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = global.REDIS.Close()
		global.REDIS = oldRedis
	})
	setViperForTest(t, "session.timeout", 60)
}

func issueLoginToken(t *testing.T) string {
	t.Helper()
	var user model.User
	if err := global.DB.Where("id = ?", "parity-user").First(&user).Error; err != nil {
		t.Fatalf("load seeded user: %v", err)
	}
	rsp, err := service.GroupApp.User.UserLoginAfter(&user)
	if err != nil {
		t.Fatalf("UserLoginAfter: %v", err)
	}
	if rsp == nil || rsp.Token == nil || *rsp.Token == "" {
		t.Fatalf("UserLoginAfter returned no token: %+v", rsp)
	}
	return *rsp.Token
}

func serveWithToken(t *testing.T, handler gin.HandlerFunc, token string) (int, *utils.UserClaims) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var got *utils.UserClaims
	r := gin.New()
	r.GET("/probe", handler, func(c *gin.Context) {
		if v, ok := c.Get("claims"); ok {
			got, _ = v.(*utils.UserClaims)
		}
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("x-token", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, got
}

func TestJWTKeyParityTrailingWhitespaceKeyIssuesVerifiableTokens(t *testing.T) {
	for name, raw := range map[string]string{
		"LF":           parityStrongKey + "\n",
		"CRLF":         parityStrongKey + "\r\n",
		"spaces":       "  " + parityStrongKey + " ",
		"already-trim": parityStrongKey,
	} {
		t.Run(name, func(t *testing.T) {
			setupKeyParityEnv(t)
			setViperForTest(t, authkeys.JWTKeyConfigKey, raw)

			token := issueLoginToken(t)

			code, claims := serveWithToken(t, JWTAuth(), token)
			if code != http.StatusOK || claims == nil || claims.ID != "parity-user" {
				t.Fatalf("JWTAuth with key %q: status=%d claims=%v, want 200 with parity-user", raw, code, claims)
			}
			code, claims = serveWithToken(t, OptionalJWTAuth(), token)
			if code != http.StatusOK || claims == nil || claims.ID != "parity-user" {
				t.Fatalf("OptionalJWTAuth with key %q: status=%d claims=%v, want claims injected", raw, code, claims)
			}
		})
	}
}

// 校验侧密钥真正改变时（轮换）旧 token 仍必须被拒绝：规范化只吸收首尾空白，不放宽签名校验。
func TestJWTKeyParityRejectsTokenAfterRealKeyChange(t *testing.T) {
	setupKeyParityEnv(t)
	setViperForTest(t, authkeys.JWTKeyConfigKey, parityStrongKey+"\n")
	token := issueLoginToken(t)

	viper.Set(authkeys.JWTKeyConfigKey, parityStrongKey+"-rotated\n")
	if code, _ := serveWithToken(t, JWTAuth(), token); code != http.StatusUnauthorized {
		t.Fatalf("token signed with old key must be rejected after rotation, got %d", code)
	}
}

// 未配置密钥时校验侧 fail-closed：空 HMAC 密钥不得把任何 token 判为有效。
func TestJWTKeyParityEmptyKeyFailsClosed(t *testing.T) {
	setupKeyParityEnv(t)
	setViperForTest(t, authkeys.JWTKeyConfigKey, parityStrongKey)
	token := issueLoginToken(t)

	viper.Set(authkeys.JWTKeyConfigKey, " \n")
	if code, _ := serveWithToken(t, JWTAuth(), token); code != http.StatusUnauthorized {
		t.Fatalf("empty jwt.key must reject tokens, got %d", code)
	}
	if code, claims := serveWithToken(t, OptionalJWTAuth(), token); code != http.StatusOK || claims != nil {
		t.Fatalf("OptionalJWTAuth with empty key must degrade to anonymous, got %d claims=%v", code, claims)
	}
}

func TestPluginAuthKeyParityTrailingWhitespace(t *testing.T) {
	r := setupPluginAuthRouter(t, "plugin-shared-secret\n")
	send := func(header string) int {
		req := httptest.NewRequest(http.MethodPost, "/plugin/heartbeat", nil)
		req.RemoteAddr = "203.0.113.9:4000"
		req.Header.Set(pluginKeyHeader, header)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	if code := send("plugin-shared-secret"); code != http.StatusOK {
		t.Fatalf("plugin key from a secret file with trailing newline must match trimmed header, got %d", code)
	}
	if code := send("wrong-secret"); code != http.StatusUnauthorized && code != http.StatusForbidden {
		t.Fatalf("wrong plugin key must be rejected, got %d", code)
	}
}

func TestPluginAuthWhitespaceOnlyKeyTreatedAsUnset(t *testing.T) {
	r := setupPluginAuthRouter(t, " \n")
	req := httptest.NewRequest(http.MethodPost, "/plugin/heartbeat", nil)
	req.RemoteAddr = "203.0.113.9:4000"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatal("whitespace-only plugin key must behave as unset: public source rejected")
	}
	req = httptest.NewRequest(http.MethodPost, "/plugin/heartbeat", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("whitespace-only plugin key must behave as unset: loopback allowed, got %d", w.Code)
	}
}
