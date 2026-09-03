package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// 这些用例只覆盖纯函数形式的可见性、权限和配额规则；本机没有 PostgreSQL，
// 所以 DAL 查询本身（SQL 谓词是否等价于这些规则）无法在这里验证。

func fleetSavedFilterFixture(id, userID string, shared bool, updatedAt time.Time) *model.FleetSavedFilter {
	return &model.FleetSavedFilter{
		ID:           id,
		TenantID:     "tenant-1",
		UserID:       userID,
		Name:         id,
		DeviceFilter: `{"is_online":1}`,
		Shared:       shared,
		CreatedAt:    updatedAt,
		UpdatedAt:    updatedAt,
	}
}

func fleetSavedFilterClaims(userID, tenantID string) *utils.UserClaims {
	return &utils.UserClaims{ID: userID, TenantID: tenantID}
}

func TestFleetSavedFilterVisibilityRules(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name   string
		filter *model.FleetSavedFilter
		viewer string
		want   bool
	}{
		{"own private filter is visible", fleetSavedFilterFixture("a", "user-1", false, now), "user-1", true},
		{"own shared filter is visible", fleetSavedFilterFixture("b", "user-1", true, now), "user-1", true},
		{"other member private filter is hidden", fleetSavedFilterFixture("c", "user-2", false, now), "user-1", false},
		{"other member shared filter is visible", fleetSavedFilterFixture("d", "user-2", true, now), "user-1", true},
		{"nil filter is hidden", nil, "user-1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fleetSavedFilterVisibleToUser(tc.filter, tc.viewer); got != tc.want {
				t.Fatalf("fleetSavedFilterVisibleToUser = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAuthorizeFleetSavedFilterWriteOnlyAllowsOwner(t *testing.T) {
	now := time.Now().UTC()
	owner := fleetSavedFilterFixture("own", "user-1", true, now)
	if err := authorizeFleetSavedFilterWrite(owner, fleetSavedFilterClaims("user-1", "tenant-1")); err != nil {
		t.Fatalf("owner write was rejected: %v", err)
	}

	// 别人共享出来的筛选器只能读，改删必须是无权限，而不是静默成功。
	shared := fleetSavedFilterFixture("shared", "user-2", true, now)
	err := authorizeFleetSavedFilterWrite(shared, fleetSavedFilterClaims("user-1", "tenant-1"))
	appErr, ok := err.(*errcode.Error)
	if !ok || appErr.Code != errcode.CodeNoPermission {
		t.Fatalf("shared filter write error = %#v, want CodeNoPermission", err)
	}

	private := fleetSavedFilterFixture("private", "user-2", false, now)
	err = authorizeFleetSavedFilterWrite(private, fleetSavedFilterClaims("user-1", "tenant-1"))
	appErr, ok = err.(*errcode.Error)
	if !ok || appErr.Code != errcode.CodeNoPermission {
		t.Fatalf("other member private filter write error = %#v, want CodeNoPermission", err)
	}
}

func TestAuthorizeFleetSavedFilterWriteRejectsCrossTenantAndMissingClaims(t *testing.T) {
	now := time.Now().UTC()
	// 同一个 user_id 落在别的租户时也必须失败，租户 id 本身不是授权。
	crossTenant := fleetSavedFilterFixture("cross", "user-1", true, now)
	crossTenant.TenantID = "tenant-2"
	err := authorizeFleetSavedFilterWrite(crossTenant, fleetSavedFilterClaims("user-1", "tenant-1"))
	if appErr, ok := err.(*errcode.Error); !ok || appErr.Code != errcode.CodeNoPermission {
		t.Fatalf("cross tenant write error = %#v, want CodeNoPermission", err)
	}

	if err := authorizeFleetSavedFilterWrite(fleetSavedFilterFixture("x", "user-1", false, now), nil); err == nil {
		t.Fatal("expected missing claims to fail")
	}

	err = authorizeFleetSavedFilterWrite(nil, fleetSavedFilterClaims("user-1", "tenant-1"))
	if appErr, ok := err.(*errcode.Error); !ok || appErr.Code != errcode.CodeNotFound {
		t.Fatalf("nil filter write error = %#v, want CodeNotFound", err)
	}
}

func TestBuildFleetSavedFilterListMarksOwnershipAndDropsPrivateRows(t *testing.T) {
	base := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	filters := []*model.FleetSavedFilter{
		fleetSavedFilterFixture("shared-newer", "user-2", true, base.Add(2*time.Hour)),
		fleetSavedFilterFixture("own-older", "user-1", false, base),
		fleetSavedFilterFixture("own-newer", "user-1", true, base.Add(time.Hour)),
		// 同租户其他成员未共享的行即使被查出来也必须被过滤掉。
		fleetSavedFilterFixture("other-private", "user-3", false, base.Add(3*time.Hour)),
		nil,
	}

	list := buildFleetSavedFilterList(filters, "user-1")
	if len(list) != 3 {
		t.Fatalf("list length = %d, want 3: %#v", len(list), list)
	}

	wantOrder := []string{"own-newer", "own-older", "shared-newer"}
	for i, want := range wantOrder {
		if list[i].ID != want {
			t.Fatalf("list[%d].ID = %q, want %q", i, list[i].ID, want)
		}
	}
	if !list[0].Owned || !list[1].Owned {
		t.Fatalf("own rows were not marked owned: %#v", list[:2])
	}
	if list[2].Owned {
		t.Fatalf("shared row from another member was marked owned: %#v", list[2])
	}
	if !list[2].Shared || list[2].OwnerUserID != "user-2" {
		t.Fatalf("shared row lost sharing metadata: %#v", list[2])
	}
	if list[1].Shared {
		t.Fatalf("private own row reported as shared: %#v", list[1])
	}
}

func TestFleetSavedFilterQuotaCountsOnlyOwnedRows(t *testing.T) {
	if err := checkFleetSavedFilterQuota(maxFleetSavedFilters - 1); err != nil {
		t.Fatalf("quota rejected below the limit: %v", err)
	}
	err := checkFleetSavedFilterQuota(maxFleetSavedFilters)
	if appErr, ok := err.(*errcode.Error); !ok || appErr.Code != errcode.CodeParamError {
		t.Fatalf("quota error at the limit = %#v, want CodeParamError", err)
	}

	// 列表上限必须能同时容纳本人配额和别人共享进来的行，否则共享会挤掉自己的筛选器。
	if fleetSavedFilterListLimit <= maxFleetSavedFilters {
		t.Fatalf("fleetSavedFilterListLimit = %d, want more than the per-user quota %d",
			fleetSavedFilterListLimit, maxFleetSavedFilters)
	}
}

func TestFleetSavedFilterSharedFlagDefaults(t *testing.T) {
	shared := true
	notShared := false
	if got := fleetSavedFilterSharedFlag(nil, true); !got {
		t.Fatal("omitted shared flag should preserve the current value on update")
	}
	if got := fleetSavedFilterSharedFlag(nil, false); got {
		t.Fatal("omitted shared flag should default to private on create")
	}
	if got := fleetSavedFilterSharedFlag(&shared, false); !got {
		t.Fatal("explicit shared=true was dropped")
	}
	if got := fleetSavedFilterSharedFlag(&notShared, true); got {
		t.Fatal("explicit shared=false did not unshare")
	}
}

func TestFleetSavedFilterRspOwnedFlagFollowsViewer(t *testing.T) {
	filter := fleetSavedFilterFixture("f1", "user-2", true, time.Now().UTC())
	rsp := fleetSavedFilterRsp(filter, "user-2")
	if !rsp.Owned {
		t.Fatalf("owner view was not marked owned: %#v", rsp)
	}
	rsp = fleetSavedFilterRsp(filter, "user-1")
	if rsp.Owned {
		t.Fatalf("non-owner view was marked owned: %#v", rsp)
	}
	if rsp.DeviceFilter["is_online"] != float64(1) {
		t.Fatalf("device_filter was not decoded: %#v", rsp.DeviceFilter)
	}
}

// TestFleetSavedFilterListScopes 管理列表读作用域：空租户(SYS_ADMIN 平台行)映射 [""]，
// 非空租户回退 self-only（无层级链接时至少包含自身，等价旧单租户）。
func TestFleetSavedFilterListScopes(t *testing.T) {
	if got := fleetSavedFilterListScopes(""); len(got) != 1 || got[0] != "" {
		t.Fatalf("platform scope = %#v, want [\"\"]", got)
	}
	if got := fleetSavedFilterListScopes("tenant-1"); len(got) != 1 || got[0] != "tenant-1" {
		t.Fatalf("tenant fallback scope = %#v, want [tenant-1]", got)
	}
}
