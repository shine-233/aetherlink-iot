// 文件用途：OIDC 协议核心与中间件的真实单测——签名算法（HS256/RS256/JWKS）、
// exp/iss/aud/nonce 校验、alg=none 拒绝，以及 start 跳转与 callback 全流程（httptest 假 IdP）。
package oidc

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func signJWT(claims map[string]interface{}, alg, kid string, sign func([]byte) ([]byte, error)) string {
	header := map[string]string{"alg": alg}
	if kid != "" {
		header["kid"] = kid
	}
	hRaw, _ := json.Marshal(header)
	pRaw, _ := json.Marshal(claims)
	// JWS 签名输入 = base64url(header) + "." + base64url(payload)
	signingInput := b64url(hRaw) + "." + b64url(pRaw)
	sig, err := sign([]byte(signingInput))
	if err != nil {
		panic(err)
	}
	return signingInput + "." + b64url(sig)
}

func hs256Sign(secret string) func([]byte) ([]byte, error) {
	return func(payload []byte) ([]byte, error) {
		mac := hmacSHA256([]byte(secret), payload)
		return mac, nil
	}
}

func testClaims(overrides map[string]interface{}) map[string]interface{} {
	m := map[string]interface{}{
		"iss":   "https://idp.example.com",
		"sub":   "user-123",
		"aud":   "aetherlink-client",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"email": "alice@example.com",
		"name":  "Alice",
	}
	for k, v := range overrides {
		m[k] = v
	}
	return m
}

func newTestClient() *Client {
	return &Client{
		Cfg: ProviderConfig{
			Issuer:       "https://idp.example.com",
			ClientID:     "aetherlink-client",
			ClientSecret: "s3cret",
			RedirectURL:  "http://localhost:9999/sso/callback",
		},
	}
}

func TestBuildAuthorizationURLIncludesCoreParams(t *testing.T) {
	c := newTestClient()
	doc := &DiscoveryDocument{AuthorizationEndpoint: "https://idp.example.com/authorize"}
	raw := c.BuildAuthorizationURL(doc, "state-1", "nonce-1")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("client_id") != "aetherlink-client" || q.Get("redirect_uri") != c.Cfg.RedirectURL {
		t.Fatalf("授权 URL 缺 client_id/redirect_uri: %s", raw)
	}
	if q.Get("response_type") != "code" || q.Get("scope") == "" {
		t.Fatalf("授权 URL 缺 response_type/scope: %s", raw)
	}
	if q.Get("state") != "state-1" || q.Get("nonce") != "nonce-1" {
		t.Fatalf("授权 URL 缺 state/nonce: %s", raw)
	}
}

func TestVerifyIDTokenHS256HappyPath(t *testing.T) {
	c := newTestClient()
	tok := signJWT(testClaims(map[string]interface{}{"nonce": "n1"}), "HS256", "", hs256Sign("s3cret"))
	claims, err := c.VerifyIDToken(tok, VerifyConfig{
		ExpectedIssuer:   "https://idp.example.com",
		ExpectedAudience: "aetherlink-client",
		ExpectedNonce:    "n1",
		ClientSecret:     "s3cret",
	})
	if err != nil {
		t.Fatalf("合法 HS256 token 应通过: %v", err)
	}
	if claims.Email != "alice@example.com" || claims.Sub != "user-123" {
		t.Fatalf("claims 不符: %+v", claims)
	}
}

func TestVerifyIDTokenHS256RejectsWrongSecret(t *testing.T) {
	c := newTestClient()
	tok := signJWT(testClaims(nil), "HS256", "", hs256Sign("s3cret"))
	if _, err := c.VerifyIDToken(tok, VerifyConfig{ClientSecret: "wrong"}); err == nil {
		t.Fatal("错误 secret 应被拒绝")
	}
}

