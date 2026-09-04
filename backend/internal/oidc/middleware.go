// 文件用途：OIDC 单点登录的独立 Gin 中间件（ROADMAP C7）。
// 核心逻辑：
//   StartPath   —— 生成 state+nonce 并写入 HttpOnly Cookie，302 到 IdP 授权页；
//   CallbackPath—— 校验 state 与 Cookie 一致 → 授权码换 Token → VerifyIDToken
//                   （iss/aud/exp/nonce）→ 交给 sessionIssuer 完成本地会话落地。
// 关键注意事项：
//   - 本中间件不做本地用户查找/建号/签发本地 JWT——这些由调用方注入的 sessionIssuer 完成，
//     便于按各自的用户模型（多租户/自动建号/企业域限制）接入与单测；
//   - nonce/state 存于 HttpOnly+SameSite=Lax Cookie；生产环境应叠加签名/加密与 PKCE(S256)，
//     本包保留接口（见 Next 重构建议）不擅自引入密钥管理；
//   - 回调任何失败都返回统一错误，不把 IdP 细节透出到浏览器日志之外。
// 重构建议：PKCE(S256) 与 state cookie 签名可作为下一步安全增强，接口已按可扩展设计。
package oidc

import (
	"crypto"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Profile 通过 SSO 验证后的用户档案（交由 sessionIssuer 落地）。
type Profile struct {
	Sub        string `json:"sub"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Issuer     string `json:"issuer"`
	TenantHint string `json:"tenant_hint,omitempty"`
}

// SessionIssuer 回调验证成功后创建本地会话并返回前端落地地址。
type SessionIssuer func(c *gin.Context, p Profile) (redirectURL string, err error)

// Middleware OIDC SSO 中间件。
type Middleware struct {
	Cfg           ProviderConfig
	Client        *Client
	StateCookie   string // 存 state:nonce 的 cookie 名，默认 "oidc_state"
	StartPath     string // 触发跳转 IdP 的路径，默认 "/sso/start"
	CallbackPath  string // IdP 回调路径，默认 "/sso/callback"
	SessionIssuer SessionIssuer
}

// NewMiddleware 构造 OIDC SSO 中间件；sessionIssuer 为 nil 时回调落地返回显式未配置错误。
func NewMiddleware(cfg ProviderConfig, sessionIssuer SessionIssuer) *Middleware {
	return &Middleware{
		Cfg:           cfg,
		Client:        &Client{Cfg: cfg},
		StateCookie:   "oidc_state",
		StartPath:     "/sso/start",
		CallbackPath:  "/sso/callback",
		SessionIssuer: sessionIssuer,
	}
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Handle 路由处理器：StartPath → 302 IdP；CallbackPath → 校验并落地会话。
func (m *Middleware) Handle(c *gin.Context) {
	switch {
	case strings.HasPrefix(c.Request.URL.Path, m.CallbackPath):
		m.handleCallback(c)
	case strings.HasPrefix(c.Request.URL.Path, m.StartPath):
		m.handleStart(c)
	default:
		// 已登录态由外层会话中间件负责；未命中 SSO 路径直接放行后续链。
		c.Next()
	}
}

func (m *Middleware) handleStart(c *gin.Context) {
	state, err := randomHex(16)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "SSO 启动失败"})
		return
	}
	nonce, err := randomHex(16)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "SSO 启动失败"})
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     m.StateCookie,
		Value:    state + "." + nonce,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request.TLS != nil,
		MaxAge:   600,
	})

	doc, err := m.Client.Discover(c.Request.Context())
	if err != nil {
		// 可缓存 discovery；失败时给出通用 502 而非堆栈。
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"code": 502, "message": "SSO 提供方不可达"})
		return
	}
	c.Redirect(http.StatusFound, m.Client.BuildAuthorizationURL(doc, state, nonce))
}

func (m *Middleware) handleCallback(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": 400, "message": "SSO 回调缺少 code/state"})
		return
	}
	stored, err := c.Cookie(m.StateCookie)
	if err != nil || stored == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": 400, "message": "SSO 回调缺少状态 Cookie"})
		return
	}
	parts := strings.SplitN(stored, ".", 2)
	if len(parts) != 2 || parts[0] != state {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": 400, "message": "SSO state 校验失败"})
		return
	}
	nonce := parts[1]
	// state 校验通过后即作废 cookie，防重放。
	// Match the secure state cookie issued by handleStart; deletion must not
	// downgrade the cookie attributes on HTTPS callbacks.
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     m.StateCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	doc, err := m.Client.Discover(c.Request.Context())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"code": 502, "message": "SSO 提供方不可达"})
		return
	}
	tr, err := m.Client.ExchangeCode(c.Request.Context(), doc, code)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"code": 502, "message": "SSO 换取令牌失败"})
		return
	}
	keys, err := m.fetchJWKS(c, doc)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"code": 502, "message": "SSO 获取签名密钥失败"})
		return
	}
	claims, err := m.Client.VerifyIDToken(tr.IDToken, VerifyConfig{
		ExpectedIssuer:   doc.Issuer,
		ExpectedAudience: m.Cfg.ClientID,
		ExpectedNonce:    nonce,
		ClientSecret:     m.Cfg.ClientSecret,
		JWKSKeys:         keys,
	})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "SSO ID Token 校验失败"})
		return
	}
	if m.SessionIssuer == nil {
		c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{"code": 501, "message": "SSO 会话落地未配置"})
		return
	}
	redirect, err := m.SessionIssuer(c, Profile{
		Sub:        claims.Sub,
		Email:      claims.Email,
		Name:       claims.Name,
		Issuer:     claims.Iss,
		TenantHint: m.Cfg.TenantHint,
	})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"code": 502, "message": "SSO 会话落地失败"})
		return
	}
	if redirect != "" {
		c.Redirect(http.StatusFound, redirect)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": map[string]string{"email": claims.Email}})
}

// fetchJWKS 拉取并解析 IdP 公钥；生产可将结果按 kid 缓存。无 JWKS 的 IdP（仅 HS256）返回空。
func (m *Middleware) fetchJWKS(c *gin.Context, doc *DiscoveryDocument) (map[string]crypto.PublicKey, error) {
	if doc.JWKSURI == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, doc.JWKSURI, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.Client.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return ParseJWKS(body)
}
