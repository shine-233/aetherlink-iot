// 文件用途：验证 CronInit 的 sync.Once 重入守卫契约。
// 核心逻辑：以计数桩替换引导函数，断言重复调用只执行一次真实装配。
// 关键注意事项：测试通过包内变量替换与恢复实现，不触达真实调度器与业务依赖。

package croninit

import (
	"sync"
	"testing"
)

func TestCronInitIsIdempotent(t *testing.T) {
	// 注意：bootstrapOnce 是 sync.Once（内含 noCopy），按值保存并恢复会触发 go vet copylocks。
	// 改为测试结束时重置为零值——本包内只有本用例依赖 Once 状态，无需还原旧副本。
	oldBootstrap := cronBootstrap
	t.Cleanup(func() {
		cronBootstrap = oldBootstrap
		bootstrapOnce = sync.Once{}
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
