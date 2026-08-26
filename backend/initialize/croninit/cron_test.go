// 文件用途：验证 CronInit 的 sync.Once 重入守卫契约。
// 核心逻辑：以计数桩替换引导函数，断言重复调用只执行一次真实装配。
// 关键注意事项：测试通过包内变量替换与恢复实现，不触达真实调度器与业务依赖。

package croninit

import (
	"sync"
	"testing"
)

func TestCronInitIsIdempotent(t *testing.T) {
	oldBootstrap := cronBootstrap
	oldOnce := bootstrapOnce
	t.Cleanup(func() {
		cronBootstrap = oldBootstrap
		bootstrapOnce = oldOnce
	})

	var calls int
	cronBootstrap = func() { calls++ }
	bootstrapOnce = sync.Once{}

	CronInit()
	CronInit()
	CronInit()

	if calls != 1 {
		t.Fatalf("cron bootstrap executed %d times, want exactly 1 (re-entry must be a no-op)", calls)
	}
}
