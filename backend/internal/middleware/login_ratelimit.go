// 文件用途：为登录接口提供按客户端 IP 的失败限流，吸收针对登录入口的高频密码爆破。
// 核心逻辑：复用 apikey.go 的固定窗口失败计数模式——中间件在进入 handler 前拒绝已被
// 限流的来源；失败/成功计数由 Login handler 在认证结果出来后回写。
// 关键注意事项：进程内 sync.Map 最小实现，多副本部署时各副本独立计数（阈值按副本放宽即可）；
// 与按账号的 LoginLock（Redis、classified-protect 配置）互补：本层不依赖任何配置开关即默认生效。
// 重构建议：若未来出现第二个需要全局限流的敏感入口，可把固定窗口计数抽成通用 limiter。

package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrCodeLoginRateLimited 标识登录失败次数超限，与 jwt_auth.go 的 4010x 错误码段保持同段递增。
const ErrCodeLoginRateLimited = 40106

const (
	// loginIPFailLimit 是单 IP 在窗口内允许的登录失败次数上限。
	loginIPFailLimit = 10
	// loginIPFailWindow 是失败计数的固定时间窗口。
	loginIPFailWindow = time.Minute
)

type loginIPFailEntry struct {
	mu        sync.Mutex
	count     int
	windowEnd time.Time
}

// loginIPFailCounts 以客户端 IP 为键记录登录失败计数；
// 键空间受活跃来源 IP 约束，过期条目在下次访问时惰性重置，不做后台清扫。
var loginIPFailCounts sync.Map

// loginRateLimited 返回该 IP 当前是否已被登录限流。
func loginRateLimited(clientIP string) bool {
	if clientIP == "" {
		return false
	}
	value, ok := loginIPFailCounts.Load(clientIP)
	if !ok {
		return false
	}
	entry, ok := value.(*loginIPFailEntry)
	if !ok {
		return false
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if time.Now().After(entry.windowEnd) {
		return false
	}
	return entry.count >= loginIPFailLimit
}

// RecordLoginFailure 记录一次登录失败；窗口首次创建即开始计时，
// 过期后下次写入自动开新窗口。
func RecordLoginFailure(clientIP string) {
	if clientIP == "" {
		return
	}
	now := time.Now()
	actual, _ := loginIPFailCounts.LoadOrStore(clientIP, &loginIPFailEntry{windowEnd: now.Add(loginIPFailWindow)})
	entry, ok := actual.(*loginIPFailEntry)
	if !ok {
		return
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if now.After(entry.windowEnd) {
		entry.count = 0
		entry.windowEnd = now.Add(loginIPFailWindow)
	}
	entry.count++
}

// ResetLoginFailures 登录成功后清除该 IP 的失败计数，避免正常用户的偶发失误累积。
func ResetLoginFailures(clientIP string) {
	if clientIP == "" {
		return
	}
	loginIPFailCounts.Delete(clientIP)
}

// LoginRateLimit 在进入登录 handler 前检查该 IP 是否已因连续登录失败被限流。
// 超限时直接返回 429，不再触发账号锁定或凭证校验链路。
func LoginRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if loginRateLimited(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Code:      ErrCodeLoginRateLimited,
				Message:   "too many failed login attempts from this address, retry later",
				RequestID: c.GetString("X-Request-ID"),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
