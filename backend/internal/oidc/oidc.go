// 文件用途：OAuth2 / OIDC 单点登录协议核心（ROADMAP C7 剩余项）。
// 核心逻辑：纯 Go 标准库实现 OIDC Discovery、Authorization URL 构造、授权码换 Token、
//   ID Token 验签（HS256 用 client secret，RS256 用 JWKS 公钥）、iss/aud/exp/nonce 校验。
// 关键注意事项：
//   - 显式拒绝 alg=none 与未知算法，杜绝签名绕过；
//   - aud 支持 string 与数组两种 IdP 常见写法；
//   - 本包不碰网络以外的数据面：所有 HTTP 走注入的 *http.Client（单测可换 httptest）。
// 重构建议：PKCE（S256）与 DPOP 可作为下一步增强；会话落地见 middleware.go 的 sessionIssuer。
package oidc

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ProviderConfig 单点登录提供方配置（每个租户一份；敏感项不入日志）。
type ProviderConfig struct {
	Issuer       string `json:"issuer"`                  // OIDC issuer，形如 https://idp.example.com
	ClientID     string `json:"client_id"`               // 客户端 ID
	ClientSecret string `json:"client_secret,omitempty"` // 客户端密钥（授权码模式必需）
	RedirectURL  string `json:"redirect_uri"`            // 回调地址
	Scopes       string `json:"scopes,omitempty"`        // 默认 "openid profile email"
	// TenantHint 租户提示：透传给 IdP 的 login_hint / 租户维度隔离键（本包不解释语义）。
	TenantHint string `json:"tenant_hint,omitempty"`
	// DiscoveryURL 显式 Discovery 地址；为空时按 issuer/.well-known/openid-configuration 推导。
	DiscoveryURL string `json:"discovery_url,omitempty"`
}

// DiscoveryDocument OIDC Discovery 文档（仅取本包所需字段）。
type DiscoveryDocument struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	UserinfoEndpoint      string `json:"userinfo_endpoint,omitempty"`
	IDTokenSigningAlg      []string `json:"id_token_signing_alg_values_supported,omitempty"`
}

// TokenResponse 授权码交换的响应。
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// IDTokenClaims 已验签 ID Token 的核心声明。
type IDTokenClaims struct {
	Sub   string `json:"sub"`
	Iss   string `json:"iss"`
	Aud   string `json:"aud"`
	Exp   int64  `json:"exp"`
	Nonce string `json:"nonce,omitempty"`
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// Client OIDC 客户端：HTTP 依赖可注入，便于单测。
type Client struct {
	Cfg    ProviderConfig
	HTTP   *http.Client
	Now    func() time.Time
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func defaultScopes(cfg ProviderConfig) string {
	if strings.TrimSpace(cfg.Scopes) != "" {
		return cfg.Scopes
	}
	return "openid profile email"
}

// Discover 拉取并返回 OIDC Discovery 文档。
func (c *Client) Discover(ctx context.Context) (*DiscoveryDocument, error) {
	u := strings.TrimSpace(c.Cfg.DiscoveryURL)
	if u == "" {
		base := strings.TrimSuffix(c.Cfg.Issuer, "/")
		u = base + "/.well-known/openid-configuration"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery: HTTP %d", resp.StatusCode)
	}
	var doc DiscoveryDocument
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	if doc.Issuer == "" || doc.AuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return nil, fmt.Errorf("oidc discovery: 文档缺少 issuer/authorization_endpoint/token_endpoint")
	}
	return &doc, nil
}

// BuildAuthorizationURL 构造 IdP 授权页地址；state 与 nonce 由调用方生成并持久化用于回调校验。
func (c *Client) BuildAuthorizationURL(doc *DiscoveryDocument, state, nonce string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", c.Cfg.ClientID)
	q.Set("redirect_uri", c.Cfg.RedirectURL)
	q.Set("scope", defaultScopes(c.Cfg))
	q.Set("state", state)
	if nonce != "" {
		q.Set("nonce", nonce)
	}
	if hint := strings.TrimSpace(c.Cfg.TenantHint); hint != "" {
		q.Set("login_hint", hint)
	}
	return doc.AuthorizationEndpoint + "?" + q.Encode()
}

// ExchangeCode 用授权码换取 Token。
func (c *Client) ExchangeCode(ctx context.Context, doc *DiscoveryDocument, code string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", c.Cfg.RedirectURL)
	form.Set("client_id", c.Cfg.ClientID)
	form.Set("client_secret", c.Cfg.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, doc.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oidc exchange: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc exchange: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("oidc exchange: %w", err)
	}
	if tr.IDToken == "" {
		return nil, fmt.Errorf("oidc exchange: IdP 未返回 id_token")
	}
	return &tr, nil
}

// VerifyConfig ID Token 验签与声明校验参数。
type VerifyConfig struct {
	ExpectedIssuer   string
	ExpectedAudience string
	ExpectedNonce    string
	ClientSecret     string // HS256 校验密钥
	JWKSKeys         map[string]crypto.PublicKey
}

// jwtSegment 解码 JWT 的一个 base64url 段。
func jwtSegment(seg string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(seg)
}

func parseRSAPublicJWK(nB64, eB64 string) (*rsa.PublicKey, error) {
	nRaw, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eRaw, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	eInt := 0
	for _, b := range eRaw {
		eInt = eInt<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nRaw), E: eInt}, nil
}

