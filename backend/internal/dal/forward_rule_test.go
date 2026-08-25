// 文件用途：数据转发规则 DAL 的 sqlite 内存库单测——CRUD、租户隔离与分页过滤。
// 核心逻辑：复用 newDalListLimitTestDB 的内存库 harness，AutoMigrate 手写模型后
// 逐项验证 Create/Update/Delete/Toggle/分页 的行为契约（未命中必须报错）。
// 关键注意事项：raw 链查询不依赖 gen；断言聚焦租户隔离与 RowsAffected 守卫。

package dal

import (
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"

	"github.com/google/uuid"
)

func TestForwardRuleCRUDAndTenantIsolation(t *testing.T) {
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.ForwardRule{}); err != nil {
		t.Fatalf("migrate forward rules: %v", err)
	}

	rule := &model.ForwardRule{
		ID:         uuid.New().String(),
		TenantID:   "tenant-a",
		Name:       "forward-http",
		Enabled:    true,
		SourceType: "telemetry",
		TargetType: "http",
	}
	if err := CreateForwardRule(rule); err != nil {
		t.Fatalf("create forward rule: %v", err)
	}

	// 租户隔离：另一租户不可见。
	if _, err := GetForwardRuleByID(rule.ID, "tenant-b"); err == nil {
		t.Fatal("expected record-not-found for foreign tenant")
	}

	got, err := GetForwardRuleByID(rule.ID, "tenant-a")
	if err != nil {
		t.Fatalf("get forward rule: %v", err)
	}
	if got.Name != "forward-http" || !got.Enabled {
		t.Fatalf("unexpected rule %#v", got)
	}

	// 更新：正常路径命中一行。
	got.Name = "forward-http-v2"
	if err := UpdateForwardRule(got, "tenant-a"); err != nil {
		t.Fatalf("update forward rule: %v", err)
	}

	// 未命中必须显式报错（RowsAffected 守卫）。
	missing := *got
	missing.ID = uuid.New().String()
	if err := UpdateForwardRule(&missing, "tenant-a"); err == nil {
		t.Fatal("expected update on missing id to fail")
	}

	// 分页 + 名称过滤。
	total, rows, err := GetForwardRuleListByPage(&model.GetForwardRuleListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10},
		Name:    strPtrForward("forward"),
	}, "tenant-a")
	if err != nil {
		t.Fatalf("list forward rules: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Name != "forward-http-v2" {
		t.Fatalf("list = total %d rows %#v", total, rows)
	}

	// 启停切换。
	if err := SetForwardRuleEnabled(rule.ID, "tenant-a", false); err != nil {
		t.Fatalf("toggle forward rule: %v", err)
	}
	rowsAfter, err := ListEnabledForwardRules("telemetry")
	if err != nil {
		t.Fatalf("list enabled forward rules: %v", err)
	}
	if len(rowsAfter) != 0 {
		t.Fatalf("disabled rule must not appear in enabled list, got %d", len(rowsAfter))
	}

	// 删除后再查应失败。
	if err := DeleteForwardRule(rule.ID, "tenant-a"); err != nil {
		t.Fatalf("delete forward rule: %v", err)
	}
	if err := DeleteForwardRule(rule.ID, "tenant-a"); err == nil {
		t.Fatal("expected second delete to fail with record-not-found")
	}
	_ = time.Now()
}

func strPtrForward(s string) *string { return &s }
