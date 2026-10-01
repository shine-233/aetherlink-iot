package kit

import (
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

func TestTenantScope(t *testing.T) {
	s := TenantScope{NilMsg: "nil msg", BlankMsg: "blank msg"}
	cases := []struct {
		name   string
		c      *utils.UserClaims
		tenant string
		want   error
	}{
		{"nil", nil, "", errcode.NewWithMessage(errcode.CodeNoPermission, "nil msg")},
		{"empty", &utils.UserClaims{}, "", errcode.NewWithMessage(errcode.CodeNoPermission, "blank msg")},
		{"spaces", &utils.UserClaims{TenantID: " \t"}, "", errcode.NewWithMessage(errcode.CodeNoPermission, "blank msg")},
		{"trimmed", &utils.UserClaims{TenantID: " t1 "}, "t1", nil},
	}
	for _, tc := range cases {
		got, err := s.Tenant(tc.c)
		if got != tc.tenant || wire(t, err) != wire(t, tc.want) {
			t.Errorf("%s: got %q %s want %q %s", tc.name, got, wire(t, err), tc.tenant, wire(t, tc.want))
		}
	}
}
