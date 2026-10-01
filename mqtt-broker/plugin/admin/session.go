// 文件用途：为内置管理页提供不可伪造、可过期、可吊销的会话令牌与 CSRF 令牌。
// 核心逻辑：令牌 = base64url(载荷) + "." + base64url(HMAC-SHA256(key, 用途 || 载荷))，
// 载荷含版本、签发/过期时间与 16 字节随机会话 ID；HMAC 密钥在进程内由 crypto/rand 生成，
// 进程重启即令全部会话失效（运维控制台可接受）。
// 安全职责：替换历史上的常量 cookie 值 "authenticated"——该值可被任意客户端伪造，
// 并经历史 cookie 回退路径绕过 http_auth_secret。
// 关键注意事项：不同用途（会话 / CSRF / 登录表单 nonce）以 purpose 前缀做域分离，
// 一种令牌不能被当作另一种使用；吊销集合按过期时间自动清理，规模受合法登录次数约束。

package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	// sessionTTL 与历史 cookie Expires 保持一致（12h）。
	sessionTTL = 12 * time.Hour
	// loginNonceTTL 是登录页表单 nonce 的有效期。
	loginNonceTTL = 30 * time.Minute
	// tokenClockSkew 容忍签发时间略晚于本机时钟的情况。
	tokenClockSkew = time.Minute

	tokenVersion   byte = 1
	tokenIDLen          = 16
	tokenPayloadSz      = 1 + 8 + 8 + tokenIDLen

	purposeSession    = "gmqtt-admin/session/v1"
	purposeLoginNonce = "gmqtt-admin/login-nonce/v1"
	purposeCSRF       = "gmqtt-admin/csrf/v1"
)

var (
	errTokenMalformed = errors.New("malformed token")
	errTokenSignature = errors.New("invalid token signature")
	errTokenExpired   = errors.New("token expired")
	errTokenRevoked   = errors.New("token revoked")
)

var tokenEncoding = base64.RawURLEncoding

// sessionClaims 是验签通过后的令牌内容。
type sessionClaims struct {
	ID        [tokenIDLen]byte
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// sessionManager 负责签发、校验、吊销管理页令牌。并发安全。
type sessionManager struct {
	key []byte
	now func() time.Time

	mu      sync.Mutex
	revoked map[[tokenIDLen]byte]time.Time // 会话 ID -> 原过期时间，过期后清理
}

// newSessionManager 以 crypto/rand 生成 32 字节 HMAC 密钥。
func newSessionManager() (*sessionManager, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return newSessionManagerWithKey(key, time.Now), nil
}

func newSessionManagerWithKey(key []byte, now func() time.Time) *sessionManager {
	if now == nil {
		now = time.Now
	}
	return &sessionManager{
		key:     append([]byte(nil), key...),
		now:     now,
		revoked: make(map[[tokenIDLen]byte]time.Time),
	}
}

func (m *sessionManager) mac(purpose string, payload []byte) []byte {
	h := hmac.New(sha256.New, m.key)
	_, _ = h.Write([]byte(purpose))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(payload)
	return h.Sum(nil)
}

// issue 签发指定用途、有效期的令牌。
func (m *sessionManager) issue(purpose string, ttl time.Duration) (string, sessionClaims, error) {
	var claims sessionClaims
	if _, err := rand.Read(claims.ID[:]); err != nil {
		return "", sessionClaims{}, err
	}
	now := m.now()
	claims.IssuedAt = time.Unix(now.Unix(), 0)
	claims.ExpiresAt = time.Unix(now.Add(ttl).Unix(), 0)

	payload := make([]byte, tokenPayloadSz)
	payload[0] = tokenVersion
	binary.BigEndian.PutUint64(payload[1:9], uint64(claims.IssuedAt.Unix()))
	binary.BigEndian.PutUint64(payload[9:17], uint64(claims.ExpiresAt.Unix()))
	copy(payload[17:], claims.ID[:])

	token := tokenEncoding.EncodeToString(payload) + "." + tokenEncoding.EncodeToString(m.mac(purpose, payload))
	return token, claims, nil
}

// verify 校验令牌签名、版本与有效期，不检查吊销状态。
func (m *sessionManager) verify(purpose, token string) (sessionClaims, error) {
	encPayload, encSig, ok := strings.Cut(token, ".")
	if !ok || encPayload == "" || encSig == "" {
		return sessionClaims{}, errTokenMalformed
	}
	payload, err := tokenEncoding.DecodeString(encPayload)
	if err != nil || len(payload) != tokenPayloadSz {
		return sessionClaims{}, errTokenMalformed
	}
	sig, err := tokenEncoding.DecodeString(encSig)
	if err != nil {
		return sessionClaims{}, errTokenMalformed
	}
	// 先验签再解析字段，避免对未认证数据做任何语义判断。
	if !hmac.Equal(sig, m.mac(purpose, payload)) {
		return sessionClaims{}, errTokenSignature
	}
	if payload[0] != tokenVersion {
		return sessionClaims{}, errTokenMalformed
	}
	var claims sessionClaims
	claims.IssuedAt = time.Unix(int64(binary.BigEndian.Uint64(payload[1:9])), 0)
	claims.ExpiresAt = time.Unix(int64(binary.BigEndian.Uint64(payload[9:17])), 0)
	copy(claims.ID[:], payload[17:])

	now := m.now()
	if claims.IssuedAt.After(now.Add(tokenClockSkew)) || !now.Before(claims.ExpiresAt) {
		return sessionClaims{}, errTokenExpired
	}
	return claims, nil
}

// issueSession 签发会话令牌。
func (m *sessionManager) issueSession() (string, sessionClaims, error) {
	return m.issue(purposeSession, sessionTTL)
}

// verifySession 校验会话令牌并检查服务端吊销集合。
func (m *sessionManager) verifySession(token string) (sessionClaims, error) {
	claims, err := m.verify(purposeSession, token)
	if err != nil {
		return sessionClaims{}, err
	}
	m.mu.Lock()
	_, revoked := m.revoked[claims.ID]
	m.mu.Unlock()
	if revoked {
		return sessionClaims{}, errTokenRevoked
	}
	return claims, nil
}

// revoke 吊销会话，使 /logout 后同一令牌在服务端失效，而不仅是清理浏览器 cookie。
func (m *sessionManager) revoke(claims sessionClaims) {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, exp := range m.revoked {
		if !now.Before(exp) {
			delete(m.revoked, id)
		}
	}
	if now.Before(claims.ExpiresAt) {
		m.revoked[claims.ID] = claims.ExpiresAt
	}
}

// csrfToken 由会话 ID 派生 CSRF 令牌（同步令牌模式，无需服务端存储）。
// 会话 cookie 为 HttpOnly，脚本无法读取，CSRF 令牌由服务端渲染进 dashboard 页面。
func (m *sessionManager) csrfToken(claims sessionClaims) string {
	return tokenEncoding.EncodeToString(m.mac(purposeCSRF, claims.ID[:]))
}

// validCSRF 恒定时间比较请求携带的 CSRF 令牌。
func (m *sessionManager) validCSRF(claims sessionClaims, presented string) bool {
	if presented == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(m.csrfToken(claims))) == 1
}

// issueLoginNonce 为登录表单签发短期 nonce，阻止跨站伪造登录（login CSRF）。
func (m *sessionManager) issueLoginNonce() (string, error) {
	token, _, err := m.issue(purposeLoginNonce, loginNonceTTL)
	return token, err
}

func (m *sessionManager) validLoginNonce(token string) bool {
	_, err := m.verify(purposeLoginNonce, token)
	return err == nil
}
