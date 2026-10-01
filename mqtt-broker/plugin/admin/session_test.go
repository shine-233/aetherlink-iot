// 文件用途：验证管理页会话令牌不可伪造、可过期、可吊销，以及 CSRF / 登录 nonce / 限流契约。
// 安全职责：回归保护 "Cookie: gmqtt_admin_session=authenticated" 绕过 http_auth_secret 的漏洞。

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

const testAdminSecret = "s3cret"

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// newAdminUITestServer 组装真实的 gateway mux：管理页路由 + 两个替身 API，
// 外层包上共享密钥中间件，与 broker 运行时的装配方式一致。
func newAdminUITestServer(t *testing.T, clock *fakeClock) (*Admin, http.Handler) {
	t.Helper()
	t.Setenv(EnvAdminUsername, "operator")
	t.Setenv(EnvAdminPassword, "change-me")

	a := &Admin{httpAuthSecret: testAdminSecret}
	if clock != nil {
		a.sessionMgr = newSessionManagerWithKey([]byte("0123456789abcdef0123456789abcdef"), clock.Now)
		a.loginLimit = newLoginLimiter(clock.Now)
	}
	if err := a.ensureAuthState(); err != nil {
		t.Fatalf("ensureAuthState: %v", err)
	}
	mux := runtime.NewServeMux()
	if err := a.registerAdminUI(context.Background(), mux, "", nil); err != nil {
		t.Fatalf("registerAdminUI: %v", err)
	}
	ok := func(w http.ResponseWriter, r *http.Request, _ map[string]string) { w.WriteHeader(http.StatusOK) }
	if err := handleStaticPath(mux, http.MethodGet, "/v1/clients", ok); err != nil {
		t.Fatal(err)
	}
	if err := handleStaticPath(mux, http.MethodPost, "/v1/publish", ok); err != nil {
		t.Fatal(err)
	}
	if err := handleStaticPath(mux, http.MethodDelete, "/v1/clients/c1", ok); err != nil {
		t.Fatal(err)
	}
	return a, a.adminSecretMiddleware()(mux)
}

type reqOpt func(*http.Request)

func withCookie(v string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: v}) }
}

func withHeader(k, v string) reqOpt { return func(r *http.Request) { r.Header.Set(k, v) } }

