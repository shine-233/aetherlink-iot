// 文件用途：锁定 email_templates 列表读路径的 C2 自上而下作用域真实结果集。
// 核心逻辑：sqlite 内存库种子多租户模板，验证 scopes 三态（0 fail-closed、1 等价旧单租户、
// >1 IN）与平台默认模板(tenant_id 为空串)由 SYS_ADMIN 以 [""] 作用域可见。
// 关键注意事项：本文件只测管理读路径 ListEmailTemplates；写路径(Update/Delete/SetDefault
// ForScope)与运行时路由 GetEffectiveAlarmEmailTemplate 仍保持严格/系统语义，不在此展开。
package dal

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupEmailTemplateScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EmailTemplate{}); err != nil {
		t.Fatalf("migrate email_templates: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

func seedEmailTemplate(t *testing.T, db *gorm.DB, id, tenantID, name, purpose string, enabled, isDefault bool) {
	t.Helper()
	now := time.Now().UTC()
	tpl := model.EmailTemplate{
		ID:              id,
		TenantID:        tenantID,
		Name:            name,
		Purpose:         purpose,
		SubjectTemplate: "subject-" + id,
		BodyTemplate:    "body-" + id,
		Enabled:         enabled,
		IsDefault:       isDefault,
		CreatedBy:       "scope-test",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&tpl).Error; err != nil {
		t.Fatalf("seed email template %s: %v", id, err)
	}
}

// TestListEmailTemplatesScopeDown 自上而下作用域真实结果集：
// [hq, child] 含两租户、[hq] 只含本租户（等价旧单租户）、空作用域 fail-closed、
// purpose=ALARM 过滤与作用域叠加、平台默认模板(tenant_id 为空串) 仅 [""] 作用域可见。
func TestListEmailTemplatesScopeDown(t *testing.T) {
	db := setupEmailTemplateScopeTestDB(t)
	seedEmailTemplate(t, db, "tpl-hq-default", "hq", "hq-alarm", model.EmailTemplatePurposeAlarm, true, true)
	seedEmailTemplate(t, db, "tpl-hq-alt", "hq", "hq-alt", model.EmailTemplatePurposeAlarm, true, false)
	seedEmailTemplate(t, db, "tpl-child-1", "child", "child-alarm", model.EmailTemplatePurposeAlarm, true, true)
	seedEmailTemplate(t, db, "tpl-x-1", "tenant-x", "x-alarm", model.EmailTemplatePurposeAlarm, false, false)
	// 平台默认模板：tenant_id=''，由 SYS_ADMIN 管理；同租户非 ALARM 用途行必须被过滤。
	seedEmailTemplate(t, db, "tpl-platform", "", "platform-default", model.EmailTemplatePurposeAlarm, true, true)
	seedEmailTemplate(t, db, "tpl-hq-other-purpose", "hq", "hq-other", "WEBHOOK", true, false)

	total, list, err := ListEmailTemplates([]string{"hq", "child"}, 1, 10)
	if err != nil {
		t.Fatalf("scoped list [hq child]: %v", err)
	}
	if total != 3 {
		t.Fatalf("scope [hq child] total=%d, want 3", total)
	}
	seen := map[string]bool{}
	for _, tpl := range list {
		seen[tpl.ID] = true
	}
	for _, want := range []string{"tpl-hq-default", "tpl-hq-alt", "tpl-child-1"} {
		if !seen[want] {
			t.Fatalf("scope [hq child] missing %s; got %#v", want, seen)
		}
	}
	if seen["tpl-x-1"] || seen["tpl-platform"] || seen["tpl-hq-other-purpose"] {
		t.Fatalf("scope [hq child] leaked out-of-scope/other-purpose rows: %#v", seen)
	}

	// 单元素作用域等价旧单租户：hq 看不到 child / tenant-x / 平台模板。
	total, list, err = ListEmailTemplates([]string{"hq"}, 1, 10)
	if err != nil {
		t.Fatalf("scoped list [hq]: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("scope [hq] total=%d rows=%d, want 2", total, len(list))
	}

	// 空作用域 fail-closed：不返回任何租户数据。
	total, list, err = ListEmailTemplates(nil, 1, 10)
	if err != nil {
		t.Fatalf("empty scope: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("empty scope total=%d rows=%d, want 0/0", total, len(list))
	}

	// 平台默认模板作用域 [""]：仅 tenant_id='' 的 ALARM 模板（SYS_ADMIN 管理视角）。
	total, list, err = ListEmailTemplates([]string{""}, 1, 10)
	if err != nil {
		t.Fatalf("platform scope [\"\"]: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != "tpl-platform" {
		t.Fatalf("platform scope total=%d rows=%#v, want only tpl-platform", total, list)
	}
}
