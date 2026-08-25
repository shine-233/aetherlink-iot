// 文件用途：维护登录失败计数、锁定窗口和解锁判断服务。
// 核心逻辑：按账号维度记录失败次数、锁定 TTL 和重置状态，供认证流程阻断暴力尝试。
// 关键注意事项：锁定策略是安全边界，需避免 key 串扰、永久锁死和错误放行。
// 重构建议：引入时钟与存储接口，补齐并发失败、Redis 异常、管理员解锁和审计测试。
package service

import (
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type LoginLock struct {
	MaxFailedAttempts int64
	LockDuration      time.Duration
	// IP 维度防爆破（安全审计 F3）：账号维度锁定无法阻止"同一出口 IP 换账号轰炸"，
	// 也无法防止单账号被恶意批量锁定；IP 与账号双维度互补，任一命中即拒绝。
	IPMaxFailedAttempts int64
	IPWindowDuration    time.Duration
}

// loginLockShouldLock 判定本次失败后是否需要锁定账号。
// 配置约定（conf.yml）：login-max-fail-times <= 0 表示不限制登录失败次数，
// 此时永不锁定；只有正整数阈值才会触发 failedAttempts >= 阈值 的锁定判断。
func loginLockShouldLock(maxFailedAttempts, failedAttempts int64) bool {
	if maxFailedAttempts <= 0 {
		return false
	}
	return failedAttempts >= maxFailedAttempts
}

// 获取登录锁定规则
func NewLoginLock() *LoginLock {
	maxFailedAttempts := viper.GetInt64("classified-protect.login-max-fail-times")
	lockDuration := viper.GetDuration("classified-protect.login-fail-locked-seconds")
	// 负值（如 -1 表示不限制/不锁定）统一归一化为 0，避免生成负 TTL 的 Redis 键。
	if lockDuration < 0 {
		lockDuration = 0
	}
	ipMax := viper.GetInt64("classified-protect.ip-login-max-fail-times")
	ipWindow := viper.GetDuration("classified-protect.ip-login-fail-window-seconds")
	if ipWindow < 0 {
		ipWindow = 0
	}
	return &LoginLock{
		MaxFailedAttempts:   maxFailedAttempts,
		LockDuration:        lockDuration * time.Second,
		IPMaxFailedAttempts: ipMax,
		// 与账号维度同约定：配置值为秒数（GetDuration 对纯数字按纳秒解析，统一乘 Second 换算）。
		IPWindowDuration: ipWindow * time.Second,
	}
}

// enabled 返回失败锁定是否生效：需要正数阈值且正的锁定时长。
func (l *LoginLock) enabled() bool {
	return l.MaxFailedAttempts > 0 && l.LockDuration > 0
}

// ipEnabled 返回 IP 维度防护是否生效：语义与账号维度一致（阈值与窗口均需为正）。
func (l *LoginLock) ipEnabled() bool {
	return l.IPMaxFailedAttempts > 0 && l.IPWindowDuration > 0
}

func (*LoginLock) getLockKey(username string) string {
	return fmt.Sprintf("user:%s:lock_until", username)
}

func (*LoginLock) getKey(username string) string {
	return fmt.Sprintf("user:%s:failed_attempts", username)
}

func (*LoginLock) getIPFailKey(ip string) string {
	return fmt.Sprintf("login-ip:%s:failed_attempts", ip)
}

func (*LoginLock) getIPLockKey(ip string) string {
	return fmt.Sprintf("login-ip:%s:lock_until", ip)
}

// GetAllowLoginForIP 判定该来源 IP 是否因失败过多被拒绝。
// 与账号维度互不影响：任一维度处于锁定期即整体拒绝登录尝试。
func (l *LoginLock) GetAllowLoginForIP(_ context.Context, ip string) error {
	if !l.ipEnabled() || ip == "" {
		return nil
	}
	lockUntil, err := global.REDIS.Get(context.Background(), l.getIPLockKey(ip)).Result()
	if err == nil {
		lockUntilTime, err := time.Parse(time.RFC3339, lockUntil)
		if err == nil && time.Now().Before(lockUntilTime) {
			return errcode.WithVars(errcode.CodeTooManyAttempts, map[string]interface{}{
				"attempts":    l.IPMaxFailedAttempts,
				"duration":    l.IPWindowDuration / time.Minute,
				"unlock_time": lockUntilTime.Format(time.DateTime),
			})
		}
	}
	return nil
}

// LoginSuccessForIP 登录成功后清除该 IP 的失败计数（仅自身维度，不动账号计数）。
func (l *LoginLock) LoginSuccessForIP(_ context.Context, ip string) error {
	if !l.ipEnabled() || ip == "" {
		return nil
	}
	return global.REDIS.Del(context.Background(), l.getIPFailKey(ip)).Err()
}

// LoginFailForIP 累计该 IP 失败次数并在达到阈值时锁定整个来源一段时间。
// 计数键带窗口 TTL：静默期过后自动衰减，避免陈旧计数永久惩罚 NAT 出口。
func (l *LoginLock) LoginFailForIP(_ context.Context, ip string) error {
	if !l.ipEnabled() || ip == "" {
		return nil
	}
	failKey := l.getIPFailKey(ip)
	failed, err := global.REDIS.Incr(context.Background(), failKey).Result()
	if err != nil {
		return errors.Errorf("Error incrementing ip failed attempts for %s: %v", ip, err)
	}
	if failed == 1 {
		global.REDIS.Expire(context.Background(), failKey, l.IPWindowDuration)
	}
	if loginLockShouldLock(l.IPMaxFailedAttempts, failed) {
		lockUntilTime := time.Now().Add(l.IPWindowDuration)
		global.REDIS.Set(context.Background(), l.getIPLockKey(ip), lockUntilTime.Format(time.RFC3339), l.IPWindowDuration)
	}
	return nil
}

func (l *LoginLock) GetAllowLogin(_ context.Context, username string) error {

	lockKey := l.getLockKey(username)

	// Check if the account is locked
	lockUntil, err := global.REDIS.Get(context.Background(), lockKey).Result()
	if err == nil {
		lockUntilTime, err := time.Parse(time.RFC3339, lockUntil)
		// 业务代码
		if err == nil && time.Now().Before(lockUntilTime) {
			return errcode.WithVars(errcode.CodeTooManyAttempts, map[string]interface{}{
				"attempts":    l.MaxFailedAttempts,
				"duration":    l.LockDuration / time.Minute,
				"unlock_time": lockUntilTime.Format(time.DateTime),
			})
		}
	}
	return nil
}

func (l *LoginLock) LoginSuccess(_ context.Context, username string) error {
	key := l.getKey(username)
	return global.REDIS.Del(context.Background(), key).Err()
}

func (l *LoginLock) LoginFail(_ context.Context, username string) error {
	// 锁定被禁用（阈值或时长 <= 0）时不维护计数：
	// 避免无界递增的 failed_attempts 键在管理员日后启用锁定时立即误锁账号。
	if !l.enabled() {
		return nil
	}

	key := l.getKey(username)
	lockKey := l.getLockKey(username)
	failedAttempts, err := global.REDIS.Incr(context.Background(), key).Result()
	if err != nil {
		return errors.Errorf("Error incrementing failed attempts for %s: %v", username, err)
	}

	// 首次失败时给计数键设置 TTL，防止陈旧计数跨窗口累积。
	if failedAttempts == 1 {
		global.REDIS.Expire(context.Background(), key, l.LockDuration)
	}

	if loginLockShouldLock(l.MaxFailedAttempts, failedAttempts) {
		lockUntilTime := time.Now().Add(l.LockDuration)
		global.REDIS.Set(context.Background(), lockKey, lockUntilTime.Format(time.RFC3339), l.LockDuration)
	}

	return nil
}
