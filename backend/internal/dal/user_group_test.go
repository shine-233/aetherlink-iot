// 文件用途：用户组与组权限 DAL 层的单测（TB-46 GPE v1）——sqlite 内存库驱动持久化与可见性映射行为。
// 核心逻辑：覆盖组成员/权限绑定的事务替换、组删除级联清理、组共享"绑定集合/组内可见集合"
//
//	两个可见性映射查询的 fail-closed 与租户隔离语义，以及看板/资产列表的组共享排除过滤。
//
// 关键注意事项：sqlite 默认不启用外键，组删除清理由 DAL 显式事务保证（不依赖 FK CASCADE）；
//
//	夹具需 query.SetDefault(db)（boardListByScopes 走 gorm gen 构建器）。
//
// 重构建议：组规模语义（分页/排序）变化时补边界用例；映射查询改 SQL 端过滤后重写本文件。
package dal

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUserGroupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open user group sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.UserGroup{},
		&model.UserGroupMember{},
		&model.GroupPermission{},
		&model.Board{},
		&model.Asset{},
		&model.User{},
	); err != nil {
		t.Fatalf("migrate user group tables: %v", err)
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

func seedUserGroup(t *testing.T, db *gorm.DB, id, name, tenantID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&model.UserGroup{ID: id, Name: name, TenantID: tenantID, CreatedAt: &now, UpdatedAt: &now}).Error; err != nil {
		t.Fatalf("seed user group %s: %v", id, err)
	}
}

func seedGroupMember(t *testing.T, db *gorm.DB, groupID, userID, tenantID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&model.UserGroupMember{GroupID: groupID, UserID: userID, TenantID: tenantID, CreatedAt: &now}).Error; err != nil {
		t.Fatalf("seed member %s->%s: %v", userID, groupID, err)
	}
}

func seedGroupPermission(t *testing.T, db *gorm.DB, id, groupID, code, tenantID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&model.GroupPermission{ID: id, GroupID: groupID, ElementCode: code, TenantID: tenantID, CreatedAt: &now}).Error; err != nil {
		t.Fatalf("seed permission %s: %v", id, err)
	}
}

func sortedIDs(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

func TestGetGroupSharedResourceIDsFailsClosedAndIsolatesTenant(t *testing.T) {
	db := setupUserGroupTestDB(t)
	seedUserGroup(t, db, "g-t1", "ops", "tenant-1")
	seedGroupMember(t, db, "g-t1", "u-1", "tenant-1")
	seedGroupPermission(t, db, "gp-1", "g-t1", "board:board-1", "tenant-1")

	// fail-closed：空 scopes / 空 user 一律空集。
	got, err := GetGroupSharedResourceIDs(nil, "u-1", "board")
	if err != nil || len(got) != 0 {
		t.Fatalf("empty scopes want empty set, got %v err=%v", got, err)
	}
	got, err = GetGroupSharedResourceIDs([]string{"tenant-1"}, "", "board")
	if err != nil || len(got) != 0 {
		t.Fatalf("empty user want empty set, got %v err=%v", got, err)
	}

	// 正常路径：组成员经组权限看到绑定资源。
	got, err = GetGroupSharedResourceIDs([]string{"tenant-1"}, "u-1", "board")
	if err != nil {
		t.Fatalf("shared ids err: %v", err)
	}
	if want := []string{"board-1"}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("shared ids = %v, want %v", got, want)
	}

	// 租户隔离：tenant-2 的成员不在 tenant-1 作用域内（成员行 tenant_id 过滤）。
	got, err = GetGroupSharedResourceIDs([]string{"tenant-1"}, "u-2", "board")
	if err != nil || len(got) != 0 {
		t.Fatalf("other-tenant user want empty set, got %v err=%v", got, err)
	}
}

func TestGetGroupBoundResourceIDsScopesAndKinds(t *testing.T) {
	db := setupUserGroupTestDB(t)
	seedUserGroup(t, db, "g-t1", "ops", "tenant-1")
	seedGroupPermission(t, db, "gp-1", "g-t1", "board:board-1", "tenant-1")
	seedGroupPermission(t, db, "gp-2", "g-t1", "asset:asset-1", "tenant-1")
	seedUserGroup(t, db, "g-t2", "other", "tenant-2")
	seedGroupPermission(t, db, "gp-3", "g-t2", "board:board-9", "tenant-2")

	got, err := GetGroupBoundResourceIDs([]string{"tenant-1"}, "board")
	if err != nil {
		t.Fatalf("bound board ids err: %v", err)
	}
	if want := []string{"board-1"}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("bound board ids = %v, want %v", got, want)
	}

	got, err = GetGroupBoundResourceIDs([]string{"tenant-1", "tenant-2"}, "board")
	if err != nil {
		t.Fatalf("bound board ids multi-scope err: %v", err)
	}
	if want := []string{"board-1", "board-9"}; len(got) != 2 {
		t.Fatalf("bound board ids multi-scope = %v, want %v", got, want)
	}

	// kind 隔离：asset 命名空间不混入 board 绑定。
	got, err = GetGroupBoundResourceIDs([]string{"tenant-1"}, "asset")
	if err != nil || len(got) != 1 || got[0] != "asset-1" {
		t.Fatalf("bound asset ids = %v err=%v, want [asset-1]", got, err)
	}

	// fail-closed：空 scopes 返回空集。
	got, err = GetGroupBoundResourceIDs(nil, "board")
	if err != nil || len(got) != 0 {
		t.Fatalf("empty scopes want empty set, got %v err=%v", got, err)
	}
}

