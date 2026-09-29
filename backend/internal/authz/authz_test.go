package authz

import (
	"errors"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
)

func strp(s string) *string { return &s }

func claims(authority, tenant, id string) *Claims {
	return &Claims{Authority: authority, TenantID: tenant, ID: id}
}

func errCode(err error) int {
	var e *errcode.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return -1
}

var (
	sysAdmin      = claims(SysAdmin, "", "root")
	tenantAdmin   = claims(TenantAdmin, "t1", "admin1")
	tenantOwner   = claims(TenantUser, "t1", "u1")
	tenantOther   = claims(TenantUser, "t1", "u2")
	foreignAdmin  = claims(TenantAdmin, "t2", "admin2")
	foreignUser   = claims(TenantUser, "t2", "u9")
	blankIDUser   = claims(TenantUser, "t1", "  ")
	unknownAuthor = claims("CUSTOMER", "t1", "c1")
)

func TestRuleMatrix(t *testing.T) {
	device := OfTenant("t1").WithOwner(strp("u1"))
	sharedWithU2 := device.WithShare(func(c *Claims) bool { return c.ID == "u2" || c.ID == "u9" })
	platform := OfTenantPtr(nil)
	public := Owned{TenantID: "t3", Public: true}
	hierarchy := func(tenantID string) []string {
		if tenantID == "t0" {
			return []string{"t0", "t1"}
		}
		return []string{tenantID}
	}
	parentAdmin := claims(TenantAdmin, "t0", "hq")

	tenant := TenantRule("no permission")
	owner := OwnerRule("no permission")
	ownerShared := owner
	ownerShared.AllowShared = true
	managers := Rule{Roles: ManagerRoles, Code: errcode.CodeNotFound}
	scoped := Rule{Scope: hierarchy}
	publicRule := Rule{AllowPublic: true, Scope: hierarchy}

	cases := []struct {
		name  string
		rule  Rule
		c     *Claims
		res   Owned
		allow bool
	}{
		{"nil claims denied", tenant, nil, device, false},
		{"sys admin any tenant", tenant, sysAdmin, OfTenant("t9"), true},
		{"sys admin platform row", tenant, sysAdmin, platform, true},
		{"tenant admin same tenant", tenant, tenantAdmin, device, true},
		{"tenant admin cross tenant", tenant, foreignAdmin, device, false},
		{"tenant admin platform row", tenant, tenantAdmin, platform, false},
		{"tenant user same tenant (tenant rule)", tenant, tenantOther, device, true},
		{"tenant user cross tenant", tenant, foreignUser, device, false},
		{"owner rule owner", owner, tenantOwner, device, true},
		{"owner rule non-owner", owner, tenantOther, device, false},
		{"owner rule blank id", owner, blankIDUser, OfTenant("t1").WithOwner(strp(" ")), false},
		{"owner rule nil owner", owner, tenantOwner, OfTenant("t1"), false},
		{"owner rule admin non-owner", owner, tenantAdmin, device, true},
		{"owner rule cross-tenant admin", owner, foreignAdmin, device, false},
		{"shared read same-tenant recipient", ownerShared, tenantOther, sharedWithU2, true},
		{"shared read cross-tenant recipient", ownerShared, foreignUser, sharedWithU2, true},
		{"share ignored without AllowShared", owner, tenantOther, sharedWithU2, false},
		{"shared read non-recipient", ownerShared, foreignAdmin, sharedWithU2, false},
		{"managers rule rejects tenant user", managers, tenantOwner, device, false},
		{"managers rule admin", managers, tenantAdmin, device, true},
		{"managers rule unknown authority", managers, unknownAuthor, device, false},
		{"hierarchical parent reads child", scoped, parentAdmin, device, true},
		{"hierarchical child cannot read parent", scoped, tenantAdmin, OfTenant("t0"), false},
		{"public readable cross tenant", publicRule, foreignUser, public, true},
		{"public flag ignored without AllowPublic", tenant, foreignUser, public, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rule.Allows(tc.c, tc.res); got != tc.allow {
				t.Fatalf("Allows=%v want %v", got, tc.allow)
			}
			err := tc.rule.Check(tc.c, tc.res)
			if tc.allow && err != nil {
				t.Fatalf("Check err=%v", err)
			}
			if !tc.allow {
				want := tc.rule.Code
				if want == 0 {
					want = errcode.CodeNoPermission
				}
				if errCode(err) != want {
					t.Fatalf("code=%d want %d", errCode(err), want)
				}
			}
		})
	}
}

func TestDenyMessage(t *testing.T) {
	var e *errcode.Error
	if !errors.As(TenantRule("no permission to query x").Deny(), &e) || !e.UseCustomMsg || e.CustomMsg != "no permission to query x" {
		t.Fatalf("custom message lost: %#v", e)
	}
	if !errors.As(Rule{}.Deny(), &e) || e.UseCustomMsg || e.Code != errcode.CodeNoPermission {
		t.Fatalf("bare deny: %#v", e)
	}
}

type fakeRow struct {
	ID       string
	TenantID string
}