func do(h http.Handler, method, target string, form url.Values, opts ...reqOpt) *httptest.ResponseRecorder {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	for _, o := range opts {
		o(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

var (
	loginNonceRe = regexp.MustCompile(`name="login_nonce" value="([^"]+)"`)
	csrfMetaRe   = regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)">`)
)

func sessionCookieFrom(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

// login 走完整浏览器流程：GET / 取 nonce -> POST /login，返回会话 cookie 值。
func login(t *testing.T, h http.Handler) string {
	t.Helper()
	page := do(h, http.MethodGet, "/", nil)
	if page.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200 (login page must bypass shared-secret middleware)", page.Code)
	}
	m := loginNonceRe.FindStringSubmatch(page.Body.String())
	if m == nil {
		t.Fatal("login page does not render a login nonce")
	}
	rec := do(h, http.MethodPost, "/login", url.Values{
		"username": {"operator"}, "password": {"change-me"}, loginNonceFormField: {m[1]},
	})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/dashboard" {
		t.Fatalf("login = %d %q, want 303 /dashboard", rec.Code, rec.Header().Get("Location"))
	}
	token := sessionCookieFrom(t, rec)
	if token == "" {
		t.Fatal("login did not set a session cookie")
	}
	return token
}

func dashboardCSRF(t *testing.T, h http.Handler, token string) string {
	t.Helper()
	rec := do(h, http.MethodGet, "/dashboard", nil, withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /dashboard = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("dashboard Cache-Control = %q, want no-store", got)
	}
	m := csrfMetaRe.FindStringSubmatch(rec.Body.String())
	if m == nil {
		t.Fatal("dashboard does not render csrf-token meta")
	}
	if !strings.Contains(rec.Body.String(), `name="csrf_token" value="`+m[1]+`"`) {
		t.Fatal("logout form does not carry csrf_token")
	}
	return m[1]
}

// 回归：历史常量 cookie 值在配置共享密钥时必须被拒绝。
func TestForgedConstantSessionCookieIsRejected(t *testing.T) {
	_, h := newAdminUITestServer(t, nil)
	for _, target := range []string{"/v1/clients"} {
		rec := do(h, http.MethodGet, target, nil, withCookie("authenticated"))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s with forged cookie = %d, want 401", target, rec.Code)
		}
	}
	rec := do(h, http.MethodPost, "/v1/publish", nil, withCookie("authenticated"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /v1/publish with forged cookie = %d, want 401", rec.Code)
	}
	rec = do(h, http.MethodDelete, "/v1/clients/c1", nil, withCookie("authenticated"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("DELETE with forged cookie = %d, want 401", rec.Code)
	}
	rec = do(h, http.MethodGet, "/dashboard", nil, withCookie("authenticated"))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("GET /dashboard with forged cookie = %d, want 303 to login", rec.Code)
	}
}

func TestSessionTokenTamperingIsRejected(t *testing.T) {
	a, h := newAdminUITestServer(t, nil)
	token := login(t, h)
	payload, sig, _ := strings.Cut(token, ".")

	// 修改载荷任意一字节（例如延长过期时间）后签名不再匹配。
	raw, err := tokenEncoding.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}
	raw[10] ^= 0xff
	tamperedPayload := tokenEncoding.EncodeToString(raw) + "." + sig

	// 其他密钥签发的令牌同样无效（例如进程重启后）。
	other := newSessionManagerWithKey([]byte("another-key"), nil)
	foreign, _, _ := other.issueSession()

	cases := map[string]string{
		"tampered payload":   tamperedPayload,
		"tampered signature": payload + "." + strings.Repeat("A", len(sig)),
		"truncated":          payload,
		"foreign key":        foreign,
		"garbage":            "!!!.???",
	}
	for name, v := range cases {
		if rec := do(h, http.MethodGet, "/v1/clients", nil, withCookie(v)); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d, want 401", name, rec.Code)
		}
	}
	if _, err := a.sessionMgr.verifySession(tamperedPayload); err != errTokenSignature {
		t.Fatalf("tampered payload err = %v, want errTokenSignature", err)
	}
	if rec := do(h, http.MethodGet, "/v1/clients", nil, withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("valid session status = %d, want 200", rec.Code)
	}
}

func TestSessionTokenPurposeSeparation(t *testing.T) {
	a, h := newAdminUITestServer(t, nil)
	nonce, err := a.sessionMgr.issueLoginNonce()
	if err != nil {
		t.Fatal(err)
	}
	// 登录 nonce 是签名令牌，但用途不同，不能当作会话 cookie。
	if rec := do(h, http.MethodGet, "/v1/clients", nil, withCookie(nonce)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("login nonce as session = %d, want 401", rec.Code)
	}
	// 反之会话令牌也不能当作登录 nonce。
	token, _, _ := a.sessionMgr.issueSession()
	if a.sessionMgr.validLoginNonce(token) {
		t.Fatal("session token must not validate as login nonce")
	}
}

func TestSessionTokenExpires(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	_, h := newAdminUITestServer(t, clock)
	token := login(t, h)

	clock.Advance(sessionTTL - time.Second)
	if rec := do(h, http.MethodGet, "/v1/clients", nil, withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("before expiry status = %d, want 200", rec.Code)
	}
	clock.Advance(time.Second)
	if rec := do(h, http.MethodGet, "/v1/clients", nil, withCookie(token)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after expiry status = %d, want 401", rec.Code)
	}
}

