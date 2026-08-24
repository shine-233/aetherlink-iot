package dal

// 批次一回归测试（2026-08-24，见 references/gen-inheritance-audit.md）：devices
// 写路径与身份预检必须从全新 gorm Statement 出发。高负载下若执行语句继承了
// 共享根残留的 Statement.Model/WHERE，gorm 会注入陈旧主键条件（UPDATE 假成功、
// INSERT 后 SELECT 读旧快照——模拟器上下线心跳热路径的 CI 实锤根因）。
// 本测试直接向共享根语句写入带非零主键的 Model 残留 + 陈旧 id WHERE，确定性
// 复现注入前提，并断言收敛后：
//  1. 身份预检（CheckDeviceNumberExists）生成的 SQL 不含毒化条件；
//  2. 心跳热路径读侧（getDeviceTenantID）与写侧（updateDeviceOnlineStatusColumns
//     的两个分支）生成的 SQL 只包含当前请求自己的条件；
//  3. UpdateDeviceStatus 完整编排仍保持"仅真实状态变化才返回 true"契约。
//
// 需要 PostgreSQL；默认跳过，设置 AETHERLINK_TEST_PSQL_DSN 后运行。

import (
	"os"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

// captureLine 读取捕获到的最近一条 SQL（复用 device_config_isolation_test.go 的 writer）。
func captureLine(writer *p1RegressionWriter) string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.line
}

func TestDeviceChainsStartFromIsolatedStatements(t *testing.T) {
	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; devices isolation regression requires PostgreSQL")
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
	t.Cleanup(func() {
		// 清除共享根语句毒化残留，避免污染同进程后续测试。
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})

	const poisonID = "00000000-0000-0000-0000-00000000p011"
	// 直接污染 devices 包级单例的底层语句：非零主键 Model + 陈旧 id WHERE，
	// 模拟高负载下跨请求残留的执行语句状态。
	root := query.Device.UnderlyingDB()
	if root == nil || root.Statement == nil {
		t.Fatalf("underlying statement unavailable")
	}
	root.Statement.Model = &model.Device{ID: poisonID}
	root.Statement.AddClause(clause.Where{Exprs: []clause.Expression{
		clause.Eq{Column: clause.Column{Table: "devices", Name: "id"}, Value: poisonID},
	}})

	target := &model.Device{
		ID:           "11111111-2222-3333-4444-555555555555",
		Voucher:      "p1-devices-isolation-voucher",
		TenantID:     "tenant-p1-devices",
		DeviceNumber: "p1-devices-isolation-number",
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		IsOnline:     0,
	}
	if err := db.Exec(`INSERT INTO devices (id, voucher, tenant_id, device_number, is_enabled, activate_flag, is_online)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET voucher = EXCLUDED.voucher`,
		target.ID, target.Voucher, target.TenantID, target.DeviceNumber,
		target.IsEnabled, target.ActivateFlag, target.IsOnline).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM devices WHERE id = ?`, target.ID)
	})

	// 身份预检：raw 链不得继承毒化 Model/WHERE。
	exists, err := CheckDeviceNumberExists(target.DeviceNumber)
	if err != nil || !exists {
		t.Fatalf("CheckDeviceNumberExists = (%v, %v); want (true, nil)", exists, err)
	}
	selectSQL := captureLine(writer)
	if strings.Contains(selectSQL, poisonID) {
		t.Fatalf("stale primary key leaked into identity precheck: %s", selectSQL)
	}
	if n := strings.Count(selectSQL, "device_number ="); n != 1 {
		t.Fatalf("expected exactly one device_number condition, got %d: %s", n, selectSQL)
	}

	// 心跳热路径读侧：租户查询必须零起点且只带当前请求自己的主键条件。
	tenantID, err := getDeviceTenantID(target.ID)
	if err != nil {
		t.Fatalf("getDeviceTenantID: %v", err)
	}
	if tenantID != target.TenantID {
		t.Fatalf("tenant id = %q; want %q", tenantID, target.TenantID)
	}
	firstSQL := captureLine(writer)
	if strings.Contains(firstSQL, poisonID) {
		t.Fatalf("stale primary key leaked into heartbeat read path: %s", firstSQL)
	}
	if n := strings.Count(firstSQL, "id ="); n != 1 {
		t.Fatalf("expected exactly one id condition in heartbeat SELECT, got %d: %s", n, firstSQL)
	}

	// 心跳热路径写侧：在线状态更新两个分支都必须只带当前请求自己的条件。
	if _, err := updateDeviceOnlineStatusColumns(target.ID, 1); err != nil {
		t.Fatalf("online update: %v", err)
	}
	onlineSQL := captureLine(writer)
	if strings.Contains(onlineSQL, poisonID) {
		t.Fatalf("stale primary key leaked into online UPDATE: %s", onlineSQL)
	}
	if n := strings.Count(onlineSQL, "id ="); n != 1 {
		t.Fatalf("expected exactly one id condition in online UPDATE, got %d: %s", n, onlineSQL)
	}

	if _, err := updateDeviceOnlineStatusColumns(target.ID, 0); err != nil {
		t.Fatalf("offline update: %v", err)
	}
	offlineSQL := captureLine(writer)
	if strings.Contains(offlineSQL, poisonID) {
		t.Fatalf("stale primary key leaked into offline UPDATE: %s", offlineSQL)
	}
	if n := strings.Count(offlineSQL, "id ="); n != 1 {
		t.Fatalf("expected exactly one id condition in offline UPDATE, got %d: %s", n, offlineSQL)
	}

	// 完整编排冒烟：REDIS 为 nil 时缓存清理安全跳过；离线→在线应报告真实变化。
	changed, err := UpdateDeviceStatus(target.ID, 1)
	if err != nil || !changed {
		t.Fatalf("UpdateDeviceStatus = (%v, %v); want (true, nil)", changed, err)
	}
}
