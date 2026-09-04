// 文件用途：验证系统用户授权、角色绑定和跨租户拒绝逻辑。
// 核心逻辑：构造用户、角色和 Casbin 绑定，断言创建、更新、删除前后的权限与回滚行为。
// 关键注意事项：用户授权测试是越权防线，必须证明权限失败先于资料写入和角色绑定副作用。
// 重构建议：抽出角色绑定仓储和事务夹具，补齐 Casbin 不可用、恢复失败和多角色边界。
package service

import (
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestSysUserTransformRejectsTenantAdminCrossTenantTarget(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "tenant-b-user", "tenant-b", constant.TENANT_USER)

	_, err := (&User{}).TransformUser(&model.TransformUserReq{BecomeUserID: "tenant-b-user"}, &utils.UserClaims{
		ID:        "tenant-a-admin",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-a",
	})

	assertErrcodeError(t, err, "tenant admin transform cross-tenant user", errcode.CodeNoPermission, "no permission to transform cross-tenant user")
}

func TestSysUserTransformRejectsTenantAdminTargetingAnotherAdmin(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "tenant-a-admin-2", "tenant-a", constant.TENANT_ADMIN)

	_, err := (&User{}).TransformUser(&model.TransformUserReq{BecomeUserID: "tenant-a-admin-2"}, &utils.UserClaims{
		ID:        "tenant-a-admin",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-a",
	})

	assertErrcodeError(t, err, "tenant admin transform another tenant admin", errcode.CodeNoPermission, "tenant admin can only transform tenant users")
}

func TestSysUserUpdateRejectsTenantUserBeforeWrite(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "tenant-a-user", "tenant-a", constant.TENANT_USER)
	newName := "new name"

	err := (&User{}).UpdateUser(&model.UpdateUserReq{
		ID:   "tenant-a-user",
		Name: &newName,
	}, &utils.UserClaims{
		ID:        "tenant-a-user",
		Authority: constant.TENANT_USER,
		TenantID:  "tenant-a",
	})

	assertErrcodeError(t, err, "tenant user update user", errcode.CodeNoPermission, "no permission to manage users")
	assertSysUserAuthorizationName(t, db, "tenant-a-user", "tenant-a-user")
}

func TestSysUserUpdateRejectsTenantAdminTargetBeforeWrite(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "tenant-a-admin-2", "tenant-a", constant.TENANT_ADMIN)
	newName := "new name"

	err := (&User{}).UpdateUser(&model.UpdateUserReq{
		ID:   "tenant-a-admin-2",
		Name: &newName,
	}, &utils.UserClaims{
		ID:        "tenant-a-admin",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-a",
	})

	assertErrcodeError(t, err, "tenant admin update another tenant admin", errcode.CodeNoPermission, "tenant admin can only manage tenant users")
	assertSysUserAuthorizationName(t, db, "tenant-a-admin-2", "tenant-a-admin-2")
}

func TestSysUserDeleteRejectsTenantAdminTargetBeforeDelete(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "tenant-a-admin-2", "tenant-a", constant.TENANT_ADMIN)

	err := (&User{}).DeleteUser("tenant-a-admin-2", &utils.UserClaims{
		ID:        "tenant-a-admin",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-a",
	})

	assertErrcodeError(t, err, "tenant admin delete another tenant admin", errcode.CodeNoPermission, "tenant admin can only manage tenant users")
	assertSysUserAuthorizationRowCount(t, db, "tenant-a-admin-2", 1)
}

func TestSysUserDeleteRejectsTenantUserBeforeDelete(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "tenant-a-user-2", "tenant-a", constant.TENANT_USER)

	err := (&User{}).DeleteUser("tenant-a-user-2", &utils.UserClaims{
		ID:        "tenant-a-user",
		Authority: constant.TENANT_USER,
		TenantID:  "tenant-a",
	})

	assertErrcodeError(t, err, "tenant user delete user", errcode.CodeNoPermission, "no permission to manage users")
	assertSysUserAuthorizationRowCount(t, db, "tenant-a-user-2", 1)
}

