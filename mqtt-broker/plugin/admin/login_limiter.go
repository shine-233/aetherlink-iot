// 文件用途：限制管理页登录失败频率，抵御针对 GMQTT_ADMIN_USERNAME/PASSWORD 的在线爆破。
// 核心逻辑：按客户端 IP（RemoteAddr，不信任 X-Forwarded-For 以免被伪造绕过）
// 在固定窗口内累计失败次数，超过阈值后拒绝该窗口内的后续登录尝试；成功登录清零。
// 关键注意事项：表规模有上限，满时先清理过期条目，仍满则拒绝新 IP 的尝试
// （fail-closed），避免分布式请求把内存撑爆或借此绕过限流。

package admin

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	loginMaxFailures   = 10
	loginFailureWindow = 15 * time.Minute
	loginLimiterMaxIPs = 4096
)

type loginAttempt struct {
	failures    int
	windowStart time.Time
}

type loginLimiter struct {
	now func() time.Time

	mu       sync.Mutex
	attempts map[string]*loginAttempt
}

func newLoginLimiter(now func() time.Time) *loginLimiter {
	if now == nil {
		now = time.Now
	}
	return &loginLimiter{now: now, attempts: make(map[string]*loginAttempt)}
}

func loginClientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// allow 报告该客户端当前是否允许尝试登录。
func (l *loginLimiter) allow(key string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.attempts[key]
	if !ok {
		if len(l.attempts) >= loginLimiterMaxIPs {
			l.pruneLocked(now)
			return len(l.attempts) < loginLimiterMaxIPs
		}
		return true
	}
	if now.Sub(a.windowStart) >= loginFailureWindow {
		delete(l.attempts, key)
		return true
	}
	return a.failures < loginMaxFailures
}

// recordFailure 记录一次失败登录。
func (l *loginLimiter) recordFailure(key string) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.attempts[key]
	if !ok || now.Sub(a.windowStart) >= loginFailureWindow {
		if !ok && len(l.attempts) >= loginLimiterMaxIPs {
			l.pruneLocked(now)
			if len(l.attempts) >= loginLimiterMaxIPs {
				return
			}
		}
		l.attempts[key] = &loginAttempt{failures: 1, windowStart: now}
		return
	}
	a.failures++
}

// recordSuccess 在登录成功后清除该客户端的失败计数。
func (l *loginLimiter) recordSuccess(key string) {
	l.mu.Lock()
	delete(l.attempts, key)
	l.mu.Unlock()
}

func (l *loginLimiter) pruneLocked(now time.Time) {
	for k, a := range l.attempts {
		if now.Sub(a.windowStart) >= loginFailureWindow {
			delete(l.attempts, k)
		}
	}
}
