// 文件用途：用户组与组权限服务层单测（TB-46 GPE v1）——权限收口、租户边界与组共享可见性判定。
// 核心逻辑：sqlite 内存库夹具驱动 service 路径：组管理仅管理员、TENANT_ADMIN 跨租户 fail-closed、
//
//	组权限元素绑定校验（未知命名空间/跨租户资源拒绝）、成员校验（users 表内同租户账号）、
//	组共享"隐藏集合 = 组绑定 − 组内可见"的调用者语义（管理员豁免）。
//
// 关键注意事项：expandTenantIDScope 走 tenants 表（夹具 AutoMigrate model.Tenant）；
//
//	错误断言用 errcode code 精确比对，避免只看非 nil 造成语义漂移。
//
// 重构建议：组共享若升级为"可见白名单"模型，本文件的隐藏集用例需整体重写。
package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	constant "aetherlink-iot/backend/pkg/constant"
	errcode "aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUserGroupServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open user group service sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.UserGroup{},
		&model.UserGroupMember{},
		&model.GroupPermission{},
		&model.Board{},
		&model.Asset{},
		&model.User{},
		&model.Tenant{},
	); err != nil {
		t.Fatalf("migrate user group service tables: %v", err)
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

func seedServiceGroup(t *testing.T, db *gorm.DB, id, name, tenantID string) *model.UserGroup {
	t.Helper()
	now := time.Now().UTC()
	g := &model.UserGroup{ID: id, Name: name, TenantID: tenantID, CreatedAt: &now, UpdatedAt: &now}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed group %s: %v", id, err)
	}
	return g
}

func seedServiceUser(t *testing.T, db *gorm.DB, id, tenantID, email, authority string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&model.User{ID: id, TenantID: &tenantID, Email: email, PhoneNumber: "10086", Password: "x", Authority: &authority, CreatedAt: &now}).Error; err != nil {
		t.Fatalf("seed user %s: %v", id, err)
	}
}

func seedServiceBoard(t *testing.T, db *gorm.DB, id, tenantID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&model.Board{ID: id, Name: "Board " + id, TenantID: tenantID, CreatedAt: now, UpdatedAt: now, HomeFlag: "N"}).Error; err != nil {
		t.Fatalf("seed board %s: %v", id, err)
	}
}

func seedServiceAsset(t *testing.T, db *gorm.DB, id, tenantID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&model.Asset{ID: id, TenantID: tenantID, Name: "Asset " + id, CreatedAt: &now, UpdatedAt: &now}).Error; err != nil {
		t.Fatalf("seed asset %s: %v", id, err)
	}
}

func assertGroupErrcode(t *testing.T, err error, want int, scenario string) {
	t.Helper()
	assertErrcodeError(t, err, scenario, want, "")
}

