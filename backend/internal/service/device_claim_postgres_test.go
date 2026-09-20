// 文件用途：TB-12 设备认领的持久化运行期证据（ROADMAP TB-12）。
// 覆盖：签发（明文只出现一次）→ 赎回成功且租户转移 → 错 key 拒绝 → 重放拒绝 →
// 过期拒绝 → 认领自己租户拒绝 → 撤销后拒绝 → 重新签发覆盖旧行（partial unique 不变量）。
// 说明：需要真实 PostgreSQL（迁移 112 建表后）。缺 DSN 或缺表一律 Skip，
// **不得**把 Skip 当作通过；活栈契约测试 automation_tests/tests/61_device_claim.test.js
// 是 HTTP 面的对等证据。
package service

import (
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openDeviceClaimPostgres 打开数据库并校验 112 的表存在。
func openDeviceClaimPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("AETHERLINK_TEST_PSQL_DSN"))
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; device claim tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	global.DB = db
	var exists bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='device_claim_tokens')").Scan(&exists).Error; err != nil {
		t.Fatalf("probe device_claim_tokens: %v", err)
	}
	if !exists {
		t.Skip("device_claim_tokens missing; apply migration 112 first")
	}
	return db
}

// seedClaimFixture 建一台属于 owner 租户的设备（表 devices），返回设备与清理函数。
func seedClaimFixture(t *testing.T, db *gorm.DB, ownerTenant, number string) *model.Device {
	t.Helper()
	device := &model.Device{
		ID:           "claim-dev-" + strings.ReplaceAll(number, ":", "-"),
		TenantID:     ownerTenant,
		DeviceNumber: number,
		IsEnabled:    "enabled",
	}
	if err := db.Exec("INSERT INTO devices (id, tenant_id, device_number, voucher, is_enabled, activate_flag) VALUES (?, ?, ?, ?, 'enabled', 'active')",
		device.ID, ownerTenant, number, `{"username":"claim-u","password":"claim-p"}`).Error; err != nil {
		t.Fatalf("seed device: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM devices WHERE id = ?", device.ID).Error
		_ = db.Exec("DELETE FROM device_claim_tokens WHERE device_id = ?", device.ID).Error
	})
	return device
}

