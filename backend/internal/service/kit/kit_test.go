package kit

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

// wire renders an error the way the response layer sees it (*errcode.Error JSON).
func wire(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return "<nil>"
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		return "raw:" + err.Error()
	}
	b, jerr := json.Marshal(e)
	if jerr != nil {
		t.Fatal(jerr)
	}
	return fmt.Sprintf("%s|%v", b, e.UseCustomMsg)
}

func TestDBErr(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name string
		got  error
		want error
	}{
		{"sql", DBErr(KeySQL, boom), errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": "boom"})},
		{"error", DBErr(KeyError, boom), errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": "boom"})},
		{"op", DBErrOp("create", boom), errcode.WithData(errcode.CodeDBError, map[string]interface{}{"operation": "create", "error": "boom"})},
		{"onDBErr", OnDBErr(KeyError)(boom), errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": "boom"})},
	}
	for _, tc := range cases {
		if g, w := wire(t, tc.got), wire(t, tc.want); g != w {
			t.Errorf("%s: got %s want %s", tc.name, g, w)
		}
	}
}

func TestGate(t *testing.T) {
	deny := errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")
	noTenant := &utils.UserClaims{}
	ok := &utils.UserClaims{TenantID: "t1"}
	cases := []struct {
		name string
		g    Gate
		c    *utils.UserClaims
		want error
	}{
		{"nil claims", ClaimsRequired, nil, deny},
		{"empty tenant allowed", ClaimsRequired, noTenant, nil},
		{"tenant gate nil", TenantRequired, nil, deny},
		{"tenant gate empty", TenantRequired, noTenant, deny},
		{"tenant gate ok", TenantRequired, ok, nil},
		{"custom code", Gate{Msg: "x", Code: errcode.CodeParamError}, nil, errcode.NewWithMessage(errcode.CodeParamError, "x")},
		{"bare code", Gate{}, nil, errcode.New(errcode.CodeNoPermission)},
		{"unauthorized nil", TenantUnauthorized, nil, errcode.New(errcode.CodeUnauthorized)},
		{"unauthorized empty tenant", TenantUnauthorized, noTenant, errcode.New(errcode.CodeUnauthorized)},
		{"unauthorized ok", TenantUnauthorized, ok, nil},
	}
	for _, tc := range cases {
		if g, w := wire(t, tc.g.Require(tc.c)), wire(t, tc.want); g != w {
			t.Errorf("%s: got %s want %s", tc.name, g, w)
		}
	}
}

func TestNotFound(t *testing.T) {
	boom := errors.New("boom")
	wrapped := fmt.Errorf("wrap: %w", gorm.ErrRecordNotFound)
	nf := errcode.NewWithMessage(errcode.CodeNotFound, "x not found")
	cases := []struct {
		name string
		n    NotFound
		in   error
		want error
	}{
		{"nil", NotFound{Msg: "x not found"}, nil, nil},
		{"mask all", NotFound{Msg: "x not found"}, boom, nf},
		{"bare", NotFound{}, boom, errcode.New(errcode.CodeNotFound)},
		{"is match", NotFound{Msg: "x not found", Match: IsRecordNotFound}, wrapped, nf},
		{"is other", NotFound{Msg: "x not found", Match: IsRecordNotFound}, boom, DBErr(KeySQL, boom)},
		{"exact rejects wrapped", NotFound{Msg: "x not found", Match: IsRecordNotFoundExact}, wrapped, DBErr(KeySQL, wrapped)},
		{"exact sentinel", NotFound{Msg: "x not found", Match: IsRecordNotFoundExact}, gorm.ErrRecordNotFound, nf},
		{"onOther", NotFound{Match: IsRecordNotFound, OnOther: OnDBErr(KeyError)}, boom, DBErr(KeyError, boom)},
		{"param code", NotFound{Code: errcode.CodeParamError, Msg: "x not found"}, boom, errcode.NewWithMessage(errcode.CodeParamError, "x not found")},
	}
	for _, tc := range cases {
		if g, w := wire(t, tc.n.Map(tc.in)), wire(t, tc.want); g != w {
			t.Errorf("%s: got %s want %s", tc.name, g, w)
		}
	}
}

type rec struct{ ID, Tenant string }

func testRepo() (TenantRepo[*rec], *[]string) {
	var calls []string
	return TenantRepo[*rec]{
		Gate:     ClaimsRequired,
		NotFound: NotFound{Msg: "rec not found"},
		Get: func(id, tenant string) (*rec, error) {
			calls = append(calls, "get:"+id+"@"+tenant)
			if id != "a" || tenant != "t1" {
				return nil, gorm.ErrRecordNotFound
			}
			return &rec{ID: id, Tenant: tenant}, nil
		},
	}, &calls
}

func TestTenantRepo(t *testing.T) {
	c := &utils.UserClaims{TenantID: "t1"}
	other := &utils.UserClaims{TenantID: "t2"}
	nf := errcode.NewWithMessage(errcode.CodeNotFound, "rec not found")
	deny := errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")

	r, calls := testRepo()
	if _, err := r.Load(nil, "a"); wire(t, err) != wire(t, deny) || len(*calls) != 0 {
		t.Fatalf("nil claims: %v calls=%v", err, *calls)
	}
	if got, err := r.Load(c, "a"); err != nil || got.ID != "a" {
		t.Fatalf("load: %v %v", got, err)
	}
	if _, err := r.Load(other, "a"); wire(t, err) != wire(t, nf) {
		t.Fatalf("cross tenant: %v", err)
	}

	var deleted []string
	del := func(id, tenant string) error { deleted = append(deleted, id+"@"+tenant); return nil }
	if err := r.Delete(c, "a", del, nil); err != nil || !reflect.DeepEqual(deleted, []string{"a@t1"}) {
		t.Fatalf("delete: %v %v", err, deleted)
	}
	if err := r.Delete(c, "zz", del, nil); wire(t, err) != wire(t, nf) || len(deleted) != 1 {
		t.Fatalf("delete missing: %v %v", err, deleted)
	}
	raw := errors.New("fk")
	if err := r.Delete(c, "a", func(string, string) error { return raw }, nil); err != raw {
		t.Fatalf("delete passthrough: %v", err)
	}
	if err := r.Delete(c, "a", func(string, string) error { return raw }, OnDBErr(KeyError)); wire(t, err) != wire(t, DBErr(KeyError, raw)) {
		t.Fatalf("delete mapped: %v", err)
	}

	r.TenantOf = func(*utils.UserClaims) string { return "t1" }
	if _, err := r.Load(other, "a"); err != nil {
		t.Fatalf("TenantOf override: %v", err)
	}
}

func TestList(t *testing.T) {
	r, _ := testRepo()
	c := &utils.UserClaims{TenantID: "t1"}
	list := func(req int, tenant string) (int64, []*rec, error) {
		return int64(req), []*rec{{ID: "a", Tenant: tenant}}, nil
	}
	m, err := List(r, c, 7, list, OnDBErr(KeyError))
	if err != nil {
		t.Fatal(err)
	}
	if total, ok := m["total"].(int64); !ok || total != 7 {
		t.Fatalf("total type/value: %#v", m["total"])
	}
	if l, ok := m["list"].([]*rec); !ok || len(l) != 1 || l[0].Tenant != "t1" {
		t.Fatalf("list type/value: %#v", m["list"])
	}
	if _, err := List(r, nil, 7, list, nil); err == nil {
		t.Fatal("gate not applied")
	}
	boom := errors.New("boom")
	failing := func(int, string) (int64, []*rec, error) { return 0, nil, boom }
	if _, err := List(r, c, 1, failing, OnDBErr(KeyError)); wire(t, err) != wire(t, DBErr(KeyError, boom)) {
		t.Fatalf("list err: %v", err)
	}
}

func TestPageJSONMatchesMap(t *testing.T) {
	p := Page[string]{Total: 2, List: []string{"x", "y"}}
	a, _ := json.Marshal(p)
	b, _ := json.Marshal(p.Map())
	if string(a) != string(b) {
		t.Fatalf("page %s != map %s", a, b)
	}
	// nil list must keep encoding as null in both forms (historical behaviour).
	a, _ = json.Marshal(Page[string]{})
	b, _ = json.Marshal(ListMap[string](0, nil))
	if string(a) != string(b) {
		t.Fatalf("empty page %s != map %s", a, b)
	}
}

func TestStamp(t *testing.T) {
	if NowUTC().Location().String() != "UTC" || NowUTCPtr().Location().String() != "UTC" {
		t.Fatal("not UTC")
	}
	if a, b := NewID(), NewID(); len(a) != 36 || a == b {
		t.Fatalf("ids %q %q", a, b)
	}
}

func TestAnyListMapKeepsDynamicType(t *testing.T) {
	if m := AnyListMap(2, interface{}([]int{1, 2})); m["total"] != int64(2) || len(m) != 2 {
		t.Errorf("AnyListMap shape: %#v", m)
	} else if _, ok := m["list"].([]int); !ok {
		t.Errorf("AnyListMap list type: %T", m["list"])
	}
}

func TestTenantRepoScopeAndMissing(t *testing.T) {
	scope := TenantScope{BlankMsg: "blank"}
	var gotTenant string
	r := TenantRepo[*rec]{
		Get: func(id, tenant string) (*rec, error) {
			gotTenant = tenant
			if id != "a" {
				return nil, nil // (nil, nil) DAL miss
			}
			return &rec{ID: id, Tenant: tenant}, nil
		},
		Scope:    scope.Tenant,
		Missing:  NilPtr[rec],
		NotFound: NotFound{Msg: "rec not found", Match: func(error) bool { return false }, OnOther: OnDBErr(KeyError)},
		Gate:     Gate{Msg: "must be ignored when Scope is set"},
	}
	if _, err := r.Load(nil, "a"); wire(t, err) != wire(t, errcode.New(errcode.CodeNoPermission)) {
		t.Fatalf("nil claims bare deny: %s", wire(t, err))
	}
	if _, err := r.Load(&utils.UserClaims{TenantID: "  "}, "a"); wire(t, err) != wire(t, errcode.NewWithMessage(errcode.CodeNoPermission, "blank")) {
		t.Fatalf("blank: %s", wire(t, err))
	}
	if got, err := r.Load(&utils.UserClaims{TenantID: " t1 "}, "a"); err != nil || got.Tenant != "t1" {
		t.Fatalf("trimmed tenant: %v %v", got, err)
	}
	if _, err := r.Load(&utils.UserClaims{TenantID: "t1"}, "zz"); wire(t, err) != wire(t, errcode.NewWithMessage(errcode.CodeNotFound, "rec not found")) {
		t.Fatalf("missing: %s", wire(t, err))
	}
	var deleted string
	if err := r.Delete(&utils.UserClaims{TenantID: " t1 "}, "a", func(id, tenant string) error { deleted = id + "@" + tenant; return nil }, nil); err != nil || deleted != "a@t1" {
		t.Fatalf("delete via scope: %v %q", err, deleted)
	}
	boom := errors.New("boom")
	r.Get = func(string, string) (*rec, error) { return nil, boom }
	if _, err := r.Load(&utils.UserClaims{TenantID: "t1"}, "a"); wire(t, err) != wire(t, DBErr(KeyError, boom)) {
		t.Fatalf("db err: %s", wire(t, err))
	}
	gotTenant = ""
	_, err := List(r, &utils.UserClaims{TenantID: " t1 "}, 0, func(_ int, tenant string) (int64, []*rec, error) {
		gotTenant = tenant
		return 0, nil, nil
	}, nil)
	if err != nil || gotTenant != "t1" {
		t.Fatalf("list via scope: %v %q", err, gotTenant)
	}
}