func TestSysUserDeleteRemovesCasbinRoleBindings(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	setupTestCasbinEnforcer()
	seedSysUserAuthorizationUser(t, db, "tenant-a-user-2", "tenant-a", constant.TENANT_USER)
	if ok := GroupApp.Casbin.AddRolesToUser("tenant-a-user-2", []string{"tenant-a-role"}); !ok {
		t.Fatal("seed user role binding failed")
	}

	err := (&User{}).DeleteUser("tenant-a-user-2", &utils.UserClaims{
		ID:        "tenant-a-admin",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-a",
	})

	if err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}
	assertSysUserAuthorizationRowCount(t, db, "tenant-a-user-2", 0)
	roles, _ := GroupApp.Casbin.GetRoleFromUser("tenant-a-user-2")
	if len(roles) != 0 {
		t.Fatalf("deleted user role bindings = %#v, want empty", roles)
	}
}

func TestSysUserCreateRejectsCrossTenantRoleBeforeUserCreate(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "tenant-a-admin", "tenant-a", constant.TENANT_ADMIN)
	seedSysUserAuthorizationRole(t, db, "tenant-b-role", "tenant-b")

	err := (&User{}).CreateUser(&model.CreateUserReq{
		Email:       "new-user@example.com",
		Password:    "Abc123!@",
		PhoneNumber: "new-user-phone",
		RoleIDs:     []string{"tenant-b-role"},
	}, &utils.UserClaims{
		ID:        "tenant-a-admin",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-a",
	})

	assertErrcodeError(t, err, "tenant admin create user with cross-tenant role", errcode.CodeNoPermission, "")
	assertSysUserAuthorizationRowCount(t, db, "new-user@example.com", 0)
}

func TestSysUserCreateRejectsMissingClaimsBeforeLookup(t *testing.T) {
	err := (&User{}).CreateUser(&model.CreateUserReq{
		Email:       "new-user@example.com",
		Password:    "Abc123!@",
		PhoneNumber: "new-user-phone",
	}, nil)

	assertErrcodeError(t, err, "create user missing claims", errcode.CodeNoPermission, "no permission to create user")
}

func TestSysUserCreateRejectsUnavailableCasbinBeforeUserCreate(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "tenant-a-admin", "tenant-a", constant.TENANT_ADMIN)
	seedSysUserAuthorizationRole(t, db, "tenant-a-role", "tenant-a")
	oldEnforcer := global.CasbinEnforcer
	global.CasbinEnforcer = nil
	t.Cleanup(func() {
		global.CasbinEnforcer = oldEnforcer
	})

	err := (&User{}).CreateUser(&model.CreateUserReq{
		Email:       "new-user@example.com",
		Password:    "Abc123!@",
		PhoneNumber: "new-user-phone",
		RoleIDs:     []string{"tenant-a-role"},
	}, &utils.UserClaims{
		ID:        "tenant-a-admin",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-a",
	})

	assertErrcodeError(t, err, "create user with unavailable casbin", errcode.CodeSystemError, "")
	assertSysUserAuthorizationRowCount(t, db, "new-user@example.com", 0)
}

func TestSysUserCreateRollsBackUserAddressBoardWhenRoleBindingFails(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	if err := db.AutoMigrate(&model.UserAddress{}, &model.Board{}); err != nil {
		t.Fatalf("migrate user address and board: %v", err)
	}
	setupTestCasbinEnforcer()
	country := "NZ"
	now := time.Now().UTC()
	user := &model.User{
		ID:                  "new-user",
		Name:                StringPtr("new user"),
		PhoneNumber:         "new-user-phone",
		Email:               "new-user@example.com",
		Status:              StringPtr("N"),
		Authority:           StringPtr(constant.TENANT_ADMIN),
		Password:            "hashed",
		TenantID:            StringPtr("tenant-a"),
		CreatedAt:           &now,
		UpdatedAt:           &now,
		PasswordLastUpdated: &now,
	}

	err := createUserWithAddressDefaultBoardAndRoles(user, &model.CreateUserReq{
		RoleIDs: []string{"tenant-a-role"},
		Address: &model.CreateUserAddressReq{Country: &country},
	}, &utils.UserClaims{
		ID:        "sys-admin",
		Authority: constant.SYS_ADMIN,
		TenantID:  "",
	})

	if err == nil {
		t.Fatal("CreateUser should fail when casbin_rule table is unavailable")
	}
	assertSysUserAuthorizationRowCount(t, db, "new-user@example.com", 0)
	assertSysUserAuthorizationAddressCount(t, db, "", 0)
	assertSysUserAuthorizationBoardCount(t, db, "", 0)
}

