// 文件用途：操作日志异步写入器的契约测试（db-schema#11）。
// 核心逻辑：锁定三条关键契约——
//   1. 未启用时 enqueue 返回 false，调用方回退同步写（默认行为不变）；
//   2. 启用后条目由后台批量落库；
//   3. 队列满时 enqueue 返回 false（回退同步写），**不丢弃**审计条目；
//   4. 停止时把队列 drain 干净（正常关停不丢条目）。
// 关键注意事项：异步写入器是包级单例，每个用例必须 resetOperationLogWriterForTest，
//   且用长 FlushInterval 阻断定时器，避免后台协程把待断言的条目提前消费掉。

package middleware

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOperationLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	old := global.DB
	dbName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sqlite pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := db.AutoMigrate(&model.OperationLog{}); err != nil {
		t.Fatalf("migrate operation log: %v", err)
	}

	global.DB = db
	query.SetDefault(db.Session(&gorm.Session{NewDB: true}))
	t.Cleanup(func() {
		global.DB = old
		if old != nil {
			query.SetDefault(old.Session(&gorm.Session{NewDB: true}))
		}
	})
	return db
}

func newOperationLogEntry(id string) *model.OperationLog {
	path := "/api/v1/device"
	method := "POST"
	return &model.OperationLog{
		ID:        id,
		IP:        "127.0.0.1",
		Path:      &path,
		UserID:    "user-1",
		Name:      &method,
		CreatedAt: time.Now(),
		TenantID:  "tenant-a",
	}
}

func countOperationLogs(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.OperationLog{}).Count(&n).Error; err != nil {
		t.Fatalf("count operation logs: %v", err)
	}
	return n
}

// TestOperationLogWriterDisabledFallsBackToSync 默认关闭时，投递必须被拒，
// 让 saveOperationLog 走原来的同步写——保证不开启就完全维持既有行为。
func TestOperationLogWriterDisabledFallsBackToSync(t *testing.T) {
	resetOperationLogWriterForTest()
	t.Cleanup(resetOperationLogWriterForTest)

	if err := StartOperationLogWriter(DefaultOperationLogWriterConfig()); err != nil {
		t.Fatalf("start disabled writer: %v", err)
	}
	if enqueueOperationLog(newOperationLogEntry("disabled-1")) {
		t.Fatalf("未启用异步写入时 enqueue 必须返回 false，否则同步回退路径永远不会走到")
	}
}

// TestOperationLogWriterQueueFullFallsBackToSync 队列满时必须回退同步写，而不是丢弃。
func TestOperationLogWriterQueueFullFallsBackToSync(t *testing.T) {
	resetOperationLogWriterForTest()
	t.Cleanup(resetOperationLogWriterForTest)

	setupOperationLogTestDB(t)

	config := DefaultOperationLogWriterConfig()
	config.Enabled = true
	config.QueueSize = 1
	config.BatchSize = 10
	// 长间隔：阻断定时器，确保队列不会被后台协程提前消费。
	config.FlushInterval = time.Hour
	if err := StartOperationLogWriter(config); err != nil {
		t.Fatalf("start writer: %v", err)
	}
	t.Cleanup(func() { StopOperationLogWriter(time.Second) })

	if !enqueueOperationLog(newOperationLogEntry("full-1")) {
		t.Fatalf("队列有空间时首次投递应成功")
	}
	if enqueueOperationLog(newOperationLogEntry("full-2")) {
		t.Fatalf("队列已满时 enqueue 必须返回 false（回退同步写），不能静默丢弃审计条目")
	}
}

// TestOperationLogWriterSurvivesMissingDatabase 数据库未初始化时后台协程不得 panic。
// 后台 goroutine 的 panic 会直接终止整个进程（不像 HTTP 路径有 gin recover 兜底），
// 所以这条是硬要求：宁可不写日志，也不能让服务挂掉。
func TestOperationLogWriterSurvivesMissingDatabase(t *testing.T) {
	resetOperationLogWriterForTest()
	t.Cleanup(resetOperationLogWriterForTest)

	// 刻意不装 DB：query 单例与 global.DB 均为空。
	old := global.DB
	global.DB = nil
	t.Cleanup(func() { global.DB = old })

	config := DefaultOperationLogWriterConfig()
	config.Enabled = true
	config.QueueSize = 8
	config.BatchSize = 4
	config.FlushInterval = time.Hour
	if err := StartOperationLogWriter(config); err != nil {
		t.Fatalf("start writer: %v", err)
	}

	for i := 0; i < 3; i++ {
		if !enqueueOperationLog(newOperationLogEntry(fmt.Sprintf("nodb-%d", i))) {
			t.Fatalf("第 %d 条投递失败", i)
		}
	}

	// 停止会触发 drain -> writeBatch：无 DB 时应计数并告警，不得 panic。
	StopOperationLogWriter(2 * time.Second)

	_, dropped, backlog := OperationLogWriterStats()
	if dropped != 3 {
		t.Fatalf("无数据库时 3 条应全部计入 dropped，实测 %d", dropped)
	}
	if backlog != 0 {
		t.Fatalf("drain 后队列应清空，实测 backlog=%d", backlog)
	}
}

