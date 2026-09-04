// 文件用途：OIDC/SSO 服务层（ROADMAP C7 剩余）——租户 IdP 配置 CRUD + 登录落地。
// 核心链路：SSO start/callback 路由按 :providerId 读取启用配置，构造 internal/oidc 中间件
//
//	（state+nonce cookie、授权码换 token、ID Token 验签），验签后经 SessionIssuer 按 email
//	绑定本地用户并签发本地 JWT，302 回前端携带 token；未匹配本地账号则回前端报错，不自动开户。
package service

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/oidc"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-basic/uuid"
	"gorm.io/gorm"
)

// OidcSso SSO 服务聚合入口。
type OidcSso struct{}

// OidcProviderReq 提供方配置入参。
type OidcProviderReq struct {
	ID               string `json:"id"`
	Name             string `json:"name" binding:"required"`
	Issuer           string `json:"issuer" binding:"required"`
	ClientID         string `json:"client_id" binding:"required"`
	ClientSecret     string `json:"client_secret"`
	DiscoveryURL     string `json:"discovery_url"`
	Scopes           string `json:"scopes"`
	FrontendRedirect string `json:"frontend_redirect" binding:"required"`
	Enabled          *bool  `json:"enabled"`
}

func normalizeOidcScopes(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "openid profile email"
	}
	return s
}

// Create 新建租户 IdP 提供方。
func (*OidcSso) Create(claims *utils.UserClaims, req *OidcProviderReq) (*model.OidcProvider, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	p := &model.OidcProvider{
		ID:               uuid.New(),
		TenantID:         claims.TenantID,
		Name:             strings.TrimSpace(req.Name),
		Issuer:           strings.TrimSpace(req.Issuer),
		ClientID:         strings.TrimSpace(req.ClientID),
		ClientSecret:     req.ClientSecret,
		DiscoveryURL:     strings.TrimSpace(req.DiscoveryURL),
		Scopes:           normalizeOidcScopes(req.Scopes),
		FrontendRedirect: strings.TrimSpace(req.FrontendRedirect),
		Enabled:          enabled,
	}
	if err := dal.CreateOidcProvider(p); err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	return p, nil
}

// ListPublic 登录页可发现提供方（仅平台级启用项；不返回任何敏感字段）。
func (*OidcSso) ListPublic() ([]*model.OidcProvider, error) {
	list, err := dal.ListPublicOidcProviders()
	if err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	for _, p := range list {
		p.ClientSecret = ""
		p.Issuer = ""
		p.ClientID = ""
	}
	return list, nil
}

// List 列出当前租户的提供方（脱敏：不返回 client_secret）。
func (*OidcSso) List(claims *utils.UserClaims) ([]*model.OidcProvider, error) {
	list, err := dal.ListOidcProvidersByTenant(claims.TenantID)
	if err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	for _, p := range list {
		p.ClientSecret = ""
	}
	return list, nil
}

// Update 更新当前租户提供方（client_secret 为空时不覆盖旧值）。
func (*OidcSso) Update(claims *utils.UserClaims, req *OidcProviderReq) (*model.OidcProvider, error) {
	if strings.TrimSpace(req.ID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "缺少 provider id")
	}
	exist, err := dal.GetOidcProviderOwned(req.ID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.CodeNotFound)
		}
		return nil, errcode.New(errcode.CodeDBError)
	}
	enabled := exist.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	secret := strings.TrimSpace(req.ClientSecret)
	if secret == "" {
		secret = exist.ClientSecret
	}
	upd := &model.OidcProvider{
		ID:               req.ID,
		TenantID:         claims.TenantID,
		Name:             strings.TrimSpace(req.Name),
		Issuer:           strings.TrimSpace(req.Issuer),
		ClientID:         strings.TrimSpace(req.ClientID),
		ClientSecret:     secret,
		DiscoveryURL:     strings.TrimSpace(req.DiscoveryURL),
		Scopes:           normalizeOidcScopes(req.Scopes),
		FrontendRedirect: strings.TrimSpace(req.FrontendRedirect),
		Enabled:          enabled,
	}
	ok, err := dal.UpdateOidcProvider(upd)
	if err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	if !ok {
		return nil, errcode.New(errcode.CodeNotFound)
	}
	got, err := dal.GetOidcProviderOwned(req.ID, claims.TenantID)
	if err != nil {
		return nil, errcode.New(errcode.CodeDBError)
	}
	got.ClientSecret = ""
	return got, nil
}