func TestLoginNonceRequiredAndExpires(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	a, h := newAdminUITestServer(t, clock)
	creds := url.Values{"username": {"operator"}, "password": {"change-me"}}

	// 跨站伪造登录：第三方页面拿不到 nonce。
	rec := do(h, http.MethodPost, "/login", creds)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), loginErrorExpired) {
		t.Fatalf("login without nonce = %d %q, want redirect error=expired", rec.Code, rec.Header().Get("Location"))
	}
	if sessionCookieFrom(t, rec) != "" {
		t.Fatal("login without nonce must not set a session")
	}

	nonce, _ := a.sessionMgr.issueLoginNonce()
	clock.Advance(loginNonceTTL)
	creds.Set(loginNonceFormField, nonce)
	rec = do(h, http.MethodPost, "/login", creds)
	if sessionCookieFrom(t, rec) != "" {
		t.Fatal("expired nonce must not yield a session")
	}

	// 凭据放在查询串里不被接受（只读请求体）。
	nonce, _ = a.sessionMgr.issueLoginNonce()
	rec = do(h, http.MethodPost, "/login?username=operator&password=change-me",
		url.Values{loginNonceFormField: {nonce}})
	if sessionCookieFrom(t, rec) != "" {
		t.Fatal("credentials in query string must not be accepted")
	}
}

func TestCookieAuthenticatedUnsafeMethodsRequireCSRF(t *testing.T) {
	_, h := newAdminUITestServer(t, nil)
	token := login(t, h)
	csrf := dashboardCSRF(t, h, token)

	// 跨站表单 / fetch 会带上 cookie，但拿不到 CSRF 令牌。
	rec := do(h, http.MethodPost, "/v1/publish", nil, withCookie(token), withHeader("Origin", "https://evil.example"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin cookie POST = %d, want 403", rec.Code)
	}
	if rec = do(h, http.MethodDelete, "/v1/clients/c1", nil, withCookie(token)); rec.Code != http.StatusForbidden {
		t.Fatalf("cookie DELETE without CSRF = %d, want 403", rec.Code)
	}
	if rec = do(h, http.MethodPost, "/v1/publish", nil, withCookie(token), withHeader(CSRFHeader, "wrong")); rec.Code != http.StatusForbidden {
		t.Fatalf("cookie POST with wrong CSRF = %d, want 403", rec.Code)
	}
	if rec = do(h, http.MethodPost, "/v1/publish", nil, withCookie(token), withHeader(CSRFHeader, csrf)); rec.Code != http.StatusOK {
		t.Fatalf("cookie POST with CSRF = %d, want 200", rec.Code)
	}

	// 另一会话的 CSRF 令牌不能复用。
	other := login(t, h)
	if rec = do(h, http.MethodPost, "/v1/publish", nil, withCookie(other), withHeader(CSRFHeader, csrf)); rec.Code != http.StatusForbidden {
		t.Fatalf("CSRF from another session = %d, want 403", rec.Code)
	}

	// 共享密钥调用方不受 CSRF 约束。
	if rec = do(h, http.MethodPost, "/v1/publish", nil, withHeader(AdminSecretHeader, testAdminSecret)); rec.Code != http.StatusOK {
		t.Fatalf("secret-header POST = %d, want 200", rec.Code)
	}
}

func TestLogoutRevokesSessionServerSide(t *testing.T) {
	_, h := newAdminUITestServer(t, nil)
	token := login(t, h)
	csrf := dashboardCSRF(t, h, token)

	// 没有 CSRF 的登出（第三方强制登出）被拒绝，会话保持有效。
	if rec := do(h, http.MethodPost, "/logout", url.Values{}, withCookie(token)); rec.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF = %d, want 403", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/v1/clients", nil, withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("session after rejected logout = %d, want 200", rec.Code)
	}

	rec := do(h, http.MethodPost, "/logout", url.Values{csrfFormField: {csrf}}, withCookie(token))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("logout = %d %q, want 303 /", rec.Code, rec.Header().Get("Location"))
	}
	// 即使客户端保留了旧 cookie，服务端也已吊销。
	if rec = do(h, http.MethodGet, "/v1/clients", nil, withCookie(token)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused cookie after logout = %d, want 401", rec.Code)
	}
	if rec = do(h, http.MethodGet, "/dashboard", nil, withCookie(token)); rec.Code != http.StatusSeeOther {
		t.Fatalf("dashboard after logout = %d, want 303", rec.Code)
	}
}

