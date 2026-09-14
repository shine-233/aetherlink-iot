// 文件用途：预注册凭证一次性下载的运行期证据（ROADMAP P0.5）。
// 覆盖：签发 → 下载（明文）→ 第二次下载被拒；过期拒绝；跨租户未命中；
// 重复签发被拒；批次为空拒绝；并发下载只有一个成功。
// 说明：需要真实 PostgreSQL（迁移 95 建表后）。缺 DSN 或缺表一律 Skip，
// **不得**把 Skip 当作通过。
package service

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openCredentialGrantPostgres 打开数据库并校验许可表存在。
func openCredentialGrantPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("AETHERLINK_TEST_PSQL_DSN"))
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; credential grant tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	global.DB = db
	var exists bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=?)",
		model.TableNameDevicePreRegisterCredentialGrant).Scan(&exists).Error; err != nil {
		t.Fatalf("probe grant table: %v", err)
	}
	if !exists {
		t.Skip("device_pre_register_credential_grants missing; apply migration 95 first")
	}
	return db
}

// seedCredentialGrantBatch 造一个批次的两台预注册设备，并登记清理。
func seedCredentialGrantBatch(t *testing.T, db *gorm.DB, tenant, batch string) {
	t.Helper()
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM devices WHERE tenant_id = ? AND batch_number = ?", tenant, batch).Error
		_ = db.Exec("DELETE FROM "+model.TableNameDevicePreRegisterCredentialGrant+" WHERE tenant_id = ? AND batch_number = ?", tenant, batch).Error
	})
	for _, number := range []string{batch + "-0001", batch + "-0002"} {
		name := "cred-device-" + number
		voucher := `{"username":"cred-` + number + `"}`
		// 明文列出列、其余列走默认：devices 表列很多且有 NOT NULL 无默认值的列，
		// 用 GORM 结构体插入容易被模型与库表不同步绊倒（本轮已踩过一次）。
		if err := db.Exec(
			`INSERT INTO devices (id, device_number, name, voucher, tenant_id, batch_number, is_online, activate_flag, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, 0, 'inactive', now())`,
			"cred-"+number, number, name, voucher, tenant, batch).Error; err != nil {
			t.Fatalf("insert device %s: %v", number, err)
		}
	}
}

// TestCredentialGrantDownloadOnceThenRefused 核心不变式：下载一次成功后必须拒绝第二次。
// 这是"凭证只出现一次"在代码层唯一真正的落点。
func TestCredentialGrantDownloadOnceThenRefused(t *testing.T) {
	db := openCredentialGrantPostgres(t)
	tenant := "tenant-cred-grant"
	batch := "cred-batch-once"
	seedCredentialGrantBatch(t, db, tenant, batch)
	claims := &utils.UserClaims{ID: "u1", TenantID: tenant, Authority: "TENANT_ADMIN"}

	granted, err := GrantPreRegisterCredentials(nil, claims, batch)
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	if granted.DeviceCount != 2 {
		t.Fatalf("device_count = %d, want 2", granted.DeviceCount)
	}

	first, err := DownloadPreRegisterCredentials(nil, claims, granted.ID)
	if err != nil {
		t.Fatalf("first download: %v", err)
	}
	if len(first.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(first.Rows))
	}
	for _, row := range first.Rows {
		if !strings.Contains(row.Voucher, `"username":"cred-`) {
			t.Fatalf("voucher = %q, want plaintext credential", row.Voucher)
		}
	}

	// 第二次必须被拒：这是本用例存在的理由。
	if _, err := DownloadPreRegisterCredentials(nil, claims, granted.ID); err == nil {
		t.Fatal("second download must be refused: one-time credential would be repeatable")
	}

	// 消费留痕：谁在什么时候取走的，必须可审计。
	row, err := dal.GetCredentialGrantInTenant(granted.ID, tenant)
	if err != nil {
		t.Fatalf("reload grant: %v", err)
	}
	if row.Status != model.CredentialGrantStatusConsumed {
		t.Fatalf("status = %q, want consumed", row.Status)
	}
	if row.ConsumedBy == nil || *row.ConsumedBy != "u1" {
		t.Fatalf("consumed_by = %v, want u1", row.ConsumedBy)
	}
	if row.ConsumedAt == nil {
		t.Fatal("consumed_at must be recorded")
	}
}

// TestCredentialGrantConcurrentDownloadOnlyOneWins 并发下载：只允许一个成功。
// 应用层 if 挡不住并发，只有数据库条件更新能挡——本用例锁住这一点。
func TestCredentialGrantConcurrentDownloadOnlyOneWins(t *testing.T) {
	db := openCredentialGrantPostgres(t)
	tenant := "tenant-cred-grant"
	batch := "cred-batch-race"
	seedCredentialGrantBatch(t, db, tenant, batch)
	claims := &utils.UserClaims{ID: "u1", TenantID: tenant, Authority: "TENANT_ADMIN"}

	granted, err := GrantPreRegisterCredentials(nil, claims, batch)
	if err != nil {
		t.Fatalf("grant: %v", err)
	}

	const workers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes, failures := 0, 0
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, err := DownloadPreRegisterCredentials(nil, claims, granted.ID)
			mu.Lock()
			if err == nil {
				successes++
			} else {
				failures++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("concurrent downloads succeeded %d times, want exactly 1 (got %d failures)", successes, failures)
	}
	if failures != workers-1 {
		t.Fatalf("failures = %d, want %d", failures, workers-1)
	}
}