func TestSysUserCreatePersistsUserAddressBoardAndCasbinRuleAtomically(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	if err := db.AutoMigrate(&model.UserAddress{}, &model.Board{}, &model.CasbinRule{}); err != nil {
		t.Fatalf("migrate user transaction tables: %v", err)
	}
	setupTestCasbinEnforcer()
	country := "NZ"
	now := time.Now().UTC()
	user := &model.User{
		ID:                  "new-user",
		Name:                StringPtr("new user"),
		PhoneNumber:         "new-user-phone",
		Email:               "new-user@example.com",
		Status:              StringPtr("N"),
		Authority:           StringPtr(constant.TENANT_ADMIN),
		Password:            "hashed",
		TenantID:            StringPtr("tenant-a"),
		CreatedAt:           &now,
		UpdatedAt:           &now,
		PasswordLastUpdated: &now,
	}

	err := createUserWithAddressDefaultBoardAndRoles(user, &model.CreateUserReq{
		RoleIDs: []string{"tenant-a-role"},
		Address: &model.CreateUserAddressReq{Country: &country},
	}, &utils.UserClaims{
		ID:        "sys-admin",
		Authority: constant.SYS_ADMIN,
		TenantID:  "",
	})

	if err != nil {
		t.Fatalf("create transaction returned error: %v", err)
	}
	var stored model.User
	if err := db.First(&stored, "email = ?", "new-user@example.com").Error; err != nil {
		t.Fatalf("read created user: %v", err)
	}
	assertSysUserAuthorizationAddressCount(t, db, stored.ID, 1)
	assertSysUserAuthorizationBoardCount(t, db, SafeDeref(stored.TenantID), 1)
	assertSysUserAuthorizationCasbinRoles(t, db, stored.ID, []string{"tenant-a-role"})
}

func TestSysUserUpdateRejectsCrossTenantRoleBeforeProfileWrite(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "tenant-a-user", "tenant-a", constant.TENANT_USER)
	seedSysUserAuthorizationRole(t, db, "tenant-b-role", "tenant-b")
	newName := "new name"

	err := (&User{}).UpdateUser(&model.UpdateUserReq{
		ID:      "tenant-a-user",
		Name:    &newName,
		RoleIDs: []string{"tenant-b-role"},
	}, &utils.UserClaims{
		ID:        "tenant-a-admin",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-a",
	})

	assertErrcodeError(t, err, "tenant admin update user with cross-tenant role", errcode.CodeNoPermission, "")
	assertSysUserAuthorizationName(t, db, "tenant-a-user", "tenant-a-user")
}

func TestSysUserUpdateRejectsUnavailableCasbinBeforeProfileWrite(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "tenant-a-user", "tenant-a", constant.TENANT_USER)
	seedSysUserAuthorizationRole(t, db, "tenant-a-role", "tenant-a")
	oldEnforcer := global.CasbinEnforcer
	global.CasbinEnforcer = nil
	t.Cleanup(func() {
		global.CasbinEnforcer = oldEnforcer
	})
	newName := "new name"

	err := (&User{}).UpdateUser(&model.UpdateUserReq{
		ID:      "tenant-a-user",
		Name:    &newName,
		RoleIDs: []string{"tenant-a-role"},
	}, &utils.UserClaims{
		ID:        "tenant-a-admin",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-a",
	})

	assertErrcodeError(t, err, "update user with unavailable casbin", errcode.CodeSystemError, "")
	assertSysUserAuthorizationName(t, db, "tenant-a-user", "tenant-a-user")
}

