// 文件用途：验证系统初始化中市场检查跳过规则。
// 核心逻辑：用表驱动输入断言初始化请求是否应跳过市场注册或外部检查。
// 关键注意事项：初始化路径通常只运行一次，测试需避免默认值变化导致部署阶段误连外部市场。
// 重构建议：把初始化判定与外部注册副作用分离，补齐配置缺失和重复初始化边界。
package service

import (
	"context"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/spf13/viper"
)

func TestShouldSkipMarketCheck(t *testing.T) {
	tests := []struct {
		name string
		req  *model.SuperAdminInitReq
		want bool
	}{
		{
			name: "nil req",
			req:  nil,
			want: false,
		},
		{
			name: "not returned from market",
			req: &model.SuperAdminInitReq{
				Email:            "user@example.com",
				MarketRegistered: false,
				MarketEmail:      "user@example.com",
			},
			want: false,
		},
		{
			name: "returned with matching email",
			req: &model.SuperAdminInitReq{
				Email:            "user@example.com",
				MarketRegistered: true,
				MarketEmail:      "user@example.com",
			},
			want: true,
		},
		{
			name: "returned with case-insensitive matching email",
			req: &model.SuperAdminInitReq{
				Email:            "User@Example.com",
				MarketRegistered: true,
				MarketEmail:      "user@example.com",
			},
			want: true,
		},
		{
			name: "returned with mismatch email",
			req: &model.SuperAdminInitReq{
				Email:            "user@example.com",
				MarketRegistered: true,
				MarketEmail:      "other@example.com",
			},
			want: false,
		},
		{
			name: "returned without market email",
			req: &model.SuperAdminInitReq{
				Email:            "user@example.com",
				MarketRegistered: true,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipMarketCheck(tt.req)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestBuildTenantSetupStateNextStep(t *testing.T) {
	tests := []struct {
		name           string
		hasAdmin       bool
		hasTenantAdmin bool
		hasTenant      bool
		wantEntry      string
		wantNextStep   string
	}{
		{
			name:         "needs super admin first",
			wantEntry:    "register",
			wantNextStep: "create_super_admin",
		},
		{
			name:         "needs tenant admin after super admin",
			hasAdmin:     true,
			wantEntry:    "login",
			wantNextStep: "create_tenant_admin",
		},
		{
			name:           "ready for login when tenant context exists",
			hasAdmin:       true,
			hasTenantAdmin: true,
			hasTenant:      true,
			wantEntry:      "login",
			wantNextStep:   "login",
		},
		{
			name:           "tenant admin without tenant id still needs tenant setup",
			hasAdmin:       true,
			hasTenantAdmin: true,
			wantEntry:      "login",
			wantNextStep:   "create_tenant_admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTenantSetupState(tt.hasAdmin, tt.hasTenantAdmin, tt.hasTenant, "https://market.example.com")

			if got.Entry != tt.wantEntry {
				t.Fatalf("Entry = %q, want %q", got.Entry, tt.wantEntry)
			}
			if got.NextStep != tt.wantNextStep {
				t.Fatalf("NextStep = %q, want %q", got.NextStep, tt.wantNextStep)
			}
			if got.HasTenantAdmin != tt.hasTenantAdmin {
				t.Fatalf("HasTenantAdmin = %v, want %v", got.HasTenantAdmin, tt.hasTenantAdmin)
			}
			if got.HasTenant != tt.hasTenant {
				t.Fatalf("HasTenant = %v, want %v", got.HasTenant, tt.hasTenant)
			}
		})
	}
}

func TestBuildTenantSetupStateOmitsMarketRegisterURLWhenMarketBaseURLIsEmpty(t *testing.T) {
	got := buildTenantSetupState(false, false, false, "   ")

	if got.MarketBaseURL != "" {
		t.Fatalf("MarketBaseURL = %q, want empty", got.MarketBaseURL)
	}
	if got.MarketRegisterURL != "" {
		t.Fatalf("MarketRegisterURL = %q, want empty", got.MarketRegisterURL)
	}
	if got.Entry != "register" || got.NextStep != "create_super_admin" {
		t.Fatalf("unexpected setup state: entry=%q next_step=%q", got.Entry, got.NextStep)
	}
}

func TestBuildTenantSetupStateTrimsMarketBaseURL(t *testing.T) {
	got := buildTenantSetupState(false, false, false, " https://market.example.com/ ")

	if got.MarketBaseURL != "https://market.example.com" {
		t.Fatalf("MarketBaseURL = %q", got.MarketBaseURL)
	}
	if got.MarketRegisterURL != "https://market.example.com/register" {
		t.Fatalf("MarketRegisterURL = %q", got.MarketRegisterURL)
	}
}

func TestEnsureSuperAdminMarketAccountRequiresExplicitEnable(t *testing.T) {
	tests := []struct {
		name       string
		setEnabled bool
	}{
		{name: "missing config key"},
		{name: "explicitly disabled", setEnabled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			if tt.setEnabled {
				viper.Set("market.enabled", false)
			}

			err := ensureSuperAdminMarketAccount(context.Background(), &model.SuperAdminInitReq{}, "user@example.com")
			got, ok := err.(*errcode.Error)
			if !ok {
				t.Fatalf("error type = %T, want *errcode.Error", err)
			}
			if got.Code != errcode.CodeMarketServiceUnavailable {
				t.Fatalf("error code = %d, want %d", got.Code, errcode.CodeMarketServiceUnavailable)
			}
		})
	}
}

func TestEnsureSuperAdminMarketAccountSkipsConfirmedMarketReturn(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	req := &model.SuperAdminInitReq{
		Email:            "User@Example.com",
		MarketRegistered: true,
		MarketEmail:      "user@example.com",
	}
	if err := ensureSuperAdminMarketAccount(context.Background(), req, req.Email); err != nil {
		t.Fatalf("ensureSuperAdminMarketAccount() error = %v, want nil", err)
	}
}

func TestInitSuperAdminRejectsWhenSysAdminExistsEvenWithMarketSkipFields(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "first-sys-admin", "", "SYS_ADMIN")

	req := &model.SuperAdminInitReq{
		Email:            "attacker@example.com",
		Password:         "Str0ng!Passw0rd",
		MarketRegistered: true,
		MarketEmail:      "attacker@example.com",
	}

	rsp, err := (&User{}).InitSuperAdmin(context.Background(), req)

	assertErrcodeError(t, err, "init super admin when sys admin exists", errcode.CodeSuperAdminExists, "")
	if rsp != nil {
		t.Fatalf("InitSuperAdmin rsp = %+v, want nil", rsp)
	}

	var count int64
	if err := db.Model(&model.User{}).Where("authority = ?", "SYS_ADMIN").Count(&count).Error; err != nil {
		t.Fatalf("count sys admin users: %v", err)
	}
	if count != 1 {
		t.Fatalf("sys admin user count = %d, want 1 (no second admin may be created)", count)
	}
}

func TestInitSuperAdminGateRunsBeforeAnyWriteOrLoginSideEffects(t *testing.T) {
	db := setupSysUserAuthorizationTestDB(t)
	seedSysUserAuthorizationUser(t, db, "first-sys-admin", "", "SYS_ADMIN")

	req := &model.SuperAdminInitReq{
		Email:            "second@example.com",
		Password:         "not-even-validated!",
		MarketRegistered: false,
	}

	if _, err := (&User{}).InitSuperAdmin(context.Background(), req); err == nil {
		t.Fatalf("InitSuperAdmin should fail when a SYS_ADMIN already exists")
	}

	var count int64
	if err := db.Model(&model.User{}).Where("email = ?", "second@example.com").Count(&count).Error; err != nil {
		t.Fatalf("count new user rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("new user rows = %d, want 0 (gate must run before any write)", count)
	}
}
