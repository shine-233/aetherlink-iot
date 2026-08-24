// 文件用途：验证设备认证 template_secret 的静态哈希存储与双读兼容语义。
// 核心逻辑：覆盖新建即摘要、历史明文行登录、登录后惰性升级为摘要、错误密钥拒绝四条路径。
// 关键注意事项：不得因兼容逻辑放宽鉴权；明文命中必须回写摘要，错误密钥不得触发任何升级写入。
package service

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDeviceAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Device{},
		&model.DeviceConfig{},
		&model.Group{},
	); err != nil {
		t.Fatalf("migrate device auth test tables: %v", err)
	}
	// 凭证哈希存储 Phase 1：voucher_hash 列由 50.sql 迁移负责，gen 模型无该字段，
	// 内存库手工补列以匹配生产 schema（CreateDevice 二段式写入需要）。
	if err := db.Exec(`ALTER TABLE devices ADD COLUMN voucher_hash varchar(64)`).Error; err != nil {
		t.Fatalf("add voucher_hash column: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}

func seedDeviceAuthConfig(t *testing.T, db *gorm.DB, id string, tenantID string, templateSecret string) {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Create(&model.DeviceConfig{
		ID:             id,
		Name:           id,
		DeviceType:     "1",
		TenantID:       tenantID,
		CreatedAt:      now,
		UpdatedAt:      now,
		AutoRegister:   1,
		TemplateSecret: &templateSecret,
	}).Error; err != nil {
		t.Fatalf("create device config %s: %v", id, err)
	}
}

func loadDeviceAuthStoredSecret(t *testing.T, db *gorm.DB, configID string) string {
	t.Helper()

	var config model.DeviceConfig
	if err := db.First(&config, "id = ?", configID).Error; err != nil {
		t.Fatalf("reload device config %s: %v", configID, err)
	}
	if config.TemplateSecret == nil {
		t.Fatalf("device config %s has no template_secret stored", configID)
	}
	return *config.TemplateSecret
}

// assertDeviceAuthDigestFormat 断言值为 64 位 hex 摘要并原样返回，供后续等值比较。
func assertDeviceAuthDigestFormat(t *testing.T, value string) string {
	t.Helper()

	if len(value) != 64 {
		t.Fatalf("stored template_secret length = %d, want 64-char sha256 hex digest", len(value))
	}
	if _, err := hex.DecodeString(value); err != nil {
		t.Fatalf("stored template_secret = %q, want hex digest: %v", value, err)
	}
	return value
}

func countDeviceAuthDevicesByNumber(t *testing.T, db *gorm.DB, deviceNumber string) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&model.Device{}).Where("device_number = ?", deviceNumber).Count(&count).Error; err != nil {
		t.Fatalf("count devices by number %s: %v", deviceNumber, err)
	}
	return count
}

// TestCreateDeviceConfigStoresTemplateSecretDigest 覆盖“新建即摘要”：
// 响应中仅回显一次明文，落库保存 SHA-256 摘要。
func TestCreateDeviceConfigStoresTemplateSecretDigest(t *testing.T) {
	db := setupDeviceAuthTestDB(t)

	result, err := (&DeviceConfig{}).CreateDeviceConfig(&model.CreateDeviceConfigReq{
		Name:       "auth-config",
		DeviceType: "1",
	}, &utils.UserClaims{TenantID: "tenant-a"})
	if err != nil {
		t.Fatalf("CreateDeviceConfig failed: %v", err)
	}
	if result.TemplateSecret == nil || *result.TemplateSecret == "" {
		t.Fatal("create response must echo the plaintext secret once")
	}

	stored := assertDeviceAuthDigestFormat(t, loadDeviceAuthStoredSecret(t, db, result.ID))
	if stored == *result.TemplateSecret {
		t.Fatal("create response must return plaintext instead of the stored digest")
	}
	if want := dal.HashTemplateSecret(*result.TemplateSecret); stored != want {
		t.Fatalf("stored digest = %q, want sha256 of echoed secret %q", stored, want)
	}
}

// TestDeviceAuthDigestRowCanLogin 覆盖摘要行的正常登录路径。
func TestDeviceAuthDigestRowCanLogin(t *testing.T) {
	db := setupDeviceAuthTestDB(t)

	result, err := (&DeviceConfig{}).CreateDeviceConfig(&model.CreateDeviceConfigReq{
		Name:       "auth-config",
		DeviceType: "1",
	}, &utils.UserClaims{TenantID: "tenant-a"})
	if err != nil {
		t.Fatalf("CreateDeviceConfig failed: %v", err)
	}
	// 新建配置默认关闭自动注册，这里模拟管理员开启后再下发设备认证。
	if err := db.Model(&model.DeviceConfig{}).Where("id = ?", result.ID).Update("auto_register", 1).Error; err != nil {
		t.Fatalf("enable auto_register: %v", err)
	}

	res, err := (&DeviceAuth{}).Auth(&model.DeviceAuthReq{
		TemplateSecret: *result.TemplateSecret,
		DeviceNumber:   "digest-device-001",
	})
	if err != nil {
		t.Fatalf("Auth with digest-stored secret failed: %v", err)
	}
	if res == nil || res.DeviceID == "" || res.Voucher == "" {
		t.Fatalf("unexpected auth response: %#v", res)
	}
	if got := countDeviceAuthDevicesByNumber(t, db, "digest-device-001"); got != 1 {
		t.Fatalf("device rows for digest-device-001 = %d, want 1", got)
	}
}