func TestDeviceClaimLifecycleOnPostgres(t *testing.T) {
	db := openDeviceClaimPostgres(t)
	_ = db
	ownerTenant := "tenant-owner-claim"
	claimerTenant := "tenant-claimer-claim"
	number := time.Now().UTC().Format("claim150405.000000000")
	seedClaimFixture(t, db, ownerTenant, number)

	svc := &DeviceClaim{}
	owner := &utils.UserClaims{ID: "user-owner", TenantID: ownerTenant, Authority: "TENANT_ADMIN"}
	claimer := &utils.UserClaims{ID: "user-claimer", TenantID: claimerTenant, Authority: "TENANT_ADMIN"}

	// 1) 签发：响应里有明文 key；库里只有哈希。
	issued, err := svc.IssueClaimToken(nil, &model.IssueDeviceClaimTokenReq{
		DeviceID: "claim-dev-" + strings.ReplaceAll(number, ":", "-"), TTLSeconds: 600,
	}, owner)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !strings.HasPrefix(issued.ClaimKey, "ack_") {
		t.Fatalf("claim key shape = %q", issued.ClaimKey)
	}
	var storedHash string
	if err := db.Raw("SELECT claim_key_hash FROM device_claim_tokens WHERE id = ?", issued.TokenID).Scan(&storedHash).Error; err != nil {
		t.Fatalf("read hash: %v", err)
	}
	if storedHash == issued.ClaimKey || storedHash == "" || len(storedHash) != 64 {
		t.Fatalf("claim key must be stored as sha256 hex only, got %q", storedHash)
	}

	// 2) 认领自己租户的设备被拒绝。
	if _, err := svc.RedeemClaim(nil, &model.RedeemDeviceClaimReq{DeviceNumber: number, ClaimKey: issued.ClaimKey}, owner); err == nil {
		t.Fatal("redeem own device must be rejected")
	}

	// 3) 错 key 拒绝。
	if _, err := svc.RedeemClaim(nil, &model.RedeemDeviceClaimReq{DeviceNumber: number, ClaimKey: "ack_" + strings.Repeat("0", 48)}, claimer); err == nil {
		t.Fatal("wrong key must be rejected")
	}

	// 4) 正确认领：设备租户转移，响应记录原租户。
	redeemed, err := svc.RedeemClaim(nil, &model.RedeemDeviceClaimReq{DeviceNumber: number, ClaimKey: issued.ClaimKey}, claimer)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if redeemed.PreviousTenID != ownerTenant {
		t.Fatalf("previous tenant = %q, want %q", redeemed.PreviousTenID, ownerTenant)
	}
	var tenantAfter string
	if err := db.Raw("SELECT tenant_id FROM devices WHERE id = ?", redeemed.DeviceID).Scan(&tenantAfter).Error; err != nil {
		t.Fatalf("read device tenant: %v", err)
	}
	if tenantAfter != claimerTenant {
		t.Fatalf("device tenant after claim = %q, want %q", tenantAfter, claimerTenant)
	}

	// 5) 重放同 key 拒绝（一次性）。
	if _, err := svc.RedeemClaim(nil, &model.RedeemDeviceClaimReq{DeviceNumber: number, ClaimKey: issued.ClaimKey}, claimer); err == nil {
		t.Fatal("replayed claim key must be rejected")
	}

	// 6) 重新签发把 consumed 行留作历史、新 active 行可再签发（partial unique 不变量）。
	issued2, err := svc.IssueClaimToken(nil, &model.IssueDeviceClaimTokenReq{DeviceID: redeemed.DeviceID, TTLSeconds: 600}, claimer)
	if err != nil {
		t.Fatalf("re-issue after consumed: %v", err)
	}
	if issued2.TokenID == issued.TokenID {
		t.Fatal("re-issue must create a new token row")
	}

	// 7) 撤销后 redeem 拒绝。
	if err := svc.RevokeClaimToken(nil, issued2.TokenID, claimer); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := svc.RedeemClaim(nil, &model.RedeemDeviceClaimReq{DeviceNumber: number, ClaimKey: issued2.ClaimKey}, owner); err == nil {
		t.Fatal("redeem after revoke must be rejected")
	}

	// 8) 过期拒绝：签发 1 秒 TTL，睡过窗口。
	issued3, err := svc.IssueClaimToken(nil, &model.IssueDeviceClaimTokenReq{DeviceID: redeemed.DeviceID, TTLSeconds: 1}, claimer)
	if err != nil {
		t.Fatalf("issue short ttl: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	if _, err := svc.RedeemClaim(nil, &model.RedeemDeviceClaimReq{DeviceNumber: number, ClaimKey: issued3.ClaimKey}, owner); err == nil {
		t.Fatal("expired claim key must be rejected")
	}
}

func TestDeviceClaimIssueRequiresOwnershipOnPostgres(t *testing.T) {
	db := openDeviceClaimPostgres(t)
	_ = db
	ownerTenant := "tenant-owner-claim-2"
	number := time.Now().UTC().Format("claim150405.000000001")
	device := seedClaimFixture(t, db, ownerTenant, number)

	svc := &DeviceClaim{}
	stranger := &utils.UserClaims{ID: "user-stranger", TenantID: "tenant-stranger-claim", Authority: "TENANT_ADMIN"}
	if _, err := svc.IssueClaimToken(nil, &model.IssueDeviceClaimTokenReq{DeviceID: device.ID}, stranger); err == nil {
		t.Fatal("issuing a claim token for another tenant's device must be rejected")
	} else if !strings.Contains(err.Error(), "100404") {
		// 存在性错误不得区分"无此设备/不属于你"——统一 not found。
		t.Fatalf("ownership violation must look like not found, got %v", err)
	}
}
