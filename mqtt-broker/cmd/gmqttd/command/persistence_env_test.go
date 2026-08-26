// 文件用途：覆盖持久化环境变量覆盖层与 fail-fast 校验的行为契约。
// 核心逻辑：验证 GMQTT_PERSISTENCE_* 覆盖映射、空值忽略语义，以及 redis 缺地址/非法类型的拒绝路径。
// 关键注意事项：Compose 依赖这些键把持久化切到 Redis；键名变更必须同步 docker-compose.yml 与 .env.example。
// 重构建议：新增覆盖键时在 applyPersistenceEnvOverrides 与本文件同步扩展表驱动用例。
package command

import (
	"testing"

	"github.com/DrmagicE/gmqtt/config"
)

func TestApplyPersistenceEnvOverridesMapsDeploymentKeys(t *testing.T) {
	t.Setenv("GMQTT_PERSISTENCE_TYPE", "redis")
	t.Setenv("GMQTT_PERSISTENCE_REDIS_ADDR", "redis:6379")
	t.Setenv("GMQTT_PERSISTENCE_REDIS_PASSWORD", "deployment-secret")
	t.Setenv("GMQTT_PERSISTENCE_REDIS_DB", "2")

	c := config.Config{}
	applyPersistenceEnvOverrides(&c)

	if c.Persistence.Type != config.PersistenceTypeRedis {
		t.Fatalf("persistence.type = %q, want %q", c.Persistence.Type, config.PersistenceTypeRedis)
	}
	if c.Persistence.Redis.Addr != "redis:6379" {
		t.Fatalf("redis.addr = %q, want redis:6379", c.Persistence.Redis.Addr)
	}
	if c.Persistence.Redis.Password != "deployment-secret" {
		t.Fatalf("redis.password = %q, want deployment override", c.Persistence.Redis.Password)
	}
	if c.Persistence.Redis.Database != 2 {
		t.Fatalf("redis.database = %d, want 2", c.Persistence.Redis.Database)
	}
}

func TestApplyPersistenceEnvOverridesIgnoresEmptyValues(t *testing.T) {
	t.Setenv("GMQTT_PERSISTENCE_TYPE", "")
	t.Setenv("GMQTT_PERSISTENCE_REDIS_ADDR", "   ")
	t.Setenv("GMQTT_PERSISTENCE_REDIS_DB", "not-a-number")

	c := config.Config{
		Persistence: config.Persistence{
			Type:  config.PersistenceTypeMemory,
			Redis: config.RedisPersistence{Addr: "file-default:6379", Database: 3},
		},
	}
	applyPersistenceEnvOverrides(&c)

	if c.Persistence.Type != config.PersistenceTypeMemory {
		t.Fatalf("empty env must not override persistence.type, got %q", c.Persistence.Type)
	}
	if c.Persistence.Redis.Addr != "file-default:6379" {
		t.Fatalf("blank env must not override redis.addr, got %q", c.Persistence.Redis.Addr)
	}
	if c.Persistence.Redis.Database != 3 {
		t.Fatalf("invalid db number must not override redis.database, got %d", c.Persistence.Redis.Database)
	}
}

func TestValidatePersistenceConfigFailFastPaths(t *testing.T) {
	cases := []struct {
		name    string
		persist config.Persistence
		wantErr bool
	}{
		{"memory-without-addr-is-valid", config.Persistence{Type: config.PersistenceTypeMemory}, false},
		{"empty-type-falls-back-to-memory", config.Persistence{}, false},
		{"redis-with-addr-is-valid", config.Persistence{Type: config.PersistenceTypeRedis, Redis: config.RedisPersistence{Addr: "redis:6379"}}, false},
		{"redis-without-addr-is-rejected", config.Persistence{Type: config.PersistenceTypeRedis}, true},
		{"unknown-type-is-rejected", config.Persistence{Type: "etcd"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePersistenceConfig(&config.Config{Persistence: tc.persist})
			if (err != nil) != tc.wantErr {
				t.Fatalf("validatePersistenceConfig error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
