package service

import (
	"context"
	"fmt"
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

func setupBoardServiceAccessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open board sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Board{}); err != nil {
		t.Fatalf("migrate board: %v", err)
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

func TestBoardMissingDetailAndRepeatedDeleteReturnNotFound(t *testing.T) {
	db := setupBoardServiceAccessTestDB(t)
	claims := &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: "tenant-a"}

	_, err := (&Board{}).GetBoard("missing-board", claims)
	assertErrcodeError(t, err, "missing detail", errcode.CodeNotFound, "board not found")

	now := time.Now().UTC()
	board := &model.Board{ID: "owned-board", Name: "Owned", TenantID: claims.TenantID, CreatedAt: now, UpdatedAt: now, HomeFlag: "N"}
	if err := db.Create(board).Error; err != nil {
		t.Fatalf("seed board: %v", err)
	}
	if err := (&Board{}).DeleteBoard(board.ID, claims); err != nil {
		t.Fatalf("first delete returned error: %v", err)
	}
	err = (&Board{}).DeleteBoard(board.ID, claims)
	assertErrcodeError(t, err, "repeated delete", errcode.CodeNotFound, "board not found")
}

func TestBoardCrossTenantDeleteIsDeniedAndPreservesBoard(t *testing.T) {
	db := setupBoardServiceAccessTestDB(t)
	now := time.Now().UTC()
	board := &model.Board{ID: "foreign-board", Name: "Foreign", TenantID: "tenant-b", CreatedAt: now, UpdatedAt: now, HomeFlag: "N"}
	if err := db.Create(board).Error; err != nil {
		t.Fatalf("seed board: %v", err)
	}
	claims := &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: "tenant-a"}
	err := (&Board{}).DeleteBoard(board.ID, claims)
	assertErrcodeError(t, err, "cross-tenant delete", errcode.CodeNoPermission, "no permission to query board")

	var stored model.Board
	if err := db.First(&stored, "id = ?", board.ID).Error; err != nil {
		t.Fatalf("foreign board was removed: %v", err)
	}
}

func TestEnsureBoardWritePermissionFailClosed(t *testing.T) {
	tenantA := "tenant-a"
	tenantB := "tenant-b"

	allowed := []struct {
		name           string
		claims         *utils.UserClaims
		targetTenantID *string
	}{
		{name: "system administrator create", claims: &utils.UserClaims{Authority: constant.SYS_ADMIN, TenantID: tenantA}, targetTenantID: &tenantA},
		{name: "system administrator cross tenant", claims: &utils.UserClaims{Authority: constant.SYS_ADMIN}, targetTenantID: &tenantB},
		{name: "tenant administrator own tenant", claims: &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: tenantA}, targetTenantID: &tenantA},
	}
	for _, tc := range allowed {
		t.Run(tc.name, func(t *testing.T) {
			if err := ensureBoardWritePermission(tc.claims, tc.targetTenantID); err != nil {
				t.Fatalf("ensureBoardWritePermission() error = %v", err)
			}
		})
	}

	denied := []struct {
		name           string
		claims         *utils.UserClaims
		targetTenantID *string
	}{
		{name: "nil claims"},
		{name: "empty authority", claims: &utils.UserClaims{TenantID: tenantA}},
		{name: "unknown authority", claims: &utils.UserClaims{Authority: "UNKNOWN", TenantID: tenantA}},
		{name: "tenant user", claims: &utils.UserClaims{Authority: constant.TENANT_USER, TenantID: tenantA}},
		{name: "tenant administrator without tenant", claims: &utils.UserClaims{Authority: constant.TENANT_ADMIN}},
		{name: "tenant administrator cross tenant", claims: &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: tenantA}, targetTenantID: &tenantB},
		{name: "empty target tenant", claims: &utils.UserClaims{Authority: constant.SYS_ADMIN}, targetTenantID: new(string)},
	}
	for _, tc := range denied {
		t.Run(tc.name, func(t *testing.T) {
			err := ensureBoardWritePermission(tc.claims, tc.targetTenantID)
			assertErrcodeError(t, err, tc.name, errcode.CodeNoPermission, "no permission to modify board")
		})
	}
}