func TestLoginRotatesExistingSession(t *testing.T) {
	a, h := newAdminUITestServer(t, nil)
	old := login(t, h)
	nonce, _ := a.sessionMgr.issueLoginNonce()
	rec := do(h, http.MethodPost, "/login", url.Values{
		"username": {"operator"}, "password": {"change-me"}, loginNonceFormField: {nonce},
	}, withCookie(old))
	fresh := sessionCookieFrom(t, rec)
	if fresh == "" || fresh == old {
		t.Fatal("re-login must issue a fresh session token")
	}
	if rec = do(h, http.MethodGet, "/v1/clients", nil, withCookie(old)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("previous session after re-login = %d, want 401", rec.Code)
	}
}

func TestLoginFailuresAreThrottled(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	a, h := newAdminUITestServer(t, clock)
	attempt := func(password string) *httptest.ResponseRecorder {
		nonce, _ := a.sessionMgr.issueLoginNonce()
		return do(h, http.MethodPost, "/login", url.Values{
			"username": {"operator"}, "password": {password}, loginNonceFormField: {nonce},
		})
	}
	for i := 0; i < loginMaxFailures; i++ {
		attempt("wrong")
	}
	rec := attempt("change-me")
	if sessionCookieFrom(t, rec) != "" || !strings.Contains(rec.Header().Get("Location"), loginErrorThrottled) {
		t.Fatalf("login after %d failures = %q, want throttled without session", loginMaxFailures, rec.Header().Get("Location"))
	}
	clock.Advance(loginFailureWindow)
	if rec = attempt("change-me"); sessionCookieFrom(t, rec) == "" {
		t.Fatal("login should be allowed again after the failure window")
	}
}

func TestLoginLimiterBoundedTable(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	l := newLoginLimiter(clock.Now)
	for i := 0; i < loginLimiterMaxIPs+100; i++ {
		l.recordFailure("ip-" + strconv.Itoa(i))
	}
	if len(l.attempts) > loginLimiterMaxIPs {
		t.Fatalf("limiter table = %d entries, want <= %d", len(l.attempts), loginLimiterMaxIPs)
	}
	clock.Advance(loginFailureWindow)
	if !l.allow("fresh-ip") {
		t.Fatal("expired entries should be pruned to admit new clients")
	}
}

func TestRevocationSetIsPruned(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	m := newSessionManagerWithKey([]byte("k"), clock.Now)
	for i := 0; i < 50; i++ {
		_, claims, _ := m.issueSession()
		m.revoke(claims)
	}
	clock.Advance(sessionTTL)
	_, claims, _ := m.issueSession()
	m.revoke(claims)
	if len(m.revoked) != 1 {
		t.Fatalf("revoked set = %d, want 1 after expired entries are pruned", len(m.revoked))
	}
}

func TestAdminCredentialsMatchDoesNotShortCircuit(t *testing.T) {
	t.Setenv(EnvAdminUsername, "operator")
	t.Setenv(EnvAdminPassword, "change-me")
	cases := []struct {
		user, pass string
		want       bool
	}{
		{"operator", "change-me", true},
		{"operator", "wrong", false},
		{"wrong", "change-me", false},
		{"", "", false},
		{"operator", "change-me-longer", false},
	}
	for _, c := range cases {
		if got := adminCredentialsMatch(c.user, c.pass); got != c.want {
			t.Fatalf("adminCredentialsMatch(%q,%q) = %v, want %v", c.user, c.pass, got, c.want)
		}
	}
}
