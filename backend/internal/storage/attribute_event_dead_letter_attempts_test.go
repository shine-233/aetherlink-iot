// attribute_event_dead_letter_attempts_test.go 覆盖 dead-letter 的 attempts 记账。
// 这里刻意使用纯 Go 的 glebarez/sqlite 驱动（与 attribute_current_upsert_test.go 一致），
// 因为本机 CGO_ENABLED=0，mattn/go-sqlite3 无法运行。
package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDeadLetterAttemptsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&uplinkStorageDeadLetter{}); err != nil {
		t.Fatalf("migrate dead-letter table: %v", err)
	}
	return db
}

func seedDeadLetterRow(t *testing.T, db *gorm.DB, row uplinkStorageDeadLetter) {
	t.Helper()
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed dead-letter row: %v", err)
	}
}

func loadDeadLetterRow(t *testing.T, db *gorm.DB, id string) uplinkStorageDeadLetter {
	t.Helper()
	var row uplinkStorageDeadLetter
	if err := db.Where("id = ?", id).Take(&row).Error; err != nil {
		t.Fatalf("load dead-letter row: %v", err)
	}
	return row
}

// 回归点：claim 必须自增 attempts。修复前只有 deferDeadLetterRetry 会记账，
// 因此 worker 在 replay 中途崩溃的行会被无限重新 claim 而永不到达上限。
func TestClaimDeadLetterIncrementsAttempts(t *testing.T) {
	db := setupDeadLetterAttemptsTestDB(t)
	ingress := &attributeEventIngress{db: db, logger: nil, config: Config{}, metrics: newMetricsCollector(false)}
	now := time.Now().UTC()

	seedDeadLetterRow(t, db, uplinkStorageDeadLetter{
		ID:        "11111111-1111-4111-8111-111111111111",
		DataType:  DataTypeEvent,
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		TS:        now.UnixMilli(),
		Payload:   []byte(`{}`),
		Status:    uplinkDeadLetterStatusPending,
		Attempts:  0,
		CreatedAt: now,
		UpdatedAt: now,
	})

	// 模拟 worker 反复崩溃：每轮只 claim，不结算。
	for round := 1; round <= attributeEventDeadLetterMaxAttempts; round++ {
		claimAt := now.Add(time.Duration(round) * time.Hour)
		row, claimed, err := ingress.claimDeadLetter(context.Background(), "11111111-1111-4111-8111-111111111111", claimAt)
		if err != nil {
			t.Fatalf("round %d claim: %v", round, err)
		}
		if !claimed {
			t.Fatalf("round %d: row was not claimable, attempts accounting regressed", round)
		}
		if row.Attempts != round {
			t.Fatalf("round %d: attempts = %d, want %d", round, row.Attempts, round)
		}
	}

	// 到达上限后必须不可再 claim，否则就是无限循环。
	exhaustedAt := now.Add(100 * time.Hour)
	if _, claimed, err := ingress.claimDeadLetter(context.Background(), "11111111-1111-4111-8111-111111111111", exhaustedAt); err != nil {
		t.Fatalf("exhausted claim: %v", err)
	} else if claimed {
		t.Fatal("row was claimed after exhausting attempts; crash-looping row would cycle forever")
	}
}

// 回归点：attempts 在 claim 时自增后，一条耗尽次数却仍停在 processing 的孤儿行
// 既不会被 claimableDeadLetterQuery 选中，也不被 operator 的状态白名单接受。
// reaper 必须把它显式落到 dead，让它可见可处置。
func TestReapExhaustedDeadLetterClaimsMarksStaleProcessingDead(t *testing.T) {
	db := setupDeadLetterAttemptsTestDB(t)
	ingress := &attributeEventIngress{db: db, logger: nil, config: Config{}, metrics: newMetricsCollector(false)}
	now := time.Now().UTC()
	expiredLease := now.Add(-time.Minute)
	activeLease := now.Add(time.Hour)
	token := "22222222-2222-4222-8222-222222222222"

	// 耗尽次数 + 租约已过期 -> 应被回收。
	seedDeadLetterRow(t, db, uplinkStorageDeadLetter{
		ID: "aaaaaaaa-1111-4111-8111-111111111111", DataType: DataTypeEvent,
		DeviceID: "device-1", TenantID: "tenant-1", TS: now.UnixMilli(), Payload: []byte(`{}`),
		Status: uplinkDeadLetterStatusProcessing, Attempts: attributeEventDeadLetterMaxAttempts,
		ClaimToken: &token, LeaseUntil: &expiredLease, CreatedAt: now, UpdatedAt: now,
	})
	// 耗尽次数但租约仍有效 -> 另一个 worker 可能还在处理，不能抢。
	seedDeadLetterRow(t, db, uplinkStorageDeadLetter{
		ID: "bbbbbbbb-1111-4111-8111-111111111111", DataType: DataTypeEvent,
		DeviceID: "device-2", TenantID: "tenant-1", TS: now.UnixMilli(), Payload: []byte(`{}`),
		Status: uplinkDeadLetterStatusProcessing, Attempts: attributeEventDeadLetterMaxAttempts,
		ClaimToken: &token, LeaseUntil: &activeLease, CreatedAt: now, UpdatedAt: now,
	})
	// 未耗尽次数、租约已过期 -> 仍应交给正常 replay 重试，不能被判死。
	seedDeadLetterRow(t, db, uplinkStorageDeadLetter{
		ID: "cccccccc-1111-4111-8111-111111111111", DataType: DataTypeEvent,
		DeviceID: "device-3", TenantID: "tenant-1", TS: now.UnixMilli(), Payload: []byte(`{}`),
		Status: uplinkDeadLetterStatusProcessing, Attempts: 1,
		ClaimToken: &token, LeaseUntil: &expiredLease, CreatedAt: now, UpdatedAt: now,
	})

	if err := ingress.reapExhaustedDeadLetterClaims(context.Background(), now); err != nil {
		t.Fatalf("reap: %v", err)
	}

	reaped := loadDeadLetterRow(t, db, "aaaaaaaa-1111-4111-8111-111111111111")
	if reaped.Status != uplinkDeadLetterStatusDead {
		t.Fatalf("exhausted stale row status = %q, want %q", reaped.Status, uplinkDeadLetterStatusDead)
	}
	if reaped.ClaimToken != nil || reaped.LeaseUntil != nil {
		t.Fatal("reaped row must release its claim token and lease")
	}
	if reaped.LastError != "PRIMARY_REPLAY_FAILED" {
		t.Fatalf("reaped row last_error = %q", reaped.LastError)
	}

	if held := loadDeadLetterRow(t, db, "bbbbbbbb-1111-4111-8111-111111111111"); held.Status != uplinkDeadLetterStatusProcessing {
		t.Fatalf("row with live lease was reaped: status = %q", held.Status)
	}
	if retryable := loadDeadLetterRow(t, db, "cccccccc-1111-4111-8111-111111111111"); retryable.Status != uplinkDeadLetterStatusProcessing {
		t.Fatalf("row below attempt cap was reaped: status = %q", retryable.Status)
	}
}