func TestGroupHiddenResourceIDsCallerSemantics(t *testing.T) {
	db := setupUserGroupServiceTestDB(t)
	seedServiceGroup(t, db, "g-t1", "ops", "tenant-1")
	seedServiceGroup(t, db, "g-t2", "other", "tenant-2")
	if err := db.Create(&model.GroupPermission{ID: "gp-1", GroupID: "g-t1", ElementCode: "board:board-1", TenantID: "tenant-1"}).Error; err != nil {
		t.Fatalf("seed group permission: %v", err)
	}
	// 跨租户绑定不参与 tenant-1 作用域（防脏数据放大）。
	if err := db.Create(&model.GroupPermission{ID: "gp-2", GroupID: "g-t2", ElementCode: "board:board-2", TenantID: "tenant-2"}).Error; err != nil {
		t.Fatalf("seed group permission 2: %v", err)
	}

	// 无 claims / 空作用域：无隐藏需求（列表自身查不到数据）。
	if hidden, err := groupHiddenResourceIDs([]string{"tenant-1"}, nil, model.GroupElementKindBoard); err != nil || hidden != nil {
		t.Fatalf("nil claims want nil hidden, got %v err=%v", hidden, err)
	}
	if hidden, err := groupHiddenResourceIDs(nil, &utils.UserClaims{ID: "u-1"}, model.GroupElementKindBoard); err != nil || hidden != nil {
		t.Fatalf("empty scopes want nil hidden, got %v err=%v", hidden, err)
	}

	// 管理员豁免（SYS_ADMIN / TENANT_ADMIN）。
	for _, authority := range []string{constant.SYS_ADMIN, constant.TENANT_ADMIN} {
		claims := &utils.UserClaims{ID: "admin-1", Authority: authority, TenantID: "tenant-1"}
		if hidden, err := groupHiddenResourceIDs([]string{"tenant-1"}, claims, model.GroupElementKindBoard); err != nil || hidden != nil {
			t.Fatalf("%s want nil hidden, got %v err=%v", authority, hidden, err)
		}
	}

	// 普通租户用户非组成员：组绑定的 board-1 被隐藏。
	nonMember := &utils.UserClaims{ID: "u-out", Authority: constant.TENANT_USER, TenantID: "tenant-1"}
	hidden, err := groupHiddenResourceIDs([]string{"tenant-1"}, nonMember, model.GroupElementKindBoard)
	if err != nil {
		t.Fatalf("hidden ids err: %v", err)
	}
	if len(hidden) != 1 || hidden[0] != "board-1" {
		t.Fatalf("non-member hidden = %v, want [board-1]", hidden)
	}

	// 组内成员：不再隐藏。
	if err := db.Create(&model.UserGroupMember{GroupID: "g-t1", UserID: "u-in", TenantID: "tenant-1"}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	member := &utils.UserClaims{ID: "u-in", Authority: constant.TENANT_USER, TenantID: "tenant-1"}
	hidden, err = groupHiddenResourceIDs([]string{"tenant-1"}, member, model.GroupElementKindBoard)
	if err != nil || len(hidden) != 0 {
		t.Fatalf("member hidden = %v err=%v, want empty", hidden, err)
	}

	// 租户边界：tenant-2 用户在 tenant-2 作用域内只受 tenant-2 绑定约束——
	// g-t2 绑定的 board-2 对其（非成员）隐藏，tenant-1 的 board-1 不越界出现。
	outUser := &utils.UserClaims{ID: "u-t2", Authority: constant.TENANT_USER, TenantID: "tenant-2"}
	hidden, err = groupHiddenResourceIDs([]string{"tenant-2"}, outUser, model.GroupElementKindBoard)
	if err != nil || len(hidden) != 1 || hidden[0] != "board-2" {
		t.Fatalf("tenant-2 user hidden = %v err=%v, want [board-2]", hidden, err)
	}
}

func TestEnsureUserGroupWriteAccessIsolatesTenant(t *testing.T) {
	db := setupUserGroupServiceTestDB(t)
	seedServiceGroup(t, db, "g-t1", "ops", "tenant-1")

	// TENANT_ADMIN 本租户可读。
	owner := &utils.UserClaims{ID: "admin-1", Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}
	group, err := (&UserGroup{}).GetUserGroup("g-t1", owner)
	if err != nil || group == nil || group.ID != "g-t1" {
		t.Fatalf("owner read group = %v err=%v", group, err)
	}

	// TENANT_ADMIN 跨租户 fail-closed（404，不泄露存在性）。
	outsider := &utils.UserClaims{ID: "admin-2", Authority: constant.TENANT_ADMIN, TenantID: "tenant-2"}
	_, err = (&UserGroup{}).GetUserGroup("g-t1", outsider)
	assertGroupErrcode(t, err, errcode.CodeNotFound, "cross-tenant admin read")

	// TENANT_USER 无组管理能力。
	user := &utils.UserClaims{ID: "u-1", Authority: constant.TENANT_USER, TenantID: "tenant-1"}
	_, err = (&UserGroup{}).GetUserGroup("g-t1", user)
	assertGroupErrcode(t, err, errcode.CodeNoPermission, "tenant user read group")

	// SYS_ADMIN 全租户可读。
	sysAdmin := &utils.UserClaims{ID: "sys-1", Authority: constant.SYS_ADMIN, TenantID: ""}
	group, err = (&UserGroup{}).GetUserGroup("g-t1", sysAdmin)
	if err != nil || group == nil {
		t.Fatalf("sys admin read group = %v err=%v", group, err)
	}
}

func TestCreateUserGroupTenantBindingAndUniqueName(t *testing.T) {
	db := setupUserGroupServiceTestDB(t)
	if err := db.Create(&model.Tenant{ID: "tenant-1", Name: "T1", ParentTenantID: ""}).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	svc := &UserGroup{}

	tenantAdmin := &utils.UserClaims{ID: "admin-1", Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}
	created, err := svc.CreateUserGroup(&model.CreateUserGroupReq{Name: "ops"}, tenantAdmin)
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if created.TenantID != "tenant-1" {
		t.Fatalf("created group tenant = %s, want tenant-1", created.TenantID)
	}

	// 同租户重名 fail-closed。
	_, err = svc.CreateUserGroup(&model.CreateUserGroupReq{Name: "ops"}, tenantAdmin)
	assertGroupErrcode(t, err, errcode.CodeParamError, "duplicate name")

	// TENANT_ADMIN 指定其他租户被拒。
	_, err = svc.CreateUserGroup(&model.CreateUserGroupReq{Name: "ops2", TenantID: "tenant-2"}, tenantAdmin)
	assertGroupErrcode(t, err, errcode.CodeNoPermission, "tenant admin with other tenant")

	// SYS_ADMIN 不带 tenant_id fail-closed；带合法 tenant_id 成功。
	sysAdmin := &utils.UserClaims{ID: "sys-1", Authority: constant.SYS_ADMIN, TenantID: ""}
	_, err = svc.CreateUserGroup(&model.CreateUserGroupReq{Name: "platform"}, sysAdmin)
	assertGroupErrcode(t, err, errcode.CodeParamError, "sys admin without tenant")
	created, err = svc.CreateUserGroup(&model.CreateUserGroupReq{Name: "platform", TenantID: "tenant-1"}, sysAdmin)
	if err != nil || created == nil || created.TenantID != "tenant-1" {
		t.Fatalf("sys admin create with tenant = %v err=%v", created, err)
	}
}

func TestAssignUserGroupPermissionsValidatesElements(t *testing.T) {
	db := setupUserGroupServiceTestDB(t)
	seedServiceGroup(t, db, "g-t1", "ops", "tenant-1")
	seedServiceBoard(t, db, "board-1", "tenant-1")
	seedServiceAsset(t, db, "asset-1", "tenant-1")
	svc := &UserGroup{}
	admin := &utils.UserClaims{ID: "admin-1", Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}

	// 未知命名空间 fail-closed。
	err := svc.AssignUserGroupPermissions("g-t1", &model.AssignUserGroupPermissionsReq{ElementCodes: []string{"menu:xyz"}}, admin)
	assertGroupErrcode(t, err, errcode.CodeParamError, "unknown namespace")

	// 形态不完整 fail-closed。
	err = svc.AssignUserGroupPermissions("g-t1", &model.AssignUserGroupPermissionsReq{ElementCodes: []string{"board:"}}, admin)
	assertGroupErrcode(t, err, errcode.CodeParamError, "empty resource id")

	// 资源不存在/跨租户 fail-closed。
	err = svc.AssignUserGroupPermissions("g-t1", &model.AssignUserGroupPermissionsReq{ElementCodes: []string{"board:board-missing"}}, admin)
	assertGroupErrcode(t, err, errcode.CodeParamError, "missing board")
	err = svc.AssignUserGroupPermissions("g-t1", &model.AssignUserGroupPermissionsReq{ElementCodes: []string{"asset:asset-other-tenant"}}, admin)
	assertGroupErrcode(t, err, errcode.CodeParamError, "missing asset")

	// 合法绑定成功并可回读。
	if err := svc.AssignUserGroupPermissions("g-t1", &model.AssignUserGroupPermissionsReq{ElementCodes: []string{"board:board-1", "asset:asset-1"}}, admin); err != nil {
		t.Fatalf("valid assign: %v", err)
	}
	resp, err := svc.GetUserGroupPermissions("g-t1", admin)
	if err != nil {
		t.Fatalf("get permissions: %v", err)
	}
	if len(resp.Elements) != 2 || resp.Elements[0].Kind != "board" || resp.Elements[0].Name != "Board board-1" {
		t.Fatalf("permissions resp = %+v", resp)
	}
}

func TestAssignUserGroupMembersValidatesTenantUsers(t *testing.T) {
	db := setupUserGroupServiceTestDB(t)
	seedServiceGroup(t, db, "g-t1", "ops", "tenant-1")
	seedServiceUser(t, db, "u-1", "tenant-1", "u1@example.com", constant.TENANT_USER)
	seedServiceUser(t, db, "u-2", "tenant-2", "u2@example.com", constant.TENANT_USER)
	svc := &UserGroup{}
	admin := &utils.UserClaims{ID: "admin-1", Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}

	// 跨租户用户 fail-closed。
	err := svc.AssignUserGroupMembers("g-t1", &model.AssignUserGroupMembersReq{UserIDs: []string{"u-1", "u-2"}}, admin)
	assertGroupErrcode(t, err, errcode.CodeParamError, "cross-tenant member")

	// 不存在的用户 fail-closed。
	err = svc.AssignUserGroupMembers("g-t1", &model.AssignUserGroupMembersReq{UserIDs: []string{"u-missing"}}, admin)
	assertGroupErrcode(t, err, errcode.CodeParamError, "missing member")

	// 合法绑定成功并可回读。
	if err := svc.AssignUserGroupMembers("g-t1", &model.AssignUserGroupMembersReq{UserIDs: []string{"u-1"}}, admin); err != nil {
		t.Fatalf("valid assign members: %v", err)
	}
	resp, err := svc.GetUserGroupMembers("g-t1", admin)
	if err != nil {
		t.Fatalf("get members: %v", err)
	}
	if len(resp.Users) != 1 || resp.Users[0].ID != "u-1" || resp.Users[0].Email != "u1@example.com" {
		t.Fatalf("members resp = %+v", resp)
	}
}

func TestParseGroupElementCode(t *testing.T) {
	cases := []struct {
		code string
		kind string
		id   string
		ok   bool
	}{
		{"board:abc", "board", "abc", true},
		{"asset:xyz", "asset", "xyz", true},
		{"menu:xyz", "", "", false},
		{"board:", "", "", false},
		{"board", "", "", false},
		{"", "", "", false},
		{"  board:abc ", "board", "abc", true},
	}
	for _, c := range cases {
		kind, id, ok := model.ParseGroupElementCode(c.code)
		if ok != c.ok || (ok && (kind != c.kind || id != c.id)) {
			t.Fatalf("ParseGroupElementCode(%q) = (%q,%q,%v), want (%q,%q,%v)", c.code, kind, id, ok, c.kind, c.id, c.ok)
		}
	}
}