// Delete 删除当前租户提供方。
func (*OidcSso) Delete(claims *utils.UserClaims, id string) error {
	if err := dal.DeleteOidcProvider(id, claims.TenantID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.CodeNotFound)
		}
		return errcode.New(errcode.CodeDBError)
	}
	return nil
}

// ssoMiddlewareFor 由 provider 构造 oidc 中间件，并绑定本地账号签发会话的 issuer。
func (*OidcSso) ssoMiddlewareFor(p *model.OidcProvider) *oidc.Middleware {
	cfg := oidc.ProviderConfig{
		Issuer:       p.Issuer,
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		DiscoveryURL: p.DiscoveryURL,
		Scopes:       normalizeOidcScopes(p.Scopes),
		RedirectURL:  ssoCallbackURL(p.ID),
	}
	id := p.ID
	mw := oidc.NewMiddleware(cfg, func(c *gin.Context, profile oidc.Profile) (string, error) {
		front := strings.TrimSpace(p.FrontendRedirect)
		notFoundURL := front + ssoErrorQuery("account_not_matched")
		if profile.Email == "" {
			return notFoundURL, nil
		}
		user, err := dal.GetUsersByEmail(profile.Email)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return notFoundURL, nil
			}
			return front + ssoErrorQuery("internal_error"), nil
		}
		if *user.Status != "N" {
			return front + ssoErrorQuery("user_disabled"), nil
		}
		// 租户归属约束：提供方归属的租户必须与本地用户租户一致（平台级提供方不限）。
		if p.TenantID != "" && user.TenantID != nil && *user.TenantID != p.TenantID {
			return front + ssoErrorQuery("tenant_mismatch"), nil
		}
		rsp, err := GroupApp.User.UserLoginAfter(user)
		if err != nil {
			return front + ssoErrorQuery("token_error"), nil
		}
		sep := "?"
		if strings.Contains(front, "?") {
			sep = "&"
		}
		token := ""
		expires := int64(0)
		if rsp != nil && rsp.Token != nil {
			token = *rsp.Token
			expires = rsp.ExpiresIn
		}
		return front + sep + "token=" + url.QueryEscape(token) + "&expires_in=" + url.QueryEscape(strconv.FormatInt(expires, 10)) + "&email=" + url.QueryEscape(profile.Email), nil
	})
	mw.StateCookie = "oidc_state_" + id
	mw.StartPath = "/sso/" + id + "/start"
	mw.CallbackPath = "/sso/" + id + "/callback"
	return mw
}

func ssoErrorQuery(kind string) string {
	return "?sso_error=" + url.QueryEscape(kind)
}

// ssoCallbackURL 回调地址由外部反向代理决定域名，此处返回平台内相对回调路径；
// IdP 侧 redirect_uri 需与部署时完整地址保持一致（见部署文档/README 配置示例）。
func ssoCallbackURL(providerID string) string {
	return "/api/v1/sso/" + providerID + "/callback"
}

// HandleSSO 路由处理器：区分 start / callback，动态装配 provider 中间件。
// 修复（2026-09-03 隔离栈回归）：start/callback 前缀需带 v1 组前缀（/api/v1），
// 否则 middleware 前缀永不匹配而落入默认分支；此处用真实请求路径覆盖两个匹配前缀。
// GET /sso/:id/start | /sso/:id/callback
func (*OidcSso) HandleSSO(c *gin.Context) {
	id := c.Param("id")
	provider, err := dal.GetOidcProviderByID(id)
	if err != nil {
		c.AbortWithStatusJSON(404, gin.H{"code": 404, "message": "SSO provider not found or disabled"})
		return
	}
	mw := GroupApp.OidcSso.ssoMiddlewareFor(provider)
	reqPath := c.Request.URL.Path
	if strings.HasSuffix(reqPath, "/start") {
		mw.StartPath = reqPath
		mw.CallbackPath = reqPath + "?not-a-callback" // 保证 callback 分支不误匹配
	} else {
		mw.CallbackPath = reqPath
		mw.StartPath = reqPath + "?not-a-start"
	}
	mw.Handle(c)
	if c.Writer.Written() {
		c.Abort()
		return
	}
	c.AbortWithStatusJSON(404, gin.H{"code": 404, "message": "unknown sso route"})
}
