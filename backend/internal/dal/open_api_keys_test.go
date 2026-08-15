// 文件用途: 覆盖 DAL 层手写查询、缓存或聚合逻辑的回归测试，验证数据访问边界不会漂移。
// 核心逻辑: 构造最小依赖场景并断言查询条件、缓存键、事务副作用或租户过滤结果。
// 关键注意事项: 测试应显式覆盖租户隔离、权限前置假设和事务失败路径，避免只验证成功路径。
// 重构建议: 随 DAL 查询拆分同步拆小测试夹具，并优先补齐跨租户、空依赖和半提交风险用例。

package dal

import (
	"context"
	"testing"

	"aetherlink-iot/backend/pkg/global"
)

func TestOpenAPIKeyCacheKeysMatchVerifier(t *testing.T) {
	keys := OpenAPIKeyCacheKeys("app-key-1")
	want := []string{"apikey:app-key-1", "apikey:createdid:app-key-1"}

	if len(keys) != len(want) {
		t.Fatalf("expected %d keys, got %d", len(want), len(keys))
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("key %d mismatch: got %q want %q", i, keys[i], want[i])
		}
	}
}

func TestInvalidateOpenAPIKeyCacheWithoutRedisIsNoop(t *testing.T) {
	oldRedis := global.REDIS
	global.REDIS = nil
	t.Cleanup(func() {
		global.REDIS = oldRedis
	})

	InvalidateOpenAPIKeyCache(context.Background(), "app-key-1")
	InvalidateOpenAPIKeyCache(context.Background(), "")
}
