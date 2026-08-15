package common

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type lockTestClient struct {
	setNX func(context.Context, string, interface{}, time.Duration) *redis.BoolCmd
	eval  func(context.Context, string, []string, ...interface{}) *redis.Cmd
}

func (c lockTestClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	return c.setNX(ctx, key, value, expiration)
}

func (c lockTestClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	return c.eval(ctx, script, keys, args...)
}

func TestAcquireLockTokenUsesRandomTokenAndRejectsInvalidInput(t *testing.T) {
	var value interface{}
	client := lockTestClient{
		setNX: func(_ context.Context, key string, got interface{}, expiration time.Duration) *redis.BoolCmd {
			if key != "lock" || expiration != 5*time.Second {
				t.Fatalf("SetNX arguments = %q, %v; want lock, 5s", key, expiration)
			}
			value = got
			return redis.NewBoolResult(true, nil)
		},
	}

	token := acquireLockToken(client, "lock", 5*time.Second)
	if token == "" || value != token {
		t.Fatalf("token = %q, stored value = %v", token, value)
	}
	if len(token) != 32 {
		t.Fatalf("token length = %d, want 32 hex characters", len(token))
	}
	if acquireLockToken(client, "", time.Second) != "" || acquireLockToken(client, "lock", 0) != "" {
		t.Fatal("acquireLockToken accepted invalid input")
	}
}

func TestAcquireLockTokenReturnsEmptyOnRedisFailure(t *testing.T) {
	client := lockTestClient{setNX: func(context.Context, string, interface{}, time.Duration) *redis.BoolCmd {
		return redis.NewBoolResult(false, errors.New("connection failed"))
	}}
	if token := acquireLockToken(client, "lock", time.Second); token != "" {
		t.Fatalf("token = %q, want empty token on Redis failure", token)
	}
}

func TestReleaseLockTokenUsesCompareAndDeleteScript(t *testing.T) {
	client := lockTestClient{
		eval: func(_ context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
			if !strings.Contains(script, "redis.call(\"get\", KEYS[1])") || !strings.Contains(script, "ARGV[1]") {
				t.Fatalf("release script does not compare ownership: %q", script)
			}
			if len(keys) != 1 || keys[0] != "lock" || len(args) != 1 || args[0] != "token" {
				t.Fatalf("Eval arguments = keys %v args %v", keys, args)
			}
			return redis.NewCmdResult(int64(1), nil)
		},
	}
	if err := releaseLockToken(client, "lock", "token"); err != nil {
		t.Fatalf("releaseLockToken() error = %v", err)
	}
	if err := releaseLockToken(client, "", "token"); err == nil {
		t.Fatal("releaseLockToken accepted empty key")
	}
}
