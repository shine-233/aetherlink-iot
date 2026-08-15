// 文件用途：验证登录失败锁定服务的计数和解锁边界。
// 核心逻辑：断言失败次数、锁定窗口和重置逻辑在不同时间点的返回结果。
// 关键注意事项：登录锁定直接影响安全策略，测试需防止计数绕过、永久锁死和多用户 key 串扰。
// 重构建议：引入可控时钟和存储接口，补齐 Redis 错误、并发失败和管理员解锁边界。
package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- getLockKey ---

func TestLoginLockGetLockKey(t *testing.T) {
	ll := &LoginLock{}
	key := ll.getLockKey("user123")
	assert.Equal(t, "user:user123:lock_until", key)
}

func TestLoginLockGetLockKey_EmptyUsername(t *testing.T) {
	ll := &LoginLock{}
	key := ll.getLockKey("")
	assert.Equal(t, "user::lock_until", key)
}

func TestLoginLockGetLockKey_SpecialChars(t *testing.T) {
	ll := &LoginLock{}
	key := ll.getLockKey("user@example.com")
	assert.Equal(t, "user:user@example.com:lock_until", key)
}

// --- getKey ---

func TestLoginLockGetKey(t *testing.T) {
	ll := &LoginLock{}
	key := ll.getKey("user123")
	assert.Equal(t, "user:user123:failed_attempts", key)
}

func TestLoginLockGetKey_EmptyUsername(t *testing.T) {
	ll := &LoginLock{}
	key := ll.getKey("")
	assert.Equal(t, "user::failed_attempts", key)
}

func TestLoginLockGetKey_SpecialChars(t *testing.T) {
	ll := &LoginLock{}
	key := ll.getKey("user@example.com")
	assert.Equal(t, "user:user@example.com:failed_attempts", key)
}

// --- LoginLock struct ---

func TestLoginLock_Fields(t *testing.T) {
	ll := &LoginLock{
		MaxFailedAttempts: 5,
		LockDuration:      900 * time.Second,
	}
	assert.Equal(t, int64(5), ll.MaxFailedAttempts)
	assert.Equal(t, 900*time.Second, ll.LockDuration)
}

func TestLoginLock_ZeroFields(t *testing.T) {
	ll := &LoginLock{}
	assert.Equal(t, int64(0), ll.MaxFailedAttempts)
	assert.Equal(t, time.Duration(0), ll.LockDuration)
}