func TestVerifyIDTokenRejectsExpired(t *testing.T) {
	c := newTestClient()
	tok := signJWT(testClaims(map[string]interface{}{"exp": time.Now().Add(-time.Hour).Unix()}), "HS256", "", hs256Sign("s3cret"))
	if _, err := c.VerifyIDToken(tok, VerifyConfig{ClientSecret: "s3cret"}); err == nil {
		t.Fatal("过期 token 应被拒绝")
	}
}

func TestVerifyIDTokenRejectsAudienceMismatch(t *testing.T) {
	c := newTestClient()
	tok := signJWT(testClaims(map[string]interface{}{"aud": "other-app"}), "HS256", "", hs256Sign("s3cret"))
	_, err := c.VerifyIDToken(tok, VerifyConfig{ExpectedAudience: "aetherlink-client", ClientSecret: "s3cret"})
	if err == nil || !strings.Contains(err.Error(), "audience") {
		t.Fatalf("aud 不匹配应报 audience 错误: %v", err)
	}
}

func TestVerifyIDTokenRejectsNonceMismatch(t *testing.T) {
	c := newTestClient()
	tok := signJWT(testClaims(map[string]interface{}{"nonce": "other"}), "HS256", "", hs256Sign("s3cret"))
	_, err := c.VerifyIDToken(tok, VerifyConfig{ExpectedNonce: "expected", ClientSecret: "s3cret"})
	if err == nil || !strings.Contains(err.Error(), "nonce") {
		t.Fatalf("nonce 不匹配应报错: %v", err)
	}
}

func TestVerifyIDTokenRejectsAlgNone(t *testing.T) {
	c := newTestClient()
	tok := signJWT(testClaims(nil), "none", "", func(payload []byte) ([]byte, error) { return []byte{}, nil })
	if _, err := c.VerifyIDToken(tok, VerifyConfig{}); err == nil || !strings.Contains(err.Error(), "none") {
		t.Fatalf("alg=none 应被拒绝: %v", err)
	}
}

func TestVerifyIDTokenRS256ViaJWKS(t *testing.T) {
	c := newTestClient()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwk := fmt.Sprintf(`{"keys":[{"kty":"RSA","kid":"key1","n":%q,"e":%q}]}`,
		b64url(priv.N.Bytes()), b64url([]byte{1, 0, 1}))
	keys, err := ParseJWKS([]byte(jwk))
	if err != nil {
		t.Fatalf("parse jwks: %v", err)
	}
	sign := func(payload []byte) ([]byte, error) {
		h := sha256.Sum256(payload)
		return rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, h[:])
	}
	tok := signJWT(testClaims(nil), "RS256", "key1", sign)
	claims, err := c.VerifyIDToken(tok, VerifyConfig{
		ExpectedIssuer:   "https://idp.example.com",
		ExpectedAudience: "aetherlink-client",
		JWKSKeys:         keys,
	})
	if err != nil {
		t.Fatalf("合法 RS256 token 应通过: %v", err)
	}
	if claims.Email != "alice@example.com" {
		t.Fatalf("claims 不符: %+v", claims)
	}
}

func TestVerifyIDTokenRS256RejectsTamperedPayload(t *testing.T) {
	c := newTestClient()
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	sign := func(payload []byte) ([]byte, error) {
		h := sha256.Sum256(payload)
		return rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, h[:])
	}
	tok := signJWT(testClaims(nil), "RS256", "", sign)
	// 篡改 email 后再放回同一签名 → 验签必须失败
	parts := strings.Split(tok, ".")
	tampered, _ := json.Marshal(testClaims(map[string]interface{}{"email": "evil@example.com"}))
	tok = parts[0] + "." + b64url(tampered) + "." + parts[2]
	jwk := fmt.Sprintf(`{"keys":[{"kty":"RSA","kid":"k","n":%q,"e":%q}]}`, b64url(priv.N.Bytes()), b64url([]byte{1, 0, 1}))
	keys, _ := ParseJWKS([]byte(jwk))
	if _, err := c.VerifyIDToken(tok, VerifyConfig{JWKSKeys: keys}); err == nil {
		t.Fatal("篡改 payload 后签名应不匹配")
	}
}

