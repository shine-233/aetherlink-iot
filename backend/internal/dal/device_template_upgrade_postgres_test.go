// 文件用途：P1.6 模板升级历史的运行期证据（升级→留回滚凭据→按凭据重放）。
// 覆盖：历史插入/按租户读取/按模板列表、最新版本行定位（created_at 序，非字典序版本）、
// previous_payload 往返保真。跨租户表现为未命中。
// 说明：需要真实 PostgreSQL。缺 DSN 或缺表一律 Skip，不得把 Skip 当作通过。
package dal

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"github.com/go-basic/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openUpgradeHistoryPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("AETHERLINK_TEST_PSQL_DSN"))
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; template upgrade tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	global.DB = db
	var exists bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='device_template_upgrade_history')").Scan(&exists).Error; err != nil {
		t.Fatalf("probe upgrade history table: %v", err)
	}
	if !exists {
		t.Skip("device_template_upgrade_history missing; apply migration 99 first")
	}
	return db
}

func TestTemplateUpgradeHistoryRoundtrip(t *testing.T) {
	openUpgradeHistoryPostgres(t)
	tenantID := "upgrade-test-tenant-" + uuid.New()[:8]

	payload := model.DeviceTemplateExport{
		Kind: "aetherlink-device-template", Name: "tpl-a", Version: ptrStringUpgrade("1.0.0"),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	history := &model.TemplateUpgradeHistory{
		ID:              uuid.New(),
		TenantID:        tenantID,
		TemplateName:    "tpl-a",
		FromVersion:     "1.0.0",
		ToVersion:       "2.0.0",
		PreviousPayload: string(raw),
		ActorID:         "actor-1",
		CreatedAt:       time.Now().UTC(),
	}
	if err := InsertTemplateUpgradeHistory(history); err != nil {
		t.Fatalf("insert history: %v", err)
	}

	// 本租户可读且载荷保真。
	got, err := GetTemplateUpgradeHistoryInTenant(history.ID, tenantID)
	if err != nil {
		t.Fatalf("get in tenant: %v", err)
	}
	var restored model.DeviceTemplateExport
	if err := json.Unmarshal([]byte(got.PreviousPayload), &restored); err != nil {
		t.Fatalf("previous payload corrupted: %v", err)
	}
	if restored.Name != "tpl-a" || restored.Version == nil || *restored.Version != "1.0.0" {
		t.Fatalf("payload roundtrip lost fields: %+v", restored)
	}

	// 跨租户未命中（不暴露存在性）。
	if _, err := GetTemplateUpgradeHistoryInTenant(history.ID, "other-tenant"); err == nil {
		t.Fatal("cross-tenant history readable")
	}

	// 按模板名列表。
	rows, err := ListTemplateUpgradeHistoryInTenant(tenantID, "tpl-a", 0)
	if err != nil || len(rows) != 1 {
		t.Fatalf("list by template: rows=%d err=%v", len(rows), err)
	}
}

func TestGetLatestDeviceTemplateByNameOrdering(t *testing.T) {
	openUpgradeHistoryPostgres(t)
	tenantID := "upgrade-test-tenant-" + uuid.New()[:8]
	now := time.Now().UTC()
	// 后插入的行 created_at 更新；版本号 1.10 若按字典序会排在 1.2 前——
	// 最新行定位必须按 created_at,这里用两条不同 created_at 验证。
	first := &model.DeviceTemplate{ID: uuid.New(), Name: "tpl-latest", TenantID: tenantID, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	second := &model.DeviceTemplate{ID: uuid.New(), Name: "tpl-latest", TenantID: tenantID, CreatedAt: now, UpdatedAt: now}
	if err := global.DB.Create(first).Error; err != nil {
		t.Skipf("create fixture template failed (schema drift): %v", err)
	}
	if err := global.DB.Create(second).Error; err != nil {
		t.Fatalf("create second fixture: %v", err)
	}
	latest, err := GetLatestDeviceTemplateByName(tenantID, "tpl-latest")
	if err != nil || latest == nil {
		t.Fatalf("latest lookup: %v %v", latest, err)
	}
	if latest.ID != second.ID {
		t.Fatalf("latest = %s, want the later-created row %s", latest.ID, second.ID)
	}
	if missing, err := GetLatestDeviceTemplateByName(tenantID, "tpl-not-exists"); missing != nil || err != nil {
		t.Fatalf("missing template lookup = %v %v, want nil/nil", missing, err)
	}
}

func ptrStringUpgrade(s string) *string { return &s }
