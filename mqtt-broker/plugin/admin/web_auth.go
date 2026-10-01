// 文件用途：内置管理页的登录、登出、会话识别与 CSRF 校验（Admin 方法）。
// 核心逻辑：登录成功签发 HMAC 签名会话令牌写入 gmqtt_admin_session cookie
// （名称与 Secure/HttpOnly/SameSite=Lax/12h 属性保持不变）；登出在服务端吊销令牌；
// 登录表单携带签名 nonce 防跨站伪造登录；登出与 cookie 认证的非安全方法要求 CSRF 令牌。
// 安全职责：管理员凭据恒定时间比较且不因用户名不匹配而短路；登录失败按 IP 限流。

package admin

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"os"
	"time"
)

const (
	// EnvAdminUsername / EnvAdminPassword 是管理页登录凭据的环境变量。
	EnvAdminUsername = "GMQTT_ADMIN_USERNAME"
	EnvAdminPassword = "GMQTT_ADMIN_PASSWORD"

	// CSRFHeader 是 dashboard 脚本在 fetch 中附带 CSRF 令牌的请求头。
	CSRFHeader = "X-CSRF-Token"

	csrfFormField       = "csrf_token"
	loginNonceFormField = "login_nonce"

	// maxAuthFormBytes 限制登录/登出表单体积，避免 ParseForm 读入超大请求体。
	maxAuthFormBytes = 64 << 10
)

// ensureAuthState 惰性初始化会话签名器与登录限流器；Load 时会主动调用一次，
// 直接构造 Admin 的测试也能安全使用。测试可在首次调用前预置 sessionMgr / loginLimit。
func (a *Admin) ensureAuthState() error {
	a.authOnce.Do(func() {
		if a.sessionMgr == nil {
			a.sessionMgr, a.authInitErr = newSessionManager()
		}
		if a.loginLimit == nil {
			a.loginLimit = newLoginLimiter(nil)
		}
	})
	return a.authInitErr
}

// sessionFromRequest 解析并校验请求携带的会话 cookie。
func (a *Admin) sessionFromRequest(r *http.Request) (sessionClaims, bool) {
	if a.ensureAuthState() != nil {
		return sessionClaims{}, false
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return sessionClaims{}, false
	}
	claims, err := a.sessionMgr.verifySession(cookie.Value)
	if err != nil {
		return sessionClaims{}, false
	}
	return claims, true
}

func (a *Admin) serveLoginPage(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	if _, ok := a.sessionFromRequest(r); ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	if err := a.ensureAuthState(); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	nonce, err := a.sessionMgr.issueLoginNonce()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeLoginPage(w, r.URL.Query().Get(loginErrorParam), nonce)
}

func (a *Admin) serveDashboardPage(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	claims, ok := a.sessionFromRequest(r)
	if !ok {
		http.Redirect(w, r, "/?"+loginErrorParam+"="+loginErrorRequired, http.StatusSeeOther)
		return
	}
	writeDashboardPage(w, a.sessionMgr.csrfToken(claims))
}

func (a *Admin) handleLogin(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	if err := a.ensureAuthState(); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	clientKey := loginClientKey(r)
	if !a.loginLimit.allow(clientKey) {
		redirectLoginError(w, r, loginErrorThrottled)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthFormBytes)
	if err := r.ParseForm(); err != nil {
		redirectLoginError(w, r, loginErrorForm)
		return
	}
	// 只读请求体字段：凭据不应出现在 URL 查询串（会进访问日志）。
	if !a.sessionMgr.validLoginNonce(r.PostFormValue(loginNonceFormField)) {
		redirectLoginError(w, r, loginErrorExpired)
		return
	}
	if !adminCredentialsMatch(r.PostFormValue("username"), r.PostFormValue("password")) {
		a.loginLimit.recordFailure(clientKey)
		redirectLoginError(w, r, loginErrorCredentials)
		return
	}
	a.loginLimit.recordSuccess(clientKey)

	// 防会话固定：登录时吊销请求中已有的会话，始终签发全新令牌。
	if previous, ok := a.sessionFromRequest(r); ok {
		a.sessionMgr.revoke(previous)
	}
	token, claims, err := a.sessionMgr.issueSession()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, token, claims.ExpiresAt)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (a *Admin) handleLogout(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	if claims, ok := a.sessionFromRequest(r); ok {
		r.Body = http.MaxBytesReader(w, r.Body, maxAuthFormBytes)
		_ = r.ParseForm()
		presented := r.Header.Get(CSRFHeader)
		if presented == "" {
			presented = r.PostFormValue(csrfFormField)
		}
		// 有效会话的登出必须带 CSRF 令牌，防止第三方页面强制登出。
		if !a.sessionMgr.validCSRF(claims, presented) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		a.sessionMgr.revoke(claims)
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func redirectLoginError(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, "/?"+loginErrorParam+"="+code, http.StatusSeeOther)
}

// adminCredentialsMatch 恒定时间比较管理员凭据：先做 SHA-256 摘要消除长度差异，
// 再对用户名、密码分别 ConstantTimeCompare 并按位与，避免按用户名短路泄露时序。
func adminCredentialsMatch(username, password string) bool {
	expectedUsername := os.Getenv(EnvAdminUsername)
	expectedPassword := os.Getenv(EnvAdminPassword)
	if expectedUsername == "" || expectedPassword == "" {
		return false
	}
	gotUser := sha256.Sum256([]byte(username))
	wantUser := sha256.Sum256([]byte(expectedUsername))
	gotPass := sha256.Sum256([]byte(password))
	wantPass := sha256.Sum256([]byte(expectedPassword))
	userOK := subtle.ConstantTimeCompare(gotUser[:], wantUser[:])
	passOK := subtle.ConstantTimeCompare(gotPass[:], wantPass[:])
	return userOK&passOK == 1
}

func setSessionCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
	})
}