func TestGuard(t *testing.T) {
	loadCalls := 0
	store := map[string]*fakeRow{"a": {ID: "a", TenantID: "t1"}}
	loadErr := errcode.New(errcode.CodeDBError)
	g := Guard[*fakeRow]{
		Load: func(id string) (*fakeRow, error) {
			loadCalls++
			if row, ok := store[id]; ok {
				return row, nil
			}
			if id == "nil" {
				return nil, nil
			}
			return nil, loadErr
		},
		Owner:       func(r *fakeRow) Owned { return OfTenant(r.TenantID) },
		Missing:     func(r *fakeRow) bool { return r == nil },
		Read:        TenantRule("no permission to query row"),
		Write:       Rule{Roles: ManagerRoles, Message: "no permission to modify row"},
		ClaimsFirst: true,
	}

	if _, err := g.RequireRead("a", nil); errCode(err) != errcode.CodeNoPermission || loadCalls != 0 {
		t.Fatalf("nil claims must be rejected before load: err=%v calls=%d", err, loadCalls)
	}
	if row, err := g.RequireRead("a", tenantOther); err != nil || row.ID != "a" {
		t.Fatalf("same tenant read: %v", err)
	}
	if _, err := g.RequireRead("a", foreignAdmin); errCode(err) != errcode.CodeNoPermission {
		t.Fatalf("cross tenant read: %v", err)
	}
	if _, err := g.RequireWrite("a", tenantOther); errCode(err) != errcode.CodeNoPermission {
		t.Fatalf("tenant user write must fail on role rule: %v", err)
	}
	if _, err := g.RequireWrite("a", tenantAdmin); err != nil {
		t.Fatalf("tenant admin write: %v", err)
	}
	if _, err := g.RequireWrite("a", sysAdmin); err != nil {
		t.Fatalf("sys admin write: %v", err)
	}
	if _, err := g.RequireRead("missing", sysAdmin); err != loadErr {
		t.Fatalf("load error must pass through verbatim: %v", err)
	}
	if _, err := g.RequireRead("nil", sysAdmin); errCode(err) != errcode.CodeNoPermission {
		t.Fatalf("missing row must be permission-shaped: %v", err)
	}

	// Load-first guard: load error wins over nil claims.
	g.ClaimsFirst = false
	if _, err := g.RequireRead("missing", nil); err != loadErr {
		t.Fatalf("load-first guard must surface load error: %v", err)
	}
	// Zero write rule falls back to read.
	g.Write = Rule{}
	if _, err := g.RequireWrite("a", tenantOther); err != nil {
		t.Fatalf("zero write rule should mirror read: %v", err)
	}
}

func TestListScopeFor(t *testing.T) {
	expand := func(tenantID string) []string { return []string{tenantID, tenantID + "-child"} }
	deny := TenantRule("no permission to list")
	cases := []struct {
		name    string
		c       *Claims
		opts    ScopeOptions
		wantErr bool
		all     bool
		tenants []string
		owner   *string
	}{
		{"nil", nil, ScopeOptions{}, true, false, nil, nil},
		{"unknown authority", unknownAuthor, ScopeOptions{}, true, false, nil, nil},
		{"all tenants non sys admin", tenantAdmin, ScopeOptions{AllTenants: true}, true, false, nil, nil},
		{"all tenants sys admin", sysAdmin, ScopeOptions{AllTenants: true}, false, true, nil, nil},
		{"admin expanded", tenantAdmin, ScopeOptions{Expand: expand}, false, false, []string{"t1", "t1-child"}, nil},
		{"user not expanded + owner", tenantOwner, ScopeOptions{Expand: expand, OwnerScoped: true}, false, false, []string{"t1"}, strp("u1")},
		{"blank user sentinel", blankIDUser, ScopeOptions{OwnerScoped: true}, false, false, []string{"t1"}, strp(NoVisibleOwnerUserID)},
		{"admin owner filter nil", tenantAdmin, ScopeOptions{OwnerScoped: true}, false, false, []string{"t1"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ListScopeFor(tc.c, tc.opts, deny)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
			if tc.wantErr {
				return
			}
			if got.AllTenants != tc.all || len(got.TenantIDs) != len(tc.tenants) {
				t.Fatalf("scope=%+v", got)
			}
			for i := range tc.tenants {
				if got.TenantIDs[i] != tc.tenants[i] {
					t.Fatalf("tenants=%v", got.TenantIDs)
				}
			}
			if (got.OwnerUserID == nil) != (tc.owner == nil) || (tc.owner != nil && *got.OwnerUserID != *tc.owner) {
				t.Fatalf("owner=%v want %v", got.OwnerUserID, tc.owner)
			}
		})
	}
}

func TestFreeFunctions(t *testing.T) {
	res := OfTenant("t1")
	if _, err := RequireRead(tenantOther, res, TenantRule("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := RequireWrite(tenantOther, res, TenantRule("x"), Rule{Roles: ManagerRoles}); err == nil {
		t.Fatal("tenant user write should fail")
	}
	if err := CheckTenant(foreignUser, "t1", "x"); errCode(err) != errcode.CodeNoPermission {
		t.Fatal("cross tenant CheckTenant should fail")
	}
	if !OwnerMatches(strp(" u1 "), tenantOwner) || OwnerMatches(nil, tenantOwner) {
		t.Fatal("OwnerMatches")
	}
}