func TestBoardListWithGroupScopeHidesNonMemberBoundBoards(t *testing.T) {
	db := setupUserGroupTestDB(t)
	now := time.Now().UTC()
	for _, b := range []struct{ id, tenant string }{
		{"board-1", "tenant-1"},
		{"board-2", "tenant-1"},
		{"board-9", "tenant-2"},
	} {
		if err := db.Create(&model.Board{ID: b.id, Name: "Board " + b.id, TenantID: b.tenant, CreatedAt: now, UpdatedAt: now, HomeFlag: "N"}).Error; err != nil {
			t.Fatalf("seed board %s: %v", b.id, err)
		}
	}

	// 无隐藏集：组内两个看板可见，跨租户不可见（既有行为不回归）。
	count, list, err := GetBoardListByPageForScopesWithGroupScope(
		&model.GetBoardListByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}},
		[]string{"tenant-1"}, nil)
	if err != nil {
		t.Fatalf("board list err: %v", err)
	}
	if count != 2 {
		t.Fatalf("board count without hidden = %d, want 2", count)
	}
	_ = list

	// 隐藏集生效：board-1 被排除且 total 同步减少。
	count, list, err = GetBoardListByPageForScopesWithGroupScope(
		&model.GetBoardListByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}},
		[]string{"tenant-1"}, []string{"board-1"})
	if err != nil {
		t.Fatalf("board list with hidden err: %v", err)
	}
	rows, ok := list.([]*model.Board)
	if !ok {
		t.Fatalf("board list type = %T", list)
	}
	if count != 1 || len(rows) != 1 || rows[0].ID != "board-2" {
		t.Fatalf("board list with hidden: count=%d rows=%v, want only board-2", count, rows)
	}
}

func TestAssetListWithGroupScopeHidesNonMemberBoundAssets(t *testing.T) {
	db := setupUserGroupTestDB(t)
	now := time.Now().UTC()
	for _, a := range []struct{ id, tenant string }{
		{"asset-1", "tenant-1"},
		{"asset-2", "tenant-1"},
	} {
		if err := db.Create(&model.Asset{ID: a.id, TenantID: a.tenant, Name: "Asset " + a.id, ParentID: "", CreatedAt: &now, UpdatedAt: &now}).Error; err != nil {
			t.Fatalf("seed asset %s: %v", a.id, err)
		}
	}

	rows, total, err := ListAssetsByPageWithGroupScope([]string{"tenant-1"}, "", "", 1, 10, []string{"asset-1"})
	if err != nil {
		t.Fatalf("asset list with hidden err: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ID != "asset-2" {
		t.Fatalf("asset list with hidden: total=%d rows=%v, want only asset-2", total, rows)
	}

	// 无隐藏集（nil）时行为不回归。
	rows, total, err = ListAssetsByPageWithGroupScope([]string{"tenant-1"}, "", "", 1, 10, nil)
	if err != nil || total != 2 {
		t.Fatalf("asset list without hidden: total=%d err=%v, want 2", total, err)
	}
}

func TestReplaceUserGroupMembersReplacesAtomicallyAndLists(t *testing.T) {
	db := setupUserGroupTestDB(t)
	seedUserGroup(t, db, "g-t1", "ops", "tenant-1")
	now := time.Now().UTC()
	for _, u := range []struct{ id, tenant, email string }{
		{"u-1", "tenant-1", "u1@example.com"},
		{"u-2", "tenant-1", "u2@example.com"},
	} {
		if err := db.Create(&model.User{ID: u.id, TenantID: &u.tenant, Email: u.email, PhoneNumber: "10086", Password: "x", CreatedAt: &now}).Error; err != nil {
			t.Fatalf("seed user %s: %v", u.id, err)
		}
	}

	if err := ReplaceUserGroupMembers(t.Context(), "g-t1", "tenant-1", []string{"u-1", "u-2"}); err != nil {
		t.Fatalf("replace members: %v", err)
	}
	members, err := ListUserGroupMembers("g-t1", "tenant-1")
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("members = %v, want 2 rows", members)
	}

	// 全量替换语义：第二次只留 u-2。
	if err := ReplaceUserGroupMembers(t.Context(), "g-t1", "tenant-1", []string{"u-2"}); err != nil {
		t.Fatalf("replace members second: %v", err)
	}
	members, err = ListUserGroupMembers("g-t1", "tenant-1")
	if err != nil {
		t.Fatalf("list members second: %v", err)
	}
	if len(members) != 1 || members[0].ID != "u-2" {
		t.Fatalf("members after replace = %v, want only u-2", members)
	}

	// 空列表 = 清空。
	if err := ReplaceUserGroupMembers(t.Context(), "g-t1", "tenant-1", nil); err != nil {
		t.Fatalf("clear members: %v", err)
	}
	members, err = ListUserGroupMembers("g-t1", "tenant-1")
	if err != nil || len(members) != 0 {
		t.Fatalf("members after clear = %v err=%v, want empty", members, err)
	}
}