// TestDeviceAuthPlaintextRowCanLogin 覆盖历史明文行无需迁移即可登录。
func TestDeviceAuthPlaintextRowCanLogin(t *testing.T) {
	db := setupDeviceAuthTestDB(t)
	seedDeviceAuthConfig(t, db, "legacy-config", "tenant-a", "legacy-template-secret")

	res, err := (&DeviceAuth{}).Auth(&model.DeviceAuthReq{
		TemplateSecret: "legacy-template-secret",
		DeviceNumber:   "plaintext-device-001",
	})
	if err != nil {
		t.Fatalf("Auth with legacy plaintext secret failed: %v", err)
	}
	if res == nil || res.DeviceID == "" {
		t.Fatalf("unexpected auth response: %#v", res)
	}
}

// TestDeviceAuthUpgradesPlaintextAfterLogin 覆盖明文旧行登录成功后被惰性升级为摘要，
// 且升级后同一明文仍可继续登录。
func TestDeviceAuthUpgradesPlaintextAfterLogin(t *testing.T) {
	db := setupDeviceAuthTestDB(t)
	seedDeviceAuthConfig(t, db, "upgrade-config", "tenant-a", "upgrade-template-secret")

	if _, err := (&DeviceAuth{}).Auth(&model.DeviceAuthReq{
		TemplateSecret: "upgrade-template-secret",
		DeviceNumber:   "upgrade-device-001",
	}); err != nil {
		t.Fatalf("first Auth with plaintext secret failed: %v", err)
	}

	stored := loadDeviceAuthStoredSecret(t, db, "upgrade-config")
	if stored == "upgrade-template-secret" {
		t.Fatal("plaintext row was not lazily upgraded after successful login")
	}
	assertDeviceAuthDigestFormat(t, stored)
	if want := dal.HashTemplateSecret("upgrade-template-secret"); stored != want {
		t.Fatalf("upgraded digest = %q, want %q", stored, want)
	}

	if _, err := (&DeviceAuth{}).Auth(&model.DeviceAuthReq{
		TemplateSecret: "upgrade-template-secret",
		DeviceNumber:   "upgrade-device-002",
	}); err != nil {
		t.Fatalf("second Auth with same secret after upgrade failed: %v", err)
	}
}

// TestDeviceAuthRejectsWrongSecret 覆盖错误密钥拒绝：
// 明文行与摘要行均拒绝，且不触发升级写入或创建设备。
func TestDeviceAuthRejectsWrongSecret(t *testing.T) {
	db := setupDeviceAuthTestDB(t)
	seedDeviceAuthConfig(t, db, "wrong-plain-config", "tenant-a", "real-template-secret")

	err := func() error {
		_, authErr := (&DeviceAuth{}).Auth(&model.DeviceAuthReq{
			TemplateSecret: "bad-template-secret",
			DeviceNumber:   "rejected-device-001",
		})
		return authErr
	}()
	assertDeviceConfigServiceError(t, err, "reject wrong secret on plaintext row", 200080, "")
	if got := loadDeviceAuthStoredSecret(t, db, "wrong-plain-config"); got != "real-template-secret" {
		t.Fatalf("plaintext row was modified after rejected login: %q", got)
	}
	if got := countDeviceAuthDevicesByNumber(t, db, "rejected-device-001"); got != 0 {
		t.Fatalf("device rows for rejected-device-001 = %d, want 0", got)
	}

	digestConfig := &model.DeviceConfig{
		ID:           "wrong-digest-config",
		Name:         "wrong-digest-config",
		DeviceType:   "1",
		TenantID:     "tenant-a",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		AutoRegister: 1,
	}
	digest := dal.HashTemplateSecret("another-real-secret")
	digestConfig.TemplateSecret = &digest
	if err := db.Create(digestConfig).Error; err != nil {
		t.Fatalf("create digest config: %v", err)
	}

	err = func() error {
		_, authErr := (&DeviceAuth{}).Auth(&model.DeviceAuthReq{
			TemplateSecret: "another-real-secret-typo",
			DeviceNumber:   "rejected-device-002",
		})
		return authErr
	}()
	assertDeviceConfigServiceError(t, err, "reject wrong secret on digest row", 200080, "")
	if got := countDeviceAuthDevicesByNumber(t, db, "rejected-device-002"); got != 0 {
		t.Fatalf("device rows for rejected-device-002 = %d, want 0", got)
	}
}
