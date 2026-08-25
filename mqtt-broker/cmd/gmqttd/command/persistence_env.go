// 文件用途：为核心 broker 持久化配置提供部署期环境变量覆盖层。
// 核心逻辑：config.ParseConfig 只解析 YAML（默认 memory），Compose 等部署环境通过
// GMQTT_PERSISTENCE_* 环境变量把持久化切换到 Redis 后端，使会话/订阅/QoS 队列在
// broker 重启后存活；空值一律忽略，显式留白不会意外清掉文件配置。
// 关键注意事项：这是与 aetherlink 插件 BindEnv 模式对齐的约定键；reload 路径必须
// 同样应用覆盖，否则 SIGHUP 会把持久化回退成 YAML 静态值。
// 重构建议：后续若 core 配置整体迁移到 viper，可删除本文件改为统一 AutomaticEnv。
package command

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/DrmagicE/gmqtt/config"
)

// applyPersistenceEnvOverrides 将 GMQTT_PERSISTENCE_* 环境变量覆盖到已解析配置。
// 支持的键：
//   - GMQTT_PERSISTENCE_TYPE            persistence.type（memory | redis）
//   - GMQTT_PERSISTENCE_REDIS_ADDR      persistence.redis.addr
//   - GMQTT_PERSISTENCE_REDIS_PASSWORD  persistence.redis.password
//   - GMQTT_PERSISTENCE_REDIS_DB        persistence.redis.database（十进制）
func applyPersistenceEnvOverrides(c *config.Config) {
	if v := nonEmptyEnv("GMQTT_PERSISTENCE_TYPE"); v != "" {
		c.Persistence.Type = v
	}
	if v := nonEmptyEnv("GMQTT_PERSISTENCE_REDIS_ADDR"); v != "" {
		c.Persistence.Redis.Addr = v
	}
	if v := nonEmptyEnv("GMQTT_PERSISTENCE_REDIS_PASSWORD"); v != "" {
		c.Persistence.Redis.Password = v
	}
	if v := nonEmptyEnv("GMQTT_PERSISTENCE_REDIS_DB"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			c.Persistence.Redis.Database = uint(n)
		}
	}
}

// validatePersistenceConfig 在启动前 fail-fast 校验持久化组合，
// 避免"声明了 redis 却连不上/没地址"的配置带病上线。
func validatePersistenceConfig(c *config.Config) error {
	switch strings.TrimSpace(strings.ToLower(c.Persistence.Type)) {
	case "", config.PersistenceTypeMemory:
		return nil
	case config.PersistenceTypeRedis:
		if strings.TrimSpace(c.Persistence.Redis.Addr) == "" {
			return fmt.Errorf("persistence.type=redis requires persistence.redis.addr or GMQTT_PERSISTENCE_REDIS_ADDR")
		}
		return nil
	default:
		return fmt.Errorf("unsupported persistence.type %q (want %q or %q)",
			c.Persistence.Type, config.PersistenceTypeMemory, config.PersistenceTypeRedis)
	}
}

func nonEmptyEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