// TestCredentialGrantRejectsDuplicatePending 一批次只能有一个待消费许可。
// 允许重复签发等于可以无限次取到明文，一次性形同虚设。
func TestCredentialGrantRejectsDuplicatePending(t *testing.T) {
	db := openCredentialGrantPostgres(t)
	tenant := "tenant-cred-grant"
	batch := "cred-batch-dup"
	seedCredentialGrantBatch(t, db, tenant, batch)
	claims := &utils.UserClaims{ID: "u1", TenantID: tenant, Authority: "TENANT_ADMIN"}

	if _, err := GrantPreRegisterCredentials(nil, claims, batch); err != nil {
		t.Fatalf("first grant: %v", err)
	}
	if _, err := GrantPreRegisterCredentials(nil, claims, batch); err == nil {
		t.Fatal("duplicate pending grant must be refused")
	}
}

// TestCredentialGrantExpiredCannotDownload 过期许可即便仍是 pending 也必须拒绝。
// 否则"设了过期时间却还能下载"等于过期形同虚设。
func TestCredentialGrantExpiredCannotDownload(t *testing.T) {
	db := openCredentialGrantPostgres(t)
	tenant := "tenant-cred-grant"
	batch := "cred-batch-exp"
	seedCredentialGrantBatch(t, db, tenant, batch)
	claims := &utils.UserClaims{ID: "u1", TenantID: tenant, Authority: "TENANT_ADMIN"}

	granted, err := GrantPreRegisterCredentials(nil, claims, batch)
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	// 直接把过期时间拨到过去，模拟"签发后一直没下载，直到过期"。
	if err := db.Exec("UPDATE "+model.TableNameDevicePreRegisterCredentialGrant+
		" SET expires_at = now() - interval '1 minute' WHERE id = ?", granted.ID).Error; err != nil {
		t.Fatalf("backdate expires_at: %v", err)
	}
	if _, err := DownloadPreRegisterCredentials(nil, claims, granted.ID); err == nil {
		t.Fatal("expired grant must not be downloadable")
	}
}

// TestCredentialGrantCrossTenantIsNotFound 跨租户表现为未命中，不泄露批次存在性。
func TestCredentialGrantCrossTenantIsNotFound(t *testing.T) {
	db := openCredentialGrantPostgres(t)
	tenant := "tenant-cred-grant"
	batch := "cred-batch-cross"
	seedCredentialGrantBatch(t, db, tenant, batch)
	owner := &utils.UserClaims{ID: "u1", TenantID: tenant, Authority: "TENANT_ADMIN"}

	granted, err := GrantPreRegisterCredentials(nil, owner, batch)
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	other := &utils.UserClaims{ID: "u2", TenantID: "tenant-cred-other", Authority: "TENANT_ADMIN"}
	if _, err := DownloadPreRegisterCredentials(nil, other, granted.ID); err == nil {
		t.Fatal("cross-tenant download must be refused")
	}
	// 本租户的许可不应被其它租户的消费尝试改写。
	row, err := dal.GetCredentialGrantInTenant(granted.ID, tenant)
	if err != nil {
		t.Fatalf("reload grant: %v", err)
	}
	if row.Status != model.CredentialGrantStatusPending {
		t.Fatalf("status = %q, want pending (cross-tenant attempt must not consume it)", row.Status)
	}
}

// TestCredentialGrantRejectsEmptyBatch 批次内没有设备时必须拒绝签发。
// 空批次签发成功会得到一个"下载了 0 条"的许可，运维会误以为下载失败。
func TestCredentialGrantRejectsEmptyBatch(t *testing.T) {
	openCredentialGrantPostgres(t)
	claims := &utils.UserClaims{ID: "u1", TenantID: "tenant-cred-grant", Authority: "TENANT_ADMIN"}
	if _, err := GrantPreRegisterCredentials(nil, claims, "cred-batch-nope"); err == nil {
		t.Fatal("grant on empty batch must be refused")
	}
	if _, err := GrantPreRegisterCredentials(nil, claims, "   "); err == nil {
		t.Fatal("grant with blank batch must be refused")
	}
}

// TestCredentialGrantStatusVocabulary 状态词表与消费留痕规则的定向校验（纯逻辑，不需要库）。
func TestCredentialGrantStatusVocabulary(t *testing.T) {
	for _, s := range []string{"pending", "consumed", "expired", "revoked"} {
		if !model.IsCredentialGrantStatus(s) {
			t.Fatalf("status %q must be valid", s)
		}
	}
	for _, s := range []string{"", "done", "USED", "pending "} {
		if model.IsCredentialGrantStatus(s) {
			t.Fatalf("status %q must be invalid", s)
		}
	}
	now := time.Now().UTC()
	by := "u1"
	// 已消费但没留痕：审计断了，必须拒绝。
	bad := &model.DevicePreRegisterCredentialGrant{
		TenantID: "t", BatchNumber: "b", Status: model.CredentialGrantStatusConsumed,
	}
	if err := model.ValidateCredentialGrant(bad); err == nil {
		t.Fatal("consumed grant without consumed_by/at must be rejected")
	}
	good := &model.DevicePreRegisterCredentialGrant{
		TenantID: "t", BatchNumber: "b", Status: model.CredentialGrantStatusConsumed,
		ConsumedBy: &by, ConsumedAt: &now,
	}
	if err := model.ValidateCredentialGrant(good); err != nil {
		t.Fatalf("valid consumed grant rejected: %v", err)
	}
}
