// 文件用途: 覆盖 device_auth.go 设备凭证认证核心的回归测试，验证摘要/明文双读与惰性升级语义不漂移。
// 核心逻辑: 构造 sqlite 内存库行，断言 GetDeviceConfigByTemplateSecret 的命中路径与回写行为。
// 关键注意事项: 摘要行只认摘要比对；历史明文行命中后必须惰性升级为摘要。
// 重构建议: 后续扩展动态认证（一型一密）链路时同步补充对应 DAL 用例。

package dal

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestHashTemplateSecretIsDeterministicHexDigest(t *testing.T) {
	first := HashTemplateSecret("presented-secret")
	second := HashTemplateSecret("presented-secret")

	if first != second {
		t.Fatalf("digest not deterministic: %q vs %q", first, second)
	}
	if len(first) != 64 {
		t.Fatalf("digest length = %d, want 64 hex chars", len(first))
	}
	if !isTemplateSecretDigest(first) {
		t.Fatalf("expected %q to be recognized as digest", first)
	}
	if isTemplateSecretDigest("plain-secret") {
		t.Fatal("plaintext should not be recognized as digest")
	}
	if isTemplateSecretDigest("zz") {
		t.Fatal("non-hex value should not be recognized as digest")
	}
}

func TestGetDeviceConfigByTemplateSecretEmptySecretReturnsNil(t *testing.T) {
	setupDeviceDALTestDB(t)

	row, err := GetDeviceConfigByTemplateSecret("")
	if err != nil {
		t.Fatalf("empty secret returned error: %v", err)
	}
	if row != nil {
		t.Fatalf("empty secret returned row %+v, want nil", row)
	}
}

func TestGetDeviceConfigByTemplateSecretMatchesDigestRow(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	digest := HashTemplateSecret("presented-secret")
	if err := db.Create(&model.DeviceConfig{
		ID:             "config-digest",
		Name:           "config",
		DeviceType:     "1",
		TenantID:       "tenant-1",
		TemplateSecret: &digest,
	}).Error; err != nil {
		t.Fatalf("create config: %v", err)
	}

	row, err := GetDeviceConfigByTemplateSecret("presented-secret")
	if err != nil {
		t.Fatalf("GetDeviceConfigByTemplateSecret returned error: %v", err)
	}
	if row == nil || row.ID != "config-digest" {
		t.Fatalf("row = %+v, want config-digest", row)
	}

	// 错误的呈现值不得命中摘要行。
	wrong, err := GetDeviceConfigByTemplateSecret("wrong-secret")
	if err != nil {
		t.Fatalf("wrong secret returned error: %v", err)
	}
	if wrong != nil {
		t.Fatalf("wrong secret matched row %+v, want nil", wrong)
	}
}

func TestGetDeviceConfigByTemplateSecretLegacyPlaintextLazyUpgrade(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	legacy := "legacy-plaintext"
	if err := db.Create(&model.DeviceConfig{
		ID:             "config-legacy",
		Name:           "config",
		DeviceType:     "1",
		TenantID:       "tenant-1",
		TemplateSecret: &legacy,
	}).Error; err != nil {
		t.Fatalf("create config: %v", err)
	}

	row, err := GetDeviceConfigByTemplateSecret("legacy-plaintext")
	if err != nil {
		t.Fatalf("GetDeviceConfigByTemplateSecret returned error: %v", err)
	}
	if row == nil || row.ID != "config-legacy" {
		t.Fatalf("row = %+v, want config-legacy", row)
	}

	var stored string
	if err := db.Raw(`SELECT template_secret FROM device_configs WHERE id = ?`, "config-legacy").Scan(&stored).Error; err != nil {
		t.Fatalf("load upgraded secret: %v", err)
	}
	if !isTemplateSecretDigest(stored) {
		t.Fatalf("stored secret = %q, want lazy-upgraded digest", stored)
	}
}

func TestGetDeviceConfigByTemplateSecretHexPlaintextOnlyMatchesAsDigest(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	// 库中是恰好 64 位 hex 的“明文”（历史上存了 hex 字符串），按约定视为摘要行，
	// 只能被其原值对应的呈现密钥命中，不能被相同字符串二次命中为明文比对。
	hexPlaintext := HashTemplateSecret("real-presented-value")
	if err := db.Create(&model.DeviceConfig{
		ID:             "config-hex-plaintext",
		Name:           "config",
		DeviceType:     "1",
		TenantID:       "tenant-1",
		TemplateSecret: &hexPlaintext,
	}).Error; err != nil {
		t.Fatalf("create config: %v", err)
	}

	row, err := GetDeviceConfigByTemplateSecret("real-presented-value")
	if err != nil {
		t.Fatalf("GetDeviceConfigByTemplateSecret returned error: %v", err)
	}
	if row == nil || row.ID != "config-hex-plaintext" {
		t.Fatalf("row = %+v, want config-hex-plaintext", row)
	}

	sameString, err := GetDeviceConfigByTemplateSecret(hexPlaintext)
	if err != nil {
		t.Fatalf("hex-as-presented returned error: %v", err)
	}
	if sameString != nil {
		t.Fatalf("hex plaintext string must not match as legacy plaintext, got %+v", sameString)
	}
}

func TestGetProductByProductKey(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	if err := db.AutoMigrate(&model.Product{}); err != nil {
		t.Fatalf("migrate product table: %v", err)
	}
	key := "product-key-1"
	if err := db.Create(&model.Product{
		ID:         "product-1",
		Name:       "product",
		ProductKey: &key,
	}).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	product, err := GetProductByProductKey("product-key-1")
	if err != nil {
		t.Fatalf("GetProductByProductKey returned error: %v", err)
	}
	if product == nil || product.ID != "product-1" {
		t.Fatalf("product = %+v, want product-1", product)
	}

	missing, err := GetProductByProductKey("missing-key")
	if err != nil {
		t.Fatalf("missing key returned error: %v", err)
	}
	if missing != nil {
		t.Fatalf("missing key returned %+v, want nil,nil", missing)
	}
}
