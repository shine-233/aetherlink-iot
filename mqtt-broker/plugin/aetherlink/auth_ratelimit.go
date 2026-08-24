// 文件用途：MQTT 认证失败的按来源 IP 滑动窗口限速，缓解 CONNECT 暴力破解。
// 核心逻辑：仅统计认证失败（成功不计数）；同一来源 IP 在窗口内失败达到上限后，
//
//	该 IP 的后续 CONNECT 在进入认证链路前直接拒绝；被拒绝的尝试不再计数，
//	避免持续轰炸把封禁无限延长。
//
// 关键注意事项：限速器为进程内存态（重启即清零），key 为可解析的来源 IP；
//
//	无法解析来源的连接不做归因也不参与统计，避免把所有未知来源聚成一个 key 互相误伤。
//	map 通过惰性清理（访问时裁剪 + 周期性全量清扫）防止内存无界增长。

package aetherlink

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/DrmagicE/gmqtt/server"
	"github.com/spf13/viper"
)

const (
	// authRateLimitConfigKey 是 aetherlink.yml 中覆盖“每 IP 每分钟认证失败上限”的配置键，
	// 同时绑定 GMQTT_AUTH_RATELIMIT_MAX_FAILURES_PER_MINUTE 环境变量（见 plugin.go BindEnv）。
	authRateLimitConfigKey = "auth_ratelimit.max_failures_per_minute"
	// defaultAuthRateLimitMaxFailuresPerMinute 是配置缺省或非法值时的兜底上限。
	defaultAuthRateLimitMaxFailuresPerMinute = 30
	// authFailureWindow 是滑动统计窗口长度。
	authFailureWindow = time.Minute
	// authFailureGCInterval 是失效 key 全量清扫的最小间隔。
	authFailureGCInterval = time.Minute
)

// errMQTTAuthRateLimited 在入口检查拒绝超限 IP 时返回，由 GMQTT 翻译成认证失败的 CONNACK。
var errMQTTAuthRateLimited = errors.New("mqtt auth rate limited")

// mqttAuthRateLimitMaxFailures 读取插件配置中的失败上限；未配置、非数字或 <=0 时回退默认值。
func mqttAuthRateLimitMaxFailures() int {
	if n := viper.GetInt(authRateLimitConfigKey); n > 0 {
		return n
	}
	return defaultAuthRateLimitMaxFailuresPerMinute
}

// authFailureLimiter 是单进程内的认证失败滑动窗口限速器。零值不可用，须用 newAuthFailureLimiter 构造。
type authFailureLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	gcEvery  time.Duration
	now      func() time.Time
	failures map[string][]time.Time
	lastGC   time.Time
}

// newAuthFailureLimiter 构造限速器。max<=0 时回退默认上限；now 可注入以便测试推进时间。
func newAuthFailureLimiter(max int, window time.Duration, gcEvery time.Duration, now func() time.Time) *authFailureLimiter {
	if max <= 0 {
		max = defaultAuthRateLimitMaxFailuresPerMinute
	}
	if now == nil {
		now = time.Now
	}
	return &authFailureLimiter{
		max:      max,
		window:   window,
		gcEvery:  gcEvery,
		now:      now,
		failures: make(map[string][]time.Time),
	}
}

// allow 报告该来源 IP 当前是否允许发起一次认证尝试（窗口内失败数未达上限）。
// 空 key 表示无法解析来源：始终放行且不参与统计。
func (l *authFailureLimiter) allow(ip string) bool {
	if ip == "" {
		return true
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.pruneLocked(ip, now)) < l.max
}

// record 记录一次认证失败；空 key 直接忽略。顺带触发惰性的过期 key 清扫。
func (l *authFailureLimiter) record(ip string) {
	if ip == "" {
		return
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gcLocked(now)
	l.failures[ip] = append(l.pruneLocked(ip, now), now)
}

// cleanup 强制执行一轮全量过期清扫并返回剩余 key 数，主要供测试与运维观察使用。
func (l *authFailureLimiter) cleanup() int {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gcLocked(now)
	return len(l.failures)
}

// pruneLocked 返回该 IP 窗口内的有效失败时间戳（已写回 map），调用方必须持锁。
func (l *authFailureLimiter) pruneLocked(ip string, now time.Time) []time.Time {
	ts := l.failures[ip]
	valid := ts[:0]
	for _, t := range ts {
		if now.Sub(t) < l.window {
			valid = append(valid, t)
		}
	}
	if len(valid) == 0 {
		delete(l.failures, ip)
		return nil
	}
	l.failures[ip] = valid
	return valid
}

// gcLocked 周期性清扫所有已过期的 key，防止长期运行时 map 无界增长；调用方必须持锁。
func (l *authFailureLimiter) gcLocked(now time.Time) {
	if !l.lastGC.IsZero() && now.Sub(l.lastGC) < l.gcEvery {
		return
	}
	l.lastGC = now
	for ip, ts := range l.failures {
		valid := ts[:0]
		for _, t := range ts {
			if now.Sub(t) < l.window {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(l.failures, ip)
			continue
		}
		l.failures[ip] = valid
	}
}

// mqttAuthRemoteIP 从 hook 上下文的 client 连接中解析来源 IP（去掉端口）。
// client/连接/地址缺失或无法解析时返回空串，表示“无法归因”，限速器对其直接放行。
func mqttAuthRemoteIP(client server.Client) string {
	if client == nil {
		return ""
	}
	conn := client.Connection()
	if conn == nil {
		return ""
	}
	addr := conn.RemoteAddr()
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil || host == "" {
		return ""
	}
	return host
}

// mqttAuthRateLimiter 是认证钩子共享的包级限速器实例，上限来自插件配置（默认 30）。
var mqttAuthRateLimiter = newAuthFailureLimiter(
	mqttAuthRateLimitMaxFailures(),
	authFailureWindow,
	authFailureGCInterval,
	time.Now,
)