// TestStopOperationLogWriterDrainsQueue 停止时必须把队列 drain 干净并落库。
func TestStopOperationLogWriterDrainsQueue(t *testing.T) {
	resetOperationLogWriterForTest()
	t.Cleanup(resetOperationLogWriterForTest)

	db := setupOperationLogTestDB(t)

	config := DefaultOperationLogWriterConfig()
	config.Enabled = true
	config.QueueSize = 16
	config.BatchSize = 8
	config.FlushInterval = time.Hour // 阻断定时器，只能靠 stop 的 drain 落库
	if err := StartOperationLogWriter(config); err != nil {
		t.Fatalf("start writer: %v", err)
	}

	const n = 5
	for i := 0; i < n; i++ {
		if !enqueueOperationLog(newOperationLogEntry(fmt.Sprintf("drain-%d", i))) {
			t.Fatalf("第 %d 条投递失败", i)
		}
	}
	if got := countOperationLogs(t, db); got != 0 {
		t.Fatalf("定时器被阻断时不应有落库，实测 %d 行", got)
	}

	StopOperationLogWriter(3 * time.Second)

	if got := countOperationLogs(t, db); got != n {
		t.Fatalf("停止后应 drain 出 %d 行，实测 %d 行", n, got)
	}
	_, dropped, backlog := OperationLogWriterStats()
	if dropped != 0 {
		t.Fatalf("正常路径不应丢条目，dropped=%d", dropped)
	}
	if backlog != 0 {
		t.Fatalf("停止后队列应清空，backlog=%d", backlog)
	}
}

// TestOperationLogWriterFlushesOnInterval 启用后由定时器批量落库。
func TestOperationLogWriterFlushesOnInterval(t *testing.T) {
	resetOperationLogWriterForTest()
	t.Cleanup(resetOperationLogWriterForTest)

	db := setupOperationLogTestDB(t)

	config := DefaultOperationLogWriterConfig()
	config.Enabled = true
	config.QueueSize = 16
	config.BatchSize = 4
	config.FlushInterval = 10 * time.Millisecond
	if err := StartOperationLogWriter(config); err != nil {
		t.Fatalf("start writer: %v", err)
	}
	t.Cleanup(func() { StopOperationLogWriter(time.Second) })

	const n = 3
	for i := 0; i < n; i++ {
		if !enqueueOperationLog(newOperationLogEntry(fmt.Sprintf("interval-%d", i))) {
			t.Fatalf("第 %d 条投递失败", i)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if countOperationLogs(t, db) == n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("定时器未在期限内落库，实测 %d 行（期望 %d）", countOperationLogs(t, db), n)
}

// TestStartOperationLogWriterIsIdempotent 重复启动不应起第二个协程/覆盖队列。
func TestStartOperationLogWriterIsIdempotent(t *testing.T) {
	resetOperationLogWriterForTest()
	t.Cleanup(resetOperationLogWriterForTest)

	config := DefaultOperationLogWriterConfig()
	config.Enabled = true
	config.FlushInterval = time.Hour
	if err := StartOperationLogWriter(config); err != nil {
		t.Fatalf("first start: %v", err)
	}
	t.Cleanup(func() { StopOperationLogWriter(time.Second) })

	if err := StartOperationLogWriter(config); err != nil {
		t.Fatalf("second start must be a no-op, got %v", err)
	}
	if !enqueueOperationLog(newOperationLogEntry("idempotent-1")) {
		t.Fatalf("重复启动后队列应仍可用")
	}
}

// TestStopOperationLogWriterIsIdempotentAndSafeWhenDisabled 未启动时停止是空操作，重复停止不 panic。
func TestStopOperationLogWriterIsIdempotentAndSafeWhenDisabled(t *testing.T) {
	resetOperationLogWriterForTest()
	t.Cleanup(resetOperationLogWriterForTest)

	StopOperationLogWriter(time.Second) // 未启动：空操作
}
