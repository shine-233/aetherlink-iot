// 文件用途：GetNotificationHisoryListByPage 的 ROADMAP C2 自上而下作用域（self∪子孙）真实结果集测试。
// 关键注意事项：scopes 语义 0→fail-closed、1→tenant_id =（旧单租户等价）、>1→tenant_id IN；
// TENANT_USER ownerUserID 与 scopes 组合下，跨作用域/跨租户私有行仍不可见。
package dal

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestGetNotificationHistoryListScopesScopeDown(t *testing.T) {
	db := setupNotificationHistoryDALTestDB(t)
	createNotificationHistoryTestDevice(t, db, "device-hq", "hq", "owner-hq-admin")
	createNotificationHistoryTestDevice(t, db, "device-child", "child", "owner-child-admin")
	createNotificationHistoryTestDevice(t, db, "device-foreign", "tenant-x", "owner-x-admin")

	createNotificationHistoryTestRow(t, "nh-hq", "hq", "device-hq")
	createNotificationHistoryTestRow(t, "nh-hq-unscoped", "hq")
	createNotificationHistoryTestRow(t, "nh-child", "child", "device-child")
	createNotificationHistoryTestRow(t, "nh-foreign", "tenant-x", "device-foreign")

	req := &model.GetNotificationHistoryListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 20},
	}

	// 总部作用域 = self∪子孙 → 只含 hq + child 行，排除 tenant-x。
	total, list, err := GetNotificationHisoryListByPage(req, []string{"hq", "child"}, nil)
	if err != nil {
		t.Fatalf("query scope-down notification history: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("scope-down histories = total %d, list %v; want hq+child 3 rows", total, notificationHistoryIDs(list))
	}
	wantScopeDown := map[string]bool{"nh-hq": true, "nh-hq-unscoped": true, "nh-child": true}
	for _, history := range list {
		if !wantScopeDown[history.ID] {
			t.Fatalf("scope-down returned out-of-scope row %q", history.ID)
		}
	}

	// 单元素作用域 = 旧单租户语义（child 只读自身）。
	total, list, err = GetNotificationHisoryListByPage(req, []string{"child"}, nil)
	if err != nil {
		t.Fatalf("query single-scope notification history: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != "nh-child" {
		t.Fatalf("single-scope histories = total %d, list %v; want only nh-child", total, notificationHistoryIDs(list))
	}

	// 空作用域 fail-closed：返回空而不查全表。
	total, list, err = GetNotificationHisoryListByPage(req, nil, nil)
	if err != nil {
		t.Fatalf("query empty-scope notification history: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("empty-scope histories = total %d, list %v; want none", total, notificationHistoryIDs(list))
	}
	total, list, err = GetNotificationHisoryListByPage(req, []string{}, nil)
	if err != nil {
		t.Fatalf("query zero-len scope notification history: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("zero-len-scope histories = total %d, list %v; want none", total, notificationHistoryIDs(list))
	}
}

func TestGetNotificationHistoryListScopesOwnerStillScoped(t *testing.T) {
	db := setupNotificationHistoryDALTestDB(t)
	createNotificationHistoryTestDevice(t, db, "device-hq-a", "hq", "owner-a")
	createNotificationHistoryTestDevice(t, db, "device-child-b", "child", "owner-b")

	createNotificationHistoryTestRow(t, "nh-hq-owned", "hq", "device-hq-a")
	createNotificationHistoryTestRow(t, "nh-child-b", "child", "device-child-b")
	createNotificationHistoryTestRow(t, "nh-hq-other", "hq")

	req := &model.GetNotificationHistoryListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 20},
	}
	ownerA := "owner-a"
	// 即使把 scope 扩到 child，owner-a 也只看得到自身租户 hq 内与其名下设备绑定的行。
	total, list, err := GetNotificationHisoryListByPage(req, []string{"hq", "child"}, &ownerA)
	if err != nil {
		t.Fatalf("query owner notification history over expanded scopes: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != "nh-hq-owned" {
		t.Fatalf("owner histories = total %d, list %v; want only nh-hq-owned", total, notificationHistoryIDs(list))
	}
}

func TestGetNotificationHistoryListScopesPlatformEmptyTenant(t *testing.T) {
	db := setupNotificationHistoryDALTestDB(t)
	createNotificationHistoryTestDevice(t, db, "device-hq-p", "hq", "owner-hq-admin")
	// 平台空租户行（tenant_id = ''）通常不带设备；CreateNotificationHistory 的写入校验要求
	// device 与 history 同租户，因此空租户行只能无设备落库。
	createNotificationHistoryTestRow(t, "nh-platform", "")
	createNotificationHistoryTestRow(t, "nh-hq-row", "hq", "device-hq-p")

	req := &model.GetNotificationHistoryListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 20},
	}
	// ["") 是 SYS_ADMIN 平台空租户行的旧行为（tenant_id = ''）。
	total, list, err := GetNotificationHisoryListByPage(req, []string{""}, nil)
	if err != nil {
		t.Fatalf("query platform-scope notification history: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != "nh-platform" {
		t.Fatalf("platform histories = total %d, list %v; want only empty-tenant row", total, notificationHistoryIDs(list))
	}
}
