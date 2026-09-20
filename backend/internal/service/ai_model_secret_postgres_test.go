// 文件用途：AI 凭证静态加密的运行期证据（ROADMAP P0.7）。
// 覆盖：明文密钥绝不落库、租户绑定 AAD 使跨租户搬运不可用、
// 出参只出不可逆掩码、主密钥缺失时 fail closed、迁移窗口内遗留明文可读且标记需重封。
// 说明：需要真实 PostgreSQL（ai_models 表）。缺 DSN 或缺表一律 Skip，不得把 Skip 当通过。
package service

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/secrets"
	"github.com/go-basic/uuid"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// withTestMasterKey 注入一个临时主密钥，测试结束恢复原配置。
// 注意 viper 是全局的，必须清理，否则会污染同进程内其他用例。
func withTestMasterKey(t *testing.T) {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("generate master key: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	prevActive := viper.GetString("secrets.active_key_id")
	prevKey := viper.GetString("secrets.master_keys.ev-test-key")
	viper.Set("secrets.active_key_id", "ev-test-key")
	viper.Set("secrets.master_keys.ev-test-key", encoded)
	t.Cleanup(func() {
		viper.Set("secrets.active_key_id", prevActive)
		viper.Set("secrets.master_keys.ev-test-key", prevKey)
	})
}

// openAiModelPostgres 打开数据库并校验 ai_models 表存在。
func openAiModelPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("AETHERLINK_TEST_PSQL_DSN"))
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; ai model secret tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	global.DB = db
	var exists bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='ai_models')").Scan(&exists).Error; err != nil {
		t.Fatalf("probe ai_models table: %v", err)
	}
	if !exists {
		t.Skip("ai_models missing; apply migrations first")
	}
	return db
}

// seedAiModel 写入一条 AI 模型档案，返回记录与所用明文密钥。
func seedAiModel(t *testing.T, db *gorm.DB, tenantID, plain string) *model.AiModel {
	t.Helper()
	now := time.Now().UTC()
	row := &model.AiModel{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Name:      "evidence-model",
		Provider:  "openai",
		BaseURL:   "https://example.invalid/v1",
		Model:     "gpt-evidence",
		APIKey:    plain,
		Purpose:   model.AiModelPurposeChat,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := sealModelAPIKey(row, plain); err != nil {
		t.Fatalf("seal api key: %v", err)
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("insert ai model: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM ai_models WHERE id = ?", row.ID).Error
	})
	return row
}

// storedAPIKey 直接读取数据库里存的值（绕过解密），用于断言"库里没有明文"。
func storedAPIKey(t *testing.T, db *gorm.DB, id string) string {
	t.Helper()
	var value string
	if err := db.Raw("SELECT api_key FROM ai_models WHERE id = ?", id).Scan(&value).Error; err != nil {
		t.Fatalf("read stored api key: %v", err)
	}
	return value
}

// TestAiModelAPIKeyNeverHitsDatabaseAsPlaintext 核心门禁：库里不得出现明文密钥。
func TestAiModelAPIKeyNeverHitsDatabaseAsPlaintext(t *testing.T) {
	db := openAiModelPostgres(t)
	withTestMasterKey(t)
	tenant := "tenant-secret-evidence"
	plain := "sk-evidence-PLAINTEXT-0123456789abcdef"

	row := seedAiModel(t, db, tenant, plain)

	stored := storedAPIKey(t, db, row.ID)
	if strings.Contains(stored, plain) {
		t.Fatalf("plaintext api key leaked into database: %s", stored)
	}
	if !secrets.IsEnvelope(stored) {
		t.Fatalf("stored value is not an envelope: %s", stored)
	}

	// 读回必须还原出同样的明文，否则加密就是有损的。
	reloaded := &model.AiModel{}
	if err := db.Where("id = ?", row.ID).First(reloaded).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	got, _, err := openModelAPIKey(reloaded)
	if err != nil {
		t.Fatalf("open api key: %v", err)
	}
	if got != plain {
		t.Fatalf("round-trip mismatch: got %q want %q", got, plain)
	}
}

// TestAiModelCiphertextCannotBeMovedAcrossTenants AAD 绑定租户：
// 把 A 租户的密文当成 B 租户的来解，必须失败，不能冒充成可用凭证。
func TestAiModelCiphertextCannotBeMovedAcrossTenants(t *testing.T) {
	db := openAiModelPostgres(t)
	withTestMasterKey(t)
	row := seedAiModel(t, db, "tenant-a", "sk-evidence-cross-tenant-0001")

	stored := storedAPIKey(t, db, row.ID)
	moved := &model.AiModel{APIKey: stored, TenantID: "tenant-b"}
	if _, _, err := openModelAPIKey(moved); err == nil {
		t.Fatalf("ciphertext moved to another tenant must not open")
	}
}

// TestAiModelMaskedOutputHidesPlaintext 出参掩码不得回显明文。
func TestAiModelMaskedOutputHidesPlaintext(t *testing.T) {
	db := openAiModelPostgres(t)
	withTestMasterKey(t)
	plain := "sk-evidence-mask-ABCDEFGHIJKLMNOP"
	row := seedAiModel(t, db, "tenant-mask", plain)

	resp := aiModelMasked(row, plain)
	if strings.Contains(resp.APIKeyMasked, plain) {
		t.Fatalf("masked output leaks plaintext: %s", resp.APIKeyMasked)
	}
	if resp.APIKeyMasked == "" {
		t.Fatalf("masked output must not be empty")
	}
}

// TestAiModelSealFailsClosedWithoutMasterKey 主密钥缺失时必须 fail closed，
// 绝不降级为明文落库。
func TestAiModelSealFailsClosedWithoutMasterKey(t *testing.T) {
	openAiModelPostgres(t)
	prevActive := viper.GetString("secrets.active_key_id")
	viper.Set("secrets.active_key_id", "")
	t.Cleanup(func() { viper.Set("secrets.active_key_id", prevActive) })

	row := &model.AiModel{TenantID: "tenant-nokey", APIKey: "sk-plain"}
	if err := sealModelAPIKey(row, "sk-plain"); err == nil {
		t.Fatalf("seal must fail closed when master key is missing")
	}
	if !strings.Contains(row.APIKey, "sk-plain") {
		// 未加密成功时保持原值是允许的；关键是绝不能"看似成功"地写入明文副本。
		t.Fatalf("unexpected api key state after failed seal: %s", row.APIKey)
	}
}

// TestAiModelLegacyPlaintextReadableAndMarkedResealable 迁移窗口内遗留明文行仍可读，
// 且必须标记为需要重新封装（自愈式轮换的前提）。
func TestAiModelLegacyPlaintextReadableAndMarkedResealable(t *testing.T) {
	openAiModelPostgres(t)
	legacy := &model.AiModel{TenantID: "tenant-legacy", APIKey: "sk-legacy-plaintext"}
	plain, needsReseal, err := openModelAPIKey(legacy)
	if err != nil {
		t.Fatalf("legacy plaintext must remain readable: %v", err)
	}
	if plain != "sk-legacy-plaintext" {
		t.Fatalf("legacy readback mismatch: %q", plain)
	}
	if !needsReseal {
		t.Fatalf("legacy plaintext must be marked as needing reseal")
	}
}