func TestBoardWriteEntryPointsRejectDisallowedClaimsBeforeDAL(t *testing.T) {
	claimsCases := []struct {
		name   string
		claims *utils.UserClaims
	}{
		{name: "nil claims"},
		{name: "tenant user", claims: &utils.UserClaims{Authority: constant.TENANT_USER, TenantID: "tenant-a"}},
		{name: "empty authority", claims: &utils.UserClaims{TenantID: "tenant-a"}},
		{name: "unknown authority", claims: &utils.UserClaims{Authority: "UNKNOWN", TenantID: "tenant-a"}},
	}

	for _, tc := range claimsCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&Board{}).CreateBoard(context.Background(), &model.CreateBoardReq{}, tc.claims)
			createMessage := "no permission to modify board"
			if tc.claims == nil {
				createMessage = "no permission to create board"
			}
			assertErrcodeError(t, err, "POST create", errcode.CodeNoPermission, createMessage)

			_, err = (&Board{}).UpdateBoard(context.Background(), &model.UpdateBoardReq{}, tc.claims)
			assertErrcodeError(t, err, "PUT without id implicit create", errcode.CodeNoPermission, "no permission to modify board")

			err = (&Board{}).DeleteBoard("board-id", tc.claims)
			deleteMessage := "no permission to modify board"
			if tc.claims == nil {
				deleteMessage = "no permission to query board"
			}
			assertErrcodeError(t, err, "DELETE", errcode.CodeNoPermission, deleteMessage)
		})
	}
}

func TestBoardCreatePathsRejectInvalidTenantContextBeforeDAL(t *testing.T) {
	claims := &utils.UserClaims{Authority: constant.TENANT_ADMIN}

	_, err := (&Board{}).CreateBoard(context.Background(), &model.CreateBoardReq{}, claims)
	assertErrcodeError(t, err, "POST create without tenant", errcode.CodeNoPermission, "no permission to modify board")

	_, err = (&Board{}).UpdateBoard(context.Background(), &model.UpdateBoardReq{}, claims)
	assertErrcodeError(t, err, "PUT implicit create without tenant", errcode.CodeNoPermission, "no permission to modify board")
}

func TestBoardTenantContextResolutionKeepsExplicitScopes(t *testing.T) {
	tenantA := "tenant-a"
	tenantB := "tenant-b"
	tenantAdmin := &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: tenantA}

	resolved, err := resolveBoardWriteTenant("", tenantAdmin)
	if err != nil || resolved != tenantA {
		t.Fatalf("tenant admin own context = (%q, %v), want (%q, nil)", resolved, err, tenantA)
	}

	_, err = resolveBoardWriteTenant(tenantB, tenantAdmin)
	assertErrcodeError(t, err, "tenant admin cross-tenant create", errcode.CodeNoPermission, "no permission to modify board")

	_, err = resolveBoardWriteTenant("", &utils.UserClaims{Authority: constant.SYS_ADMIN})
	assertErrcodeError(t, err, "system administrator missing create context", errcode.CodeParamError, "tenant context is required for board creation")

	resolved, err = resolveBoardListTenant(nil, &utils.UserClaims{Authority: constant.SYS_ADMIN})
	if err != nil || resolved != "" {
		t.Fatalf("system administrator default list context = (%q, %v), want all-tenant empty scope", resolved, err)
	}

	resolved, err = resolveBoardListTenant(&tenantA, tenantAdmin)
	if err != nil || resolved != tenantA {
		t.Fatalf("tenant admin own list context = (%q, %v), want (%q, nil)", resolved, err, tenantA)
	}

	_, err = resolveBoardListTenant(&tenantB, tenantAdmin)
	assertErrcodeError(t, err, "tenant admin cross-tenant list", errcode.CodeNoPermission, "no permission to query board")

	_, err = resolveBoardHomeTenant("", &utils.UserClaims{Authority: constant.SYS_ADMIN})
	assertErrcodeError(t, err, "system administrator missing home context", errcode.CodeParamError, "tenant context is required for the board home")

	resolved, err = resolveBoardHomeTenant("", &utils.UserClaims{Authority: constant.TENANT_USER, TenantID: tenantA})
	if err != nil || resolved != tenantA {
		t.Fatalf("tenant user home context = (%q, %v), want (%q, nil)", resolved, err, tenantA)
	}

	_, err = resolveBoardHomeTenant(tenantB, &utils.UserClaims{Authority: constant.TENANT_USER, TenantID: tenantA})
	assertErrcodeError(t, err, "tenant user cross-tenant home", errcode.CodeNoPermission, "no permission to query board")
}

func TestBuildCreateBoardPayloadUsesResolvedTenantContext(t *testing.T) {
	name := "Native board"
	description := "created for tenant-2"
	config := `{"version":1,"columns":24,"rowHeight":60,"widgets":[]}`
	visType := "native"
	req := &model.CreateBoardReq{
		Name:        name,
		Description: &description,
		Config:      &config,
		HomeFlag:    "N",
		MenuFlag:    "N",
		VisType:     &visType,
	}

	board := buildCreateBoardPayload(req, "tenant-2", time.Now().UTC())
	if board.TenantID != "tenant-2" {
		t.Fatalf("created board tenant_id = %q, want tenant-2", board.TenantID)
	}
	if board.Name != name || board.Description == nil || *board.Description != description {
		t.Fatalf("created board result did not preserve request fields: %+v", board)
	}
}
