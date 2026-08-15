// 文件用途: 覆盖 DAL 层手写查询、缓存或聚合逻辑的回归测试，验证数据访问边界不会漂移。
// 核心逻辑: 构造最小依赖场景并断言查询条件、缓存键、事务副作用或租户过滤结果。
// 关键注意事项: 测试应显式覆盖租户隔离、权限前置假设和事务失败路径，避免只验证成功路径。
// 重构建议: 随 DAL 查询拆分同步拆小测试夹具，并优先补齐跨租户、空依赖和半提交风险用例。

package dal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAggregateQueryArgsOrderAndWindowUnit(t *testing.T) {
	args := aggregateQueryArgs(TelemetryDatasAggregate{
		STime:           1778311391217,
		ETime:           1778311401217,
		Key:             "temperature",
		DeviceID:        "e2079484-33a5-43b6-7dd5-0f913c8a2eb4",
		AggregateWindow: 30000,
	})

	require.Equal(t, []interface{}{
		int64(1778311391217),
		int64(1778311401217),
		"temperature",
		"e2079484-33a5-43b6-7dd5-0f913c8a2eb4",
		int64(30),
		int64(30),
	}, args)
}

func TestAggregateQueryArgsMinimumWindow(t *testing.T) {
	args := aggregateQueryArgs(TelemetryDatasAggregate{AggregateWindow: 500})

	require.Equal(t, int64(1), args[4])
	require.Equal(t, int64(1), args[5])
}