// ParseJWKS 解析 JWKS JSON（仅支持 RSA 的 n/e 表示，供 RS256 验签使用）。
func ParseJWKS(jwksJSON []byte) (map[string]crypto.PublicKey, error) {
	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(jwksJSON, &set); err != nil {
		return nil, fmt.Errorf("parse jwks: %w", err)
	}
	out := map[string]crypto.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := parseRSAPublicJWK(k.N, k.E)
		if err != nil {
			return nil, fmt.Errorf("parse jwks kid=%s: %w", k.Kid, err)
		}
		out[k.Kid] = pub
	}
	return out, nil
}

// ParsePEMPublicKey 解析 PEM 公钥（测试与运维兜底用）。
func ParsePEMPublicKey(pemData []byte) (crypto.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("pem: 空块")
	}
	return x509.ParsePKIXPublicKey(block.Bytes)
}

// VerifyIDToken 验签并校验 ID Token 声明（签名 → exp → iss → aud → nonce）。
func (c *Client) VerifyIDToken(rawToken string, vc VerifyConfig) (*IDTokenClaims, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("oidc: id_token 不是三段式 JWT")
	}
	headerRaw, err := jwtSegment(parts[0])
	if err != nil {
		return nil, fmt.Errorf("oidc: id_token header 解码失败")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return nil, fmt.Errorf("oidc: id_token header 解析失败")
	}
	if header.Alg == "none" || header.Alg == "" {
		return nil, fmt.Errorf("oidc: 拒绝 alg=none 的 id_token")
	}

	signed := []byte(parts[0] + "." + parts[1])
	sig, err := jwtSegment(parts[2])
	if err != nil {
		return nil, fmt.Errorf("oidc: id_token 签名解码失败")
	}

	switch header.Alg {
	case "HS256":
		if vc.ClientSecret == "" {
			return nil, fmt.Errorf("oidc: HS256 需要 client_secret")
		}
		mac := hmac.New(sha256.New, []byte(vc.ClientSecret))
		mac.Write(signed)
		if !hmac.Equal(mac.Sum(nil), sig) {
			return nil, fmt.Errorf("oidc: id_token HS256 签名不匹配")
		}
	case "RS256":
		var pub crypto.PublicKey
		if k, ok := vc.JWKSKeys[header.Kid]; ok {
			pub = k
		} else if len(vc.JWKSKeys) == 1 {
			for _, v := range vc.JWKSKeys {
				pub = v
			}
		} else {
			return nil, fmt.Errorf("oidc: 找不到 kid=%q 的 JWKS 公钥", header.Kid)
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("oidc: JWKS 公钥不是 RSA")
		}
		hashed := sha256.Sum256(signed)
		if err := rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, hashed[:], sig); err != nil {
			return nil, fmt.Errorf("oidc: id_token RS256 签名不匹配: %v", err)
		}
	default:
		return nil, fmt.Errorf("oidc: 不支持的签名算法 %q", header.Alg)
	}

	payloadRaw, err := jwtSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oidc: id_token payload 解码失败")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(payloadRaw, &raw); err != nil {
		return nil, fmt.Errorf("oidc: id_token payload 解析失败")
	}
	claims, err := normalizeClaims(raw)
	if err != nil {
		return nil, err
	}
	now := c.now().Unix()
	if claims.Exp <= now {
		return nil, fmt.Errorf("oidc: id_token 已过期")
	}
	if vc.ExpectedIssuer != "" && claims.Iss != vc.ExpectedIssuer {
		return nil, fmt.Errorf("oidc: issuer 不匹配")
	}
	if vc.ExpectedAudience != "" && claims.Aud != vc.ExpectedAudience {
		return nil, fmt.Errorf("oidc: audience 不匹配")
	}
	if vc.ExpectedNonce != "" && claims.Nonce != vc.ExpectedNonce {
		return nil, fmt.Errorf("oidc: nonce 不匹配（防重放）")
	}
	return claims, nil
}

func normalizeClaims(raw map[string]interface{}) (*IDTokenClaims, error) {
	out := &IDTokenClaims{}
	var ok bool
	if out.Sub, ok = raw["sub"].(string); !ok || out.Sub == "" {
		return nil, fmt.Errorf("oidc: id_token 缺少 sub")
	}
	if out.Iss, ok = raw["iss"].(string); !ok {
		return nil, fmt.Errorf("oidc: id_token 缺少 iss")
	}
	if out.Email, _ = raw["email"].(string); out.Email == "" {
		out.Email, _ = raw["preferred_username"].(string)
	}
	if out.Name, _ = raw["name"].(string); out.Name == "" {
		out.Name = out.Email
	}
	out.Nonce, _ = raw["nonce"].(string) // 可选声明；无 nonce 时不拦截（兼容不签发 nonce 的 IdP）
	switch exp := raw["exp"].(type) {
	case float64:
		out.Exp = int64(exp)
	case json.Number:
		out.Exp, _ = exp.Int64()
	default:
		return nil, fmt.Errorf("oidc: id_token 缺少合法 exp")
	}
	switch aud := raw["aud"].(type) {
	case string:
		out.Aud = aud
	case []interface{}:
		if len(aud) == 0 {
			return nil, fmt.Errorf("oidc: id_token 缺少 aud")
		}
		first, _ := aud[0].(string)
		out.Aud = first
	default:
		return nil, fmt.Errorf("oidc: id_token 缺少合法 aud")
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
