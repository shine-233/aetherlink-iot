// 文件用途：为 /api/v1/login/totp 第二因子登录提供按用户维度的失败次数上限（防暴力枚举）。
// 核心逻辑：Redis 计数 login-totp:<uid>:failed_attempts，窗口与挑战票据 TTL 一致；
// 达到上限后即使验证码正确也拒绝，直到窗口过期；成功登录清零。
// 关键注意事项：Redis 不可用时 fail-closed（登录会话本身也依赖 Redis，放行无意义且削弱控制）。
// 背景：修复前单张 5 分钟票据可无限次猜 6 位 TOTP 与恢复码，且重新 /login 即可换新票据，
// 第二因子退化为可在线爆破。

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

const (
	// defaultTotpLoginMaxFailures 默认每个用户在一个窗口内最多 5 次第二因子失败。
	defaultTotpLoginMaxFailures = 5
	totpGuardRedisTimeout       = 2 * time.Second
)

// totpLoginMaxFailures 读取 classified-protect.totp-max-fail-times，未配置或非正数时回落默认值。
// 刻意不允许配置为 0 关闭：第二因子必须有尝试上限。
func totpLoginMaxFailures() int64 {
	if v := viper.GetInt64("classified-protect.totp-max-fail-times"); v > 0 {
		return v
	}
	return defaultTotpLoginMaxFailures
}

// totpLoginFailureWindow 失败计数窗口：与挑战票据有效期一致，并且不短于 15 分钟，
// 避免攻击者在票据过期后立刻重新拿票继续猜。
func totpLoginFailureWindow() time.Duration {
	window := totpChallengeTTLMinutes * time.Minute
	if window < 15*time.Minute {
		window = 15 * time.Minute
	}
	return window
}

func totpLoginFailKey(userID string) string {
	return fmt.Sprintf("login-totp:%s:failed_attempts", userID)
}

func totpGuardContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), totpGuardRedisTimeout)
}

// ensureTotpLoginAllowed 在校验验证码前调用；已达上限或 Redis 异常时返回错误。
func ensureTotpLoginAllowed(userID string) error {
	if global.REDIS == nil {
		return errcode.New(errcode.CodeInvalidAuth)
	}
	ctx, cancel := totpGuardContext()
	defer cancel()
	failed, err := global.REDIS.Get(ctx, totpLoginFailKey(userID)).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		return errcode.New(errcode.CodeInvalidAuth)
	}
	maxFailures := totpLoginMaxFailures()
	if failed >= maxFailures {
		window := totpLoginFailureWindow()
		return errcode.WithVars(errcode.CodeTooManyAttempts, map[string]interface{}{
			"attempts":    maxFailures,
			"duration":    window / time.Minute,
			"unlock_time": time.Now().Add(window).Format(time.DateTime),
		})
	}
	return nil
}

// registerTotpLoginFailure 记录一次第二因子失败；首次失败设置过期，后续失败不续期（固定窗口）。
func registerTotpLoginFailure(userID string) {
	if global.REDIS == nil {
		return
	}
	ctx, cancel := totpGuardContext()
	defer cancel()
	key := totpLoginFailKey(userID)
	failed, err := global.REDIS.Incr(ctx, key).Result()
	if err != nil {
		return
	}
	if failed == 1 {
		global.REDIS.Expire(ctx, key, totpLoginFailureWindow())
	}
}

// clearTotpLoginFailures 第二因子成功后清零计数。
func clearTotpLoginFailures(userID string) {
	if global.REDIS == nil {
		return
	}
	ctx, cancel := totpGuardContext()
	defer cancel()
	global.REDIS.Del(ctx, totpLoginFailKey(userID))
}
