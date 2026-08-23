package dal

// P1 回归测试（2026-08-23，见 VALIDATION.md）：device_config 链必须从全新
// gorm Statement 出发。高负载下若执行语句继承了残留的 Statement.Model，
// gorm 会在 UPDATE/DELETE 上注入陈旧主键 WHERE（间歇 record-not-found /
// 假成功删除）。本测试直接向单例底层语句写入带非零主键的 Model 残留，
// 确定性复现注入前提，并断言修复后：
//  1. DELETE 语句只包含当前请求自己的主键条件；
//  2. 删除未命中行时返回显式错误而非假成功。
//
// 需要 PostgreSQL；默认跳过，设置 AETHERLINK_TEST_PSQL_DSN 后运行。

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type p1RegressionWriter struct {
	mu   sync.Mutex
	line string
}

func (w *p1RegressionWriter) Printf(format string, args ...interface{}) {
	w.mu.Lock()
	w.line = fmt.Sprintf(format, args...)
	w.mu.Unlock()
}

func TestDeviceConfigChainsStartFromIsolatedStatements(t *testing.T) {
	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; device_config isolation regression requires PostgreSQL")
	}

	writer := &p1RegressionWriter{}
	capture := gormlogger.New(writer, gormlogger.Config{
		LogLevel:                  gormlogger.Info,
		IgnoreRecordNotFoundError: true,
	})

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 capture,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	oldDB := global.DB
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	query.SetDefault(db)

	const poisonID = "00000000-0000-0000-0000-00000000p010"
	// 直接污染单例底层语句的 Model：模拟高负载下跨请求残留的非零主键。
	root := query.DeviceConfig.UnderlyingDB()
	if root == nil || root.Statement == nil {
		t.Fatalf("underlying statement unavailable")
	}
	root.Statement.Model = &model.DeviceConfig{ID: poisonID}

	target := &model.DeviceConfig{
		ID:       "11111111-2222-3333-4444-555555555555",
		Name:     "p1-isolation-regression",
		TenantID: "tenant-p1",
	}
	if err := db.Exec(`INSERT INTO device_configs (id, name, device_type, tenant_id, created_at, updated_at)
		VALUES (?, ?, '1', ?, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`, target.ID, target.Name, target.TenantID).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM device_configs WHERE id = ?`, target.ID)
	})

	if err := DeleteDeviceConfig(target.ID); err != nil {
		t.Fatalf("delete existing row failed: %v", err)
	}

	writer.mu.Lock()
	deleteSQL := writer.line
	writer.mu.Unlock()

	if strings.Contains(deleteSQL, poisonID) {
		t.Fatalf("stale primary key leaked into DELETE statement: %s", deleteSQL)
	}
	if n := strings.Count(deleteSQL, `"device_configs"."id"`); n != 1 {
		t.Fatalf("expected exactly one qualified id condition, got %d: %s", n, deleteSQL)
	}

	err = DeleteDeviceConfig(target.ID)
	if err == nil {
		t.Fatalf("deleting missing row must return an explicit no-rows-affected error")
	}
	if !strings.Contains(err.Error(), "no rows affected") {
		t.Fatalf("unexpected error for missing-row delete: %v", err)
	}
}