func TestAttributeEventDeadLetterManualUpdateUsesExpectedStatus(t *testing.T) {
	db := setupDeadLetterAttemptsTestDB(t)
	ingress := &attributeEventIngress{db: db, logger: nil, config: Config{}, metrics: newMetricsCollector(false)}
	now := time.Now().UTC()
	id := "dddddddd-1111-4111-8111-111111111111"

	seedDeadLetterRow(t, db, uplinkStorageDeadLetter{
		ID: id, DataType: DataTypeEvent, DeviceID: "device-1", TenantID: "tenant-1",
		TS: now.UnixMilli(), Payload: []byte(`{}`), Status: uplinkDeadLetterStatusPending,
		CreatedAt: now, UpdatedAt: now,
	})
	filter := AttributeEventDeadLetterFilter{
		TenantID: "tenant-1",
		Status:   uplinkDeadLetterStatusPending,
	}
	if err := ingress.UpdateAttributeEventDeadLetter(
		context.Background(),
		id,
		AttributeEventDeadLetterActionResolve,
		filter,
	); err != nil {
		t.Fatalf("resolve pending row: %v", err)
	}

	err := ingress.UpdateAttributeEventDeadLetter(
		context.Background(),
		id,
		AttributeEventDeadLetterActionIgnore,
		filter,
	)
	if !errors.Is(err, ErrAttributeEventDeadLetterStatusConflict) {
		t.Fatalf("stale expected status error = %v, want status conflict", err)
	}
	if row := loadDeadLetterRow(t, db, id); row.Status != uplinkDeadLetterStatusResolved {
		t.Fatalf("stale action overwrote status: got %q", row.Status)
	}
}

func TestAttributeEventDeadLetterDrainReapsOnlyScopedExhaustedClaims(t *testing.T) {
	db := setupDeadLetterAttemptsTestDB(t)
	ingress := &attributeEventIngress{db: db, logger: nil, config: Config{}, metrics: newMetricsCollector(false)}
	now := time.Now().UTC()
	expiredLease := now.Add(-time.Minute)
	token := "eeeeeeee-1111-4111-8111-111111111111"

	for _, tenantID := range []string{"tenant-1", "tenant-2"} {
		seedDeadLetterRow(t, db, uplinkStorageDeadLetter{
			ID: fmt.Sprintf("%s-1111-4111-8111-111111111111", map[string]string{
				"tenant-1": "11111111",
				"tenant-2": "22222222",
			}[tenantID]),
			DataType: DataTypeEvent, DeviceID: "device-1", TenantID: tenantID,
			TS: now.UnixMilli(), Payload: []byte(`{}`), Status: uplinkDeadLetterStatusProcessing,
			Attempts: attributeEventDeadLetterMaxAttempts, ClaimToken: &token, LeaseUntil: &expiredLease,
			CreatedAt: now, UpdatedAt: now,
		})
	}

	result, err := ingress.DrainAttributeEventDeadLetters(
		context.Background(),
		AttributeEventDeadLetterFilter{
			TenantID: "tenant-1",
			Status:   uplinkDeadLetterStatusPending,
		},
		10,
	)
	if err != nil {
		t.Fatalf("drain tenant scope: %v", err)
	}
	if result.Attempted != 0 || result.Replayed != 0 {
		t.Fatalf("drain result = %+v, exhausted claims should be reaped outside the candidate status selector", result)
	}
	if row := loadDeadLetterRow(t, db, "11111111-1111-4111-8111-111111111111"); row.Status != uplinkDeadLetterStatusDead {
		t.Fatalf("scoped exhausted row status = %q, want dead", row.Status)
	}
	if row := loadDeadLetterRow(t, db, "22222222-1111-4111-8111-111111111111"); row.Status != uplinkDeadLetterStatusProcessing {
		t.Fatalf("other tenant row was mutated: status = %q", row.Status)
	}
}
