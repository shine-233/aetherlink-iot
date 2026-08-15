// 文件用途：提供基于 Redis 的带所有权校验的分布式锁获取和释放函数。
// 核心逻辑：通过随机 token 配合 SETNX 获取锁，释放时用 Lua 原子比较并删除。
// 关键注意事项：锁必须使用 AcquireLockToken/ReleaseLockToken 配对；过期后 token 不再代表所有权。
// 重构建议：后续可注入 Redis 客户端和 context，避免依赖全局初始化顺序。
package common

import (
	"aetherlink-iot/backend/pkg/global"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const releaseLockScript = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`

type lockClient interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

// AcquireLockToken 获取锁并返回持有权 token；Redis 未初始化或操作失败时返回空 token。
func AcquireLockToken(lockKey string, expiration time.Duration) string {
	if global.REDIS == nil {
		return ""
	}
	return acquireLockToken(global.REDIS, lockKey, expiration)
}

func acquireLockToken(client lockClient, lockKey string, expiration time.Duration) string {
	if client == nil || lockKey == "" || expiration <= 0 {
		return ""
	}

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return ""
	}
	token := hex.EncodeToString(tokenBytes)
	ok, err := client.SetNX(context.Background(), lockKey, token, expiration).Result()
	if err != nil || !ok {
		return ""
	}
	return token
}

// AcquireLock 保留原有布尔契约；新调用方应使用 AcquireLockToken 以便安全释放。
func AcquireLock(lockKey string, expiration time.Duration) bool {
	return AcquireLockToken(lockKey, expiration) != ""
}

// ReleaseLockToken 仅在 token 仍是当前锁值时原子释放锁，避免误删新持有者的锁。
func ReleaseLockToken(lockKey, token string) error {
	if global.REDIS == nil {
		return fmt.Errorf("redis is not initialized")
	}
	return releaseLockToken(global.REDIS, lockKey, token)
}

func releaseLockToken(client lockClient, lockKey, token string) error {
	if client == nil {
		return fmt.Errorf("redis is not initialized")
	}
	if lockKey == "" || token == "" {
		return fmt.Errorf("lock key and token are required")
	}
	if err := client.Eval(context.Background(), releaseLockScript, []string{lockKey}, token).Err(); err != nil {
		return err
	}
	return nil
}

// ReleaseLock 保留旧接口但不再执行无条件删除；旧调用方不会破坏其他持有者的锁。
func ReleaseLock(lockKey string) {
	logrus.WithField("lock_key", lockKey).Warn("ReleaseLock without ownership token ignored; use ReleaseLockToken")
}
