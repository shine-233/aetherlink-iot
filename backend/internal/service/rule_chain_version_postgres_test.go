// 文件用途：规则链版本持久化的运行期证据（ROADMAP P1.2）。
// 覆盖：草稿创建、图哈希去重、发布、重复发布 fail closed、回滚造新草稿且原版本只读、
// 双向审计、跨租户隔离、单 published 不变式由数据库守住。
// 说明：需要真实 PostgreSQL（迁移 93 建表后）。缺 DSN 或缺表一律 Skip，
// **不得**把 Skip 当作通过。
package service

import (
	"os"
	"strings"
	"testing"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/global"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openRuleChainVersionPostgres 打开数据库并校验版本表存在。
func openRuleChainVersionPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("AETHERLINK_TEST_PSQL_DSN"))
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; rule chain version tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	global.DB = db
	var exists bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='rule_chain_versions')").Scan(&exists).Error; err != nil {
		t.Fatalf("probe version table: %v", err)
	}
	if !exists {
		t.Skip("rule_chain_versions missing; apply migrations first")
	}
	return db
}

// uniqueRuleChainID 生成互不干扰的链 ID，便于并发或重复执行。
func uniqueRuleChainID(t *testing.T) string {
	t.Helper()
	id := "chain-v93-" + time.Now().UTC().Format("20060102150405.000000000")
	t.Cleanup(func() {
		if global.DB != nil {
			_, _ = global.DB.DB()
			_ = global.DB.Exec("DELETE FROM rule_chain_versions WHERE chain_id = ?", id).Error
		}
	})
	return id
}

func TestRuleChainVersionPersistenceLifecycle(t *testing.T) {
	db := openRuleChainVersionPostgres(t)
	tenant := "tenant-verify"
	chain := uniqueRuleChainID(t)

	// 1) 创建草稿 v1
	v1, _, err := CreateRuleChainDraftVersion(tenant, chain, "hash-a", []byte(`{"nodes":[]}`))
	if err != nil {
		t.Fatalf("create draft v1: %v", err)
	}
	if v1 == nil || v1.Version != 1 || v1.Status != "draft" {
		t.Fatalf("draft v1 unexpected: %+v", v1)
	}

	// 2) 相同图哈希不得产生空版本
	dup, _, err := CreateRuleChainDraftVersion(tenant, chain, "hash-a", []byte(`{"nodes":[]}`))
	if err != nil {
		t.Fatalf("duplicate draft: %v", err)
	}
	if dup != nil {
		t.Fatalf("identical graph must not create another draft, got v%d", dup.Version)
	}

	// 3) 发布
	if _, err := PublishRuleChainVersionRecord(tenant, chain, 1); err != nil {
		t.Fatalf("publish v1: %v", err)
	}

	// 4) 重复发布必须失败
	if _, err := PublishRuleChainVersionRecord(tenant, chain, 1); err == nil {
		t.Fatalf("re-publish must fail closed")
	}

	// 5) 回滚造新草稿，原版本保持 published
	rolled, audits, err := RollbackRuleChainVersionRecord(tenant, chain, 1)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rolled == nil || rolled.Version != 2 || rolled.Status != "draft" {
		t.Fatalf("rollback result unexpected: %+v", rolled)
	}
	if rolled.RolledBackFrom == nil || *rolled.RolledBackFrom != 1 {
		t.Fatalf("rollback must record source version 1, got %v", rolled.RolledBackFrom)
	}
	if len(audits) != 2 {
		t.Fatalf("rollback must emit 2 audit entries, got %d", len(audits))
	}
	back, err := dal.GetRuleChainVersion(tenant, chain, 1)
	if err != nil {
		t.Fatalf("reload v1: %v", err)
	}
	if back.Status != "published" {
		t.Fatalf("original published version must stay read-only, got %s", back.Status)
	}

	// 6) 跨租户隔离
	rows, err := dal.ListRuleChainVersions("tenant-other", chain)
	if err != nil {
		t.Fatalf("list other tenant: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("cross-tenant must be empty, got %d rows", len(rows))
	}

	_ = db
}

func TestRuleChainVersionSinglePublishedEnforcedByDatabase(t *testing.T) {
	openRuleChainVersionPostgres(t)
	tenant := "tenant-verify"
	chain := uniqueRuleChainID(t)

	v1, _, err := CreateRuleChainDraftVersion(tenant, chain, "hash-a", []byte(`{"nodes":[]}`))
	if err != nil || v1 == nil {
		t.Fatalf("create draft v1: %v", err)
	}
	if _, err := PublishRuleChainVersionRecord(tenant, chain, v1.Version); err != nil {
		t.Fatalf("publish v1: %v", err)
	}

	v2, _, err := CreateRuleChainDraftVersion(tenant, chain, "hash-b", []byte(`{"nodes":[]}`))
	if err != nil || v2 == nil {
		t.Fatalf("create draft v2: %v", err)
	}
	// 第二个 published 必须被部分唯一索引拒绝；若应用层放行了就是不变式失守。
	if _, err := PublishRuleChainVersionRecord(tenant, chain, v2.Version); err == nil {
		t.Fatalf("second published version must be rejected by partial unique index")
	}
}
