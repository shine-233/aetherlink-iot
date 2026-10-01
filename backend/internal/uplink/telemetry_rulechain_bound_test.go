package uplink

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// 规则链副作用必须受并发闸门约束：闸门占满时丢弃并计数，释放后恢复执行。
func TestRunBoundedRuleChainDropsWhenSaturated(t *testing.T) {
	f := &TelemetryUplink{uplinkBase: uplinkBase{logger: logrus.New()}, ruleChainSem: make(chan struct{}, 2)}
	release := make(chan struct{})
	var started atomic.Int32

	for i := 0; i < 5; i++ {
		f.runBoundedRuleChain("dev-1", func() {
			started.Add(1)
			<-release
		})
	}
	if got := atomic.LoadUint64(&f.ruleChainDropped); got != 3 {
		t.Fatalf("dropped = %d, want 3", got)
	}
	close(release)

	deadline := time.Now().Add(time.Second)
	for len(f.ruleChainSem) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(f.ruleChainSem) != 0 {
		t.Fatal("semaphore slots must be released after run returns")
	}
	if got := started.Load(); got != 2 {
		t.Fatalf("started = %d, want 2", got)
	}

	done := make(chan struct{})
	f.runBoundedRuleChain("dev-1", func() { close(done) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("rule chain must run again once slots are free")
	}
}

// 未初始化闸门（测试直接构造结构体）保持历史的无闸门异步执行。
func TestRunBoundedRuleChainNilSemaphoreStillRuns(t *testing.T) {
	f := &TelemetryUplink{uplinkBase: uplinkBase{logger: logrus.New()}}
	done := make(chan struct{})
	f.runBoundedRuleChain("dev-1", func() { close(done) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run must execute when semaphore is nil")
	}
}