func TestReplaceGroupPermissionsReplacesAtomically(t *testing.T) {
	db := setupUserGroupTestDB(t)
	seedUserGroup(t, db, "g-t1", "ops", "tenant-1")

	if err := ReplaceGroupPermissions(t.Context(), "g-t1", "tenant-1", []string{"board:board-1", "asset:asset-1"}); err != nil {
		t.Fatalf("replace permissions: %v", err)
	}
	rows, err := ListGroupPermissions("g-t1", "tenant-1")
	if err != nil {
		t.Fatalf("list permissions: %v", err)
	}
	got := make([]string, 0, len(rows))
	for _, r := range rows {
		got = append(got, r.ElementCode)
	}
	if want := sortedIDs([]string{"board:board-1", "asset:asset-1"}); len(got) != 2 || sortedIDs(got)[0] != want[0] || sortedIDs(got)[1] != want[1] {
		t.Fatalf("permissions = %v, want %v", got, want)
	}

	// 去重 + 全量替换。
	if err := ReplaceGroupPermissions(t.Context(), "g-t1", "tenant-1", []string{"board:board-2", "board:board-2"}); err != nil {
		t.Fatalf("replace permissions second: %v", err)
	}
	rows, err = ListGroupPermissions("g-t1", "tenant-1")
	if err != nil || len(rows) != 1 || rows[0].ElementCode != "board:board-2" {
		t.Fatalf("permissions after replace = %v err=%v, want [board:board-2]", rows, err)
	}
}

func TestDeleteUserGroupForTenantCleansChildren(t *testing.T) {
	db := setupUserGroupTestDB(t)
	seedUserGroup(t, db, "g-t1", "ops", "tenant-1")
	seedGroupMember(t, db, "g-t1", "u-1", "tenant-1")
	seedGroupPermission(t, db, "gp-1", "g-t1", "board:board-1", "tenant-1")

	// 跨租户删除 fail-closed：不命中且不清理。
	if err := DeleteUserGroupForTenant("g-t1", "tenant-2"); err == nil {
		t.Fatalf("cross-tenant delete want error, got nil")
	}
	var count int64
	if err := db.Model(&model.UserGroup{}).Where("id = ?", "g-t1").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("group survived cross-tenant delete: count=%d err=%v", count, err)
	}

	if err := DeleteUserGroupForTenant("g-t1", "tenant-1"); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	for _, table := range []string{"user_groups", "r_group_user", "group_permissions"} {
		var n int64
		if err := db.Table(table).Where("1 = 1").Count(&n).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("table %s not cleaned after group delete: %d rows", table, n)
		}
	}
}

func TestGetUserGroupNameExistsTenantScoped(t *testing.T) {
	db := setupUserGroupTestDB(t)
	seedUserGroup(t, db, "g-t1", "ops", "tenant-1")
	seedUserGroup(t, db, "g-t2", "ops", "tenant-2")

	exists, err := GetUserGroupNameExists("ops", "tenant-1", "")
	if err != nil || !exists {
		t.Fatalf("same-tenant duplicate want exists, got %v err=%v", exists, err)
	}
	exists, err = GetUserGroupNameExists("ops", "tenant-1", "g-t1")
	if err != nil || exists {
		t.Fatalf("exclude-self want not exists, got %v err=%v", exists, err)
	}
	exists, err = GetUserGroupNameExists("ops", "tenant-2", "g-t2")
	if err != nil || exists {
		t.Fatalf("exclude-self tenant-2 want not exists, got %v err=%v", exists, err)
	}
	exists, err = GetUserGroupNameExists("other", "tenant-1", "")
	if err != nil || exists {
		t.Fatalf("fresh name want not exists, got %v err=%v", exists, err)
	}
}