func TestSysUserUpdateReplacesRoleBindings(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	if err := db.AutoMigrate(&model.CasbinRule{}); err != nil {
		t.Fatalf("migrate casbin_rule: %v", err)
	}
	setupTestCasbinEnforcer()
	seedSysUserAuthorizationUser(t, db, "tenant-a-user", "tenant-a", constant.TENANT_USER)
	seedSysUserAuthorizationRole(t, db, "tenant-a-role-old", "tenant-a")
	seedSysUserAuthorizationRole(t, db, "tenant-a-role-new", "tenant-a")
	seedSysUserAuthorizationCasbinRole(t, db, "tenant-a-user", "tenant-a-role-old")
	newName := "new name"

	err := (&User{}).UpdateUser(&model.UpdateUserReq{
		ID:      "tenant-a-user",
		Name:    &newName,
		RoleIDs: []string{"tenant-a-role-new"},
	}, &utils.UserClaims{
		ID:        "tenant-a-admin",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-a",
	})

	if err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	assertSysUserAuthorizationName(t, db, "tenant-a-user", "new name")
	assertSysUserAuthorizationCasbinRoles(t, db, "tenant-a-user", []string{"tenant-a-role-new"})
}

func TestSysUserUpdateRestoresRoleBindingsWhenProfileWriteFails(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	if err := db.AutoMigrate(&model.CasbinRule{}); err != nil {
		t.Fatalf("migrate casbin_rule: %v", err)
	}
	setupTestCasbinEnforcer()
	seedSysUserAuthorizationUser(t, db, "tenant-a-user", "tenant-a", constant.TENANT_USER)
	seedSysUserAuthorizationRole(t, db, "tenant-a-role-old", "tenant-a")
	seedSysUserAuthorizationRole(t, db, "tenant-a-role-new", "tenant-a")
	seedSysUserAuthorizationCasbinRole(t, db, "tenant-a-user", "tenant-a-role-old")
	newName := "new name"
	country := "NZ"

	err := (&User{}).UpdateUser(&model.UpdateUserReq{
		ID:      "tenant-a-user",
		Name:    &newName,
		RoleIDs: []string{"tenant-a-role-new"},
		Address: &model.UpdateUserAddressReq{
			Country: &country,
		},
	}, &utils.UserClaims{
		ID:        "tenant-a-admin",
		Authority: constant.TENANT_ADMIN,
		TenantID:  "tenant-a",
	})

	if err == nil {
		t.Fatal("UpdateUser should fail when user_address table is unavailable")
	}
	assertSysUserAuthorizationName(t, db, "tenant-a-user", "tenant-a-user")
	assertSysUserAuthorizationCasbinRoles(t, db, "tenant-a-user", []string{"tenant-a-role-old"})
}

func setupSysUserAuthorizationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Role{}); err != nil {
		t.Fatalf("migrate authz tables: %v", err)
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

func seedSysUserAuthorizationUser(t *testing.T, db *gorm.DB, id string, tenantID string, authority string) {
	t.Helper()

	now := time.Now().UTC()
	status := "N"
	name := id
	if err := db.Create(&model.User{
		ID:                  id,
		Name:                &name,
		PhoneNumber:         id + "-phone",
		Email:               id + "@example.com",
		Status:              &status,
		Authority:           &authority,
		Password:            "hashed",
		TenantID:            &tenantID,
		CreatedAt:           &now,
		UpdatedAt:           &now,
		PasswordLastUpdated: &now,
	}).Error; err != nil {
		t.Fatalf("seed user %s: %v", id, err)
	}
}

func seedSysUserAuthorizationRole(t *testing.T, db *gorm.DB, id string, tenantID string) {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Create(&model.Role{
		ID:        id,
		Name:      id,
		TenantID:  &tenantID,
		CreatedAt: &now,
		UpdatedAt: &now,
	}).Error; err != nil {
		t.Fatalf("seed role %s: %v", id, err)
	}
}

func seedSysUserAuthorizationCasbinRole(t *testing.T, db *gorm.DB, userID string, roleID string) {
	t.Helper()

	if err := db.Create(&model.CasbinRule{
		Ptype: StringPtr("g"),
		V0:    StringPtr(userID),
		V1:    StringPtr(roleID),
	}).Error; err != nil {
		t.Fatalf("seed casbin role %s/%s: %v", userID, roleID, err)
	}
}

func assertSysUserAuthorizationName(t *testing.T, db *gorm.DB, id string, want string) {
	t.Helper()

	var user model.User
	if err := db.First(&user, "id = ?", id).Error; err != nil {
		t.Fatalf("read user %s: %v", id, err)
	}
	if user.Name == nil || *user.Name != want {
		t.Fatalf("user %s name = %v, want %q", id, user.Name, want)
	}
}

func assertSysUserAuthorizationRowCount(t *testing.T, db *gorm.DB, id string, want int64) {
	t.Helper()

	var count int64
	if err := db.Model(&model.User{}).Where("id = ? OR email = ?", id, id).Count(&count).Error; err != nil {
		t.Fatalf("count user %s: %v", id, err)
	}
	if count != want {
		t.Fatalf("user %s row count = %d, want %d", id, count, want)
	}
}

func assertSysUserAuthorizationAddressCount(t *testing.T, db *gorm.DB, userID string, want int64) {
	t.Helper()

	var count int64
	query := db.Model(&model.UserAddress{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Count(&count).Error; err != nil {
		if strings.Contains(err.Error(), "no such table") {
			count = 0
		} else {
			t.Fatalf("count user addresses: %v", err)
		}
	}
	if count != want {
		t.Fatalf("user address count = %d, want %d", count, want)
	}
}

func assertSysUserAuthorizationBoardCount(t *testing.T, db *gorm.DB, tenantID string, want int64) {
	t.Helper()

	var count int64
	query := db.Model(&model.Board{})
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Count(&count).Error; err != nil {
		if strings.Contains(err.Error(), "no such table") {
			count = 0
		} else {
			t.Fatalf("count boards: %v", err)
		}
	}
	if count != want {
		t.Fatalf("board count = %d, want %d", count, want)
	}
}

func assertSysUserAuthorizationCasbinRoles(t *testing.T, db *gorm.DB, userID string, want []string) {
	t.Helper()

	var rules []model.CasbinRule
	if err := db.Where("ptype = ? AND v0 = ?", "g", userID).Find(&rules).Error; err != nil {
		t.Fatalf("read casbin roles for %s: %v", userID, err)
	}
	if len(rules) != len(want) {
		t.Fatalf("user %s casbin roles = %#v, want %#v", userID, rules, want)
	}
	seen := map[string]bool{}
	for _, rule := range rules {
		if rule.V1 != nil {
			seen[*rule.V1] = true
		}
	}
	for _, role := range want {
		if !seen[role] {
			t.Fatalf("user %s casbin roles = %#v, missing %s", userID, rules, role)
		}
	}
}

func assertSysUserAuthorizationRoles(t *testing.T, userID string, want []string) {
	t.Helper()

	roles, _ := GroupApp.Casbin.GetRoleFromUser(userID)
	if len(roles) != len(want) {
		t.Fatalf("user %s roles = %#v, want %#v", userID, roles, want)
	}
	seen := map[string]bool{}
	for _, role := range roles {
		seen[role] = true
	}
	for _, role := range want {
		if !seen[role] {
			t.Fatalf("user %s roles = %#v, missing %s", userID, roles, role)
		}
	}
}

func TestSysUserCreateWithoutRoleIDsBindsAuthorityFallback(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	if err := db.AutoMigrate(&model.UserAddress{}, &model.Board{}, &model.CasbinRule{}); err != nil {
		t.Fatalf("migrate user transaction tables: %v", err)
	}
	setupTestCasbinEnforcer()
	now := time.Now().UTC()
	user := &model.User{
		ID:                  "fallback-user",
		Name:                StringPtr("fallback user"),
		PhoneNumber:         "fallback-phone",
		Email:               "fallback-user@example.com",
		Status:              StringPtr("N"),
		Authority:           StringPtr(constant.TENANT_USER),
		Password:            "hashed",
		TenantID:            StringPtr("tenant-a"),
		CreatedAt:           &now,
		UpdatedAt:           &now,
		PasswordLastUpdated: &now,
	}

	// 未显式给 RoleIDs 的创建请求：按 users.authority 兜底绑定（RBAC 激活后
	// 无绑定用户会被全量 403，兜底保证新建用户继承其权限类型的全量授权）。
	err := createUserWithAddressDefaultBoardAndRoles(user, &model.CreateUserReq{
		RoleIDs: nil,
	}, &utils.UserClaims{
		ID:        "sys-admin",
		Authority: constant.SYS_ADMIN,
		TenantID:  "",
	})
	if err != nil {
		t.Fatalf("create transaction returned error: %v", err)
	}

	assertSysUserAuthorizationCasbinRoles(t, db, "fallback-user", []string{constant.TENANT_USER})
}
