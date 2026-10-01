// 文件用途：验证第二因子登录尝试上限（防 TOTP/恢复码在线爆破）。
package service

import (
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

func useTotpGuardTestRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	oldRedis := global.REDIS
	global.REDIS = client
	t.Cleanup(func() {
		global.REDIS = oldRedis
		_ = client.Close()
	})
	return server
}

func TestTotpLoginGuardLocksAfterMaxFailures(t *testing.T) {
	server := useTotpGuardTestRedis(t)
	viper.Set("classified-protect.totp-max-fail-times", 3)
	t.Cleanup(func() { viper.Set("classified-protect.totp-max-fail-times", nil) })

	const uid = "user-guard-1"
	for i := 0; i < 3; i++ {
		if err := ensureTotpLoginAllowed(uid); err != nil {
			t.Fatalf("attempt %d should be allowed, got %v", i+1, err)
		}
		registerTotpLoginFailure(uid)
	}
	if code := errCodeOf(ensureTotpLoginAllowed(uid)); code != errcode.CodeTooManyAttempts {
		t.Fatalf("expected CodeTooManyAttempts after 3 failures, got %d", code)
	}
	if ttl := server.TTL(totpLoginFailKey(uid)); ttl != totpLoginFailureWindow() {
		t.Fatalf("expected fixed window ttl %v, got %v", totpLoginFailureWindow(), ttl)
	}
	// 其他用户不受影响（按用户隔离）。
	if err := ensureTotpLoginAllowed("user-guard-2"); err != nil {
		t.Fatalf("other user must not be locked: %v", err)
	}
	// 窗口过期后解锁。
	server.FastForward(totpLoginFailureWindow())
	if err := ensureTotpLoginAllowed(uid); err != nil {
		t.Fatalf("expected unlock after window, got %v", err)
	}
}

func TestTotpLoginGuardDefaultLimitAndClear(t *testing.T) {
	useTotpGuardTestRedis(t)
	const uid = "user-guard-3"
	for i := 0; i < defaultTotpLoginMaxFailures; i++ {
		registerTotpLoginFailure(uid)
	}
	if code := errCodeOf(ensureTotpLoginAllowed(uid)); code != errcode.CodeTooManyAttempts {
		t.Fatalf("expected default limit %d to lock, got code %d", defaultTotpLoginMaxFailures, code)
	}
	clearTotpLoginFailures(uid)
	if err := ensureTotpLoginAllowed(uid); err != nil {
		t.Fatalf("expected cleared counter to allow, got %v", err)
	}
}

func TestTotpLoginGuardFailsClosedWithoutRedis(t *testing.T) {
	old := global.REDIS
	global.REDIS = nil
	t.Cleanup(func() { global.REDIS = old })
	if err := ensureTotpLoginAllowed("user-guard-4"); err == nil {
		t.Fatal("expected fail-closed when redis is unavailable")
	}
}

// 已锁定时 LoginWithSecondFactor 必须在任何 DB 查询之前拒绝（本测试无 DB，若走到 DB 会 panic）。
func TestLoginWithSecondFactorRejectsWhenLocked(t *testing.T) {
	useTotpGuardTestRedis(t)
	viper.Set("jwt.key", "unit-test-jwt-key-for-totp-guard")
	t.Cleanup(func() { viper.Set("jwt.key", nil) })

	const uid = "user-guard-5"
	ticket, err := (&UserTotp{}).IssueChallenge(uid)
	if err != nil {
		t.Fatalf("issue challenge: %v", err)
	}
	for i := 0; i < defaultTotpLoginMaxFailures; i++ {
		registerTotpLoginFailure(uid)
	}
	_, err = (&UserTotp{}).LoginWithSecondFactor(ticket, "123456")
	if code := errCodeOf(err); code != errcode.CodeTooManyAttempts {
		t.Fatalf("expected CodeTooManyAttempts, got %v", err)
	}
}