// ---- 中间件：start 跳转 ----

func TestMiddlewareHandleStartRedirectsToIdP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"issuer":"https://idp.example.com","authorization_endpoint":"https://idp.example.com/authorize","token_endpoint":"https://idp.example.com/token"}`)
	}))
	defer fake.Close()

	cfg := ProviderConfig{
		Issuer: "https://idp.example.com", ClientID: "aetherlink-client",
		ClientSecret: "s3cret", RedirectURL: "http://localhost:9999/sso/callback",
		DiscoveryURL: fake.URL,
	}
	m := NewMiddleware(cfg, nil)
	r := gin.New()
	r.GET("/sso/start", m.handleStart)

	req := httptest.NewRequest(http.MethodGet, "/sso/start", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("start 应 302: %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "client_id=aetherlink-client") || !strings.Contains(loc, "state=") {
		t.Fatalf("Location 应含 client_id 与 state: %s", loc)
	}
	cookies := rec.Result().Cookies()
	var found bool
	for _, ck := range cookies {
		if ck.Name == m.StateCookie && !ck.HttpOnly {
			t.Fatal("state cookie 必须 HttpOnly")
		}
		if ck.Name == m.StateCookie && ck.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("start 应写入 state cookie")
	}
}

// ---- 中间件：callback 全流程（httptest 假 IdP，HS256）----

func TestMiddlewareCallbackFullFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "s3cret"
	var fake *httptest.Server
	fake = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			fmt.Fprintf(w, `{"issuer":"%s","authorization_endpoint":"%s/authorize","token_endpoint":"%s/token"}`, fake.URL, fake.URL, fake.URL)
		case strings.HasSuffix(r.URL.Path, "/token"):
			tok := signJWT(testClaims(map[string]interface{}{"iss": fake.URL, "nonce": "NONCE1"}), "HS256", "", hs256Sign(secret))
			fmt.Fprintf(w, `{"access_token":"at","id_token":%q,"token_type":"Bearer"}`, tok)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer fake.Close()

	cfg := ProviderConfig{
		Issuer: fake.URL, ClientID: "aetherlink-client", ClientSecret: secret,
		RedirectURL: "http://localhost:9999/sso/callback",
		DiscoveryURL: fake.URL + "/.well-known/openid-configuration",
	}
	var got Profile
	m := NewMiddleware(cfg, func(c *gin.Context, p Profile) (string, error) {
		got = p
		return "/dashboard?fresh=1", nil
	})

	r := gin.New()
	r.GET(m.CallbackPath, m.handleCallback)

	req := httptest.NewRequest(http.MethodGet, m.CallbackPath+"?code=abc&state=STATE1", nil)
	req.AddCookie(&http.Cookie{Name: m.StateCookie, Value: "STATE1.NONCE1", Path: "/"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		body := rec.Body.String()
		t.Fatalf("callback 应 302 落地, got %d: %s", rec.Code, body)
	}
	if got.Email != "alice@example.com" || got.Sub != "user-123" {
		t.Fatalf("sessionIssuer 应收到验签后的 profile: %+v", got)
	}
	if loc := rec.Header().Get("Location"); loc != "/dashboard?fresh=1" {
		t.Fatalf("应重定向到 sessionIssuer 返回地址: %s", loc)
	}
}

func TestMiddlewareCallbackRejectsStateMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := NewMiddleware(ProviderConfig{}, nil)
	r := gin.New()
	r.GET(m.CallbackPath, m.handleCallback)
	req := httptest.NewRequest(http.MethodGet, m.CallbackPath+"?code=abc&state=OTHER", nil)
	req.AddCookie(&http.Cookie{Name: m.StateCookie, Value: "STATE1.NONCE1", Path: "/"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("state 不匹配应 400: %d", rec.Code)
	}
}

// 供 helper 使用的小工具
func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
